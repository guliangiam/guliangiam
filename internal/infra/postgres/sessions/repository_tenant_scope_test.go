package sessions

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestRepository_GetByIDForOrganization_ScopesThroughUsers(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	db := &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			capturedSQL, capturedArgs = sql, args
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(db)

	_, err := repo.GetByIDForOrganization(context.Background(), "sess-1", "org-1")
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	assert.Contains(t, capturedSQL, "u.organization_id = guliangiam.resolve_org_id($2)")
	assert.Equal(t, []any{"sess-1", "org-1"}, capturedArgs)
}

func TestRepository_DeleteForOrganization_ScopesThroughUsers(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL, capturedArgs = sql, args
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	repo := NewRepository(db)

	require.NoError(t, repo.DeleteForOrganization(context.Background(), "sess-1", "org-1"))
	assert.Contains(t, capturedSQL, "user_id IN (SELECT id FROM guliangiam.users WHERE organization_id = guliangiam.resolve_org_id($2))")
	assert.Equal(t, []any{"sess-1", "org-1"}, capturedArgs)
}

func TestRepository_DeleteForOrganization_ZeroRowsNotFound(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 0"), nil
		},
	}
	repo := NewRepository(db)

	err := repo.DeleteForOrganization(context.Background(), "sess-1", "org-1")
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

func TestRepository_RevokeForOrganization_ScopesThroughUsers(t *testing.T) {
	var capturedSQL string
	var capturedArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			capturedSQL, capturedArgs = sql, args
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(db)
	revokedAt := time.Unix(0, 0).UTC()

	require.NoError(t, repo.RevokeForOrganization(context.Background(), "sess-1", "org-1", revokedAt))
	assert.Contains(t, capturedSQL, "user_id IN (SELECT id FROM guliangiam.users WHERE organization_id = guliangiam.resolve_org_id($3))")
	assert.Equal(t, []any{"sess-1", revokedAt, "org-1"}, capturedArgs)
}

func TestRepository_RevokeForOrganization_ForeignRowNotFound(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(db)

	err := repo.RevokeForOrganization(context.Background(), "sess-1", "org-1", time.Unix(0, 0).UTC())
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}
