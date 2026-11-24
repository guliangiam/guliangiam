package shared

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// MockDBTX is a mock implementation of DBTX for testing.
type MockDBTX struct {
	ExecFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (m *MockDBTX) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if m.ExecFn != nil {
		return m.ExecFn(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, nil
}

func (m *MockDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.QueryFn != nil {
		return m.QueryFn(ctx, sql, args...)
	}
	return &MockRows{}, nil
}

func (m *MockDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.QueryRowFn != nil {
		return m.QueryRowFn(ctx, sql, args...)
	}
	return &MockRow{}
}

// MockRow implements pgx.Row for testing.
type MockRow struct {
	values []any
	err    error
}

func NewMockRow(values []any, err error) *MockRow {
	return &MockRow{values: values, err: err}
}

func (r *MockRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) > len(r.values) {
		return errors.New("not enough values")
	}
	for i, d := range dest {
		if i < len(r.values) && r.values[i] != nil {
			// Simple type assignment - works for basic types
			switch v := r.values[i].(type) {
			case string:
				if ptr, ok := d.(*string); ok {
					*ptr = v
				}
			case int:
				if ptr, ok := d.(*int); ok {
					*ptr = v
				}
			case int64:
				if ptr, ok := d.(*int64); ok {
					*ptr = v
				}
			case float64:
				if ptr, ok := d.(*float64); ok {
					*ptr = v
				}
			case bool:
				if ptr, ok := d.(*bool); ok {
					*ptr = v
				}
			case []byte:
				if ptr, ok := d.(*[]byte); ok {
					*ptr = v
				}
			}
		}
	}
	return nil
}

// MockRows implements pgx.Rows for testing.
type MockRows struct {
	rows   [][]any
	idx    int
	closed bool
	err    error
}

func NewMockRows(rows [][]any) *MockRows {
	return &MockRows{rows: rows}
}

func (r *MockRows) Close() { r.closed = true }

func (r *MockRows) Err() error { return r.err }

func (r *MockRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *MockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

// TypeMap satisfies the pgx.Rows interface (required by pgx v5.5+).
func (r *MockRows) TypeMap() *pgtype.Map { return nil }

func (r *MockRows) Next() bool {
	if r.idx < len(r.rows) {
		r.idx++
		return true
	}
	return false
}

func (r *MockRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return errors.New("no row to scan")
	}
	row := r.rows[r.idx-1]
	if len(dest) > len(row) {
		return errors.New("not enough values")
	}
	for i, d := range dest {
		if i < len(row) && row[i] != nil {
			switch v := row[i].(type) {
			case string:
				if ptr, ok := d.(*string); ok {
					*ptr = v
				}
			case int:
				if ptr, ok := d.(*int); ok {
					*ptr = v
				}
			case int64:
				if ptr, ok := d.(*int64); ok {
					*ptr = v
				}
			case float64:
				if ptr, ok := d.(*float64); ok {
					*ptr = v
				}
			case bool:
				if ptr, ok := d.(*bool); ok {
					*ptr = v
				}
			case []byte:
				if ptr, ok := d.(*[]byte); ok {
					*ptr = v
				}
			}
		}
	}
	return nil
}

func (r *MockRows) Values() ([]any, error) {
	if r.idx == 0 || r.idx > len(r.rows) {
		return nil, errors.New("no row")
	}
	return r.rows[r.idx-1], nil
}

func (r *MockRows) RawValues() [][]byte { return nil }

func (r *MockRows) Conn() *pgx.Conn { return nil }
