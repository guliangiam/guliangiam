package crypto

import (
	"context"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// EncryptedPayloadClass is the on-disk generation of one value found in a
// registered encrypted column. It exists so rotation and backfill verification
// can distinguish a stale-but-readable envelope from a value that is neither an
// envelope nor a decryptable legacy token. The latter must never be reported as
// "ciphertext" (it may be plaintext) nor silently encrypted a second time (it
// may be an old v0 token).
type EncryptedPayloadClass string

const (
	PayloadClassSkipped      EncryptedPayloadClass = "skipped"      // NULL or empty: "not configured"
	PayloadClassV2Current    EncryptedPayloadClass = "v2-current"   // v2 envelope at the current KEK/algorithm, AAD verified
	PayloadClassV2Stale      EncryptedPayloadClass = "v2-stale"     // v2 envelope that needs rotation (older KEK/algorithm)
	PayloadClassV1Legacy     EncryptedPayloadClass = "v1-legacy"    // old v1 envelope without row AAD
	PayloadClassV0Legacy     EncryptedPayloadClass = "v0-legacy"    // pre-envelope direct-KEK token
	PayloadClassUnrecognized EncryptedPayloadClass = "unrecognized" // neither envelope nor decryptable legacy token
	PayloadClassUnreadable   EncryptedPayloadClass = "unreadable"   // envelope shape, but cannot be opened with the configured KEKs
)

// NeedsRotation reports whether reencrypt can upgrade the value to current v2.
func (c EncryptedPayloadClass) NeedsRotation() bool {
	switch c {
	case PayloadClassV2Stale, PayloadClassV1Legacy, PayloadClassV0Legacy:
		return true
	default:
		return false
	}
}

// NeedsBackfill reports whether the value may be plaintext awaiting the
// backfill. It is deliberately named "may": a non-envelope value can also be
// corrupted ciphertext, so the operator gets both repair paths.
func (c EncryptedPayloadClass) NeedsBackfill() bool {
	return c == PayloadClassUnrecognized
}

// IsBroken reports whether the value has an envelope shape but cannot be
// opened, which neither backfill nor rotation can repair without the missing
// key material.
func (c EncryptedPayloadClass) IsBroken() bool {
	return c == PayloadClassUnreadable
}

// Action is the operator-facing repair suggestion for a class. It never
// includes the stored value.
func (c EncryptedPayloadClass) Action() string {
	switch c {
	case PayloadClassV2Current:
		return "none"
	case PayloadClassV2Stale:
		return "run `migrate reencrypt` to move the row to the current KEK/algorithm"
	case PayloadClassV1Legacy:
		return "run `migrate reencrypt` to upgrade the v1 envelope to the AAD-bound v2 generation"
	case PayloadClassV0Legacy:
		return "run `migrate reencrypt` to upgrade the v0 token to the AAD-bound v2 generation"
	case PayloadClassUnrecognized:
		return "if the row is plaintext run `migrate backfill-encrypted-columns`; if it held ciphertext restore it from backup"
	case PayloadClassUnreadable:
		return "restore the matching KEK version/algorithm or restore the row from backup; rotation cannot open it"
	default:
		return "none"
	}
}

// EncryptedPayloadFailure is one row that could not be backfilled or rotated.
// PK is included on purpose: verifying operators need a reproducible key to
// locate the row. The stored plaintext/ciphertext is never retained here.
type EncryptedPayloadFailure struct {
	Table  string
	Column string
	PK     string
	Class  EncryptedPayloadClass
	Stage  string // "encrypt", "rotate" or "update"
}

// EncryptedPayloadIssue is one non-current value found by a read-only scan.
type EncryptedPayloadIssue struct {
	Table  string
	Column string
	PK     string
	Class  EncryptedPayloadClass
}

// EncryptedPayloadColumnStat is the per-column classification result of a
// read-only scan. Scanned includes NULL/empty rows; Skipped counts them.
type EncryptedPayloadColumnStat struct {
	Table        string
	Column       string
	Scanned      int64
	Skipped      int64
	V2Current    int64
	V2Stale      int64
	V1Legacy     int64
	V0Legacy     int64
	Unrecognized int64
	Unreadable   int64
}

// EncryptedPayloadScanReport is the result of ScanEncryptedPayloads. Issues is
// capped by EncryptedPayloadScanOptions.MaxIssues; IssueCount is the uncapped
// number so a verify summary is still truthful when the queue is truncated.
type EncryptedPayloadScanReport struct {
	Columns         []EncryptedPayloadColumnStat
	Issues          []EncryptedPayloadIssue
	IssueCount      int64
	IssuesTruncated bool
}

// EncryptedPayloadScanOptions control a read-only classification scan.
// MaxIssues <= 0 means unlimited; callers should pass a finite value for
// operator output. QueueOnlyFailures keeps advisory legacy classes out of the
// issue queue so a capped queue cannot hide plaintext/unreadable rows.
type EncryptedPayloadScanOptions struct {
	BatchSize         int
	MaxIssues         int
	QueueOnlyFailures bool
}

// ClassifyEncryptedPayload reports the generation of a stored value. A v2
// envelope is authenticated against its row AAD before being called current or
// stale, so a cross-row ciphertext surfaces as unreadable instead of current.
// A non-envelope value is only called v0 when the legacy direct-KEK decrypt
// succeeds; everything else is unrecognized.
func ClassifyEncryptedPayload(encryptor *FieldEncryptor, col EncryptedColumn, pk, value string) (EncryptedPayloadClass, error) {
	if value == "" {
		return PayloadClassSkipped, nil
	}
	if encryptor == nil {
		return PayloadClassUnreadable, apperrors.NewInternal("encrypted payload classification requires an encryptor")
	}

	if isV2EnvelopeToken(value) {
		aad, err := col.AAD(pk).Bytes()
		if err != nil {
			return PayloadClassUnreadable, err
		}
		if _, err := encryptor.envelope.decryptEnvelope(value, aad); err != nil {
			return PayloadClassUnreadable, err
		}
		if encryptor.envelope.isCurrentEnvelope(value, aad) {
			return PayloadClassV2Current, nil
		}
		return PayloadClassV2Stale, nil
	}
	if IsEnvelopeToken(value) {
		return PayloadClassV1Legacy, nil
	}
	if _, err := encryptor.envelope.decryptLegacy(value); err == nil {
		return PayloadClassV0Legacy, nil
	}
	return PayloadClassUnrecognized, ErrNotEnvelope
}

// ScanEncryptedPayloads walks every row of the given columns in primary-key
// order and classifies it without writing. The PayloadBatchSource owns the
// keyset SQL; this function owns classification and queue semantics. Column
// order and PK order are preserved, so the issue queue is stable across runs.
// NULL/empty values are counted as skipped and never classified or queued.
func ScanEncryptedPayloads(ctx context.Context, source PayloadBatchSource, encryptor *FieldEncryptor, columns []EncryptedColumn, opts EncryptedPayloadScanOptions) (EncryptedPayloadScanReport, error) {
	var report EncryptedPayloadScanReport
	if encryptor == nil {
		return report, apperrors.NewInternal("encrypted payload scan requires an encryptor")
	}
	if source == nil {
		return report, apperrors.NewInternal("encrypted payload scan requires a payload batch source")
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
			rows, err := fetchPayloadBatch(ctx, source, col, cursor, opts.BatchSize)
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

				class, _ := ClassifyEncryptedPayload(encryptor, col, row.PK, row.Value.String)
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

			// Advance on the last row of the batch even when every value was
			// NULL/empty; otherwise an all-skipped batch would strand the scan.
			cursor = rows[len(rows)-1].PK
			if len(rows) < opts.BatchSize {
				break
			}
		}
		report.Columns = append(report.Columns, stat)
	}
	return report, nil
}

func countPayloadClass(stat *EncryptedPayloadColumnStat, class EncryptedPayloadClass) {
	switch class {
	case PayloadClassV2Current:
		stat.V2Current++
	case PayloadClassV2Stale:
		stat.V2Stale++
	case PayloadClassV1Legacy:
		stat.V1Legacy++
	case PayloadClassV0Legacy:
		stat.V0Legacy++
	case PayloadClassUnrecognized:
		stat.Unrecognized++
	case PayloadClassUnreadable:
		stat.Unreadable++
	}
}

// fetchPayloadBatch validates the registry identifiers before handing them to
// the storage port, then requests one keyset page. The empty cursor keeps the
// first page free of an empty-string cursor predicate.
func fetchPayloadBatch(ctx context.Context, source PayloadBatchSource, col EncryptedColumn, cursor string, batchSize int) ([]PayloadRow, error) {
	if err := col.Validate(); err != nil {
		return nil, err
	}
	return source.FetchPayloadBatch(ctx, PayloadBatch{
		Table:    col.Table,
		Column:   col.Column,
		PKColumn: col.PKColumn,
		AfterPK:  cursor,
		Limit:    batchSize,
	})
}
