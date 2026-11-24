// Command audit-verify verifies the tamper-evident hash chain of the audit
// records table (ADR-031 / RISK-013). It loads the full record fields, then
// uses the pure verifier in platform/outbox/providers/audit to recompute every
// entry_hash, check prev_hash continuity, and report the first broken chain_seq.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	platformdb "github.com/guliangiam/guliangiam/internal/platform/db"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

const queryChainRecordsFull = `
SELECT organization_id::text, COALESCE(actor_id::text, ''), action,
       COALESCE(resource_type, ''), COALESCE(resource_id, ''),
       COALESCE(HOST(ip), ''), COALESCE(user_agent, ''),
       COALESCE(surface, ''), COALESCE(request_id, ''), COALESCE(trace_id, ''),
       COALESCE(metadata, '{}'::jsonb), created_at,
       chain_seq, prev_hash, entry_hash, chain_algo
FROM guliangiam.records
WHERE chain_id = $1 AND entry_hash IS NOT NULL
ORDER BY chain_seq ASC`

func main() {
	os.Exit(run())
}

func run() int {
	dsn := flag.String("dsn", "", "database DSN (default: DB_DSN, then POSTGRES_DSN, then DATABASE_URL)")
	chainID := flag.String("chain", "", "chain ID to verify (organization UUID)")
	timeout := flag.Duration("timeout", 60*time.Second, "verification timeout")
	flag.Parse()

	if *chainID == "" {
		fmt.Fprintln(os.Stderr, "audit-verify: -chain is required")
		flag.Usage()
		return 2
	}
	if *dsn == "" {
		*dsn = platformdb.PostgresDataSourceName()
	}
	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "audit-verify: database DSN is required (-dsn or DB_DSN/POSTGRES_DSN/DATABASE_URL)")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	count, err := verifyChain(ctx, *dsn, *chainID)
	if err != nil {
		var invalid *chainInvalidError
		if errors.As(err, &invalid) {
			// VerifyChainRecords 的错误已带 chain_id、chain_seq 与算法名；这里只补
			// 已验证条数，便于定位断点在整条链中的位置。
			fmt.Fprintf(os.Stderr, "audit-verify: chain %s INVALID after %d verified entries: %v\n", *chainID, invalid.verified, invalid.err)
			return 1
		}
		fmt.Fprintf(os.Stderr, "audit-verify: %v\n", err)
		return 2
	}
	fmt.Printf("audit-verify: chain %s OK (%d entries verified)\n", *chainID, count)
	return 0
}

// chainInvalidError marks a chain-integrity failure (exit 1) and carries the
// number of entries verified before the first broken chain_seq. Connect/query/
// scan failures stay plain errors and map to exit 2.
type chainInvalidError struct {
	verified int
	err      error
}

func (e *chainInvalidError) Error() string { return e.err.Error() }
func (e *chainInvalidError) Unwrap() error { return e.err }

// verifyChain is the injectable command seam: it opens the DSN, loads the chain
// fields, and runs the pure verifier. run() keeps flag parsing and exit-code
// mapping so the CLI contract is unchanged.
func verifyChain(ctx context.Context, dsn, chainID string) (int, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return 0, fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	// 维护命令读取的是 FORCE RLS 的 audit 表；没有系统作用域时会读到 0 行，
	// 从而把「看不到」误报为「链 OK」。先在同一连接上提升到维护作用域。
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return 0, fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT set_config('app.current_role','super_admin', false)"); err != nil {
		return 0, fmt.Errorf("set maintenance scope: %w", err)
	}

	records, err := loadChainRecords(ctx, conn, chainID)
	if err != nil {
		return 0, err
	}
	if len(records) == 0 {
		// fail-closed：不可见的链不得被报告为 OK（S-17 / ADR-031）。
		return 0, fmt.Errorf("chain %s: no hashed records visible; refusing to report OK (use a maintenance DSN with BYPASSRLS/superuser and confirm the chain id)", chainID)
	}
	count, err := audit.VerifyChainRecords(chainID, records)
	if err != nil {
		return count, &chainInvalidError{verified: count, err: err}
	}
	return count, nil
}

// loadChainRecords reads every hashed record of one chain in chain_seq order and
// decodes the full field set VerifyChainRecords recomputes (metadata included).
func loadChainRecords(ctx context.Context, conn *pgxpool.Conn, chainID string) ([]audit.ChainRecord, error) {
	rows, err := conn.Query(ctx, queryChainRecordsFull, chainID)
	if err != nil {
		return nil, fmt.Errorf("query chain: %w", err)
	}
	defer rows.Close()

	var records []audit.ChainRecord
	for rows.Next() {
		var rec audit.ChainRecord
		var metadataJSON []byte
		if err := rows.Scan(
			&rec.OrganizationID, &rec.ActorID, &rec.Action, &rec.ResourceType, &rec.ResourceID,
			&rec.IP, &rec.UserAgent, &rec.Surface, &rec.RequestID, &rec.TraceID,
			&metadataJSON, &rec.CreatedAt, &rec.ChainSeq, &rec.PrevHash, &rec.EntryHash, &rec.ChainAlgo,
		); err != nil {
			return nil, fmt.Errorf("scan chain row %d: %w", len(records), err)
		}
		rec.Metadata = map[string]any{}
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &rec.Metadata); err != nil {
				return nil, fmt.Errorf("decode metadata seq %d: %w", rec.ChainSeq, err)
			}
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chain: %w", err)
	}
	return records, nil
}
