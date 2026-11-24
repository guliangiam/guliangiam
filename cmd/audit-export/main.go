// Command audit-export exports one audit chain segment as a WORM-archive
// ready NDJSON payload plus a signed manifest (checksums + chain head).
//
// The export is read-only. It reads through a maintenance connection because
// records is a FORCE-RLS table: without a system scope the query would
// silently return zero rows and produce an empty archive.
//
// Usage:
//
//	audit-export -dsn "$POSTGRES_DSN" -chain <org-uuid> -from 1 -to 0 -out seg.ndjson
//	audit-export -verify -payload seg.ndjson -manifest seg.ndjson.manifest.json
//
// Signature: when AUDIT_ARCHIVE_HMAC_KEY is set, the manifest carries an
// HMAC-SHA256 over (chain_id, from_seq, to_seq, count, payload_sha256,
// payload_bytes, head_entry_hash). External timestamping is deliberately left
// to the archive target; this command only proves the bytes it wrote.
package main

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const manifestSchema = "guliangiam.audit.archive.v1"

const (
	queryRecords = `SELECT to_jsonb(r)
FROM guliangiam.records r
WHERE r.chain_id = $1::text
  AND r.chain_seq >= $2
  AND ($3 = 0 OR r.chain_seq <= $3)
ORDER BY r.chain_seq ASC`

	queryHeadHash = `SELECT chain_seq, coalesce(encode(entry_hash,'hex'), '')
FROM guliangiam.records
WHERE chain_id = $1::text
  AND chain_seq >= $2
  AND ($3 = 0 OR chain_seq <= $3)
ORDER BY chain_seq DESC
LIMIT 1`
)

// manifest is the archive receipt written next to the NDJSON payload.
type manifest struct {
	Schema        string    `json:"schema"`
	ChainID       string    `json:"chain_id"`
	FromSeq       int64     `json:"from_seq"`
	ToSeq         int64     `json:"to_seq"`
	Count         int64     `json:"count"`
	PayloadSHA256 string    `json:"payload_sha256"`
	PayloadBytes  int64     `json:"payload_bytes"`
	HeadEntryHash string    `json:"head_entry_hash"`
	GeneratedAt   time.Time `json:"generated_at"`
	Signature     string    `json:"signature,omitempty"`
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("audit-export", flag.ContinueOnError)
	dsn := fs.String("dsn", "", "database DSN (default: DB_DSN/POSTGRES_DSN/DATABASE_URL)")
	chain := fs.String("chain", "", "chain id (organization UUID) to export")
	from := fs.Int64("from", 1, "first chain_seq to include (inclusive)")
	to := fs.Int64("to", 0, "last chain_seq to include (inclusive, 0 = chain head)")
	out := fs.String("out", "", "NDJSON payload path (default: audit-<chain>.ndjson)")
	manifestPath := fs.String("manifest", "", "manifest path (default: <out>.manifest.json)")
	verify := fs.Bool("verify", false, "verify an existing payload/manifest pair instead of exporting")
	payload := fs.String("payload", "", "payload path for -verify")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *verify {
		if *payload == "" || *manifestPath == "" {
			fmt.Fprintln(os.Stderr, "audit-export: -verify requires -payload and -manifest")
			return 2
		}
		if err := verifyArchive(*payload, *manifestPath, os.Getenv("AUDIT_ARCHIVE_HMAC_KEY")); err != nil {
			fmt.Fprintf(os.Stderr, "audit-export: %v\n", err)
			return 1
		}
		fmt.Println("audit-export: archive manifest OK")
		return 0
	}

	if *chain == "" {
		fmt.Fprintln(os.Stderr, "audit-export: -chain is required")
		fs.Usage()
		return 2
	}
	if *dsn == "" {
		*dsn = firstNonEmpty(os.Getenv("DB_DSN"), os.Getenv("POSTGRES_DSN"), os.Getenv("DATABASE_URL"))
	}
	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "audit-export: database DSN is required (-dsn or DB_DSN/POSTGRES_DSN/DATABASE_URL)")
		return 2
	}
	if *out == "" {
		*out = "audit-" + *chain + ".ndjson"
	}
	if *manifestPath == "" {
		*manifestPath = *out + ".manifest.json"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	m, err := exportSegment(ctx, *dsn, *chain, *from, *to, *out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "audit-export: %v\n", err)
		return 2
	}
	if key := os.Getenv("AUDIT_ARCHIVE_HMAC_KEY"); key != "" {
		m.Signature = signManifest(*m, key)
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "audit-export: encode manifest: %v\n", err)
		return 2
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(*manifestPath, raw, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "audit-export: write manifest: %v\n", err)
		return 2
	}
	fmt.Printf("audit-export: chain %s seq %d..%d -> %s (%d records, sha256 %s)\n",
		m.ChainID, m.FromSeq, m.ToSeq, *out, m.Count, m.PayloadSHA256)
	return 0
}

func exportSegment(ctx context.Context, dsn, chainID string, from, to int64, out string) (*manifest, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT set_config('app.current_role','super_admin', false)"); err != nil {
		return nil, fmt.Errorf("set maintenance scope: %w", err)
	}

	file, err := os.CreateTemp(".", ".audit-export-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("create payload: %w", err)
	}
	tmpName := file.Name()
	defer os.Remove(tmpName)

	hasher := sha256.New()
	w := bufio.NewWriter(io.MultiWriter(file, hasher))
	rows, err := conn.Query(ctx, queryRecords, chainID, from, to)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("query chain segment: %w", err)
	}
	var count int64
	for rows.Next() {
		var doc []byte
		if err := rows.Scan(&doc); err != nil {
			rows.Close()
			file.Close()
			return nil, fmt.Errorf("scan record: %w", err)
		}
		if _, err := w.Write(doc); err != nil {
			rows.Close()
			file.Close()
			return nil, fmt.Errorf("write payload: %w", err)
		}
		if err := w.WriteByte('\n'); err != nil {
			rows.Close()
			file.Close()
			return nil, fmt.Errorf("write payload: %w", err)
		}
		count++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		file.Close()
		return nil, fmt.Errorf("iterate chain segment: %w", err)
	}
	if err := w.Flush(); err != nil {
		file.Close()
		return nil, fmt.Errorf("flush payload: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close payload: %w", err)
	}
	if count == 0 {
		return nil, fmt.Errorf("chain %s: no records in seq %d..%d (refusing to write an empty archive; check the chain id and maintenance DSN)", chainID, from, to)
	}
	if err := os.Rename(tmpName, out); err != nil {
		return nil, fmt.Errorf("publish payload: %w", err)
	}
	info, err := os.Stat(out)
	if err != nil {
		return nil, fmt.Errorf("stat payload: %w", err)
	}

	var headSeq int64
	var head string
	if err := conn.QueryRow(ctx, queryHeadHash, chainID, from, to).Scan(&headSeq, &head); err != nil {
		return nil, fmt.Errorf("read chain head: %w", err)
	}
	// 清单里记录显式区间：-to 0 表示「到链头」，落盘时固化为实际尾 seq，
	// 便于 WORM 归档核对段边界（DEF-29）。
	if to == 0 {
		to = headSeq
	}
	return &manifest{
		Schema:        manifestSchema,
		ChainID:       chainID,
		FromSeq:       from,
		ToSeq:         to,
		Count:         count,
		PayloadSHA256: hex.EncodeToString(hasher.Sum(nil)),
		PayloadBytes:  info.Size(),
		HeadEntryHash: head,
		GeneratedAt:   time.Now().UTC(),
	}, nil
}

// verifyArchive re-hashes the payload and compares it with the manifest,
// including the optional HMAC signature. Any mismatch is a hard failure.
func verifyArchive(payloadPath, manifestPath, hmacKey string) error {
	rawManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	var m manifest
	if err := json.Unmarshal(rawManifest, &m); err != nil {
		return fmt.Errorf("decode manifest: %w", err)
	}
	if m.Schema != manifestSchema {
		return fmt.Errorf("manifest schema %q != %q", m.Schema, manifestSchema)
	}
	if hmacKey != "" {
		if m.Signature == "" {
			return errors.New("manifest has no signature but AUDIT_ARCHIVE_HMAC_KEY is set")
		}
		if !hmac.Equal([]byte(m.Signature), []byte(signManifest(m, hmacKey))) {
			return errors.New("manifest signature mismatch")
		}
	}

	file, err := os.Open(payloadPath)
	if err != nil {
		return fmt.Errorf("open payload: %w", err)
	}
	defer file.Close()
	hasher := sha256.New()
	bytes, lines := int64(0), int64(0)
	buf := make([]byte, 64*1024)
	for {
		n, readErr := file.Read(buf)
		if n > 0 {
			bytes += int64(n)
			hasher.Write(buf[:n])
			lines += int64(strings.Count(string(buf[:n]), "\n"))
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read payload: %w", readErr)
		}
	}
	got := hex.EncodeToString(hasher.Sum(nil))
	if got != m.PayloadSHA256 {
		return fmt.Errorf("payload sha256 %s != manifest %s", got, m.PayloadSHA256)
	}
	if bytes != m.PayloadBytes {
		return fmt.Errorf("payload bytes %d != manifest %d", bytes, m.PayloadBytes)
	}
	if lines != m.Count {
		return fmt.Errorf("payload records %d != manifest %d", lines, m.Count)
	}
	return nil
}

// signManifest returns the hex HMAC-SHA256 over the manifest's immutable facts.
func signManifest(m manifest, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	fmt.Fprintf(mac, "%s|%d|%d|%d|%s|%d|%s",
		m.ChainID, m.FromSeq, m.ToSeq, m.Count, m.PayloadSHA256, m.PayloadBytes, m.HeadEntryHash)
	return hex.EncodeToString(mac.Sum(nil))
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
