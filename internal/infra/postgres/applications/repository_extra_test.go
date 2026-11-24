package applications

import (
	"context"
	"fmt"
	"testing"

	"github.com/guliangiam/guliangiam/internal/domain/applications"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestRepository_GetByClientID_QueryRowError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetByClientID(context.Background(), "client-1")
	assert.Error(t, err)
}

func TestRepository_GetByClientID_NotFoundV2(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetByClientID(context.Background(), "client-1")
	assert.Error(t, err)
}

func TestRepository_GetByOrganizationAndSlug_QueryRowError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetByOrganizationAndSlug(context.Background(), "org-1", "my-app")
	assert.Error(t, err)
}

func TestRepository_GetByOrganizationAndSlug_NotFound(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetByOrganizationAndSlug(context.Background(), "org-1", "my-app")
	assert.Error(t, err)
}

func TestRepository_Delete_ExecError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("exec error")
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Delete(context.Background(), "app-1", "org-1")
	assert.Error(t, err)
}

func TestRepository_Delete_Success(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Delete(context.Background(), "app-1", "org-1")
	assert.NoError(t, err)
}

func TestRepository_ReplaceProviderBindings_ExecError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("exec error")
		},
	}
	repo := NewRepository(mockDB)
	err := repo.ReplaceProviderBindings(context.Background(), "app-1", []applications.ApplicationProviderBinding{})
	assert.Error(t, err)
}

func TestRepository_HasDeletedWithOrgAndSlug_QueryRowError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.HasDeletedWithOrgAndSlug(context.Background(), "org-1", "my-app")
	assert.Error(t, err)
}

func TestRepository_HasDeletedWithOrgAndSlug_Success(t *testing.T) {
	exists := true
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{exists}, nil)
		},
	}
	repo := NewRepository(mockDB)
	result, err := repo.HasDeletedWithOrgAndSlug(context.Background(), "org-1", "my-app")
	assert.NoError(t, err)
	assert.True(t, result)
}
