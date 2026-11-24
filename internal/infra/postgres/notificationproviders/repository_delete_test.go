package notificationproviders

import (
	"context"
	"testing"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationproviders_NewRepository(t *testing.T) {
	db := &shared.MockDBTX{}
	repo := NewRepository(db)
	assert.NotNil(t, repo)
}

func TestNotificationproviders_Delete_Success(t *testing.T) {
	db := &shared.MockDBTX{}
	var gotArgs []any
	db.ExecFn = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		gotArgs = args
		return pgconn.NewCommandTag("DELETE 1"), nil
	}
	repo := NewRepository(db)
	require.NoError(t, repo.Delete(context.Background(), "test-id", "org-1"))
	require.Equal(t, []any{"test-id", "org-1"}, gotArgs)
}

func TestNotificationproviders_Delete_Error(t *testing.T) {
	db := &shared.MockDBTX{}
	db.ExecFn = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		return pgconn.CommandTag{}, assert.AnError
	}
	repo := NewRepository(db)
	err := repo.Delete(context.Background(), "test-id", "org-1")
	assert.Error(t, err)
}

func TestNotificationproviders_Delete_ZeroRowsNotFound(t *testing.T) {
	db := &shared.MockDBTX{}
	db.ExecFn = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("DELETE 0"), nil
	}
	repo := NewRepository(db)
	err := repo.Delete(context.Background(), "foreign", "org-1")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}
