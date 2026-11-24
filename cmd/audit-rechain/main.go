// Command audit-rechain repairs legacy audit-chain hashes that were computed
// with sub-microsecond timestamps (see LEFT-R85-01 / DEF-47).
//
// Records created before the DEF-47 fix hashed time.Now() at nanosecond
// precision while records.created_at only stores microseconds, so those rows can
// never verify. The chain data itself is intact; only prev_hash/entry_hash need
// deterministic recomputation over the stored (microsecond) fields.
//
// Default is -dry-run (read-only): report how many rows differ. Repair requires
// -apply plus an explicit backup gate (AUDIT_RECHAIN_ALLOW_APPLY=1), and runs in
// one transaction so a crash cannot leave a half-rewritten chain.
//
// Usage:
//
//	audit-rechain -dsn "$POSTGRES_DSN" -chain <org-uuid>            # dry run
//	audit-rechain -dsn "$POSTGRES_DSN" -chain <org-uuid> -apply     # after backup
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	auditprovider "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

const queryChainRows = `SELECT id, coalesce(organization_id::text, ''), coalesce(actor_id::text, ''),
       coalesce(action, ''), coalesce(resource_type, ''), coalesce(resource_id, ''),
       coalesce(host(ip), ''), coalesce(user_agent, ''), coalesce(surface, ''),
       coalesce(request_id, ''), coalesce(trace_id, ''), coalesce(metadata, '{}'::jsonb),
       created_at, chain_seq, chain_algo, coalesce(encode(entry_hash,'hex'), '')
FROM guliangiam.records
WHERE chain_id = $1::text AND entry_hash IS NOT NULL
ORDER BY chain_seq ASC`

const queryUpsertStatements = 2

type chainRow struct {
	id     string
	record auditprovider.ChainRecord
	old    []byte
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("audit-rechain", flag.ContinueOnError)
	dsn := fs.String("dsn", "", "database DSN (defaults to DB_DSN/POSTGRES_DSN/DATABASE_URL)")
	chain := fs.String("chain", "", "chain id (organization UUID) to repair")
	apply := fs.Bool("apply", false, "write repaired hashes (requires AUDIT_RECHAIN_ALLOW_APPLY=1)")
	batch := fs.Int("batch", 200, "update batch size inside the repair transaction")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *chain == "" {
		fmt.Fprintln(os.Stderr, "audit-rechain: -chain is required")
		return 2
	}
	if *dsn == "" {
		*dsn = firstNonEmpty(os.Getenv("DB_DSN"), os.Getenv("POSTGRES_DSN"), os.Getenv("DATABASE_URL"))
	}
	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "audit-rechain: database DSN is required (-dsn or DB_DSN/POSTGRES_DSN/DATABASE_URL)")
		return 2
	}
	if *apply && os.Getenv("AUDIT_RECHAIN_ALLOW_APPLY") != "1" {
		fmt.Fprintln(os.Stderr, "audit-rechain: refusing to write without AUDIT_RECHAIN_ALLOW_APPLY=1 (back up first)")
		return 2
	}
	if *batch <= 0 || *batch > 5000 {
		fmt.Fprintln(os.Stderr, "audit-rechain: -batch must be 1..5000")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "audit-rechain: connect: %v\n", err)
		return 2
	}
	defer pool.Close()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "audit-rechain: acquire: %v\n", err)
		return 2
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT set_config('app.current_role','super_admin', false)"); err != nil {
		fmt.Fprintf(os.Stderr, "audit-rechain: set maintenance scope: %v\n", err)
		return 2
	}

	rows, err := load(ctx, conn, *chain)
	if err != nil {
		fmt.Fprintf(os.Stderr, "audit-rechain: %v\n", err)
		return 2
	}
	if len(rows) == 0 {
		fmt.Fprintf(os.Stderr, "audit-rechain: chain %s has no hashed records (check the chain id)\n", *chain)
		return 2
	}

	// Recompute the whole chain deterministically from stored fields.
	var prev []byte
	changed := 0
	repaired := make([]*chainRow, 0, len(rows))
	for i, row := range rows {
		algo, err := platformcrypto.NormalizeChainAlgorithm(row.record.ChainAlgo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "audit-rechain: seq %d: %v\n", row.record.ChainSeq, err)
			return 2
		}
		row.record.ChainAlgo = algo
		row.record.PrevHash = prev
		hash, err := auditprovider.RecomputeEntryHash(algo, row.record)
		if err != nil {
			fmt.Fprintf(os.Stderr, "audit-rechain: seq %d: %v\n", row.record.ChainSeq, err)
			return 2
		}
		if string(hash) != string(row.old) || !sameBytes(row.record.PrevHash, prev) {
			changed++
		}
		row.record.EntryHash = hash
		prev = hash
		repaired = append(repaired, row)
		_ = i
	}
	fmt.Printf("audit-rechain: chain %s rows=%d repaired=%d apply=%v\n", *chain, len(rows), changed, *apply)
	if !*apply {
		fmt.Println("audit-rechain: dry run only; re-run with -apply and AUDIT_RECHAIN_ALLOW_APPLY=1 after backup")
		return 0
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "audit-rechain: begin: %v\n", err)
		return 2
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for start := 0; start < len(repaired); start += *batch {
		end := start + *batch
		if end > len(repaired) {
			end = len(repaired)
		}
		for _, row := range repaired[start:end] {
			if _, err := tx.Exec(ctx,
				`UPDATE guliangiam.records SET prev_hash = $2, entry_hash = $3 WHERE id = $1::uuid`,
				row.id, nullableBytes(row.record.PrevHash), row.record.EntryHash); err != nil {
				fmt.Fprintf(os.Stderr, "audit-rechain: update seq %d: %v\n", row.record.ChainSeq, err)
				return 2
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "audit-rechain: commit: %v\n", err)
		return 2
	}
	fmt.Printf("audit-rechain: committed %d rows; run cmd/audit-verify to confirm\n", len(repaired))
	return 0
}

func load(ctx context.Context, conn *pgxpool.Conn, chainID string) ([]*chainRow, error) {
	pgRows, err := conn.Query(ctx, queryChainRows, chainID)
	if err != nil {
		return nil, fmt.Errorf("query chain: %w", err)
	}
	defer pgRows.Close()
	var out []*chainRow
	for pgRows.Next() {
		row := &chainRow{}
		var metadata []byte
		var ip, userAgent, surface, requestID, traceID string
		var createdAt time.Time
		var oldHashHex string
		if err := pgRows.Scan(&row.id, &row.record.OrganizationID, &row.record.ActorID, &row.record.Action,
			&row.record.ResourceType, &row.record.ResourceID, &ip, &userAgent, &surface, &requestID, &traceID,
			&metadata, &createdAt, &row.record.ChainSeq, &row.record.ChainAlgo, &oldHashHex); err != nil {
			return nil, fmt.Errorf("scan chain row: %w", err)
		}
		if net.ParseIP(ip) != nil {
			row.record.IP = ip
		}
		row.record.UserAgent = userAgent
		row.record.Surface = surface
		row.record.RequestID = requestID
		row.record.TraceID = traceID
		row.record.Metadata = map[string]any{}
		if len(metadata) > 0 {
			if err := json.Unmarshal(metadata, &row.record.Metadata); err != nil {
				return nil, fmt.Errorf("decode metadata seq %d: %w", row.record.ChainSeq, err)
			}
		}
		row.record.CreatedAt = createdAt.UTC()
		row.old, err = hex.DecodeString(oldHashHex)
		if err != nil {
			return nil, fmt.Errorf("decode old hash seq %d: %w", row.record.ChainSeq, err)
		}
		out = append(out, row)
	}
	if err := pgRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chain: %w", err)
	}
	return out, nil
}

func nullableBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

func sameBytes(a, b []byte) bool { return string(a) == string(b) }

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
