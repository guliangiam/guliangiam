package providers

import (
	"context"
	"testing"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviders_NewRepository(t *testing.T) {
	db := &shared.MockDBTX{}
	repo := NewRepository(db)
	assert.NotNil(t, repo)
}

func TestProviders_Delete_Success(t *testing.T) {
	db := &shared.MockDBTX{}
	db.ExecFn = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("DELETE 1"), nil
	}
	repo := NewRepository(db)
	err := repo.Delete(context.Background(), "test-id")
	require.NoError(t, err)
}

func TestProviders_Delete_Error(t *testing.T) {
	db := &shared.MockDBTX{}
	db.ExecFn = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		return pgconn.CommandTag{}, assert.AnError
	}
	repo := NewRepository(db)
	err := repo.Delete(context.Background(), "test-id")
	assert.Error(t, err)
}
