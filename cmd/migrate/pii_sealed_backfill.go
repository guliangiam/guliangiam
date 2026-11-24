package main

// A-6.3B seals legacy plaintext PII into the `_sealed` columns added by A-6.3A.
// Targets carry v2 envelopes bound to (table, target column, row PK); the job
// walks in PK order and commits one CAS update per row (target IS NULL).

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/stdlib"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/pii"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// defaultPIISealedMaxFailures stops a column before a broken KEK can keep
// hammering the table.
const defaultPIISealedMaxFailures = 10

func runBackfillPIISealedColumns(argv []string) int {
	ctx := context.Background()

	fs := flag.NewFlagSet("migrate backfill-pii-sealed-columns", flag.ContinueOnError)
	dsn := fs.String("dsn", "", "PostgreSQL DSN (defaults to $DATABASE_URL, then $POSTGRES_DSN)")
	batchSize := fs.Int("batch-size", pii.DefaultLegacySealedBatchSize, "rows per keyset batch (max 1000)")
	dryRun := fs.Bool("dry-run", false, "count rows that would be sealed without writing")
	verify := fs.Bool("verify", false, "report encrypted/unsealed/decrypt-failed counts; never writes")
	maxFailures := fs.Int("max-failures", defaultPIISealedMaxFailures, "stop a column after this many failed rows (0 = unlimited)")
	if !parseFlags(fs, argv) {
		return 2
	}
	if *batchSize < 1 || *batchSize > pii.MaxLegacySealedBatchSize {
		fmt.Fprintf(os.Stderr, "-batch-size must be between 1 and %d\n", pii.MaxLegacySealedBatchSize)
		return 2
	}
	if *maxFailures < 0 {
		fmt.Fprintln(os.Stderr, "-max-failures must be >= 0")
		return 2
	}
	if *dryRun && *verify {
		fmt.Fprintln(os.Stderr, "-dry-run and -verify are mutually exclusive")
		return 2
	}

	// Fail before connecting or writing: a missing KEK would either panic on the
	// first row or leave plaintext in a column that now advertises ciphertext.
	if !crypto.FieldEncryptionConfigured() {
		fmt.Fprintln(os.Stderr, "ENCRYPTION_KEY_CURRENT (or legacy ENCRYPTION_KEY) is required; refusing to seal PII columns")
		return 1
	}

	pool, code := openPool(ctx, resolveDSN(*dsn))
	if code != 0 {
		return code
	}
	defer pool.Close()

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	return backfillPIISealedColumnsWithDB(ctx, sqlDB, crypto.GlobalFieldEncryptor(), pii.LegacySealedBackfillOptions{
		BatchSize:   *batchSize,
		DryRun:      *dryRun,
		Verify:      *verify,
		MaxFailures: *maxFailures,
	})
}

// backfillPIISealedColumnsWithDB is the testable body of the command. The
// context gets the system scope because the job spans every tenant and the two
// tables are RLS-protected.
func backfillPIISealedColumnsWithDB(ctx context.Context, sqlDB *sql.DB, encryptor *crypto.FieldEncryptor, opts pii.LegacySealedBackfillOptions) int {
	ctx = tenantctx.System(ctx)

	report, err := pii.BackfillLegacySealedColumns(ctx, sqlDB, encryptor, crypto.LegacySealedColumns, opts)
	printPIISealedReport(report, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backfill pii sealed columns: %v\n", err)
		return 1
	}

	switch {
	case opts.Verify:
		unsealed, failed := piiSealedVerifyTotals(report)
		if unsealed+failed > 0 {
			fmt.Fprintf(os.Stderr, "PII sealed-column verify failed: %d unsealed row(s) and %d unreadable envelope(s) remain\n", unsealed, failed)
			return 1
		}
		return 0
	case opts.DryRun:
		return 0
	default:
		var failed int64
		for _, stat := range report.Columns {
			failed += stat.Failed
		}
		if failed > 0 {
			fmt.Fprintln(os.Stderr, "PII sealed-column backfill incomplete: fix the failed rows and re-run; sealed rows are skipped")
			return 1
		}
		return 0
	}
}

func printPIISealedReport(report pii.LegacySealedBackfillReport, opts pii.LegacySealedBackfillOptions) {
	var selected, encrypted, skipped, failed int64
	var verifyEncrypted, unsealed, decryptFailed int64

	for _, stat := range report.Columns {
		switch {
		case opts.Verify:
			fmt.Printf("%s.%s -> %s: encrypted=%d unsealed=%d decrypt_failed=%d duration=%s\n",
				stat.Table, stat.SourceColumn, stat.TargetColumn,
				stat.VerifyEncrypted, stat.VerifyUnsealed, stat.VerifyDecryptFailed, stat.Duration.Round(time.Millisecond))
			verifyEncrypted += stat.VerifyEncrypted
			unsealed += stat.VerifyUnsealed
			decryptFailed += stat.VerifyDecryptFailed
		case opts.DryRun:
			fmt.Printf("%s.%s -> %s: would_seal=%d duration=%s\n",
				stat.Table, stat.SourceColumn, stat.TargetColumn, stat.Selected, stat.Duration.Round(time.Millisecond))
			selected += stat.Selected
		default:
			fmt.Printf("%s.%s -> %s: selected=%d encrypted=%d skipped=%d failed=%d duration=%s\n",
				stat.Table, stat.SourceColumn, stat.TargetColumn,
				stat.Selected, stat.Encrypted, stat.Skipped, stat.Failed, stat.Duration.Round(time.Millisecond))
			selected += stat.Selected
			encrypted += stat.Encrypted
			skipped += stat.Skipped
			failed += stat.Failed
		}
	}

	switch {
	case opts.Verify:
		fmt.Printf("verify summary: encrypted=%d unsealed=%d decrypt_failed=%d duration=%s\n",
			verifyEncrypted, unsealed, decryptFailed, report.Duration.Round(time.Millisecond))
	case opts.DryRun:
		fmt.Printf("dry-run summary: would_seal=%d duration=%s\n", selected, report.Duration.Round(time.Millisecond))
	default:
		fmt.Printf("summary: selected=%d encrypted=%d skipped=%d failed=%d duration=%s\n",
			selected, encrypted, skipped, failed, report.Duration.Round(time.Millisecond))
	}
}

func piiSealedVerifyTotals(report pii.LegacySealedBackfillReport) (unsealed, decryptFailed int64) {
	for _, stat := range report.Columns {
		unsealed += stat.VerifyUnsealed
		decryptFailed += stat.VerifyDecryptFailed
	}
	return unsealed, decryptFailed
}
