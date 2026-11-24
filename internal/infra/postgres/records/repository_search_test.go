package records

import (
	"context"
	"testing"
	"time"

	"github.com/guliangiam/guliangiam/internal/domain/records"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
)

func TestRepository_Search_QueryError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, assert.AnError
		},
	}
	repo := NewRepository(db)
	result, total, err := repo.Search(context.Background(), records.SearchCriteria{OrganizationID: "org1"})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, int64(0), total)
}

func TestRepository_Search_ScanError(t *testing.T) {
	rows := shared.NewMockRows([][]any{
		{"id1", 123}, // wrong number of columns
	})
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return rows, nil
		},
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{int64(0)}, nil)
		},
	}
	repo := NewRepository(db)
	_, _, err := repo.Search(context.Background(), records.SearchCriteria{OrganizationID: "org1"})
	assert.Error(t, err)
}

func TestRepository_Search_CountError(t *testing.T) {
	rows := shared.NewMockRows(nil) // empty rows
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return rows, nil
		},
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, assert.AnError)
		},
	}
	repo := NewRepository(db)
	_, _, err := repo.Search(context.Background(), records.SearchCriteria{OrganizationID: "org1"})
	assert.Error(t, err)
}

func TestRepository_Search_Success(t *testing.T) {
	now := time.Now()
	rows := shared.NewMockRows([][]any{
		{"id1", "org1", "actor1", "login", "user", "res1", "127.0.0.1", "Mozilla", "management", "req-1", "trace-1", []byte(`{"key":"val"}`), now},
	})
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return rows, nil
		},
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{int64(1)}, nil)
		},
	}
	repo := NewRepository(db)
	result, total, err := repo.Search(context.Background(), records.SearchCriteria{OrganizationID: "org1"})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, result, 1)
	assert.Equal(t, "id1", result[0].ID)
}

func TestRepository_Search_Empty(t *testing.T) {
	rows := shared.NewMockRows(nil) // empty
	db := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return rows, nil
		},
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{int64(0)}, nil)
		},
	}
	repo := NewRepository(db)
	result, total, err := repo.Search(context.Background(), records.SearchCriteria{OrganizationID: "org1"})
	assert.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, result)
}
