package applications

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// TestRepository_GetByIDForOrganization_Predicate pins the by-ID tenant
// predicate and argument order: id first, organization second, resolved by
// guliangiam.resolve_org_id so slugs and UUIDs both work.
func TestRepository_GetByIDForOrganization_Predicate(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			gotSQL = sql
			gotArgs = args
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}

	_, err := NewRepository(mockDB).GetByIDForOrganization(context.Background(), "app-1", "org-1")

	var notFound *apperrors.Error
	require.ErrorAs(t, err, &notFound)
	assert.Equal(t, apperrors.CodeNotFound, notFound.Code)
	assert.Contains(t, gotSQL, "apps.id = $1::uuid")
	assert.Contains(t, gotSQL, "apps.organization_id = guliangiam.resolve_org_id($2)")
	assert.Equal(t, []any{"app-1", "org-1"}, gotArgs)
}

// TestRepository_Delete_TenantPredicate pins that the main delete carries the
// tenant predicate and short-circuits on 0 rows, so a foreign id never reaches
// the child-table cleanup.
func TestRepository_Delete_TenantPredicate(t *testing.T) {
	var execSQL []string
	var execArgs [][]any
	mockDB := &shared.MockDBTX{
		ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			execSQL = append(execSQL, sql)
			execArgs = append(execArgs, args)
			return pgconn.NewCommandTag("DELETE 0"), nil
		},
	}

	err := NewRepository(mockDB).Delete(context.Background(), "app-1", "org-1")

	var notFound *apperrors.Error
	require.ErrorAs(t, err, &notFound)
	assert.Equal(t, apperrors.CodeNotFound, notFound.Code)
	require.Len(t, execSQL, 1, "0-row scoped delete must not clean up child tables")
	assert.Contains(t, execSQL[0], "organization_id = guliangiam.resolve_org_id($2)")
	assert.Equal(t, []any{"app-1", "org-1"}, execArgs[0])
}

// TestRepository_Update_TenantPredicate pins that the wide UPDATE filters on the
// entity organization, not just the id.
func TestRepository_Update_TenantPredicate(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			gotSQL = sql
			gotArgs = args
			return shared.NewMockRow([]any{int64(2)}, nil)
		},
		ExecFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}

	application := &domain.Application{ID: "app-1", OrganizationID: "org-1", Revision: 1}
	err := NewRepository(mockDB).Update(context.Background(), application)
	require.NoError(t, err)
	assert.Equal(t, int64(2), application.Revision)
	assert.Contains(t, gotSQL, "WHERE id = $19::uuid AND organization_id = guliangiam.resolve_org_id($20)")
	require.Len(t, gotArgs, 20)
	assert.Equal(t, "app-1", gotArgs[18])
	assert.Equal(t, "org-1", gotArgs[19])
}
