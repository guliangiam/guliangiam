package ldapsync

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/domain/ldaps"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestRepository_GetSyncConfigByIDForOrganization_PassesTrustedOrganization(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			gotSQL = sql
			gotArgs = args
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(db)

	_, err := repo.GetSyncConfigByIDForOrganization(context.Background(), "config-1", "org-1")
	require.Error(t, err)
	assert.Contains(t, gotSQL, "organization_id = guliangiam.resolve_org_id($2)")
	require.Len(t, gotArgs, 2)
	assert.Equal(t, "config-1", gotArgs[0])
	assert.Equal(t, "org-1", gotArgs[1])
}

func TestRepository_GetSyncConfigByIDForOrganization_ForeignIsNotFound(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(db)

	_, err := repo.GetSyncConfigByIDForOrganization(context.Background(), "config-1", "org-foreign")
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

func TestRepository_UpdateSyncConfig_PassesEntityOrganization(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			gotSQL = sql
			gotArgs = args
			return shared.NewMockRow([]any{int64(2)}, nil)
		},
	}
	repo := NewRepository(db)

	config := &ldaps.LdapSyncConfig{
		ID: "config-1", OrganizationID: "org-1", SyncIntervalMinutes: 15,
		BaseDN: "dc=example,dc=com", Enabled: true,
	}
	err := repo.UpdateSyncConfig(context.Background(), config)
	require.NoError(t, err)
	assert.Equal(t, int64(2), config.Revision)
	assert.Contains(t, gotSQL, "organization_id = guliangiam.resolve_org_id($8)")
	require.Len(t, gotArgs, 8)
	assert.Equal(t, "config-1", gotArgs[0])
	assert.Equal(t, "org-1", gotArgs[7], "update must carry the entity organization predicate")
}

func TestRepository_UpdateSyncConfig_ZeroRowsIsNotFound(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
	repo := NewRepository(db)

	err := repo.UpdateSyncConfig(context.Background(), &ldaps.LdapSyncConfig{ID: "config-1", OrganizationID: "org-1"})
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

func TestRepository_DeleteSyncConfig_PassesTrustedOrganization(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			gotSQL = sql
			gotArgs = args
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	repo := NewRepository(db)

	require.NoError(t, repo.DeleteSyncConfig(context.Background(), "config-1", "org-1"))
	assert.Contains(t, gotSQL, "organization_id = guliangiam.resolve_org_id($2)")
	require.Len(t, gotArgs, 2)
	assert.Equal(t, "config-1", gotArgs[0])
	assert.Equal(t, "org-1", gotArgs[1])
}

func TestRepository_DeleteSyncConfig_ZeroRowsIsNotFound(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 0"), nil
		},
	}
	repo := NewRepository(db)

	err := repo.DeleteSyncConfig(context.Background(), "config-1", "org-foreign")
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

func TestRepository_ListSyncTasksForOrganization_PassesTrustedOrganization(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{
		QueryFn: func(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
			gotSQL = sql
			gotArgs = args
			return shared.NewMockRows(nil), nil
		},
	}
	repo := NewRepository(db)

	tasks, err := repo.ListSyncTasksForOrganization(context.Background(), "ldap-1", "org-1", 20)
	require.NoError(t, err)
	assert.Empty(t, tasks)
	assert.Contains(t, gotSQL, "organization_id = guliangiam.resolve_org_id($2)")
	require.Len(t, gotArgs, 3)
	assert.Equal(t, "ldap-1", gotArgs[0])
	assert.Equal(t, "org-1", gotArgs[1])
	assert.Equal(t, 20, gotArgs[2])
}
