package main

// GM N8b: normalize legacy app_oauth_config.token_signing_method before the
// V10 CHECK. btrim(...) returns empty for "" and spaces, not tabs (TrimSpace
// would overcount); enum values compare exactly. The LOCAL super_admin bypass
// is mandatory because app_oauth_config is FORCE RLS.

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
)

const (
	tokenSigningTable    = "guliangiam.app_oauth_config"
	tokenSigningColumn   = "token_signing_method"
	tokenSigningRS256    = "RS256"
	tokenSigningSM2SM3   = "SM2-SM3"
	tokenSigningZeroUUID = "00000000-0000-0000-0000-000000000000"
	defaultSigningBatch  = 200
)

const tokenSigningRLSBypassSQL = `SELECT set_config('app.current_role', 'super_admin', true), set_config('app.current_organization_id', '', true)`
const tokenSigningScanSQL = `SELECT application_id::text, token_signing_method FROM guliangiam.app_oauth_config
WHERE application_id > $1::uuid ORDER BY application_id ASC LIMIT $2`

type tokenSigningOptions struct {
	BatchSize int
	DryRun    bool
	Verify    bool
}
type tokenSigningStats struct {
	Scanned, Null, Empty, Whitespace, Valid, Invalid, Updated int64
	Batches                                                   int
	Elapsed                                                   time.Duration
}

func (s tokenSigningStats) blank() int64 { return s.Empty + s.Whitespace }

type tokenSigningRow struct {
	id     string
	method sql.NullString
}

type signingClass int

const (
	signingNull signingClass = iota
	signingEmpty
	signingWhitespace
	signingValid
	signingInvalid
)

func (r tokenSigningRow) class() signingClass {
	if !r.method.Valid {
		return signingNull
	}
	if strings.Trim(r.method.String, " ") == "" { // same rule as btrim(x) is empty
		if r.method.String == "" {
			return signingEmpty
		}
		return signingWhitespace
	}
	if r.method.String == tokenSigningRS256 || r.method.String == tokenSigningSM2SM3 {
		return signingValid
	}
	return signingInvalid
}

func runBackfillApplicationTokenSigning(argv []string) int {
	ctx := context.Background()
	fs := flag.NewFlagSet("migrate backfill-application-token-signing", flag.ContinueOnError)
	dsn := fs.String("dsn", "", "PostgreSQL DSN (defaults to $DATABASE_URL, then $POSTGRES_DSN)")
	batchSize := fs.Int("batch-size", defaultSigningBatch, "rows per scan/update batch")
	dryRun := fs.Bool("dry-run", false, "classify rows without writing")
	verify := fs.Bool("verify", false, "fail unless no blank, NULL, or out-of-enum rows remain; never writes")
	if !parseFlags(fs, argv) {
		return 2
	}
	if *batchSize < 1 {
		fmt.Fprintln(os.Stderr, "-batch-size must be >= 1")
		return 2
	}
	if *dryRun && *verify {
		fmt.Fprintln(os.Stderr, "-dry-run and -verify are mutually exclusive")
		return 2
	}
	pool, code := openPool(ctx, resolveDSN(*dsn))
	if code != 0 {
		return code
	}
	defer pool.Close()
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	_, code = backfillApplicationTokenSigningWithDB(ctx, sqlDB, tokenSigningOptions{
		BatchSize: *batchSize, DryRun: *dryRun, Verify: *verify,
	})
	return code
}

func backfillApplicationTokenSigningWithDB(ctx context.Context, sqlDB *sql.DB, opts tokenSigningOptions) (tokenSigningStats, int) {
	start := time.Now()
	var stats tokenSigningStats
	batchSize := opts.BatchSize
	readOnly := opts.DryRun || opts.Verify
	cursor := tokenSigningZeroUUID
	for {
		batch, updated, err := tokenSigningBatch(ctx, sqlDB, cursor, batchSize, readOnly)
		if err != nil {
			slog.Error("application token signing backfill: batch failed",
				"table", tokenSigningTable, "batch", stats.Batches+1, "dry_run", opts.DryRun, "verify", opts.Verify, "err", err)
			return stats, 1
		}
		stats.Batches++
		stats.Scanned += int64(len(batch))
		stats.Updated += updated
		var blank, nulls, invalid int64
		for _, row := range batch {
			switch row.class() {
			case signingNull:
				nulls++
				stats.Null++
			case signingEmpty:
				blank++
				stats.Empty++
			case signingWhitespace:
				blank++
				stats.Whitespace++
			case signingValid:
				stats.Valid++
			case signingInvalid:
				invalid++
				stats.Invalid++
				slog.Warn("application token signing backfill: out-of-enum value left untouched",
					"table", tokenSigningTable, "column", tokenSigningColumn,
					"pk_fingerprint", tokenSigningFingerprint(row.id), "value", strconv.Quote(row.method.String),
					"allowed", tokenSigningRS256+","+tokenSigningSM2SM3)
			}
		}
		slog.Info("application token signing backfill: batch",
			"table", tokenSigningTable, "batch", stats.Batches, "scanned_rows", len(batch),
			"blank_rows", blank, "updated_rows", updated, "invalid_rows", invalid, "null_rows", nulls,
			"dry_run", opts.DryRun, "verify", opts.Verify)
		if len(batch) < batchSize {
			break
		}
		cursor = batch[len(batch)-1].id
	}
	stats.Elapsed = time.Since(start)
	slog.Info("application token signing backfill: summary",
		"table", tokenSigningTable, "column", tokenSigningColumn, "dry_run", opts.DryRun, "verify", opts.Verify,
		"batches", stats.Batches, "scanned_rows", stats.Scanned, "null_rows", stats.Null,
		"empty_string_rows", stats.Empty, "whitespace_rows", stats.Whitespace, "blank_rows", stats.blank(),
		"valid_rows", stats.Valid, "invalid_values", stats.Invalid, "updated_rows", stats.Updated,
		"elapsed_ms", stats.Elapsed.Milliseconds())
	return stats, tokenSigningExitCode(stats, opts)
}

func tokenSigningExitCode(stats tokenSigningStats, opts tokenSigningOptions) int {
	manual := stats.Invalid + stats.Null
	if opts.Verify {
		if stats.blank() > 0 || manual > 0 {
			slog.Error("application token signing backfill: verify failed", "table", tokenSigningTable,
				"blank_rows", stats.blank(), "invalid_values", stats.Invalid, "null_rows", stats.Null)
			return 1
		}
		slog.Info("application token signing backfill: verify OK", "table", tokenSigningTable, "scanned_rows", stats.Scanned)
		return 0
	}
	if manual > 0 {
		slog.Error("application token signing backfill: manual remediation required", "table", tokenSigningTable,
			"invalid_values", stats.Invalid, "null_rows", stats.Null, "updated_rows", stats.Updated,
			"allowed", tokenSigningRS256+","+tokenSigningSM2SM3)
		return 1
	}
	if opts.DryRun {
		slog.Info("application token signing backfill: dry-run complete", "table", tokenSigningTable,
			"would_update_rows", stats.blank(), "target", tokenSigningRS256)
		return 0
	}
	slog.Info("application token signing backfill: complete", "table", tokenSigningTable,
		"updated_rows", stats.Updated, "target", tokenSigningRS256)
	return 0
}

// tokenSigningBatch scans one keyset page and, in write mode, updates its blank
// rows in the same transaction. The UPDATE repeats btrim(...) is empty so a
// concurrent writer that already fixed a row makes it affect zero rows.
func tokenSigningBatch(ctx context.Context, sqlDB *sql.DB, cursor string, batchSize int, readOnly bool) ([]tokenSigningRow, int64, error) {
	tx, err := sqlDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: readOnly})
	if err != nil {
		return nil, 0, fmt.Errorf("begin batch: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, tokenSigningRLSBypassSQL); err != nil {
		return nil, 0, fmt.Errorf("set rls context: %w", err)
	}
	rows, err := tx.QueryContext(ctx, tokenSigningScanSQL, cursor, batchSize)
	if err != nil {
		return nil, 0, fmt.Errorf("scan rows: %w", err)
	}
	defer rows.Close()
	var batch []tokenSigningRow
	for rows.Next() {
		var row tokenSigningRow
		if err := rows.Scan(&row.id, &row.method); err != nil {
			return nil, 0, fmt.Errorf("scan row: %w", err)
		}
		batch = append(batch, row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate rows: %w", err)
	}
	var updated int64
	if !readOnly {
		blankIDs := make([]string, 0, len(batch))
		for _, row := range batch {
			if class := row.class(); class == signingEmpty || class == signingWhitespace {
				blankIDs = append(blankIDs, row.id)
			}
		}
		if len(blankIDs) > 0 {
			query, args := tokenSigningUpdateStatement(blankIDs)
			result, err := tx.ExecContext(ctx, query, args...)
			if err != nil {
				return nil, 0, fmt.Errorf("update blank rows: %w", err)
			}
			if updated, err = result.RowsAffected(); err != nil {
				return nil, 0, fmt.Errorf("read affected rows: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, 0, fmt.Errorf("commit batch: %w", err)
	}
	return batch, updated, nil
}

func tokenSigningUpdateStatement(ids []string) (string, []any) {
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d::uuid", i+1)
		args[i] = id
	}
	return fmt.Sprintf(`UPDATE guliangiam.app_oauth_config
SET token_signing_method = 'RS256', revision = revision + 1
WHERE application_id IN (%s) AND btrim(token_signing_method) = ''`, strings.Join(placeholders, ", ")), args
}

func tokenSigningFingerprint(applicationID string) string {
	sum := sha256.Sum256([]byte(applicationID))
	return hex.EncodeToString(sum[:8])
}
