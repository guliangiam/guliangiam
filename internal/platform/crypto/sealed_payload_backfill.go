package crypto

// Sealed-payload backfill for the v1 registry (llm_providers.api_key_sealed and
// pay_channel_configs.config_sealed).
//
// The application write paths still seal these columns with EncryptField (v1,
// no AAD). The backfill therefore writes the same generation: a v2 token in a
// sealed column is readable through DecryptColumn(row AAD) but not through the
// remaining DecryptField readers, so this command never upgrades a sealed value.
// Instead it:
//
//   - classifies every non-empty value with the AAD-aware classifier, so a v2
//     token is authenticated against the registered (table|column|pk) AAD;
//   - seals only values that are not envelope tokens (plaintext candidates);
//   - skips readable v1/v0/v2 values unless -force re-seals a readable v1/v0 at
//     the current KEK (v2 is never downgraded);
//   - reports plaintext, AAD mismatches and unreadable envelopes without ever
//     printing a stored plaintext or ciphertext value.
//
// The storage port owns both the keyset scan and the compare-and-swap UPDATE, so
// this package contains no SQL. The registry is the only input: adding a sealed
// payload column to SealedPayloadColumns extends the command automatically.

import (
	"context"
	"errors"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/pagination"
)

// SealedPayloadBackfillOptions control one table-driven run.
//
// MaxFailures bounds both the printed queue and the tolerated row failures in
// the written pass (0 = unlimited), preserving the historical
// `encrypt-sealed-payloads` semantics. In -verify/-dry-run it only bounds the
// printed queue.
type SealedPayloadBackfillOptions struct {
	BatchSize   int
	DryRun      bool
	Verify      bool
	Force       bool
	MaxFailures int
}

// SealedPayloadBackfillFailure is one row the run could not seal or verify.
// Only identifiers, the classifier result and a reason are retained; the stored
// value (plaintext or ciphertext) is never copied into the report.
type SealedPayloadBackfillFailure struct {
	Table  string
	Column string
	PK     string
	Class  EncryptedPayloadClass
	Stage  string // "encrypt", "reseal", "update" or "verify"
	Reason string // stable operator-facing reason
}

// SealedPayloadBackfillStat is the per-column result.
//
// In the written pass Selected/Encrypted/Resealed/Skipped/Failed are the flow
// counters. In -dry-run Selected is the number of plaintext candidates and
// Resealed the number of readable v1/v0 values that -force would re-seal. In
// -verify Plaintext/AADMismatch/Unreadable are the failure counters and
// LegacyV1/LegacyV0/V2Current/V2Stale are the readable values.
type SealedPayloadBackfillStat struct {
	Table  string
	Column string

	Scanned   int64
	Selected  int64
	Encrypted int64
	Resealed  int64
	Skipped   int64
	Failed    int64

	LegacyV1    int64
	LegacyV0    int64
	V2Current   int64
	V2Stale     int64
	AADMismatch int64
	Unreadable  int64
	Plaintext   int64

	Duration          time.Duration
	Failures          []SealedPayloadBackfillFailure
	FailuresTruncated bool
}

// SealedPayloadBackfillReport aggregates the finished columns. A scan error
// aborts the run but completed work stays in the report.
type SealedPayloadBackfillReport struct {
	Columns  []SealedPayloadBackfillStat
	Duration time.Duration
}

// BackfillSealedPayloads seals plaintext values in the supplied sealed-payload
// registry entries, or classifies them without writing when DryRun/Verify is set.
// The command must supply a context carrying the system tenant scope; the port
// implementation is responsible for FORCE RLS bypass.
func BackfillSealedPayloads(ctx context.Context, source SealedPayloadBackfillSource, encryptor *FieldEncryptor, columns []SealedPayloadColumn, opts SealedPayloadBackfillOptions) (SealedPayloadBackfillReport, error) {
	var report SealedPayloadBackfillReport
	if encryptor == nil {
		return report, apperrors.NewInternal("sealed payload backfill requires an encryptor")
	}
	if source == nil {
		return report, apperrors.NewInternal("sealed payload backfill requires a payload source")
	}
	if opts.DryRun && opts.Verify {
		return report, apperrors.NewInvalidArgument("dry-run and verify are mutually exclusive")
	}
	if opts.Force && opts.Verify {
		return report, apperrors.NewInvalidArgument("force cannot be combined with verify")
	}
	// Reuse the single pagination boundary instead of defining a second scan
	// limit: a non-positive size becomes the default page size and an oversized
	// one is capped like every repository call.
	opts.BatchSize = pagination.ClampPageSize(opts.BatchSize)

	start := time.Now()
	for _, col := range columns {
		if err := col.Validate(); err != nil {
			report.Duration = time.Since(start)
			return report, err
		}
		stat := SealedPayloadBackfillStat{Table: col.Table, Column: col.Column}
		columnStart := time.Now()

		var err error
		switch {
		case opts.Verify:
			err = verifySealedPayloadColumn(ctx, source, encryptor, col, opts, &stat)
		case opts.DryRun:
			err = dryRunSealedPayloadColumn(ctx, source, encryptor, col, opts, &stat)
		default:
			err = writeSealedPayloadColumn(ctx, source, encryptor, col, opts, &stat)
		}

		stat.Duration = time.Since(columnStart)
		report.Columns = append(report.Columns, stat)
		if err != nil {
			report.Duration = time.Since(start)
			return report, err
		}
	}
	report.Duration = time.Since(start)
	return report, nil
}

func walkSealedPayloadColumn(ctx context.Context, source SealedPayloadBackfillSource, col SealedPayloadColumn, batchSize int, visit func(PayloadRow)) error {
	cursor := ""
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		rows, err := fetchPayloadBatch(ctx, source, col.Encrypted(), cursor, batchSize)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for _, row := range rows {
			visit(row)
		}
		cursor = rows[len(rows)-1].PK
		if len(rows) < batchSize {
			return nil
		}
	}
}

func writeSealedPayloadColumn(ctx context.Context, source SealedPayloadBackfillSource, encryptor *FieldEncryptor, col SealedPayloadColumn, opts SealedPayloadBackfillOptions, stat *SealedPayloadBackfillStat) error {
	return walkSealedPayloadColumn(ctx, source, col, opts.BatchSize, func(row PayloadRow) {
		stat.Scanned++
		if !row.Value.Valid || row.Value.String == "" {
			stat.Skipped++
			return
		}

		class, classErr := ClassifyEncryptedPayload(encryptor, col.Encrypted(), row.PK, row.Value.String)
		switch class {
		case PayloadClassV2Current:
			stat.Skipped++
			stat.V2Current++
		case PayloadClassV2Stale:
			// The AAD matched, only the KEK/algorithm is older. Re-sealing as v1
			// would downgrade an authenticated token, so leave it to the v2
			// rotation path.
			stat.Skipped++
			stat.V2Stale++
		case PayloadClassV1Legacy:
			if _, err := encryptor.DecryptColumn(col.AAD(row.PK), row.Value.String); err != nil {
				stat.Failed++
				stat.Unreadable++
				recordSealedPayloadFailure(stat, col, row.PK, class, "verify", "decrypt_failed", opts.MaxFailures)
				return
			}
			stat.LegacyV1++
			if !opts.Force {
				stat.Skipped++
				return
			}
			resealSealedPayloadRow(ctx, source, encryptor, col, row, class, opts.MaxFailures, stat)
		case PayloadClassV0Legacy:
			stat.LegacyV0++
			if !opts.Force {
				stat.Skipped++
				return
			}
			resealSealedPayloadRow(ctx, source, encryptor, col, row, class, opts.MaxFailures, stat)
		case PayloadClassUnrecognized:
			if !errors.Is(classErr, ErrNotEnvelope) {
				// A classifier contract violation must never be treated as
				// plaintext; report it as unreadable instead.
				stat.Failed++
				stat.Unreadable++
				recordSealedPayloadFailure(stat, col, row.PK, class, "verify", "classifier_contract", opts.MaxFailures)
				return
			}
			stat.Selected++
			stat.Plaintext++
			sealed, err := encryptor.EncryptField(row.Value.String)
			if err != nil {
				stat.Failed++
				recordSealedPayloadFailure(stat, col, row.PK, class, "encrypt", "encrypt_failed", opts.MaxFailures)
				return
			}
			affected, err := source.UpdateSealedPayload(ctx, SealedPayloadWrite{
				Column: col, PK: row.PK, Expected: row.Value.String, Sealed: sealed,
			})
			if err != nil {
				stat.Failed++
				recordSealedPayloadFailure(stat, col, row.PK, class, "update", "update_failed", opts.MaxFailures)
				return
			}
			if affected == 0 {
				// A concurrent writer replaced the value between the scan and the
				// compare-and-swap; never overwrite newer data.
				stat.Skipped++
				return
			}
			stat.Encrypted++
		case PayloadClassUnreadable:
			reason := "decrypt_failed"
			if errors.Is(classErr, ErrAADMismatch) {
				reason = "aad_mismatch"
				stat.AADMismatch++
			}
			stat.Failed++
			stat.Unreadable++
			recordSealedPayloadFailure(stat, col, row.PK, class, "verify", reason, opts.MaxFailures)
		default:
			stat.Skipped++
		}
	})
}

func resealSealedPayloadRow(ctx context.Context, source SealedPayloadBackfillSource, encryptor *FieldEncryptor, col SealedPayloadColumn, row PayloadRow, class EncryptedPayloadClass, maxFailures int, stat *SealedPayloadBackfillStat) {
	resealed, changed, err := encryptor.RotateSealedPayload(row.Value.String)
	if err != nil {
		stat.Failed++
		recordSealedPayloadFailure(stat, col, row.PK, class, "reseal", "reseal_failed", maxFailures)
		return
	}
	if !changed {
		stat.Skipped++
		return
	}
	affected, err := source.UpdateSealedPayload(ctx, SealedPayloadWrite{
		Column: col, PK: row.PK, Expected: row.Value.String, Sealed: resealed,
	})
	if err != nil {
		stat.Failed++
		recordSealedPayloadFailure(stat, col, row.PK, class, "update", "update_failed", maxFailures)
		return
	}
	if affected == 0 {
		stat.Skipped++
		return
	}
	stat.Resealed++
}

func dryRunSealedPayloadColumn(ctx context.Context, source SealedPayloadBackfillSource, encryptor *FieldEncryptor, col SealedPayloadColumn, opts SealedPayloadBackfillOptions, stat *SealedPayloadBackfillStat) error {
	return walkSealedPayloadColumn(ctx, source, col, opts.BatchSize, func(row PayloadRow) {
		stat.Scanned++
		if !row.Value.Valid || row.Value.String == "" {
			stat.Skipped++
			return
		}

		class, classErr := ClassifyEncryptedPayload(encryptor, col.Encrypted(), row.PK, row.Value.String)
		switch class {
		case PayloadClassV2Current:
			stat.V2Current++
		case PayloadClassV2Stale:
			stat.V2Stale++
		case PayloadClassV1Legacy:
			if _, err := encryptor.DecryptColumn(col.AAD(row.PK), row.Value.String); err != nil {
				stat.Unreadable++
				recordSealedPayloadFailure(stat, col, row.PK, class, "verify", "decrypt_failed", opts.MaxFailures)
				return
			}
			stat.LegacyV1++
			countWouldReseal(encryptor, col, row, class, opts, stat)
		case PayloadClassV0Legacy:
			stat.LegacyV0++
			countWouldReseal(encryptor, col, row, class, opts, stat)
		case PayloadClassUnrecognized:
			if !errors.Is(classErr, ErrNotEnvelope) {
				stat.Unreadable++
				recordSealedPayloadFailure(stat, col, row.PK, class, "verify", "classifier_contract", opts.MaxFailures)
				return
			}
			stat.Selected++
			stat.Plaintext++
			recordSealedPayloadFailure(stat, col, row.PK, class, "encrypt", "plaintext_candidate", opts.MaxFailures)
		case PayloadClassUnreadable:
			reason := "decrypt_failed"
			if errors.Is(classErr, ErrAADMismatch) {
				reason = "aad_mismatch"
				stat.AADMismatch++
			}
			stat.Unreadable++
			recordSealedPayloadFailure(stat, col, row.PK, class, "verify", reason, opts.MaxFailures)
		default:
			stat.Skipped++
		}
	})
}

func countWouldReseal(encryptor *FieldEncryptor, col SealedPayloadColumn, row PayloadRow, class EncryptedPayloadClass, opts SealedPayloadBackfillOptions, stat *SealedPayloadBackfillStat) {
	if !opts.Force {
		stat.Skipped++
		return
	}
	_, changed, err := encryptor.RotateSealedPayload(row.Value.String)
	if err != nil {
		stat.Unreadable++
		recordSealedPayloadFailure(stat, col, row.PK, class, "reseal", "reseal_failed", opts.MaxFailures)
		return
	}
	if !changed {
		stat.Skipped++
		return
	}
	stat.Resealed++
	recordSealedPayloadFailure(stat, col, row.PK, class, "reseal", "reseal_candidate", opts.MaxFailures)
}

func verifySealedPayloadColumn(ctx context.Context, source SealedPayloadBackfillSource, encryptor *FieldEncryptor, col SealedPayloadColumn, opts SealedPayloadBackfillOptions, stat *SealedPayloadBackfillStat) error {
	return walkSealedPayloadColumn(ctx, source, col, opts.BatchSize, func(row PayloadRow) {
		stat.Scanned++
		if !row.Value.Valid || row.Value.String == "" {
			stat.Skipped++
			return
		}

		class, classErr := ClassifyEncryptedPayload(encryptor, col.Encrypted(), row.PK, row.Value.String)
		switch class {
		case PayloadClassV2Current:
			// The classifier authenticated the v2 token against the registered
			// (table|column|pk) AAD before reporting current.
			stat.V2Current++
		case PayloadClassV2Stale:
			stat.V2Stale++
		case PayloadClassV1Legacy:
			// v1 has no AAD by design; verifying it means proving it still decrypts.
			if _, err := encryptor.DecryptColumn(col.AAD(row.PK), row.Value.String); err != nil {
				stat.Unreadable++
				recordSealedPayloadFailure(stat, col, row.PK, class, "verify", "decrypt_failed", opts.MaxFailures)
				return
			}
			stat.LegacyV1++
		case PayloadClassV0Legacy:
			if _, err := encryptor.DecryptColumn(col.AAD(row.PK), row.Value.String); err != nil {
				stat.Unreadable++
				recordSealedPayloadFailure(stat, col, row.PK, class, "verify", "decrypt_failed", opts.MaxFailures)
				return
			}
			stat.LegacyV0++
		case PayloadClassUnrecognized:
			stat.Plaintext++
			recordSealedPayloadFailure(stat, col, row.PK, class, "verify", "plaintext_remaining", opts.MaxFailures)
		case PayloadClassUnreadable:
			reason := "decrypt_failed"
			if errors.Is(classErr, ErrAADMismatch) {
				reason = "aad_mismatch"
				stat.AADMismatch++
			}
			stat.Unreadable++
			recordSealedPayloadFailure(stat, col, row.PK, class, "verify", reason, opts.MaxFailures)
		default:
			stat.Skipped++
		}
	})
}

func recordSealedPayloadFailure(stat *SealedPayloadBackfillStat, col SealedPayloadColumn, pk string, class EncryptedPayloadClass, stage, reason string, maxFailures int) {
	if maxFailures > 0 && len(stat.Failures) >= maxFailures {
		stat.FailuresTruncated = true
		return
	}
	stat.Failures = append(stat.Failures, SealedPayloadBackfillFailure{
		Table: col.Table, Column: col.Column, PK: pk, Class: class, Stage: stage, Reason: reason,
	})
}
