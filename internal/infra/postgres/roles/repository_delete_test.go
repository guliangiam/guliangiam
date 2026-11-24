package roles

import (
	"context"
	"testing"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoles_NewRepository(t *testing.T) {
	db := &shared.MockDBTX{}
	repo := NewRepository(db)
	assert.NotNil(t, repo)
}

func TestRoles_Delete_Success(t *testing.T) {
	db := &shared.MockDBTX{}
	db.ExecFn = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		assert.Equal(t, []any{"test-id", "org-1"}, args, "delete must scope by organization")
		return pgconn.NewCommandTag("DELETE 1"), nil
	}
	repo := NewRepository(db)
	err := repo.Delete(context.Background(), "test-id", "org-1")
	require.NoError(t, err)
}

func TestRoles_Delete_Error(t *testing.T) {
	db := &shared.MockDBTX{}
	db.ExecFn = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		return pgconn.CommandTag{}, assert.AnError
	}
	repo := NewRepository(db)
	err := repo.Delete(context.Background(), "test-id", "org-1")
	assert.Error(t, err)
}

func TestRoles_Delete_ForeignTenantNotFound(t *testing.T) {
	db := &shared.MockDBTX{}
	db.ExecFn = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("DELETE 0"), nil
	}
	repo := NewRepository(db)
	err := repo.Delete(context.Background(), "test-id", "org-1")
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}
