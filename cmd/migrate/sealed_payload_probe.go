// scan-sealed-payloads: read-only generation probe for the v1 sealed-payload
// registry.
//
// F2-01c Batch 1 switches the two application read paths to DecryptColumn(row
// AAD) without changing writes. This command measures the on-disk distribution
// (v0/v1/v2/unrecognized/unreadable) before the write and rotation batches. It
// uses the AAD-aware v2 classifier, never writes, and never reads or clears the
// reencrypt checkpoint.

package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/stdlib"

	"github.com/guliangiam/guliangiam/internal/bootstrap"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

func runScanSealedPayloads(argv []string) int {
	ctx := context.Background()

	fs := flag.NewFlagSet("migrate scan-sealed-payloads", flag.ContinueOnError)
	dsn := fs.String("dsn", "", "PostgreSQL DSN (defaults to $DATABASE_URL, then $POSTGRES_DSN)")
	batchSize := fs.Int("batch-size", 500, "rows read per keyset page")
	if !parseFlags(fs, argv) {
		return 2
	}
	if *batchSize <= 0 {
		fmt.Fprintln(os.Stderr, "-batch-size must be > 0")
		return 2
	}
	if !crypto.FieldEncryptionConfigured() {
		fmt.Fprintln(os.Stderr, "ENCRYPTION_KEY_CURRENT (or legacy ENCRYPTION_KEY) is required; refusing to classify sealed payloads without a KEK")
		return 1
	}

	pool, code := openPool(ctx, resolveDSN(*dsn))
	if code != 0 {
		return code
	}
	defer pool.Close()

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	return scanSealedPayloadsWithDB(ctx, sqlDB, crypto.GlobalFieldEncryptor(), *batchSize)
}

// scanSealedPayloadsWithDB is the testable body of the probe. The registry
// identifiers are reused as EncryptedColumn bindings so the probe cannot drift
// from the column names the application reads.
func scanSealedPayloadsWithDB(ctx context.Context, sqlDB *sql.DB, encryptor *crypto.FieldEncryptor, batchSize int) int {
	ctx = tenantctx.System(ctx)

	columns := make([]crypto.EncryptedColumn, 0, len(crypto.SealedPayloadColumns))
	for _, col := range crypto.SealedPayloadColumns {
		columns = append(columns, crypto.EncryptedColumn{
			Table: col.Table, Column: col.Column, PKColumn: col.PKColumn,
		})
	}

	report, err := crypto.ScanEncryptedPayloads(ctx, bootstrap.NewPayloadBatchSource(sqlDB), encryptor, columns, crypto.EncryptedPayloadScanOptions{
		BatchSize: batchSize,
		MaxIssues: defaultMaxFailures,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan sealed payloads: %v\n", err)
		return 1
	}

	printPayloadScanColumns(report.Columns)
	scanned, skipped, current, stale, v1, v0, unrecognized, unreadable := payloadScanTotals(report)
	fmt.Printf("sealed payload scan summary: scanned=%d skipped=%d current-v2=%d stale-v2=%d legacy-v1=%d legacy-v0=%d unrecognized=%d unreadable=%d (read-only, no rows written)\n",
		scanned, skipped, current, stale, v1, v0, unrecognized, unreadable)
	return 0
}
