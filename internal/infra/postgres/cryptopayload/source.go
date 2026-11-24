// Package cryptopayload owns the PostgreSQL keyset scan shared by the read-only
// encrypted-payload classification jobs (`migrate backfill-encrypted-columns
// -verify/-dry-run`, `migrate reencrypt -verify/-dry-run` and
// `migrate scan-sealed-payloads`). The SQL lives here; platform/crypto defines
// only the PayloadBatchSource port, the identifier rule and classification.
package cryptopayload

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Source implements crypto.PayloadBatchSource on top of a PostgreSQL *sql.DB.
type Source struct {
	db *sql.DB
}

var (
	_ crypto.PayloadBatchSource          = Source{}
	_ crypto.SealedPayloadBackfillSource = Source{}
)

// NewSource returns a keyset-scan source backed by db.
func NewSource(db *sql.DB) Source {
	return Source{db: db}
}

// FetchPayloadBatch returns one primary-key-ordered page. The first page omits
// the cursor predicate so a UUID primary key is never compared with an empty
// string; later pages use the last PK of the previous batch as a keyset cursor.
func (s Source) FetchPayloadBatch(ctx context.Context, batch crypto.PayloadBatch) ([]crypto.PayloadRow, error) {
	// Reuse the registry's identifier rule: these values are interpolated into
	// the SQL text below.
	col := crypto.EncryptedColumn{Table: batch.Table, Column: batch.Column, PKColumn: batch.PKColumn}
	if err := col.Validate(); err != nil {
		return nil, err
	}
	if s.db == nil {
		return nil, apperrors.NewInternal("payload batch source requires a database")
	}

	query, args := payloadBatchQuery(batch)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "scan %s.%s", batch.Table, batch.Column)
	}
	defer rows.Close()

	var out []crypto.PayloadRow
	for rows.Next() {
		var pk, value sql.NullString
		if err := rows.Scan(&pk, &value); err != nil {
			return nil, apperrors.WrapInternalf(err, "scan %s.%s", batch.Table, batch.Column)
		}
		if !pk.Valid {
			return nil, apperrors.NewInternalf("%s.%s row has NULL primary key", batch.Table, batch.Column)
		}
		// The value stays a NullString on purpose: NULL ("not configured") and
		// empty string are different rows, and the scan must not turn one into
		// the other.
		out = append(out, crypto.PayloadRow{PK: pk.String, Value: value})
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.WrapInternalf(err, "scan %s.%s", batch.Table, batch.Column)
	}
	return out, nil
}

func payloadBatchQuery(batch crypto.PayloadBatch) (string, []any) {
	if batch.AfterPK == "" {
		// #nosec G201 -- identifiers validated by FetchPayloadBatch via EncryptedColumn.Validate().
		return fmt.Sprintf(
			`SELECT %s, %s FROM %s ORDER BY %s ASC LIMIT $1`,
			batch.PKColumn, batch.Column, batch.Table, batch.PKColumn,
		), []any{batch.Limit}
	}
	// #nosec G201 -- identifiers validated by FetchPayloadBatch via EncryptedColumn.Validate().
	return fmt.Sprintf(
		`SELECT %s, %s FROM %s WHERE %s > $1 ORDER BY %s ASC LIMIT $2`,
		batch.PKColumn, batch.Column, batch.Table, batch.PKColumn, batch.PKColumn,
	), []any{batch.AfterPK, batch.Limit}
}

// UpdateSealedPayload performs the single-row compare-and-swap the sealed
// payload backfill uses. The predicate compares the full stored value, not just
// the primary key, so a concurrent application write wins and the caller counts
// the row as skipped instead of overwriting newer ciphertext. The replacement
// value is always a sealed token; this method never writes plaintext by itself.
func (s Source) UpdateSealedPayload(ctx context.Context, write crypto.SealedPayloadWrite) (int64, error) {
	col := write.Column
	if err := col.Validate(); err != nil {
		return 0, err
	}
	if s.db == nil {
		return 0, apperrors.NewInternal("sealed payload backfill source requires a database")
	}
	if write.Sealed == "" {
		return 0, apperrors.NewInvalidArgument("sealed payload replacement must be a non-empty token")
	}

	// #nosec G201 -- identifiers validated by SealedPayloadColumn.Validate().
	query := fmt.Sprintf(
		`UPDATE %s SET %s = $1 WHERE %s = $2 AND %s = $3`,
		col.Table, col.Column, col.PKColumn, col.Column,
	)
	result, err := s.db.ExecContext(ctx, query, write.Sealed, write.PK, write.Expected)
	if err != nil {
		return 0, apperrors.WrapInternalf(err, "update %s.%s", col.Table, col.Column)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, apperrors.WrapInternalf(err, "rows affected %s.%s", col.Table, col.Column)
	}
	return affected, nil
}
