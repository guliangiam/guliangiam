package main

// A-6.1: seal pre-existing plaintext values in
// guliangiam.face_id_providers.access_key_secret. The column keeps its
// VARCHAR(255) definition and now stores a v2 envelope bound to the row id;
// the scan only selects values that are not envelope tokens, so the command is
// idempotent and never double-encrypts a row that a concurrent writer already
// sealed. Each row is updated independently: a failure is counted and the run
// continues, leaving previously committed rows in place.

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/stdlib"

	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

const (
	faceIdSecretTable  = "guliangiam.face_id_providers"
	faceIdSecretColumn = "access_key_secret"
	faceIdSecretPK     = "id"
)

var faceIdAccessKeySecretColumn = crypto.EncryptedColumn{
	Table: faceIdSecretTable, Column: faceIdSecretColumn, PKColumn: faceIdSecretPK,
}

type faceIdSecretsOptions struct {
	BatchSize   int
	DryRun      bool
	Verify      bool
	MaxFailures int
}

type faceIdSecretsStats struct {
	Scanned   int64
	Encrypted int64
	Skipped   int64
	Failed    int64
	LegacyV1  int64
	LegacyV0  int64
	Elapsed   time.Duration
}

func runEncryptFaceIDSecrets(argv []string) int {
	ctx := context.Background()

	fs := flag.NewFlagSet("migrate encrypt-faceid-secrets", flag.ContinueOnError)
	dsn := fs.String("dsn", "", "PostgreSQL DSN (defaults to $DATABASE_URL, then $POSTGRES_DSN)")
	batchSize := fs.Int("batch-size", 200, "rows per scan batch")
	dryRun := fs.Bool("dry-run", false, "count plaintext rows without writing")
	verify := fs.Bool("verify", false, "fail if plaintext remains; never writes")
	maxFailures := fs.Int("max-failures", defaultMaxFailures, "maximum failing rows retained in the queue (0 = unlimited)")
	if !parseFlags(fs, argv) {
		return 2
	}
	if *batchSize < 1 {
		fmt.Fprintln(os.Stderr, "-batch-size must be >= 1")
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

	// Fail before connecting: without a KEK every attempt would be a plaintext
	// write, and NewFieldEncryptor would panic instead of reporting it.
	if !crypto.FieldEncryptionConfigured() {
		fmt.Fprintln(os.Stderr, "ENCRYPTION_KEY_CURRENT (or legacy ENCRYPTION_KEY) is required; refusing to write face id provider secrets")
		return 1
	}

	pool, code := openPool(ctx, resolveDSN(*dsn))
	if code != 0 {
		return code
	}
	defer pool.Close()

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	_, code = encryptFaceIDSecretsWithDB(ctx, sqlDB, crypto.GlobalFieldEncryptor(), faceIdSecretsOptions{
		BatchSize:   *batchSize,
		DryRun:      *dryRun,
		Verify:      *verify,
		MaxFailures: *maxFailures,
	})
	return code
}

// encryptFaceIDSecretsWithDB is the testable body. It prints counts only and
// never prints a stored plaintext or ciphertext value.
func encryptFaceIDSecretsWithDB(ctx context.Context, sqlDB *sql.DB, encryptor *crypto.FieldEncryptor, opts faceIdSecretsOptions) (faceIdSecretsStats, int) {
	start := time.Now()
	var stats faceIdSecretsStats

	// The backfill spans every tenant, so the context carries the system scope
	// required by the FORCE RLS policy on face_id_providers.
	ctx = tenantctx.System(ctx)
	if opts.BatchSize <= 0 {
		opts.BatchSize = 200
	}

	columnStats, plaintextRemaining, err := crypto.BackfillEncryptedColumns(ctx, sqlDB, encryptor,
		[]crypto.EncryptedColumn{faceIdAccessKeySecretColumn},
		crypto.EncryptedColumnBackfillOptions{
			BatchSize:   opts.BatchSize,
			DryRun:      opts.DryRun,
			Verify:      opts.Verify,
			MaxFailures: opts.MaxFailures,
		})
	for _, stat := range columnStats {
		stats.Scanned += stat.Scanned
		stats.Encrypted += stat.Encrypted
		stats.Skipped += stat.Skipped
		stats.Failed += stat.Failed
		stats.LegacyV1 += stat.LegacyV1
		stats.LegacyV0 += stat.LegacyV0
	}
	stats.Elapsed = time.Since(start)

	if err != nil {
		slog.Error("face id provider secret backfill failed", "table", faceIdSecretTable, "column", faceIdSecretColumn, "err", err)
		fmt.Fprintf(os.Stderr, "encrypt face id provider secrets: %v\n", err)
		return stats, 1
	}

	switch {
	case opts.Verify:
		fmt.Printf("%s.%s: plaintext_remaining=%d elapsed_ms=%d\n",
			faceIdSecretTable, faceIdSecretColumn, stats.Scanned, stats.Elapsed.Milliseconds())
		if plaintextRemaining {
			fmt.Fprintf(os.Stderr, "verify failed: %d plaintext face id provider secret(s) remain; run without -verify to seal them\n", stats.Scanned)
			return stats, 1
		}
		fmt.Println("verify OK: no plaintext face id provider access_key_secret found")
		return stats, 0
	case opts.DryRun:
		fmt.Printf("%s.%s: would_encrypt=%d elapsed_ms=%d (dry-run, no rows written)\n",
			faceIdSecretTable, faceIdSecretColumn, stats.Scanned, stats.Elapsed.Milliseconds())
		return stats, 0
	default:
		fmt.Printf("%s.%s: selected=%d encrypted=%d skipped=%d failed=%d legacy_v1=%d legacy_v0=%d elapsed_ms=%d\n",
			faceIdSecretTable, faceIdSecretColumn, stats.Scanned, stats.Encrypted, stats.Skipped, stats.Failed,
			stats.LegacyV1, stats.LegacyV0, stats.Elapsed.Milliseconds())
		fmt.Printf("summary: selected=%d encrypted=%d skipped=%d failed=%d legacy_v1=%d legacy_v0=%d\n",
			stats.Scanned, stats.Encrypted, stats.Skipped, stats.Failed, stats.LegacyV1, stats.LegacyV0)
		if stats.Failed > 0 {
			fmt.Fprintf(os.Stderr, "%d row(s) failed to encrypt; correct the data and re-run (sealed rows are skipped)\n", stats.Failed)
			return stats, 1
		}
		if stats.LegacyV1+stats.LegacyV0 > 0 {
			fmt.Fprintf(os.Stderr, "%d legacy envelope(s) remain; run `migrate reencrypt` to upgrade them\n",
				stats.LegacyV1+stats.LegacyV0)
			return stats, 1
		}
		return stats, 0
	}
}
