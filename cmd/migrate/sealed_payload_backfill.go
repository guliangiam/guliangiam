package main

// Table-driven backfill for the v1 sealed-payload registry
// (crypto.SealedPayloadColumns: llm_providers.api_key_sealed and
// pay_channel_configs.config_sealed).
//
// The application write paths store these columns with EncryptField, i.e. a v1
// envelope without AAD. The generic A-6.2 backfill writes v2 (AAD-bound)
// envelopes, so it cannot be reused here: a v2 token would make the remaining
// DecryptField readers fail with ErrAADContext, and
// scripts/check-encrypted-columns.py rejects a column that is both a sealed
// payload and a v2 backfill target.
//
// Only values the AAD-aware classifier reports as "not an envelope"
// (crypto.ErrNotEnvelope) are selected. Empty/NULL values, readable v1
// envelopes, decryptable v0 tokens, AAD-authenticated v2 tokens and unreadable
// envelopes are left untouched; -force re-seals readable v1/v0 at the current
// KEK without ever downgrading v2. Every selected row is encrypted with the
// same EncryptField call as the write path and committed by a single-row
// compare-and-swap UPDATE. That statement is its own implicit transaction: no
// batch transaction is held open, a failed row does not roll back earlier rows,
// and an interrupted run can simply be re-run.
//
// -verify never writes. It decrypts every non-empty value with the registered
// (table|column|pk) AAD, so a v2 token that was moved to another row/column is
// reported as an AAD mismatch instead of silently passing.
//
// Output contains table/column names, primary keys and counters only; no key
// fragment, plaintext, ciphertext or value length is printed.

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/stdlib"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/cryptopayload"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

func runEncryptSealedPayloads(argv []string) int {
	ctx := context.Background()

	fs := flag.NewFlagSet("migrate encrypt-sealed-payloads", flag.ContinueOnError)
	dsn := fs.String("dsn", "", "PostgreSQL DSN (defaults to $DATABASE_URL, then $POSTGRES_DSN)")
	tables := fs.String("table", "", "comma-separated registered table filter (default: every sealed payload column)")
	batchSize := fs.Int("batch-size", pagination.MaxPageSize,
		fmt.Sprintf("rows per keyset page (1..%d; shared pagination bound)", pagination.MaxPageSize))
	dryRun := fs.Bool("dry-run", false, "count and list plaintext rows without writing")
	verify := fs.Bool("verify", false, "verify every sealed value decrypts with the registered AAD; never writes")
	force := fs.Bool("force", false, "re-seal readable v1/v0 values at the current KEK (never downgrades v2)")
	maxFailures := fs.Int("max-failures", defaultMaxFailures,
		"row failures tolerated before the run exits non-zero (0 = no limit); also bounds the printed queue")
	if !parseFlags(fs, argv) {
		return 2
	}
	if !pagination.ValidateQueryPageSize(*batchSize) {
		fmt.Fprintf(os.Stderr, "-batch-size must be between 1 and %d\n", pagination.MaxPageSize)
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
	if *force && *verify {
		fmt.Fprintln(os.Stderr, "-force cannot be combined with -verify (verify never writes)")
		return 2
	}

	columns, err := selectSealedPayloadColumns(*tables)
	if err != nil {
		fmt.Fprintf(os.Stderr, "encrypt sealed payloads: %v\n", err)
		return 2
	}

	// Fail before connecting: without a KEK every attempt would be a plaintext
	// write, and NewFieldEncryptor would panic instead of reporting it.
	if !crypto.FieldEncryptionConfigured() {
		fmt.Fprintln(os.Stderr, "ENCRYPTION_KEY_CURRENT (or legacy ENCRYPTION_KEY) is required; refusing to write sealed payloads")
		return 1
	}

	pool, code := openPool(ctx, resolveDSN(*dsn))
	if code != 0 {
		return code
	}
	defer pool.Close()

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	return encryptSealedPayloadsWithDB(ctx, sqlDB, crypto.GlobalFieldEncryptor(), columns, crypto.SealedPayloadBackfillOptions{
		BatchSize:   *batchSize,
		DryRun:      *dryRun,
		Verify:      *verify,
		Force:       *force,
		MaxFailures: *maxFailures,
	})
}

// selectSealedPayloadColumns resolves the -table filter against the registry.
// An empty filter means every registered column, so adding an entry to
// crypto.SealedPayloadColumns extends the command without touching this file.
func selectSealedPayloadColumns(raw string) ([]crypto.SealedPayloadColumn, error) {
	if strings.TrimSpace(raw) == "" {
		return crypto.SealedPayloadColumns, nil
	}

	wanted := map[string]struct{}{}
	for _, token := range strings.Split(raw, ",") {
		if name := normalizeSealedPayloadTable(token); name != "" {
			wanted[name] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return nil, apperrors.NewInvalidArgument("-table must name at least one registered sealed payload table")
	}

	matched := map[string]struct{}{}
	var columns []crypto.SealedPayloadColumn
	for _, col := range crypto.SealedPayloadColumns {
		name := normalizeSealedPayloadTable(col.Table)
		if _, ok := wanted[name]; ok {
			columns = append(columns, col)
			matched[name] = struct{}{}
		}
	}

	var unknown []string
	for name := range wanted {
		if _, ok := matched[name]; !ok {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, apperrors.NewInvalidArgumentf(
			"no registered sealed payload column matches -table %s", strings.Join(unknown, ","))
	}
	return columns, nil
}

func normalizeSealedPayloadTable(raw string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	return name
}

// encryptSealedPayloadsWithDB is the testable body. It prints counts only and
// never prints a stored plaintext or ciphertext value. The context carries the
// system scope used by operator jobs; the calling role must still bypass FORCE
// RLS.
func encryptSealedPayloadsWithDB(ctx context.Context, sqlDB *sql.DB, encryptor *crypto.FieldEncryptor, columns []crypto.SealedPayloadColumn, opts crypto.SealedPayloadBackfillOptions) int {
	ctx = tenantctx.System(ctx)

	source := cryptopayload.NewSource(sqlDB)
	report, err := crypto.BackfillSealedPayloads(ctx, source, encryptor, columns, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "encrypt sealed payloads: %v\n", err)
		return 1
	}

	switch {
	case opts.Verify:
		return printSealedPayloadVerify(report, opts)
	case opts.DryRun:
		return printSealedPayloadBackfillDryRun(report, opts)
	default:
		return printSealedPayloadWrite(report, opts)
	}
}

type sealedPayloadBackfillTotals struct {
	scanned, selected, encrypted, resealed, skipped, failed int64
	legacyV1, legacyV0, v2Current, v2Stale                  int64
	aadMismatch, unreadable, plaintext                      int64
}

func sumSealedPayloadBackfill(report crypto.SealedPayloadBackfillReport) sealedPayloadBackfillTotals {
	var total sealedPayloadBackfillTotals
	for _, stat := range report.Columns {
		total.scanned += stat.Scanned
		total.selected += stat.Selected
		total.encrypted += stat.Encrypted
		total.resealed += stat.Resealed
		total.skipped += stat.Skipped
		total.failed += stat.Failed
		total.legacyV1 += stat.LegacyV1
		total.legacyV0 += stat.LegacyV0
		total.v2Current += stat.V2Current
		total.v2Stale += stat.V2Stale
		total.aadMismatch += stat.AADMismatch
		total.unreadable += stat.Unreadable
		total.plaintext += stat.Plaintext
	}
	return total
}

func printSealedPayloadWrite(report crypto.SealedPayloadBackfillReport, opts crypto.SealedPayloadBackfillOptions) int {
	total := sumSealedPayloadBackfill(report)
	for _, stat := range report.Columns {
		fmt.Printf("%s.%s: scanned=%d selected=%d encrypted=%d resealed=%d skipped=%d failed=%d legacy_v1=%d legacy_v0=%d v2_current=%d v2_stale=%d aad_mismatch=%d unreadable=%d elapsed_ms=%d\n",
			stat.Table, stat.Column, stat.Scanned, stat.Selected, stat.Encrypted, stat.Resealed, stat.Skipped, stat.Failed,
			stat.LegacyV1, stat.LegacyV0, stat.V2Current, stat.V2Stale, stat.AADMismatch, stat.Unreadable,
			stat.Duration.Milliseconds())
	}
	fmt.Printf("summary: scanned=%d selected=%d encrypted=%d resealed=%d skipped=%d failed=%d legacy_v1=%d legacy_v0=%d v2_current=%d v2_stale=%d aad_mismatch=%d unreadable=%d columns=%d\n",
		total.scanned, total.selected, total.encrypted, total.resealed, total.skipped, total.failed,
		total.legacyV1, total.legacyV0, total.v2Current, total.v2Stale, total.aadMismatch, total.unreadable,
		len(report.Columns))
	printSealedPayloadFailures(report, "write", opts.MaxFailures)

	if total.failed > 0 {
		if opts.MaxFailures > 0 && total.failed > int64(opts.MaxFailures) {
			fmt.Fprintf(os.Stderr, "encrypt sealed payloads failed: %d row(s) exceed -max-failures=%d; re-run to retry\n",
				total.failed, opts.MaxFailures)
			return 1
		}
		fmt.Fprintf(os.Stderr, "%d row(s) failed but stayed within -max-failures=%d; re-run to retry\n",
			total.failed, opts.MaxFailures)
	}
	if total.encrypted+total.resealed == 0 && total.failed == 0 {
		fmt.Println("nothing to seal: every non-empty value is already a readable sealed payload")
	}
	if total.legacyV1+total.legacyV0 > 0 && !opts.Force {
		fmt.Printf("note: %d readable v1/v0 sealed payload(s) were skipped; pass -force to re-seal them at the current KEK\n",
			total.legacyV1+total.legacyV0)
	}
	return 0
}

func printSealedPayloadBackfillDryRun(report crypto.SealedPayloadBackfillReport, opts crypto.SealedPayloadBackfillOptions) int {
	total := sumSealedPayloadBackfill(report)
	for _, stat := range report.Columns {
		fmt.Printf("%s.%s: would_encrypt=%d would_reseal=%d skipped=%d legacy_v1=%d legacy_v0=%d v2_current=%d v2_stale=%d aad_mismatch=%d unreadable=%d elapsed_ms=%d (dry-run, no rows written)\n",
			stat.Table, stat.Column, stat.Selected, stat.Resealed, stat.Skipped,
			stat.LegacyV1, stat.LegacyV0, stat.V2Current, stat.V2Stale, stat.AADMismatch, stat.Unreadable,
			stat.Duration.Milliseconds())
	}
	fmt.Printf("dry-run summary: would_encrypt=%d would_reseal=%d skipped=%d legacy_v1=%d legacy_v0=%d v2_current=%d v2_stale=%d aad_mismatch=%d unreadable=%d (dry-run, no rows written)\n",
		total.selected, total.resealed, total.skipped,
		total.legacyV1, total.legacyV0, total.v2Current, total.v2Stale, total.aadMismatch, total.unreadable)
	printSealedPayloadFailures(report, "dry-run", opts.MaxFailures)

	if total.unreadable > 0 {
		fmt.Printf("note: %d sealed-payload value(s) are not readable envelopes; restore the matching KEK or the row (an AAD mismatch means the ciphertext was moved to another row/column)\n", total.unreadable)
	}
	if total.legacyV1+total.legacyV0 > 0 && !opts.Force {
		fmt.Printf("note: %d readable v1/v0 sealed payload(s) would be left untouched; pass -force to re-seal them at the current KEK\n",
			total.legacyV1+total.legacyV0)
	}
	return 0
}

func printSealedPayloadVerify(report crypto.SealedPayloadBackfillReport, opts crypto.SealedPayloadBackfillOptions) int {
	total := sumSealedPayloadBackfill(report)
	readable := total.legacyV1 + total.legacyV0 + total.v2Current + total.v2Stale
	for _, stat := range report.Columns {
		fmt.Printf("%s.%s: encrypted=%d plaintext_remaining=%d aad_mismatch=%d unreadable=%d legacy_v1=%d legacy_v0=%d v2_current=%d v2_stale=%d skipped=%d elapsed_ms=%d\n",
			stat.Table, stat.Column, stat.LegacyV1+stat.LegacyV0+stat.V2Current+stat.V2Stale,
			stat.Plaintext, stat.AADMismatch, stat.Unreadable,
			stat.LegacyV1, stat.LegacyV0, stat.V2Current, stat.V2Stale, stat.Skipped,
			stat.Duration.Milliseconds())
	}
	fmt.Printf("verify summary: encrypted=%d plaintext_remaining=%d aad_mismatch=%d unreadable=%d legacy_v1=%d legacy_v0=%d v2_current=%d v2_stale=%d skipped=%d\n",
		readable, total.plaintext, total.aadMismatch, total.unreadable,
		total.legacyV1, total.legacyV0, total.v2Current, total.v2Stale, total.skipped)
	printSealedPayloadFailures(report, "verify", opts.MaxFailures)

	if total.legacyV1+total.legacyV0 > 0 {
		fmt.Printf("note: %d readable v1/v0 sealed payload(s) carry no AAD by design; pass -force without -verify to re-seal them at the current KEK\n",
			total.legacyV1+total.legacyV0)
	}
	if total.plaintext+total.aadMismatch+total.unreadable > 0 {
		unreadableOnly := total.unreadable - total.aadMismatch
		fmt.Fprintf(os.Stderr, "verify failed: %d plaintext sealed payload(s), %d AAD mismatch(es) and %d other unreadable value(s) remain; run without -verify to seal plaintext, and restore/re-seal mismatched rows\n",
			total.plaintext, total.aadMismatch, unreadableOnly)
		return 1
	}
	fmt.Println("verify OK: no plaintext sealed payload remains and every v2 token matches the registered (table|column|pk) AAD")
	return 0
}

// printSealedPayloadFailures prints the stable queue. Dry-run labels rows it
// would change as WOULD-SEAL/WOULD-RESEAL; write and verify label every row
// FAIL. PK values go to operator stdout only; structured slog output still uses
// the ADR-015 fingerprint.
func printSealedPayloadFailures(report crypto.SealedPayloadBackfillReport, mode string, maxFailures int) {
	queued := 0
	truncated := false
	for _, stat := range report.Columns {
		queued += len(stat.Failures)
		truncated = truncated || stat.FailuresTruncated
	}

	shown := 0
	for _, stat := range report.Columns {
		for _, failure := range stat.Failures {
			if maxFailures > 0 && shown >= maxFailures {
				break
			}
			label := "FAIL"
			if mode == "dry-run" {
				switch failure.Reason {
				case "plaintext_candidate":
					label = "WOULD-SEAL"
				case "reseal_candidate":
					label = "WOULD-RESEAL"
				}
			}
			fmt.Printf("%s %s.%s pk=%s class=%s stage=%s reason=%s\n",
				label, failure.Table, failure.Column, failure.PK, failure.Class, failure.Stage, failure.Reason)
			shown++
		}
	}
	if truncated || (maxFailures > 0 && queued > maxFailures) {
		fmt.Fprintln(os.Stderr, "failure queue truncated; raise -max-failures to print more keys")
	}
}
