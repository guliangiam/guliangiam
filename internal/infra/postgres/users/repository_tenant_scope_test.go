package users

import (
	"context"
	"errors"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsers_GetByIDForOrganization_ScopesQuery(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			assert.Contains(t, sql, "organization_id = guliangiam.resolve_org_id($2)")
			require.Len(t, args, 2)
			assert.Equal(t, "user-id", args[0])
			assert.Equal(t, "org-id", args[1])
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(db)

	_, err := repo.GetByIDForOrganization(context.Background(), "user-id", "org-id")

	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

func TestUsers_UpdateForOrganization_ScopesQuery(t *testing.T) {
	calls := 0
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			calls++
			if calls == 1 {
				assert.Contains(t, sql, "organization_id = guliangiam.resolve_org_id($20)")
				require.Len(t, args, 20)
				assert.Equal(t, "org-id", args[19])
			}
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := NewRepository(db)

	err := repo.UpdateForOrganization(context.Background(), &domain.User{ID: "user-id", OrganizationID: "org-id"}, "org-id")

	require.NoError(t, err)
	assert.Equal(t, 6, calls, "one scoped UPDATE + five user_* upserts")
}

func TestUsers_UpdateForOrganization_ForeignRowIsNotFound(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		// Same organization predicate as the UPDATE: a foreign user is invisible
		// to the probe, so the result stays NotFound.
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			assert.Contains(t, sql, "organization_id = guliangiam.resolve_org_id($2)")
			require.Len(t, args, 2)
			assert.Equal(t, "user-id", args[0])
			assert.Equal(t, "org-id", args[1])
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(db)

	err := repo.UpdateForOrganization(context.Background(), &domain.User{ID: "user-id", OrganizationID: "org-id", Revision: 1}, "org-id")

	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// TestUsers_UpdateForOrganization_StaleRevisionIsConflict pins the distinction
// the probe exists for: a zero-row scoped UPDATE on a row that still exists in
// the caller's tenant is an optimistic-lock conflict, not a missing resource.
func TestUsers_UpdateForOrganization_StaleRevisionIsConflict(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
		QueryRowFn: func(context.Context, string, ...any) pgx.Row {
			return shared.NewMockRow([]any{1}, nil)
		},
	}
	repo := NewRepository(db)

	err := repo.UpdateForOrganization(context.Background(), &domain.User{ID: "user-id", OrganizationID: "org-id", Revision: 3}, "org-id")

	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperrors.CodeConflict, appErr.Code)
}

func TestUsers_Delete_ScopesQueryAndForeignRowIsNotFound(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			assert.Contains(t, sql, "organization_id = guliangiam.resolve_org_id($2)")
			require.Len(t, args, 2)
			assert.Equal(t, "user-id", args[0])
			assert.Equal(t, "org-id", args[1])
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	repo := NewRepository(db)

	err := repo.Delete(context.Background(), "user-id", "org-id")

	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}
