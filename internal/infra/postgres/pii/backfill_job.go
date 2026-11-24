// Package pii owns the PostgreSQL backfill for the legacy plaintext PII columns
// whose sealed targets landed in A-6.3A; the SQL lives here, not in cmd.
package pii

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	DefaultLegacySealedBatchSize = 200
	// MaxLegacySealedBatchSize bounds one keyset page so a mistaken flag cannot
	// hold a very large result set in memory.
	MaxLegacySealedBatchSize = 1000
)

// LegacySealedBackfillOptions control one run. MaxFailures stops a column after
// that many failed rows (0 = unlimited); no value or primary key is ever logged.
type LegacySealedBackfillOptions struct {
	BatchSize   int
	DryRun      bool
	Verify      bool
	MaxFailures int
}

// LegacySealedBackfillStat is the per-column result; Verify counters are set
// only by Verify, the others only by the write and dry-run paths.
type LegacySealedBackfillStat struct {
	Table        string
	SourceColumn string
	TargetColumn string
	Selected     int64 // write scan selections (dry-run: rows that would be sealed)
	Encrypted    int64 // compare-and-swap updates that committed
	Skipped      int64 // CAS updates that lost the row to a concurrent writer
	Failed       int64

	VerifyEncrypted     int64 // target decrypts with the row AAD
	VerifyUnsealed      int64 // source is non-empty and target is NULL/empty
	VerifyDecryptFailed int64 // target cannot be opened

	Duration time.Duration
}

// LegacySealedBackfillReport aggregates the finished columns; a per-column
// error aborts the run but completed work stays in the report.
type LegacySealedBackfillReport struct {
	Columns  []LegacySealedBackfillStat
	Duration time.Duration
}

// BackfillLegacySealedColumns seals legacy plaintext into the registered
// `_sealed` targets, or classifies rows without writing when DryRun/Verify is
// set.
func BackfillLegacySealedColumns(ctx context.Context, db *sql.DB, encryptor *crypto.FieldEncryptor, columns []crypto.LegacySealedColumn, opts LegacySealedBackfillOptions) (LegacySealedBackfillReport, error) {
	report := LegacySealedBackfillReport{}
	if encryptor == nil {
		return report, apperrors.NewInternal("PII sealed-column backfill requires an encryptor")
	}
	if opts.DryRun && opts.Verify {
		return report, apperrors.NewInvalidArgument("dry-run and verify are mutually exclusive")
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = DefaultLegacySealedBatchSize
	}
	if opts.BatchSize > MaxLegacySealedBatchSize {
		return report, apperrors.NewInvalidArgumentf("batch size %d exceeds the %d-row limit", opts.BatchSize, MaxLegacySealedBatchSize)
	}

	start := time.Now()
	for _, col := range columns {
		if err := col.Validate(); err != nil {
			report.Duration = time.Since(start)
			return report, err
		}
		stat := LegacySealedBackfillStat{Table: col.Table, SourceColumn: col.SourceColumn, TargetColumn: col.TargetColumn}
		columnStart := time.Now()

		var err error
		switch {
		case opts.Verify:
			err = verifyLegacySealedColumn(ctx, db, encryptor, col, opts, &stat)
		case opts.DryRun:
			err = countLegacySealedRows(ctx, db, col, opts, &stat)
		default:
			err = backfillLegacySealedColumn(ctx, db, encryptor, col, opts, &stat)
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

func backfillLegacySealedColumn(ctx context.Context, db *sql.DB, encryptor *crypto.FieldEncryptor, col crypto.LegacySealedColumn, opts LegacySealedBackfillOptions, stat *LegacySealedBackfillStat) error {
	cursor := ""
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		rows, err := fetchUnsealedRows(ctx, db, col, cursor, opts.BatchSize)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}

		for _, row := range rows {
			cursor = row.pk
			stat.Selected++

			sealed, err := encryptor.EncryptColumn(col.AAD(row.pk), row.source)
			if err != nil {
				stat.Failed++
				slog.Warn("pii sealed backfill: row encryption failed",
					"table", col.Table, "source_column", col.SourceColumn, "target_column", col.TargetColumn)
				if limitErr := failureLimit(stat, opts.MaxFailures, col); limitErr != nil {
					return limitErr
				}
				continue
			}

			affected, err := updateSealedRow(ctx, db, col, row.pk, sealed)
			if err != nil {
				stat.Failed++
				slog.Warn("pii sealed backfill: update failed",
					"table", col.Table, "source_column", col.SourceColumn, "target_column", col.TargetColumn)
				if limitErr := failureLimit(stat, opts.MaxFailures, col); limitErr != nil {
					return limitErr
				}
				continue
			}
			if affected == 0 {
				// A concurrent writer sealed (or deleted) the row first; the
				// NULL guard means its ciphertext is never overwritten.
				stat.Skipped++
				continue
			}
			stat.Encrypted++
		}

		if len(rows) < opts.BatchSize {
			return nil
		}
	}
}

// countLegacySealedRows walks the write pass's selections and counts them
// without encrypting or updating anything.
func countLegacySealedRows(ctx context.Context, db *sql.DB, col crypto.LegacySealedColumn, opts LegacySealedBackfillOptions, stat *LegacySealedBackfillStat) error {
	cursor := ""
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		rows, err := fetchUnsealedRows(ctx, db, col, cursor, opts.BatchSize)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for _, row := range rows {
			cursor = row.pk
			stat.Selected++
		}
		if len(rows) < opts.BatchSize {
			return nil
		}
	}
}

// verifyLegacySealedColumn classifies rows into exactly three counters: a
// target that decrypts with the row AAD, a non-empty source whose target is
// still NULL/empty, and a target that cannot be opened. Rows without any
// legacy value are ignored because there is nothing to seal for them.
func verifyLegacySealedColumn(ctx context.Context, db *sql.DB, encryptor *crypto.FieldEncryptor, col crypto.LegacySealedColumn, opts LegacySealedBackfillOptions, stat *LegacySealedBackfillStat) error {
	cursor := ""
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		rows, err := fetchSealedStateRows(ctx, db, col, cursor, opts.BatchSize)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}

		for _, row := range rows {
			cursor = row.pk
			if row.sealed.Valid && row.sealed.String != "" {
				if _, err := encryptor.DecryptColumn(col.AAD(row.pk), row.sealed.String); err != nil {
					stat.VerifyDecryptFailed++
					slog.Warn("pii sealed backfill: sealed value cannot be opened",
						"table", col.Table, "target_column", col.TargetColumn)
					continue
				}
				stat.VerifyEncrypted++
				continue
			}
			if row.source != "" {
				stat.VerifyUnsealed++
			}
		}

		if len(rows) < opts.BatchSize {
			return nil
		}
	}
}

func failureLimit(stat *LegacySealedBackfillStat, maxFailures int, col crypto.LegacySealedColumn) error {
	if maxFailures > 0 && stat.Failed >= int64(maxFailures) {
		return apperrors.NewInternalf("aborting %s.%s after %d failed row(s) (-max-failures)", col.Table, col.TargetColumn, stat.Failed)
	}
	return nil
}

type legacySealedRow struct {
	pk     string
	source string
}

type sealedStateRow struct {
	pk     string
	source string
	sealed sql.NullString
}

func fetchUnsealedRows(ctx context.Context, db *sql.DB, col crypto.LegacySealedColumn, cursor string, batchSize int) ([]legacySealedRow, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if cursor == "" {
		// The first batch omits the cursor predicate: comparing a UUID PK with
		// an empty string makes PostgreSQL reject the query.
		// #nosec G201 -- identifiers validated by col.Validate().
		rows, err = db.QueryContext(ctx, fmt.Sprintf(
			`SELECT %s, %s::text FROM %s WHERE %s IS NULL AND COALESCE(%s::text, '') <> '' ORDER BY %s ASC LIMIT $1`,
			col.PKColumn, col.SourceColumn, col.Table, col.TargetColumn, col.SourceColumn, col.PKColumn,
		), batchSize)
	} else {
		// #nosec G201 -- identifiers validated by col.Validate().
		rows, err = db.QueryContext(ctx, fmt.Sprintf(
			`SELECT %s, %s::text FROM %s WHERE %s IS NULL AND COALESCE(%s::text, '') <> '' AND %s > $1 ORDER BY %s ASC LIMIT $2`,
			col.PKColumn, col.SourceColumn, col.Table, col.TargetColumn, col.SourceColumn, col.PKColumn, col.PKColumn,
		), cursor, batchSize)
	}
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "scan %s.%s", col.Table, col.TargetColumn)
	}
	defer rows.Close()

	var out []legacySealedRow
	for rows.Next() {
		var pk, source sql.NullString
		if err := rows.Scan(&pk, &source); err != nil {
			return nil, apperrors.WrapInternalf(err, "scan %s.%s", col.Table, col.TargetColumn)
		}
		if !pk.Valid || !source.Valid || source.String == "" {
			continue
		}
		out = append(out, legacySealedRow{pk: pk.String, source: source.String})
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.WrapInternalf(err, "scan %s.%s", col.Table, col.TargetColumn)
	}
	return out, nil
}

func fetchSealedStateRows(ctx context.Context, db *sql.DB, col crypto.LegacySealedColumn, cursor string, batchSize int) ([]sealedStateRow, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if cursor == "" {
		// #nosec G201 -- identifiers validated by col.Validate().
		rows, err = db.QueryContext(ctx, fmt.Sprintf(
			`SELECT %s, COALESCE(%s::text, ''), %s FROM %s WHERE (COALESCE(%s::text, '') <> '' OR %s IS NOT NULL) ORDER BY %s ASC LIMIT $1`,
			col.PKColumn, col.SourceColumn, col.TargetColumn, col.Table, col.SourceColumn, col.TargetColumn, col.PKColumn,
		), batchSize)
	} else {
		// #nosec G201 -- identifiers validated by col.Validate().
		rows, err = db.QueryContext(ctx, fmt.Sprintf(
			`SELECT %s, COALESCE(%s::text, ''), %s FROM %s WHERE (COALESCE(%s::text, '') <> '' OR %s IS NOT NULL) AND %s > $1 ORDER BY %s ASC LIMIT $2`,
			col.PKColumn, col.SourceColumn, col.TargetColumn, col.Table, col.SourceColumn, col.TargetColumn, col.PKColumn, col.PKColumn,
		), cursor, batchSize)
	}
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "scan %s.%s", col.Table, col.TargetColumn)
	}
	defer rows.Close()

	var out []sealedStateRow
	for rows.Next() {
		var pk, source, sealed sql.NullString
		if err := rows.Scan(&pk, &source, &sealed); err != nil {
			return nil, apperrors.WrapInternalf(err, "scan %s.%s", col.Table, col.TargetColumn)
		}
		if !pk.Valid {
			continue
		}
		out = append(out, sealedStateRow{pk: pk.String, source: source.String, sealed: sealed})
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.WrapInternalf(err, "scan %s.%s", col.Table, col.TargetColumn)
	}
	return out, nil
}

// updateSealedRow commits one compare-and-swap: the target must still be NULL,
// so an existing ciphertext is never overwritten. Zero affected rows is a skip.
func updateSealedRow(ctx context.Context, db *sql.DB, col crypto.LegacySealedColumn, pk, sealed string) (int64, error) {
	// #nosec G201 -- identifiers validated by col.Validate().
	result, err := db.ExecContext(ctx, fmt.Sprintf(
		`UPDATE %s SET %s = $1 WHERE %s = $2 AND %s IS NULL`,
		col.Table, col.TargetColumn, col.PKColumn, col.TargetColumn,
	), sealed, pk)
	if err != nil {
		return 0, apperrors.WrapInternalf(err, "update %s.%s", col.Table, col.TargetColumn)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, apperrors.WrapInternalf(err, "rows affected %s.%s", col.Table, col.TargetColumn)
	}
	return affected, nil
}
