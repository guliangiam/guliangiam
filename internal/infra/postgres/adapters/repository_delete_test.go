package adapters

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestAdapters_NewRepository(t *testing.T) {
	db := &shared.MockDBTX{}
	repo := NewRepository(db)
	assert.NotNil(t, repo)
}

func TestAdapters_Delete_Success(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{}
	db.ExecFn = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		gotSQL, gotArgs = sql, args
		return pgconn.NewCommandTag("DELETE 1"), nil
	}
	repo := NewRepository(db)
	err := repo.Delete(context.Background(), "test-id", "test-org")
	require.NoError(t, err)

	assert.Contains(t, gotSQL, "organization_id = guliangiam.resolve_org_id($2)")
	assert.Equal(t, []any{"test-id", "test-org"}, gotArgs)
}

func TestAdapters_Delete_Error(t *testing.T) {
	db := &shared.MockDBTX{}
	db.ExecFn = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		return pgconn.CommandTag{}, assert.AnError
	}
	repo := NewRepository(db)
	err := repo.Delete(context.Background(), "test-id", "test-org")
	assert.Error(t, err)
}

// TestAdapters_Delete_ZeroRowsIsNotFound pins that a delete which matched no
// row (missing or foreign tenant) is reported as not-found, never as success.
func TestAdapters_Delete_ZeroRowsIsNotFound(t *testing.T) {
	db := &shared.MockDBTX{}
	db.ExecFn = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("DELETE 0"), nil
	}
	repo := NewRepository(db)
	err := repo.Delete(context.Background(), "test-id", "test-org")
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}
