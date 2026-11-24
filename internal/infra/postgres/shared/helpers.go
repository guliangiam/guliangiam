package shared

import (
	"bytes"
	"context"
	"encoding/json"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/jackc/pgx/v5"
)

// Perf: pre-allocated []byte for "null" — avoids per-call allocation in MarshalJSON/UnmarshalJSON hot paths.
var nullBytes = []byte("null")

// MarshalJSON serializes value to JSON bytes. Nil produces "null".
func MarshalJSON(value any) ([]byte, error) {
	if value == nil {
		// Perf: return pre-allocated slice instead of allocating []byte("null") on every nil call.
		return nullBytes, nil
	}
	return json.Marshal(value)
}

// UnmarshalJSON deserializes JSON bytes into target. Empty or "null" data is a no-op.
func UnmarshalJSON(data []byte, target any) error {
	// Perf: use bytes.Equal instead of string(data) == "null" to avoid string allocation on every call.
	if len(data) == 0 || bytes.Equal(data, nullBytes) {
		return nil
	}
	return json.Unmarshal(data, target)
}

// ClampPageSize and PageOffset are the repository-side net over the pagination
// bounds (手册 4.7.1), delegating to the one definition in platform/pagination so
// the middleware and the repositories cannot disagree.
//
// They replace the `max(query.PageSize, 20)` this package used to expose as
// `Paginate` in two ways: that helper was a floor rather than a ceiling (so
// `pageSize=5` returned 20 rows), it returned limit and offset from the same
// unclamped value only by accident, and nothing called it — the 43 repositories
// each wrote the expression out by hand instead. A single named call site is what
// makes the bound greppable.
func ClampPageSize(pageSize int) int {
	return platformpagination.ClampPageSize(pageSize)
}

func PageOffset(page, pageSize int) int {
	return platformpagination.Offset(page, pageSize)
}

// ScanOne executes a single-row query and scans the result using the provided scan function.
// Returns apperrors.CodeNotFound if no row is found.
func ScanOne[T any](db DBTX, ctx context.Context, sql string, args []any, scan func(pgx.Row) (T, error)) (T, error) {
	var zero T
	row := db.QueryRow(ctx, sql, args...)
	result, err := scan(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return zero, apperrors.NewNotFound("record not found")
		}
		return zero, err
	}
	return result, nil
}

// ScanList executes a multi-row query and collects results using the provided scan function.
//
// The rows.Err() check after the loop is what separates "the query returned N
// rows" from "the query failed after N rows": without it a dropped connection
// mid-stream is reported as a successful, short result.
func ScanList[T any](db DBTX, ctx context.Context, sql string, args []any, scan func(pgx.Rows) (T, error)) ([]T, error) {
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "query")
	}
	defer rows.Close()

	// Perf: pre-allocate with default page size capacity to avoid slice growth reallocations in the common case.
	items := make([]T, 0, 20)
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "scan row")
		}
		items = append(items, item)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, apperrors.WrapInternalf(rowsErr, "iterate rows")
	}
	return items, nil
}

// Count executes a COUNT query and returns the result.
func Count(db DBTX, ctx context.Context, sql string, args []any) (int64, error) {
	var total int64
	if err := db.QueryRow(ctx, sql, args...).Scan(&total); err != nil {
		return 0, apperrors.WrapInternalf(err, "count")
	}
	return total, nil
}
