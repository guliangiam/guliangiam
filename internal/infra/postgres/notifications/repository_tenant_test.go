package notifications

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestRepository_GetByIDForOrganization_UsesTenantPredicate(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			gotSQL = sql
			gotArgs = args
			return shared.NewMockRow([]any{
				"notif-1", "org-1", "provider-1", "", "", "", "user@example.com",
				"Subject", "Content", "tmpl-1", nil,
				"pending", nil, nil, 0, nil, nil,
			}, nil)
		},
	}

	repo := NewRepository(mockDB)
	n, err := repo.GetByIDForOrganization(context.Background(), "notif-1", "org-1")
	require.NoError(t, err)
	require.NotNil(t, n)
	assert.Equal(t, "org-1", n.OrganizationID)
	assert.Contains(t, gotSQL, "organization_id = guliangiam.resolve_org_id($2)")
	require.Equal(t, []any{"notif-1", "org-1"}, gotArgs)
}

func TestRepository_GetByIDForOrganization_NotFound(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(context.Context, string, ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}

	repo := NewRepository(mockDB)
	_, err := repo.GetByIDForOrganization(context.Background(), "foreign", "org-1")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

func TestRepository_GetByIDForOrganization_QueryError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(context.Context, string, ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("db error"))
		},
	}

	repo := NewRepository(mockDB)
	_, err := repo.GetByIDForOrganization(context.Background(), "notif-1", "org-1")
	require.Error(t, err)
}

func TestRepository_MarkSentForOrganization_UsesTenantPredicate(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	mockDB := &shared.MockDBTX{
		ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			gotSQL = sql
			gotArgs = args
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}

	repo := NewRepository(mockDB)
	err := repo.MarkSentForOrganization(context.Background(), "notif-1", "org-1")
	require.NoError(t, err)
	assert.Contains(t, gotSQL, "organization_id = guliangiam.resolve_org_id($3)")
	require.Len(t, gotArgs, 3)
	assert.Equal(t, "notif-1", gotArgs[0])
	assert.Equal(t, "org-1", gotArgs[2])
}

func TestRepository_MarkSentForOrganization_ZeroRowsNotFound(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}

	repo := NewRepository(mockDB)
	err := repo.MarkSentForOrganization(context.Background(), "foreign", "org-1")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

func TestRepository_MarkSentForOrganization_ExecError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("db error")
		},
	}

	repo := NewRepository(mockDB)
	require.Error(t, repo.MarkSentForOrganization(context.Background(), "notif-1", "org-1"))
}

func TestNotificationTenantQueriesShape(t *testing.T) {
	assert.NotContains(t, queryGetNotificationByID, "WHERE id = $1 AND organization_id")
	assert.Contains(t, queryGetNotificationByIDForOrganization, "organization_id = guliangiam.resolve_org_id($2)")
	assert.NotContains(t, queryMarkSent, "organization_id")
	assert.Contains(t, queryMarkSentForOrganization, "organization_id = guliangiam.resolve_org_id($3)")
}
