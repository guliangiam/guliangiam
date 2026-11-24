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

func TestRepository_GetByOrganizationAndSlug_Error(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.GetByOrganizationAndSlug(context.Background(), "org-1", "myapp")
	assert.Error(t, err)
}

func TestRepository_HasDeletedWithOrgAndSlug_Error(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.HasDeletedWithOrgAndSlug(context.Background(), "org-1", "myapp")
	assert.Error(t, err)
}

func TestRepository_HasDeletedWithOrgAndSlug_True(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{true}, nil)
		},
	}
	repo := NewRepository(mockDB)
	result, err := repo.HasDeletedWithOrgAndSlug(context.Background(), "org-1", "myapp")
	assert.NoError(t, err)
	assert.True(t, result)
}

func TestRepository_HasDeletedWithOrgAndSlug_False(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow([]any{false}, nil)
		},
	}
	repo := NewRepository(mockDB)
	result, err := repo.HasDeletedWithOrgAndSlug(context.Background(), "org-1", "myapp")
	assert.NoError(t, err)
	assert.False(t, result)
}

func TestRepository_ReplaceProviderBindings_DeleteError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("delete error")
		},
	}
	repo := NewRepository(mockDB)
	err := repo.ReplaceProviderBindings(context.Background(), "app-1", nil)
	assert.Error(t, err)
}

func TestRepository_ReplaceProviderBindings_Success(t *testing.T) {
	callCount := 0
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			callCount++
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	err := repo.ReplaceProviderBindings(context.Background(), "app-1", nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestRepository_Update_QueryRowError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}
	repo := NewRepository(mockDB)
	err := repo.Update(context.Background(), &applications.Application{
		ID: "app-1", OrganizationID: "org-1", DisplayName: "Test", Slug: "test", ClientID: "client-1", Revision: 1,
	})
	assert.Error(t, err)
}

func TestRepository_listProviderBindings_QueryError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, fmt.Errorf("query error")
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.listProviderBindings(context.Background(), "app-1")
	assert.Error(t, err)
}

func TestRepository_batchListProviderBindings_QueryError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, fmt.Errorf("query error")
		},
	}
	repo := NewRepository(mockDB)
	_, err := repo.batchListProviderBindings(context.Background(), []string{"app-1", "app-2"})
	assert.Error(t, err)
}

func TestRepository_batchListProviderBindings_EmptyIDs(t *testing.T) {
	mockDB := &shared.MockDBTX{}
	repo := NewRepository(mockDB)
	result, err := repo.batchListProviderBindings(context.Background(), nil)
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestMax(t *testing.T) {
	assert.Equal(t, 5, maxInt(5, 0))   // value > 0, returns value
	assert.Equal(t, 10, maxInt(0, 10)) // value <= 0, returns fallback
	assert.Equal(t, 5, maxInt(-1, 5))  // value <= 0, returns fallback
}

func TestNewRepository(t *testing.T) {
	mockDB := &shared.MockDBTX{}
	repo := NewRepository(mockDB)
	assert.NotNil(t, repo)
}

func TestRepository_withinTransaction_NoDBTX(t *testing.T) {
	repo := Repository{}
	err := repo.withinTransaction(context.Background(), func(ctx context.Context, tx shared.DBTX) error {
		return nil
	})
	assert.NoError(t, err)
}
