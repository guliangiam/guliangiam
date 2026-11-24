package consents

import (
	"context"
	"fmt"
	"testing"

	"github.com/guliangiam/guliangiam/internal/domain/consents"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestRepository_GetByUserAndApplication_QueryRowError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetByUserAndApplication(context.Background(), "user-1", "app-1")
	assert.Error(t, err)
}

func TestRepository_GetByUserAndApplication_NotFound(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetByUserAndApplication(context.Background(), "user-1", "app-1")
	assert.Error(t, err)
}

func TestRepository_CountActiveByApplication_QueryRowError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.CountActiveByApplication(context.Background(), "app-1")
	assert.Error(t, err)
}

func TestRepository_CountActiveByApplication_Success(t *testing.T) {
	var count int64 = 5
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{count}, nil)
		},
	}
	repo := NewRepository(mockDB)
	result, err := repo.CountActiveByApplication(context.Background(), "app-1")
	assert.NoError(t, err)
	assert.Equal(t, int64(5), result)
}

func TestRepository_Upsert_ExecError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("exec error")
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Upsert(context.Background(), &consents.Consent{})
	assert.Error(t, err)
}

func TestRepository_Upsert_Success(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Upsert(context.Background(), &consents.Consent{})
	assert.NoError(t, err)
}

func TestRepository_Revoke_ExecError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("exec error")
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Revoke(context.Background(), "user-1", "app-1")
	assert.Error(t, err)
}

func TestRepository_Revoke_Success(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Revoke(context.Background(), "user-1", "app-1")
	assert.NoError(t, err)
}
