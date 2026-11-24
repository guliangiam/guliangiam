package crypto

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// envelopeTokenSQLPattern is the structural envelope shape used by the
// backfill scan. Rows that do not match are plaintext (or a legacy token these
// columns never contained) and must be encrypted. The optional second suffix
// keeps v2 tokens ("v1-aes-aad:...") out of the scan; without it the backfill
// would double-encrypt values written after GM-G7.
//
// #nosec G101 -- this is a regex, not a credential.
const envelopeTokenSQLPattern = `^v[0-9]+(-[a-z0-9]+)?(-aad)?:[A-Za-z0-9+/=]+:[A-Za-z0-9+/=]+:[A-Za-z0-9+/=]+$`

// ErrPlaintextProviderSecrets reports that -verify found plaintext in one or
// more provider credential columns.
var ErrPlaintextProviderSecrets = errors.New("crypto: provider credential columns still contain plaintext")

// ErrPlaintextMFASecrets reports that verify found plaintext in one or more
// user MFA secret columns.
var ErrPlaintextMFASecrets = errors.New("crypto: user MFA secret columns still contain plaintext")

// EncryptedColumnBackfillOptions control a plaintext-to-envelope backfill run.
// MaxFailures caps the per-column failure queue (0 = unlimited); Failed still
// counts every row, so truncation is visible in the summary.
type EncryptedColumnBackfillOptions struct {
	BatchSize   int
	DryRun      bool
	Verify      bool
	MaxFailures int
}

// ProviderSecretBackfillOptions is the historical name used by the
// encrypt-provider-secrets command.
type ProviderSecretBackfillOptions = EncryptedColumnBackfillOptions

// ProviderSecretBackfillStat is the per-column result of a backfill run. It is
// shared by every column list, not just provider credentials.
type ProviderSecretBackfillStat struct {
	Table             string
	Column            string
	Plaintext         int64
	Scanned           int64
	Encrypted         int64
	Skipped           int64
	Failed            int64
	V2Current         int64
	LegacyV1          int64
	LegacyV0          int64
	Failures          []EncryptedPayloadFailure
	FailuresTruncated bool
}

// EncryptedColumnBackfillStat is the generic name for ProviderSecretBackfillStat.
type EncryptedColumnBackfillStat = ProviderSecretBackfillStat

// MFASecretColumns are the user MFA secret columns sealed by the users
// repositories (GM-G2). Pre-GM-G2 rows are plaintext, so reads fail closed until
// the columns are backfilled (`migrate backfill-encrypted-columns` covers them
// through DefaultEncryptedColumns; BackfillMFASecrets remains the narrow API).
var MFASecretColumns = []EncryptedColumn{
	{Table: tableUserMfaConfig, Column: "totp_secret", PKColumn: pkUserID},
}

// BackfillProviderSecrets encrypts the plaintext values of
// ProviderCredentialColumns in place. It never decrypts or rotates existing
// envelope tokens, and leaves decryptable v0 tokens for reencrypt.
func BackfillProviderSecrets(ctx context.Context, db *sql.DB, encryptor *FieldEncryptor, opts ProviderSecretBackfillOptions) ([]ProviderSecretBackfillStat, error) {
	stats, plaintextRemaining, err := BackfillEncryptedColumns(ctx, db, encryptor, ProviderCredentialColumns, opts)
	if err != nil {
		return stats, err
	}
	if plaintextRemaining {
		return stats, ErrPlaintextProviderSecrets
	}
	return stats, nil
}

// BackfillMFASecrets encrypts MFASecretColumns in place for a future
// `migrate reencrypt`; same Postgres-only scan as the provider backfill.
func BackfillMFASecrets(ctx context.Context, db *sql.DB, encryptor *FieldEncryptor, opts EncryptedColumnBackfillOptions) ([]EncryptedColumnBackfillStat, error) {
	stats, plaintextRemaining, err := BackfillEncryptedColumns(ctx, db, encryptor, MFASecretColumns, opts)
	if err != nil {
		return stats, err
	}
	if plaintextRemaining {
		return stats, ErrPlaintextMFASecrets
	}
	return stats, nil
}

// BackfillEncryptedColumns encrypts the plaintext values of the given columns in
// place without touching envelope rows or decryptable v0 tokens. The boolean
// reports whether verify found plaintext remaining so each command surfaces its
// own sentinel error.
//
// DryRun/Verify keep the historical count query for narrow callers. The
// `migrate` CLI uses ScanEncryptedPayloads instead so v1/v0 payloads are
// classified and verification can print a primary-key queue.
func BackfillEncryptedColumns(ctx context.Context, db *sql.DB, encryptor *FieldEncryptor, columns []EncryptedColumn, opts EncryptedColumnBackfillOptions) ([]EncryptedColumnBackfillStat, bool, error) {
	if encryptor == nil {
		return nil, false, apperrors.NewInternal("encrypted column backfill requires an encryptor")
	}
	if opts.DryRun && opts.Verify {
		return nil, false, apperrors.NewInvalidArgument("dry-run and verify are mutually exclusive")
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 200
	}

	stats := make([]EncryptedColumnBackfillStat, 0, len(columns))
	plaintextRemaining := false
	for _, col := range columns {
		if err := col.Validate(); err != nil {
			return stats, plaintextRemaining, err
		}
		stat := EncryptedColumnBackfillStat{Table: col.Table, Column: col.Column}

		if opts.DryRun || opts.Verify {
			count, err := countPlaintextColumn(ctx, db, col)
			if err != nil {
				return stats, plaintextRemaining, err
			}
			stat.Plaintext = count
			stat.Scanned = count
			if count > 0 && opts.Verify {
				plaintextRemaining = true
			}
			stats = append(stats, stat)
			continue
		}

		if err := backfillColumn(ctx, db, encryptor, col, opts.BatchSize, opts.MaxFailures, &stat); err != nil {
			return stats, plaintextRemaining, err
		}
		stats = append(stats, stat)
	}

	return stats, plaintextRemaining, nil
}

// backfillColumn processes one column in PK order, batch by batch.
// Every update is a compare-and-swap on the old plaintext value: a concurrent
// writer that already replaced the value makes the update affect zero rows and
// is counted as skipped instead of overwriting newer data.
func backfillColumn(ctx context.Context, db *sql.DB, encryptor *FieldEncryptor, col EncryptedColumn, batchSize, maxFailures int, stat *EncryptedColumnBackfillStat) error {
	cursor := ""
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		rows, err := fetchPlaintextBatch(ctx, db, col, cursor, batchSize)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}

		for _, row := range rows {
			cursor = row.pk
			stat.Scanned++

			// Belt and braces: the scan predicate already excludes envelope tokens.
			// Re-checking here keeps a query regression from double-encrypting data.
			if IsEnvelopeToken(row.value) {
				stat.Skipped++
				if isV2EnvelopeToken(row.value) {
					stat.V2Current++
				} else {
					stat.LegacyV1++
				}
				continue
			}

			// A v0 token is an old ciphertext, not plaintext. Encrypting it now
			// would seal an already-sealed value; leave it for reencrypt.
			if class, _ := ClassifyEncryptedPayload(encryptor, col, row.pk, row.value); class == PayloadClassV0Legacy {
				stat.Skipped++
				stat.LegacyV0++
				continue
			}

			sealed, err := encryptor.EncryptColumn(col.AAD(row.pk), row.value)
			if err != nil {
				slog.Warn("backfill: encrypt row failed", "table", col.Table, "column", col.Column,
					"pk_fingerprint", pkFingerprint(row.pk), "err", err)
				recordBackfillFailure(stat, maxFailures, col, row.pk, PayloadClassUnrecognized, "encrypt")
				continue
			}

			affected, err := updatePlaintextRow(ctx, db, col, row.pk, row.value, sealed)
			if err != nil {
				slog.Warn("backfill: update row failed", "table", col.Table, "column", col.Column,
					"pk_fingerprint", pkFingerprint(row.pk), "err", err)
				recordBackfillFailure(stat, maxFailures, col, row.pk, PayloadClassUnrecognized, "update")
				continue
			}
			if affected == 0 {
				stat.Skipped++
				continue
			}
			stat.Encrypted++
		}

		if len(rows) < batchSize {
			return nil
		}
	}
}

func recordBackfillFailure(stat *EncryptedColumnBackfillStat, maxFailures int, col EncryptedColumn, pk string, class EncryptedPayloadClass, stage string) {
	stat.Failed++
	if maxFailures > 0 && len(stat.Failures) >= maxFailures {
		stat.FailuresTruncated = true
		return
	}
	stat.Failures = append(stat.Failures, EncryptedPayloadFailure{
		Table: col.Table, Column: col.Column, PK: pk, Class: class, Stage: stage,
	})
}

type plaintextRow struct {
	pk    string
	value string
}

func fetchPlaintextBatch(ctx context.Context, db *sql.DB, col EncryptedColumn, cursor string, batchSize int) ([]plaintextRow, error) {
	// The first batch has no cursor. Keeping the predicate out of the SQL avoids
	// comparing a UUID PK against an empty string; later batches use the last
	// processed PK as a keyset cursor.
	var (
		query string
		args  []any
	)
	if cursor == "" {
		// #nosec G201 -- identifiers validated by EncryptedColumn.Validate().
		query = fmt.Sprintf(
			`SELECT %s, %s FROM %s WHERE %s <> '' AND %s !~ $1 ORDER BY %s ASC LIMIT $2`,
			col.PKColumn, col.Column, col.Table, col.Column, col.Column, col.PKColumn,
		)
		args = []any{envelopeTokenSQLPattern, batchSize}
	} else {
		// #nosec G201 -- identifiers validated by EncryptedColumn.Validate().
		query = fmt.Sprintf(
			`SELECT %s, %s FROM %s WHERE %s <> '' AND %s !~ $1 AND %s > $2 ORDER BY %s ASC LIMIT $3`,
			col.PKColumn, col.Column, col.Table, col.Column, col.Column, col.PKColumn, col.PKColumn,
		)
		args = []any{envelopeTokenSQLPattern, cursor, batchSize}
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "scan %s.%s", col.Table, col.Column)
	}
	defer rows.Close()

	var out []plaintextRow
	for rows.Next() {
		var pk, value sql.NullString
		if err := rows.Scan(&pk, &value); err != nil {
			return nil, apperrors.WrapInternalf(err, "scan %s.%s", col.Table, col.Column)
		}
		// NULL and empty values mean "not configured"; they must stay untouched
		// instead of becoming an encryption of an empty string.
		if !pk.Valid || !value.Valid || value.String == "" {
			continue
		}
		out = append(out, plaintextRow{pk: pk.String, value: value.String})
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.WrapInternalf(err, "scan %s.%s", col.Table, col.Column)
	}
	return out, nil
}

func updatePlaintextRow(ctx context.Context, db *sql.DB, col EncryptedColumn, pk, plaintext, sealed string) (int64, error) {
	// #nosec G201 -- identifiers validated by EncryptedColumn.Validate().
	query := fmt.Sprintf(
		`UPDATE %s SET %s = $1 WHERE %s = $2 AND %s = $3`,
		col.Table, col.Column, col.PKColumn, col.Column,
	)
	result, err := db.ExecContext(ctx, query, sealed, pk, plaintext)
	if err != nil {
		return 0, apperrors.WrapInternalf(err, "update %s.%s", col.Table, col.Column)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, apperrors.WrapInternalf(err, "rows affected %s.%s", col.Table, col.Column)
	}
	return affected, nil
}

func countPlaintextColumn(ctx context.Context, db *sql.DB, col EncryptedColumn) (int64, error) {
	// #nosec G201 -- identifiers validated by EncryptedColumn.Validate().
	query := fmt.Sprintf(
		`SELECT COUNT(*) FROM %s WHERE %s <> '' AND %s !~ $1`,
		col.Table, col.Column, col.Column,
	)
	var count int64
	if err := db.QueryRowContext(ctx, query, envelopeTokenSQLPattern).Scan(&count); err != nil {
		return 0, apperrors.WrapInternalf(err, "count %s.%s", col.Table, col.Column)
	}
	return count, nil
}
