package captchaproviders

import (
	"context"
	"testing"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaptchaproviders_NewRepository(t *testing.T) {
	db := &shared.MockDBTX{}
	repo := NewRepository(db)
	assert.NotNil(t, repo)
}

func TestCaptchaproviders_Delete_Success(t *testing.T) {
	db := &shared.MockDBTX{}
	db.ExecFn = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("DELETE 1"), nil
	}
	repo := NewRepository(db)
	err := repo.Delete(context.Background(), "test-id", "org-1")
	require.NoError(t, err)
}

func TestCaptchaproviders_Delete_Error(t *testing.T) {
	db := &shared.MockDBTX{}
	db.ExecFn = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		return pgconn.CommandTag{}, assert.AnError
	}
	repo := NewRepository(db)
	err := repo.Delete(context.Background(), "test-id", "org-1")
	assert.Error(t, err)
}

func TestCaptchaproviders_Delete_PassesTrustedOrganization(t *testing.T) {
	var gotArgs []any
	db := &shared.MockDBTX{}
	db.ExecFn = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		gotArgs = args
		return pgconn.NewCommandTag("DELETE 1"), nil
	}
	repo := NewRepository(db)
	err := repo.Delete(context.Background(), "test-id", "org-1")
	require.NoError(t, err)
	require.Len(t, gotArgs, 2)
	require.Equal(t, "test-id", gotArgs[0])
	require.Equal(t, "org-1", gotArgs[1])
}

func TestCaptchaproviders_Delete_ForeignOrganizationIsNotFound(t *testing.T) {
	db := &shared.MockDBTX{}
	db.ExecFn = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("DELETE 0"), nil
	}
	repo := NewRepository(db)
	err := repo.Delete(context.Background(), "test-id", "org-1")
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, apperrors.CodeNotFound, appErr.Code)
}
