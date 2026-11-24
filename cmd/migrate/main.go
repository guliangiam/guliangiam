// Command migrate is the operator entry point of the versioned migration chain
// (ADR-022 决策 3 / 附录 A.2.2). It is what the release pipeline's migration Job
// runs; the server itself only auto-migrates outside production.
//
//	migrate up --dsn "$POSTGRES_DSN"            # apply pending structure migrations
//	migrate up --dsn "$POSTGRES_DSN" --seed     # also apply the seed chain (initial environments only)
//	migrate status --dsn "$POSTGRES_DSN"        # print the migration ledger
//	migrate encrypt-provider-secrets --dsn "$POSTGRES_DSN"            # encrypt legacy provider plaintext
//	migrate encrypt-provider-secrets --dsn "$POSTGRES_DSN" --verify   # fail if plaintext remains
//	migrate backfill-encrypted-columns --dsn "$POSTGRES_DSN"            # seal plaintext in every registered encrypted column
//	migrate backfill-encrypted-columns --dsn "$POSTGRES_DSN" --dry-run  # classify rows without writing
//	migrate backfill-encrypted-columns --dsn "$POSTGRES_DSN" --verify   # fail if plaintext/unrecognized payloads remain
//	migrate reencrypt --dsn "$POSTGRES_DSN"     # rewrite v1 encrypted columns into the AAD-bound v2 generation and re-seal v1 sealed payloads
//	migrate reencrypt --dsn "$POSTGRES_DSN" --include-sealed-payloads=false  # encrypted columns only
//	migrate reencrypt --dsn "$POSTGRES_DSN" --verify   # read-only: fail unless every value is current (v2 columns, v1 sealed payloads)
//	migrate reencrypt --dsn "$POSTGRES_DSN" --dry-run  # read-only: report what rotation would change
//	migrate scan-sealed-payloads --dsn "$POSTGRES_DSN"            # read-only: v0/v1/v2 distribution for the two sealed-payload columns
//	migrate scan-sealed-payloads --dsn "$POSTGRES_DSN" -batch-size 500
//	migrate backfill-application-token-signing --dsn "$POSTGRES_DSN"            # normalize blank token_signing_method to RS256
//	migrate backfill-application-token-signing --dsn "$POSTGRES_DSN" --dry-run  # classify rows without writing
//	migrate backfill-application-token-signing --dsn "$POSTGRES_DSN" --verify   # fail unless no blank/NULL/out-of-enum rows remain
//	migrate encrypt-faceid-secrets --dsn "$POSTGRES_DSN"            # seal legacy plaintext face id provider access_key_secret
//	migrate encrypt-faceid-secrets --dsn "$POSTGRES_DSN" --dry-run  # count plaintext rows without writing
//	migrate encrypt-faceid-secrets --dsn "$POSTGRES_DSN" --verify   # fail if plaintext remains
//	migrate encrypt-credential-secrets --dsn "$POSTGRES_DSN"            # seal legacy plaintext mcp_servers.token / identity_verifications.session_token
//	migrate encrypt-credential-secrets --dsn "$POSTGRES_DSN" --dry-run  # count plaintext rows without writing
//	migrate encrypt-credential-secrets --dsn "$POSTGRES_DSN" --verify   # fail if plaintext remains
//	migrate encrypt-sealed-payloads --dsn "$POSTGRES_DSN"            # seal legacy plaintext in every registered sealed payload column as v1 (never v2)
//	migrate encrypt-sealed-payloads --dsn "$POSTGRES_DSN" -table pay_channel_configs  # narrow the registry-driven run to one table
//	migrate encrypt-sealed-payloads --dsn "$POSTGRES_DSN" --dry-run  # count and list plaintext rows without writing
//	migrate encrypt-sealed-payloads --dsn "$POSTGRES_DSN" --verify   # verify sealed values decrypt with the registered AAD; never writes
//	migrate encrypt-sealed-payloads --dsn "$POSTGRES_DSN" -force     # re-seal readable v1/v0 values at the current KEK (v2 is never downgraded)
//
//	migrate backfill-tenant-topology --dsn "$POSTGRES_DSN"            # backfill tenant identity + primary topology for legacy organizations
//	migrate backfill-tenant-topology --dsn "$POSTGRES_DSN" --dry-run  # count missing identities/topologies without writing
//	migrate backfill-tenant-topology --dsn "$POSTGRES_DSN" --verify   # fail if any active organization still lacks an identity or primary topology
//
// Sealed payloads (llm_providers.api_key_sealed, pay_channel_configs.config_sealed)
// are included by default because an opt-in switch would let an operator rotate
// every KEK while leaving those values bound to the previous key. They are
// re-sealed with DecryptField -> EncryptField, so they stay v1 (no AAD); a v2
// token in a sealed column is reported as unrecognized and never rewritten,
// because the legacy sealed-payload path still reads them with DecryptField,
// which rejects v2.
//
// encrypt-sealed-payloads is table-driven: it scans every entry of
// crypto.SealedPayloadColumns and accepts -table to narrow the run. Only
// non-envelope values are sealed; readable v1/v0/v2 values are skipped unless
// -force re-seals v1/v0 at the current KEK. -verify decrypts every non-empty
// value with the registry-derived (table|column|pk) AAD, so a v2 token that was
// moved to another row/column is reported as an AAD mismatch.
//
// scan-sealed-payloads is the F2-01c Batch 1 read-only probe: it classifies the
// two registered columns through the AAD-aware v2 classifier, so operators can
// measure the v0/v1/v2 distribution before the write and rotation batches. It
// never writes and never touches the reencrypt checkpoint.
//
// -verify prints a stable primary-key queue (bounded by -max-failures, default
// 100) without ever printing the stored value; -dry-run reports the same class
// counts without a per-row queue. Neither mode reads or writes the checkpoint.
//
// backfill-encrypted-columns is the generic form of encrypt-provider-secrets:
// it scans DefaultEncryptedColumns (including the V8 credential columns and the
// TOTP seed) and selects only rows that are not already envelope tokens or
// decryptable v0 legacy tokens. Rows already sealed at any generation are left
// untouched, so the command can be interrupted and re-run without
// double-encrypting data; no checkpoint file is needed because each re-run
// re-scans for remaining plaintext. The written pass never touches sealed
// payload columns (their terminal generation is v1, not v2); -verify and
// -dry-run still classify them so plaintext/unreadable values are reported.
//
// reencrypt stores resumable progress in ./reencrypt.checkpoint by default
// (relative to the working directory); override it with -checkpoint, or pass
// -checkpoint "" to disable resume for a one-shot run.
//
// The command word must come first; the flags follow it.
//
// The command deliberately does not implement rollback: rollback is a distinct,
// reviewed operator action executed from the generated `db/rollback/*.down.sql`
// artifacts, not something a deployment process can trigger by accident.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/guliangiam/guliangiam/internal/bootstrap"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/guliangiam/guliangiam/internal/platform/db"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

func main() {
	os.Exit(run())
}

func run() int {
	return runWithArgs(os.Args[1:])
}

func runWithArgs(argv []string) int {
	// The command word comes first and the flags follow (`migrate up --seed`),
	// which is the form the release pipeline's Job uses. It is peeled off before
	// parsing because Go's flag package stops at the first non-flag argument:
	// parsing `up --seed` wholesale left `--seed` unparsed and seeded nothing.
	command := "up"
	if len(argv) > 0 && !strings.HasPrefix(argv[0], "-") {
		command, argv = argv[0], argv[1:]
	}

	switch command {
	case "up":
		return runUp(argv)
	case "status":
		return runStatus(argv)
	case "encrypt-provider-secrets":
		return runEncryptProviderSecrets(argv)
	case "backfill-encrypted-columns":
		return runBackfillEncryptedColumns(argv)
	case "reencrypt":
		return runReencrypt(argv)
	case "backfill-application-token-signing":
		return runBackfillApplicationTokenSigning(argv)
	case "encrypt-faceid-secrets":
		return runEncryptFaceIDSecrets(argv)
	case "encrypt-credential-secrets":
		return runEncryptCredentialSecrets(argv)
	case "encrypt-sealed-payloads":
		return runEncryptSealedPayloads(argv)
	case "scan-sealed-payloads":
		return runScanSealedPayloads(argv)
	case "backfill-pii-sealed-columns":
		return runBackfillPIISealedColumns(argv)
	case "backfill-tenant-topology":
		return runBackfillTenantTopology(argv)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q: expected up, status, encrypt-provider-secrets, backfill-encrypted-columns, reencrypt, scan-sealed-payloads, backfill-application-token-signing, encrypt-faceid-secrets, encrypt-credential-secrets, encrypt-sealed-payloads, backfill-pii-sealed-columns, or backfill-tenant-topology\n", command)
		return 2
	}
}

// defaultReencryptCheckpoint is the operator-visible default resume file. It
// lives in the working directory so a release Job can mount a volume over it.
const defaultReencryptCheckpoint = "reencrypt.checkpoint"

// defaultMaxFailures bounds the per-run failure queue printed by -verify and by
// the real backfill/rotation pass. The summary still counts every failure.
const defaultMaxFailures = 100

func runUp(argv []string) int {
	ctx := context.Background()

	fs := flag.NewFlagSet("migrate up", flag.ContinueOnError)
	dsn := fs.String("dsn", "", "PostgreSQL DSN (defaults to $DATABASE_URL, then $POSTGRES_DSN)")
	seed := fs.Bool("seed", false, "also apply the seed (data) chain; initial environments only")
	if !parseFlags(fs, argv) {
		return 2
	}

	pool, code := openPool(ctx, resolveDSN(*dsn))
	if code != 0 {
		return code
	}
	defer pool.Close()

	// The gates are set here rather than read from the ambient environment: this
	// command exists precisely to apply migrations, so its intent must not depend
	// on how the shell happened to be configured. Seeding stays opt-in because a
	// structure migration Job must never write business data.
	os.Setenv("AUTO_MIGRATE", "true")
	if *seed {
		os.Setenv("AUTO_SEED", "true")
	}

	if err := db.Migrate(ctx, pool, bootstrap.Migrations()); err != nil {
		fmt.Fprintf(os.Stderr, "migrate up: %v\n", err)
		return 1
	}
	if err := bootstrap.EnsureBootstrapAdminCredential(ctx, pool); err != nil {
		fmt.Fprintf(os.Stderr, "initialize bootstrap admin: %v\n", err)
		return 1
	}

	fmt.Printf("structure content hash: %s\n", db.StructureContentHash(bootstrap.Migrations()))
	return status(ctx, pool)
}

func runStatus(argv []string) int {
	ctx := context.Background()

	fs := flag.NewFlagSet("migrate status", flag.ContinueOnError)
	dsn := fs.String("dsn", "", "PostgreSQL DSN (defaults to $DATABASE_URL, then $POSTGRES_DSN)")
	if !parseFlags(fs, argv) {
		return 2
	}

	pool, code := openPool(ctx, resolveDSN(*dsn))
	if code != 0 {
		return code
	}
	defer pool.Close()

	return status(ctx, pool)
}

func runEncryptProviderSecrets(argv []string) int {
	ctx := context.Background()

	fs := flag.NewFlagSet("migrate encrypt-provider-secrets", flag.ContinueOnError)
	dsn := fs.String("dsn", "", "PostgreSQL DSN (defaults to $DATABASE_URL, then $POSTGRES_DSN)")
	batchSize := fs.Int("batch-size", 200, "rows per scan batch")
	dryRun := fs.Bool("dry-run", false, "count plaintext rows without writing")
	verify := fs.Bool("verify", false, "fail if plaintext remains; never writes")
	maxFailures := fs.Int("max-failures", defaultMaxFailures, "maximum failing keys to print (0 = unlimited)")
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

	// Fail before connecting or writing: without a KEK every attempt would be a
	// plaintext write, and NewFieldEncryptor would panic instead of reporting it.
	if !crypto.FieldEncryptionConfigured() {
		fmt.Fprintln(os.Stderr, "ENCRYPTION_KEY_CURRENT (or legacy ENCRYPTION_KEY) is required; refusing to write provider credentials")
		return 1
	}

	pool, code := openPool(ctx, resolveDSN(*dsn))
	if code != 0 {
		return code
	}
	defer pool.Close()

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	return encryptProviderSecretsWithDB(ctx, sqlDB, crypto.GlobalFieldEncryptor(), crypto.ProviderSecretBackfillOptions{
		BatchSize:   *batchSize,
		DryRun:      *dryRun,
		Verify:      *verify,
		MaxFailures: *maxFailures,
	})
}

func runBackfillEncryptedColumns(argv []string) int {
	ctx := context.Background()

	fs := flag.NewFlagSet("migrate backfill-encrypted-columns", flag.ContinueOnError)
	dsn := fs.String("dsn", "", "PostgreSQL DSN (defaults to $DATABASE_URL, then $POSTGRES_DSN)")
	batchSize := fs.Int("batch-size", 200, "rows per scan batch")
	dryRun := fs.Bool("dry-run", false, "count plaintext rows without writing")
	verify := fs.Bool("verify", false, "fail if plaintext remains; never writes")
	maxFailures := fs.Int("max-failures", defaultMaxFailures, "maximum failing keys to print (0 = unlimited)")
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

	// Fail before connecting or writing: without a KEK every attempt would be a
	// plaintext write, and NewFieldEncryptor would panic instead of reporting it.
	if !crypto.FieldEncryptionConfigured() {
		fmt.Fprintln(os.Stderr, "ENCRYPTION_KEY_CURRENT (or legacy ENCRYPTION_KEY) is required; refusing to write encrypted columns")
		return 1
	}

	pool, code := openPool(ctx, resolveDSN(*dsn))
	if code != 0 {
		return code
	}
	defer pool.Close()

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	return backfillEncryptedColumnsWithDB(ctx, sqlDB, crypto.GlobalFieldEncryptor(), crypto.DefaultEncryptedColumns, crypto.SealedPayloadColumns, crypto.EncryptedColumnBackfillOptions{
		BatchSize:   *batchSize,
		DryRun:      *dryRun,
		Verify:      *verify,
		MaxFailures: *maxFailures,
	})
}

type reencryptOptions struct {
	Checkpoint            string
	DryRun                bool
	Verify                bool
	MaxFailures           int
	IncludeSealedPayloads bool
}

func runReencrypt(argv []string) int {
	ctx := context.Background()

	fs := flag.NewFlagSet("migrate reencrypt", flag.ContinueOnError)
	dsn := fs.String("dsn", "", "PostgreSQL DSN (defaults to $DATABASE_URL, then $POSTGRES_DSN)")
	checkpoint := fs.String("checkpoint", defaultReencryptCheckpoint,
		"resume checkpoint file (default ./reencrypt.checkpoint; pass \"\" to disable resume)")
	dryRun := fs.Bool("dry-run", false, "classify rows and report what rotation would change; never writes")
	verify := fs.Bool("verify", false, "fail if any row is not current v2; never writes")
	maxFailures := fs.Int("max-failures", defaultMaxFailures, "maximum failing keys to print (0 = unlimited)")
	// Sealed payloads are included by default: an opt-in flag would let an
	// operator rotate every KEK while leaving registered v1 payloads bound to
	// the previous key. -include-sealed-payloads=false is the narrow escape hatch.
	includeSealed := fs.Bool("include-sealed-payloads", true,
		"also re-seal registered non-column payloads (kept v1; never upgraded to v2)")
	if !parseFlags(fs, argv) {
		return 2
	}
	if *dryRun && *verify {
		fmt.Fprintln(os.Stderr, "-dry-run and -verify are mutually exclusive")
		return 2
	}
	if *maxFailures < 0 {
		fmt.Fprintln(os.Stderr, "-max-failures must be >= 0")
		return 2
	}

	// Fail before connecting: without a KEK every row would be left unreadable
	// (or NewFieldEncryptor would panic on first use instead of here).
	if !crypto.FieldEncryptionConfigured() {
		fmt.Fprintln(os.Stderr, "ENCRYPTION_KEY_CURRENT (or legacy ENCRYPTION_KEY) is required; refusing to re-encrypt encrypted columns")
		return 1
	}

	pool, code := openPool(ctx, resolveDSN(*dsn))
	if code != 0 {
		return code
	}
	defer pool.Close()

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	return reencryptColumnsWithDB(ctx, sqlDB, crypto.GlobalFieldEncryptor(), reencryptOptions{
		Checkpoint:            *checkpoint,
		DryRun:                *dryRun,
		Verify:                *verify,
		MaxFailures:           *maxFailures,
		IncludeSealedPayloads: *includeSealed,
	})
}

// reencryptColumnsWithDB rewrites every registered encrypted column into v2,
// then (unless disabled) re-seals the registered v1 sealed payloads, and
// reports one line per column plus an aggregate summary. Rotation spans all
// tenants, so the context carries the system scope required by RLS.
func reencryptColumnsWithDB(ctx context.Context, sqlDB *sql.DB, encryptor *crypto.FieldEncryptor, opts reencryptOptions) int {
	var sealed []crypto.SealedPayloadColumn
	if opts.IncludeSealedPayloads {
		sealed = crypto.SealedPayloadColumns
	}
	return reencryptTargetsFor(ctx, sqlDB, encryptor, crypto.DefaultEncryptedColumns, sealed, opts)
}

// reencryptColumnsFor is the testable body of reencryptColumnsWithDB for the
// v2 column registry alone; tests pass a narrow column slice so one sqlmock
// fixture exercises the command contract. reencryptTargetsFor also covers the
// sealed-payload registry.
func reencryptColumnsFor(ctx context.Context, sqlDB *sql.DB, encryptor *crypto.FieldEncryptor, columns []crypto.EncryptedColumn, opts reencryptOptions) int {
	return reencryptTargetsFor(ctx, sqlDB, encryptor, columns, nil, opts)
}

func reencryptTargetsFor(ctx context.Context, sqlDB *sql.DB, encryptor *crypto.FieldEncryptor, columns []crypto.EncryptedColumn, sealed []crypto.SealedPayloadColumn, opts reencryptOptions) int {
	ctx = tenantctx.System(ctx)

	// verify/dry-run are read-only classification scans and must not read or
	// write the resume checkpoint (that would let a verify change the next real
	// run's starting point).
	if opts.Verify || opts.DryRun {
		return scanReencryptReadOnly(ctx, sqlDB, encryptor, columns, sealed, opts)
	}

	// An empty checkpoint is allowed on purpose: a one-shot rotation on a small
	// database needs no resume file, and refusing would force operators to pass
	// a dummy path. The trade-off (restart from the first column) is stated here.
	if opts.Checkpoint == "" {
		fmt.Fprintln(os.Stderr, "checkpoint disabled: an interrupted run restarts from the first column")
	} else {
		fmt.Printf("checkpoint file: %s (delete it to restart from the beginning)\n", opts.Checkpoint)
	}

	job := crypto.NewReencryptJob(sqlDB, encryptor, opts.Checkpoint)
	job.SetMaxFailures(opts.MaxFailures)
	start := time.Now()
	var total crypto.ReencryptStats
	code := 0
	for _, column := range columns {
		stats, err := job.RunFor(ctx, []crypto.EncryptedColumn{column})
		total = accumulateStats(total, stats)
		fmt.Printf("%s.%s: scanned=%d updated=%d skipped=%d errors=%d duration=%s\n",
			column.Table, column.Column, stats.Scanned, stats.Updated, stats.Skipped, stats.Errors, stats.Duration.Round(time.Millisecond))
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s.%s: %v\n", column.Table, column.Column, err)
			code = 1
		}
		if !stats.Completed {
			code = 1
		}
	}
	for _, column := range sealed {
		stats, err := job.RunForSealedPayloads(ctx, []crypto.SealedPayloadColumn{column})
		total = accumulateStats(total, stats)
		fmt.Printf("%s.%s (sealed payload): scanned=%d updated=%d skipped=%d errors=%d duration=%s\n",
			column.Table, column.Column, stats.Scanned, stats.Updated, stats.Skipped, stats.Errors, stats.Duration.Round(time.Millisecond))
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s.%s: %v\n", column.Table, column.Column, err)
			code = 1
		}
		if !stats.Completed {
			code = 1
		}
	}
	total.Duration = time.Since(start)
	total.Completed = ctx.Err() == nil
	shown, queueTruncated := printFailureQueue(total.Failures, total.FailuresTruncated, opts.MaxFailures)
	fmt.Printf("summary: scanned=%d updated=%d skipped=%d errors=%d failures_shown=%d duration=%s completed=%t truncated=%t\n",
		total.Scanned, total.Updated, total.Skipped, total.Errors, shown, total.Duration.Round(time.Millisecond), total.Completed, queueTruncated)
	if total.Errors > 0 || !total.Completed {
		fmt.Fprintln(os.Stderr, "re-encryption incomplete: fix the reported rows and re-run; rows already rewritten are skipped")
		return 1
	}
	if err := job.ClearCheckpoint(); err != nil {
		fmt.Fprintf(os.Stderr, "clear checkpoint: %v\n", err)
		return 1
	}
	return code
}

func accumulateStats(total, stats crypto.ReencryptStats) crypto.ReencryptStats {
	total.Scanned += stats.Scanned
	total.Updated += stats.Updated
	total.Skipped += stats.Skipped
	total.Errors += stats.Errors
	total.Failures = append(total.Failures, stats.Failures...)
	total.FailuresTruncated = total.FailuresTruncated || stats.FailuresTruncated
	return total
}

func scanReencryptReadOnly(ctx context.Context, sqlDB *sql.DB, encryptor *crypto.FieldEncryptor, columns []crypto.EncryptedColumn, sealed []crypto.SealedPayloadColumn, opts reencryptOptions) int {
	verb := "verify"
	if opts.DryRun {
		verb = "dry-run"
	}
	source := bootstrap.NewPayloadBatchSource(sqlDB)
	report, err := crypto.ScanEncryptedPayloads(ctx, source, encryptor, columns, crypto.EncryptedPayloadScanOptions{
		MaxIssues: opts.MaxFailures,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "reencrypt %s: %v\n", verb, err)
		return 1
	}
	var sealedReport crypto.EncryptedPayloadScanReport
	if len(sealed) > 0 {
		sealedReport, err = crypto.ScanSealedPayloads(ctx, source, encryptor, sealed, crypto.EncryptedPayloadScanOptions{
			MaxIssues: opts.MaxFailures,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "reencrypt %s: %v\n", verb, err)
			return 1
		}
	}

	printPayloadScanColumns(report.Columns)
	if len(sealed) > 0 {
		printSealedPayloadScanColumns(sealedReport.Columns)
	}
	scanned, skipped, current, stale, v1, v0, unrecognized, unreadable := payloadScanTotals(report)
	if opts.DryRun {
		fmt.Printf("summary: scanned=%d skipped=%d would-update=%d current-v2=%d legacy-v1=%d legacy-v0=%d unrecognized=%d unreadable=%d (dry-run, no rows written)\n",
			scanned, skipped, stale+v1+v0, current, v1, v0, unrecognized, unreadable)
		printClassAdvice(v1, v0, unrecognized, unreadable,
			"run `migrate backfill-encrypted-columns` if they are plaintext (restore corrupt ciphertext first)")
		printSealedPayloadDryRun(sealedReport)
		return 0
	}

	printPayloadIssues(report, func(class crypto.EncryptedPayloadClass) bool {
		return class.NeedsRotation() || class.NeedsBackfill() || class.IsBroken()
	})
	failures := stale + v1 + v0 + unrecognized + unreadable
	fmt.Printf("verify summary: scanned=%d skipped=%d current-v2=%d issues=%d failures=%d shown=%d truncated=%t\n",
		scanned, skipped, current, report.IssueCount, failures, len(report.Issues), report.IssuesTruncated)
	if report.IssuesTruncated {
		fmt.Fprintf(os.Stderr, "failure queue truncated at %d; fix the listed keys first or raise -max-failures\n", len(report.Issues))
	}
	sealedFailures := int64(0)
	if len(sealed) > 0 {
		// A sealed payload is terminal as v1. v0 is readable but must be re-sealed
		// to v1 before verify can claim the rotation is complete; v2/plaintext and
		// unreadable values are always fatal.
		printSealedPayloadIssues(sealedReport, func(class crypto.EncryptedPayloadClass) bool {
			return class == crypto.PayloadClassV0Legacy || class.NeedsBackfill() || class.IsBroken()
		})
		sealedScanned, sealedSkipped, _, _, sealedV1, sealedV0, sealedUnrecognized, sealedUnreadable := payloadScanTotals(sealedReport)
		sealedFailures = sealedV0 + sealedUnrecognized + sealedUnreadable
		fmt.Printf("sealed payload verify summary: scanned=%d skipped=%d legacy-v1=%d legacy-v0=%d unrecognized=%d unreadable=%d issues=%d failures=%d shown=%d truncated=%t\n",
			sealedScanned, sealedSkipped, sealedV1, sealedV0, sealedUnrecognized, sealedUnreadable,
			sealedReport.IssueCount, sealedFailures, len(sealedReport.Issues), sealedReport.IssuesTruncated)
		if sealedReport.IssuesTruncated {
			fmt.Fprintf(os.Stderr, "sealed payload failure queue truncated at %d; fix the listed keys first or raise -max-failures\n", len(sealedReport.Issues))
		}
	}
	if failures > 0 || sealedFailures > 0 {
		fmt.Fprintf(os.Stderr, "reencrypt verify failed: %d encrypted-column row(s) and %d sealed-payload row(s) are not current; fix the keys above and re-run without -verify\n",
			failures, sealedFailures)
		return 1
	}
	if len(sealed) > 0 {
		fmt.Println("verify OK: every non-empty encrypted-column value is current v2 and every non-empty sealed payload is a v1 envelope")
		return 0
	}
	fmt.Println("verify OK: every non-empty encrypted-column value is current v2")
	return 0
}

func printSealedPayloadDryRun(report crypto.EncryptedPayloadScanReport) {
	if len(report.Columns) == 0 {
		return
	}
	scanned, skipped, _, _, v1, v0, unrecognized, unreadable := payloadScanTotals(report)
	fmt.Printf("sealed payload summary: scanned=%d skipped=%d legacy-v1=%d legacy-v0=%d unrecognized=%d unreadable=%d (dry-run, no rows written)\n",
		scanned, skipped, v1, v0, unrecognized, unreadable)
	if v0 > 0 {
		fmt.Printf("note: %d v0 sealed payload(s) are re-sealed as v1 by `migrate reencrypt` (never v2)\n", v0)
	}
	if unrecognized+unreadable > 0 {
		fmt.Printf("note: %d sealed payload value(s) are not readable envelopes; sealed payloads must stay v1 (restore or re-seal manually)\n", unrecognized+unreadable)
	}
}

func printSealedPayloadScanColumns(columns []crypto.EncryptedPayloadColumnStat) {
	for _, stat := range columns {
		fmt.Printf("%s.%s (sealed payload): scanned=%d skipped=%d legacy-v1=%d legacy-v0=%d unrecognized=%d unreadable=%d\n",
			stat.Table, stat.Column, stat.Scanned, stat.Skipped,
			stat.V1Legacy, stat.V0Legacy, stat.Unrecognized, stat.Unreadable)
	}
}

func printSealedPayloadIssues(report crypto.EncryptedPayloadScanReport, fatal func(crypto.EncryptedPayloadClass) bool) {
	for _, issue := range report.Issues {
		label := "LEGACY"
		if fatal(issue.Class) {
			label = "FAIL"
		}
		fmt.Printf("%s %s.%s (sealed payload) pk=%s class=%s action=%s\n",
			label, issue.Table, issue.Column, issue.PK, issue.Class, crypto.SealedPayloadAction(issue.Class))
	}
}

// encryptProviderSecretsWithDB is the provider-only form of the generic
// backfill. It keeps the historical command name while sharing the scan,
// statistics and exit-code semantics with backfill-encrypted-columns.
func encryptProviderSecretsWithDB(ctx context.Context, sqlDB *sql.DB, encryptor *crypto.FieldEncryptor, opts crypto.ProviderSecretBackfillOptions) int {
	return backfillColumnsWithDB(ctx, sqlDB, encryptor, crypto.ProviderCredentialColumns, nil, opts, "provider credentials")
}

// backfillEncryptedColumnsWithDB runs the generic backfill over a registry and
// returns the process exit code. Production passes DefaultEncryptedColumns plus
// the v1 sealed-payload registry for verify/dry-run coverage; tests use narrow
// slices to exercise one column's SQL.
func backfillEncryptedColumnsWithDB(ctx context.Context, sqlDB *sql.DB, encryptor *crypto.FieldEncryptor, columns []crypto.EncryptedColumn, sealed []crypto.SealedPayloadColumn, opts crypto.EncryptedColumnBackfillOptions) int {
	return backfillColumnsWithDB(ctx, sqlDB, encryptor, columns, sealed, opts, "encrypted columns")
}

// backfillColumnsWithDB is the shared body of both backfill commands. A failed
// row is counted and the batch continues; only a failed scan of the column
// itself aborts.
func backfillColumnsWithDB(ctx context.Context, sqlDB *sql.DB, encryptor *crypto.FieldEncryptor, columns []crypto.EncryptedColumn, sealed []crypto.SealedPayloadColumn, opts crypto.EncryptedColumnBackfillOptions, subject string) int {
	// The operator job reads and writes every tenant's rows; without an
	// explicit system scope the RLS policies deny the scan.
	ctx = tenantctx.System(ctx)

	if opts.Verify || opts.DryRun {
		return scanBackfillReadOnly(ctx, sqlDB, encryptor, columns, sealed, opts, subject)
	}

	// The written pass seals columns as v2 (AAD-bound). Sealed payloads are
	// deliberately excluded: the application reads them with DecryptField, so a
	// v2 token would be unreadable. `migrate reencrypt` re-seals them as v1;
	// -verify/-dry-run classify them so plaintext is never silently skipped.
	stats, _, err := crypto.BackfillEncryptedColumns(ctx, sqlDB, encryptor, columns, opts)
	var selected, encrypted, skipped, failed, v1Legacy, v0Legacy int64
	var failures []crypto.EncryptedPayloadFailure
	truncated := false
	for _, stat := range stats {
		selected += stat.Scanned
		encrypted += stat.Encrypted
		skipped += stat.Skipped
		failed += stat.Failed
		v1Legacy += stat.LegacyV1
		v0Legacy += stat.LegacyV0
		failures = append(failures, stat.Failures...)
		truncated = truncated || stat.FailuresTruncated
		fmt.Printf("%s.%s: selected=%d encrypted=%d skipped=%d failed=%d legacy-v1=%d legacy-v0=%d\n",
			stat.Table, stat.Column, stat.Scanned, stat.Encrypted, stat.Skipped, stat.Failed, stat.LegacyV1, stat.LegacyV0)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "backfill %s: %v\n", subject, err)
		return 1
	}
	shown, queueTruncated := printFailureQueue(failures, truncated, opts.MaxFailures)
	fmt.Printf("summary: selected=%d encrypted=%d skipped=%d failed=%d legacy-v1=%d legacy-v0=%d columns=%d failures_shown=%d truncated=%t\n",
		selected, encrypted, skipped, failed, v1Legacy, v0Legacy, len(stats), shown, queueTruncated)
	if failed > 0 {
		fmt.Fprintf(os.Stderr, "%d row(s) failed to encrypt; fix the listed keys and re-run\n", failed)
		return 1
	}
	if v1Legacy+v0Legacy > 0 {
		fmt.Printf("note: %d legacy envelope(s) were left untouched; run `migrate reencrypt` to upgrade them\n", v1Legacy+v0Legacy)
	}
	return 0
}

// scanBackfillReadOnly classifies every row without writing, so -verify and
// -dry-run report v1/v0 legacy envelopes separately from plaintext and cannot
// be mistaken for the written pass. Sealed payload columns are classified here
// too: plaintext in them is a failure even though this command never writes
// them (their terminal generation is v1, written by `migrate reencrypt`).
func scanBackfillReadOnly(ctx context.Context, sqlDB *sql.DB, encryptor *crypto.FieldEncryptor, columns []crypto.EncryptedColumn, sealed []crypto.SealedPayloadColumn, opts crypto.EncryptedColumnBackfillOptions, subject string) int {
	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = 200
	}
	source := bootstrap.NewPayloadBatchSource(sqlDB)
	report, err := crypto.ScanEncryptedPayloads(ctx, source, encryptor, columns, crypto.EncryptedPayloadScanOptions{
		BatchSize:         batchSize,
		MaxIssues:         opts.MaxFailures,
		QueueOnlyFailures: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "backfill %s: %v\n", subject, err)
		return 1
	}
	var sealedReport crypto.EncryptedPayloadScanReport
	if len(sealed) > 0 {
		sealedReport, err = crypto.ScanSealedPayloads(ctx, source, encryptor, sealed, crypto.EncryptedPayloadScanOptions{
			BatchSize:         batchSize,
			MaxIssues:         opts.MaxFailures,
			QueueOnlyFailures: true,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "backfill %s: %v\n", subject, err)
			return 1
		}
	}

	printPayloadScanColumns(report.Columns)
	if len(sealed) > 0 {
		printSealedPayloadScanColumns(sealedReport.Columns)
	}
	scanned, skipped, current, stale, v1Legacy, v0Legacy, unrecognized, unreadable := payloadScanTotals(report)
	sealedScanned, sealedSkipped, _, _, sealedV1, sealedV0, sealedUnrecognized, sealedUnreadable := payloadScanTotals(sealedReport)
	if opts.DryRun {
		fmt.Printf("summary: scanned=%d skipped=%d would-encrypt=%d current-v2=%d legacy-v1=%d legacy-v0=%d stale-v2=%d unreadable=%d (dry-run, no rows written)\n",
			scanned, skipped, unrecognized, current, v1Legacy, v0Legacy, stale, unreadable)
		printClassAdvice(v1Legacy, v0Legacy, unrecognized, unreadable,
			"run without -dry-run to seal them (restore corrupt ciphertext first)")
		printBackfillSealedDryRun(sealedScanned, sealedSkipped, sealedV1, sealedV0, sealedUnrecognized, sealedUnreadable)
		return 0
	}

	printPayloadIssues(report, func(class crypto.EncryptedPayloadClass) bool {
		return class.NeedsBackfill() || class.IsBroken()
	})
	if len(sealed) > 0 {
		printSealedPayloadIssues(sealedReport, func(class crypto.EncryptedPayloadClass) bool {
			return class.NeedsBackfill() || class.IsBroken()
		})
	}
	failures := unrecognized + unreadable
	sealedFailures := sealedUnrecognized + sealedUnreadable
	legacy := stale + v1Legacy + v0Legacy
	fmt.Printf("summary: scanned=%d skipped=%d unrecognized=%d legacy-v1=%d legacy-v0=%d stale-v2=%d unreadable=%d failures=%d shown=%d truncated=%t\n",
		scanned, skipped, unrecognized, v1Legacy, v0Legacy, stale, unreadable, failures, len(report.Issues), report.IssuesTruncated)
	if len(sealed) > 0 {
		fmt.Printf("sealed payload summary: scanned=%d skipped=%d legacy-v1=%d legacy-v0=%d unrecognized=%d unreadable=%d failures=%d shown=%d truncated=%t\n",
			sealedScanned, sealedSkipped, sealedV1, sealedV0, sealedUnrecognized, sealedUnreadable,
			sealedFailures, len(sealedReport.Issues), sealedReport.IssuesTruncated)
	}
	if report.IssuesTruncated || sealedReport.IssuesTruncated {
		fmt.Fprintf(os.Stderr, "failure queue truncated at %d; fix the listed keys first or raise -max-failures\n", len(report.Issues)+len(sealedReport.Issues))
	}
	if failures+sealedFailures > 0 {
		if sealedFailures > 0 {
			fmt.Fprintf(os.Stderr, "unrecognized payload(s) remain in %s (including sealed payload columns); re-seal sealed payloads as v1 manually or restore corrupt ciphertext from backup\n", subject)
		} else {
			fmt.Fprintf(os.Stderr, "unrecognized payload(s) remain in %s; run without -verify to seal plaintext, or restore corrupt ciphertext from backup\n", subject)
		}
		return 1
	}
	if legacy+sealedV0 > 0 {
		fmt.Printf("verify OK: no plaintext %s found; %d legacy/stale envelope(s) remain — run `migrate reencrypt` to upgrade them\n", subject, legacy+sealedV0)
		return 0
	}
	fmt.Printf("verify OK: no plaintext %s found\n", subject)
	return 0
}

func printBackfillSealedDryRun(scanned, skipped, v1, v0, unrecognized, unreadable int64) {
	if scanned == 0 && skipped == 0 && v1 == 0 && v0 == 0 && unrecognized == 0 && unreadable == 0 {
		return
	}
	fmt.Printf("sealed payload summary: scanned=%d skipped=%d legacy-v1=%d legacy-v0=%d unrecognized=%d unreadable=%d (dry-run, no rows written)\n",
		scanned, skipped, v1, v0, unrecognized, unreadable)
	if unrecognized+unreadable > 0 {
		fmt.Printf("note: %d sealed-payload value(s) are not readable envelopes; this backfill does not seal them (they must stay v1) — restore or re-seal manually\n", unrecognized+unreadable)
	}
	if v0 > 0 {
		fmt.Printf("note: %d v0 sealed payload(s) are encrypted; run `migrate reencrypt` to re-seal them as v1\n", v0)
	}
}

func printPayloadScanColumns(columns []crypto.EncryptedPayloadColumnStat) {
	for _, stat := range columns {
		fmt.Printf("%s.%s: scanned=%d skipped=%d current-v2=%d stale-v2=%d legacy-v1=%d legacy-v0=%d unrecognized=%d unreadable=%d\n",
			stat.Table, stat.Column, stat.Scanned, stat.Skipped, stat.V2Current, stat.V2Stale,
			stat.V1Legacy, stat.V0Legacy, stat.Unrecognized, stat.Unreadable)
	}
}

func payloadScanTotals(report crypto.EncryptedPayloadScanReport) (scanned, skipped, current, stale, v1Legacy, v0Legacy, unrecognized, unreadable int64) {
	for _, stat := range report.Columns {
		scanned += stat.Scanned
		skipped += stat.Skipped
		current += stat.V2Current
		stale += stat.V2Stale
		v1Legacy += stat.V1Legacy
		v0Legacy += stat.V0Legacy
		unrecognized += stat.Unrecognized
		unreadable += stat.Unreadable
	}
	return
}

// printPayloadIssues prints the stable primary-key queue. A fatal class uses
// FAIL; a legacy envelope uses LEGACY and is advisory for the command currently
// running. PK values go to operator stdout only; structured slog output still
// uses the ADR-015 fingerprint.
func printPayloadIssues(report crypto.EncryptedPayloadScanReport, fatal func(crypto.EncryptedPayloadClass) bool) {
	for _, issue := range report.Issues {
		label := "LEGACY"
		if fatal(issue.Class) {
			label = "FAIL"
		}
		fmt.Printf("%s %s.%s pk=%s class=%s action=%s\n",
			label, issue.Table, issue.Column, issue.PK, issue.Class, issue.Class.Action())
	}
}

func printFailureQueue(failures []crypto.EncryptedPayloadFailure, truncated bool, maxFailures int) (shown int, queueTruncated bool) {
	for _, failure := range failures {
		if maxFailures > 0 && shown >= maxFailures {
			truncated = true
			break
		}
		fmt.Printf("FAIL %s.%s pk=%s class=%s stage=%s action=%s\n",
			failure.Table, failure.Column, failure.PK, failure.Class, failure.Stage, failureAction(failure))
		shown++
	}
	if truncated {
		fmt.Fprintln(os.Stderr, "failure queue truncated; raise -max-failures to print more keys")
	}
	return shown, truncated
}

func failureAction(failure crypto.EncryptedPayloadFailure) string {
	switch failure.Stage {
	case "encrypt":
		return "encryption failed; check the KEK and the row, then re-run"
	case "update":
		return "the row was not committed; re-run to retry it"
	case "reseal":
		return "sealed payloads are re-sealed as v1 only; restore or fix the row, then re-run `migrate reencrypt`"
	case "rotate":
		if failure.Class.NeedsRotation() {
			return "restore the matching KEK/algorithm if needed, then re-run `migrate reencrypt`"
		}
		return failure.Class.Action()
	default:
		return failure.Class.Action()
	}
}

func printClassAdvice(v1Legacy, v0Legacy, unrecognized, unreadable int64, unrecognizedAdvice string) {
	if v1Legacy+v0Legacy > 0 {
		fmt.Printf("note: %d legacy v1/v0 envelope(s) need rotation; run `migrate reencrypt` to upgrade them\n", v1Legacy+v0Legacy)
	}
	if unrecognized > 0 {
		fmt.Printf("note: %d value(s) are not envelope tokens; %s\n", unrecognized, unrecognizedAdvice)
	}
	if unreadable > 0 {
		fmt.Printf("note: %d v2 envelope(s) cannot be opened with the configured KEKs; restore the key or the row\n", unreadable)
	}
}

func parseFlags(fs *flag.FlagSet, argv []string) bool {
	if err := fs.Parse(argv); err != nil {
		return false
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "unexpected arguments %v: the command must come first, flags after\n", fs.Args())
		return false
	}
	return true
}

func resolveDSN(value string) string {
	if value != "" {
		return value
	}
	if env := os.Getenv("DATABASE_URL"); env != "" {
		return env
	}
	return os.Getenv("POSTGRES_DSN")
}

func openPool(ctx context.Context, dsn string) (pool *pgxpool.Pool, code int) {
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "no DSN: pass -dsn or set DATABASE_URL/POSTGRES_DSN")
		return nil, 2
	}
	pool, err := db.NewPostgresPool(ctx, db.PostgresConfig{DataSourceName: dsn, MaxConns: 2})
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		return nil, 1
	}
	return pool, 0
}

// status prints the ledger so an operator can answer "which structure version is
// this database at" — the question the ledger exists for.
func status(ctx context.Context, pool *pgxpool.Pool) int {
	applied, err := db.Applied(ctx, pool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read ledger: %v\n", err)
		return 1
	}
	if len(applied) == 0 {
		fmt.Println("no migrations applied")
		return 0
	}
	for _, a := range applied {
		fmt.Println(formatLedgerLine(a))
	}
	return 0
}

// formatLedgerLine renders one ledger row.
//
// The layout is time.RFC3339 rather than a hand-written literal: in a literal
// like "2026-09-01T15:04:05Z07:00" Go sees no reference year and treats the
// leading digits as text, so operators were shown years such as 28286.
func formatLedgerLine(a db.AppliedMigration) string {
	return fmt.Sprintf("applied %s V%d %s checksum=%s at=%s",
		a.Kind, a.Version, a.Name, a.Checksum, a.AppliedAt.Format(time.RFC3339))
}
