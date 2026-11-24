package crypto

import (
	"context"
	"errors"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ErrSealedPayloadV2 reports a v2 (AAD-bound) token found by the legacy
// sealed-payload rotation/verify path. That path still uses DecryptField ->
// EncryptField and cannot open a v2 token, so rotation refuses to rewrite it
// instead of silently storing an unreadable value. The F2-01c Batch 1 read
// paths use DecryptColumn instead and do open v2.
var ErrSealedPayloadV2 = errors.New("crypto: sealed payload column contains a v2 (AAD-bound) token")

// SealedPayloadColumn describes a non-column payload sealed with EncryptField
// (v1, no AAD) inside a table column. It is deliberately a distinct type from
// EncryptedColumn: rotation uses DecryptField -> EncryptField and must never
// call EncryptColumn. F2-01c Batch 1 already reads the two registered columns
// with DecryptColumn(row AAD), so a v2 token is readable there, but writes and
// rotation stay v1 until Batch 2 moves them into DefaultEncryptedColumns.
//
// SealedPayloadColumns is registered separately from DefaultEncryptedColumns
// (which is the AAD-bound v2 column registry) so `migrate reencrypt` can reach
// these values without upgrading their generation.
type SealedPayloadColumn struct {
	Table    string // fully-qualified table, e.g. "guliangiam.llm_providers"
	Column   string // sealed payload column name, e.g. "api_key_sealed"
	PKColumn string // primary key column name, e.g. "id"
}

// Validate returns nil iff all three identifiers are safe for SQL construction.
func (c SealedPayloadColumn) Validate() error {
	return validateIdentifiers(c.Table, c.Column, c.PKColumn)
}

// Encrypted returns the AAD-bound view of this sealed payload column. The AAD
// is derived from the registered (table, sealed column, primary key) triple, so
// a v2 token found in the column is authenticated against exactly the same
// binding as LegacySealedColumn.AAD and the application read paths. Callers must
// never hand-build an AADContext for a sealed payload.
func (c SealedPayloadColumn) Encrypted() EncryptedColumn {
	return EncryptedColumn{Table: c.Table, Column: c.Column, PKColumn: c.PKColumn}
}

// AAD returns the row binding used to authenticate a v2 token in this sealed
// payload column. It deliberately delegates to LegacySealedColumn.AAD, the
// single table|column|pk constructor used by the legacy PII backfill, so a
// sealed-payload write/verify can never invent a different AAD shape. The
// sealed column is the target column; the plaintext source is never part of the
// binding.
func (c SealedPayloadColumn) AAD(pk string) AADContext {
	return LegacySealedColumn{Table: c.Table, TargetColumn: c.Column, PKColumn: c.PKColumn}.AAD(pk)
}

// SealedPayloadColumns lists every stored value that is still written with
// EncryptField (v1 envelope, no ADR-015 AAD). These values have row identity;
// F2-01c Batch 1 reads them with DecryptColumn(row AAD), while writes and
// rotation stay v1 until Batch 2 moves them into DefaultEncryptedColumns.
//
// llm_providers.api_key_sealed is written by the AI gateway admin service and
// pay_channel_configs.config_sealed by the native payment provider service.
// Both are registered here so a KEK rotation re-seals them as v1; keep them out
// of DefaultEncryptedColumns until the v2 write/rotation batch.
var SealedPayloadColumns = []SealedPayloadColumn{
	{Table: "guliangiam.llm_providers", Column: "api_key_sealed", PKColumn: "id"},
	{Table: "guliangiam.pay_channel_configs", Column: "config_sealed", PKColumn: "id"},
}

// LLMProviderAPIKeySealedColumn and PayChannelConfigSealedColumn are the
// AAD-bound read views of the two sealed-payload registry entries above. F2-01c
// Batch 1 reads them with DecryptColumn(col.AAD(pk)), so a v2 token is
// authenticated against its row while existing v1/v0 values stay readable.
// They are not rotation entries: DefaultEncryptedColumns stays untouched until
// Batch 2, and TestSealedPayloadAAD_MatchesRegistry pins these identifiers to
// SealedPayloadColumns so the read paths cannot drift.
var (
	LLMProviderAPIKeySealedColumn = EncryptedColumn{Table: "guliangiam.llm_providers", Column: "api_key_sealed", PKColumn: "id"}
	PayChannelConfigSealedColumn  = EncryptedColumn{Table: "guliangiam.pay_channel_configs", Column: "config_sealed", PKColumn: "id"}
)

// ClassifySealedPayload reports the on-disk generation of a sealed payload.
// Unlike ClassifyEncryptedPayload it never supplies an AAD context: a v2 token
// is reported as unrecognized (with ErrSealedPayloadV2) because DecryptField
// cannot open it, never as current/stale v2. The F2-01c Batch 1 read-only
// probe instead uses ClassifyEncryptedPayload with the row AAD above.
func ClassifySealedPayload(encryptor *FieldEncryptor, value string) (EncryptedPayloadClass, error) {
	if value == "" {
		return PayloadClassSkipped, nil
	}
	if encryptor == nil {
		return PayloadClassUnreadable, apperrors.NewInternal("sealed payload classification requires an encryptor")
	}

	if isV2EnvelopeToken(value) {
		return PayloadClassUnrecognized, ErrSealedPayloadV2
	}
	if IsEnvelopeToken(value) {
		if _, err := encryptor.envelope.decryptEnvelope(value, nil); err != nil {
			return PayloadClassUnreadable, err
		}
		return PayloadClassV1Legacy, nil
	}
	if _, err := encryptor.envelope.decryptLegacy(value); err == nil {
		return PayloadClassV0Legacy, nil
	}
	return PayloadClassUnrecognized, ErrNotEnvelope
}

// SealedPayloadAction is the operator-facing repair suggestion for a sealed
// payload class. It never suggests the v2/AAD upgrade used for encrypted
// columns: these values must stay v1 so DecryptField keeps working.
func SealedPayloadAction(class EncryptedPayloadClass) string {
	switch class {
	case PayloadClassV1Legacy:
		return "readable v1 sealed payload; `migrate reencrypt` re-seals it as v1 at the current KEK (never v2)"
	case PayloadClassV0Legacy:
		return "run `migrate reencrypt` to re-seal the v0 token as a v1 envelope (no AAD)"
	case PayloadClassUnrecognized:
		return "value is not an envelope token; restore it or re-seal it manually as v1 (a v2 token cannot be read by DecryptField)"
	case PayloadClassUnreadable:
		return "restore the matching KEK version/algorithm or restore the row from backup; rotation cannot open it"
	default:
		return "none"
	}
}

// RotateSealedPayload re-seals a non-column payload at the current KEK and
// algorithm while keeping the v1 generation. A v2 token is refused with
// ErrSealedPayloadV2; it is never downgraded to v1 and never rewritten. An
// empty value stays empty ("not configured") and no ciphertext is produced.
func (e *FieldEncryptor) RotateSealedPayload(token string) (string, bool, error) {
	if token == "" {
		return "", false, nil
	}
	if isV2EnvelopeToken(token) {
		return "", false, ErrSealedPayloadV2
	}
	// A v1 token already at the current KEK/algorithm is as current as this API
	// can express; authenticate it before skipping so a tampered/unreadable
	// payload surfaces instead of being reported as current.
	if e.envelope.isCurrentKeyEnvelope(token) {
		if _, err := e.envelope.decryptEnvelope(token, nil); err != nil {
			return "", false, err
		}
		return token, false, nil
	}
	plaintext, err := e.DecryptField(token)
	if err != nil {
		return "", false, err
	}
	sealed, err := e.EncryptField(plaintext)
	if err != nil {
		return "", false, err
	}
	return sealed, true, nil
}

// ScanSealedPayloads walks every row of the given sealed payload columns in
// primary-key order and classifies it without writing. It mirrors
// ScanEncryptedPayloads (same storage port, report, caps and queue semantics)
// but has no AAD context, so a v2 token is counted as unrecognized instead of
// authenticated.
func ScanSealedPayloads(ctx context.Context, source PayloadBatchSource, encryptor *FieldEncryptor, columns []SealedPayloadColumn, opts EncryptedPayloadScanOptions) (EncryptedPayloadScanReport, error) {
	var report EncryptedPayloadScanReport
	if encryptor == nil {
		return report, apperrors.NewInternal("sealed payload scan requires an encryptor")
	}
	if source == nil {
		return report, apperrors.NewInternal("sealed payload scan requires a payload batch source")
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 500
	}

	for _, col := range columns {
		if err := col.Validate(); err != nil {
			return report, err
		}
		stat := EncryptedPayloadColumnStat{Table: col.Table, Column: col.Column}
		cursor := ""
		for {
			if err := ctx.Err(); err != nil {
				return report, err
			}
			rows, err := fetchPayloadBatch(ctx, source, EncryptedColumn{
				Table: col.Table, Column: col.Column, PKColumn: col.PKColumn,
			}, cursor, opts.BatchSize)
			if err != nil {
				return report, err
			}
			if len(rows) == 0 {
				break
			}

			for _, row := range rows {
				stat.Scanned++
				if !row.Value.Valid || row.Value.String == "" {
					stat.Skipped++
					continue
				}

				class, _ := ClassifySealedPayload(encryptor, row.Value.String)
				countPayloadClass(&stat, class)
				if class == PayloadClassV2Current || class == PayloadClassSkipped {
					continue
				}
				if opts.QueueOnlyFailures && !class.NeedsBackfill() && !class.IsBroken() {
					continue
				}
				report.IssueCount++
				if opts.MaxIssues <= 0 || len(report.Issues) < opts.MaxIssues {
					report.Issues = append(report.Issues, EncryptedPayloadIssue{
						Table: col.Table, Column: col.Column, PK: row.PK, Class: class,
					})
				} else {
					report.IssuesTruncated = true
				}
			}

			cursor = rows[len(rows)-1].PK
			if len(rows) < opts.BatchSize {
				break
			}
		}
		report.Columns = append(report.Columns, stat)
	}
	return report, nil
}
