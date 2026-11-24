package shared

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

// mockDBTX implements DBTX for testing
type mockDBTX struct {
	execFn     func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	queryFn    func(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
	queryRowFn func(ctx context.Context, sql string, arguments ...any) pgx.Row
}

func (m *mockDBTX) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	if m.execFn != nil {
		return m.execFn(ctx, sql, arguments...)
	}
	return pgconn.CommandTag{}, nil
}

func (m *mockDBTX) Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error) {
	if m.queryFn != nil {
		return m.queryFn(ctx, sql, arguments...)
	}
	return nil, nil
}

func (m *mockDBTX) QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row {
	if m.queryRowFn != nil {
		return m.queryRowFn(ctx, sql, arguments...)
	}
	return nil
}

func TestMockDBTX_Exec(t *testing.T) {
	db := &mockDBTX{}
	tag, err := db.Exec(context.Background(), "SELECT 1")
	assert.NoError(t, err)
	assert.NotNil(t, tag)
}

func TestMockDBTX_Query(t *testing.T) {
	db := &mockDBTX{}
	rows, err := db.Query(context.Background(), "SELECT 1")
	assert.NoError(t, err)
	assert.Nil(t, rows)
}

func TestMockDBTX_QueryRow(t *testing.T) {
	db := &mockDBTX{}
	row := db.QueryRow(context.Background(), "SELECT 1")
	assert.Nil(t, row)
}
