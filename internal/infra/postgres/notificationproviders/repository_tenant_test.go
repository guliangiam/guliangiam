package notificationproviders

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
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
				"np-001", "org-001", "ses", "email", "https://hooks.example.com",
				"", []byte(`{}`), true, nil, nil, int64(1),
			}, nil)
		},
	}

	repo := NewRepository(mockDB)
	provider, err := repo.GetByIDForOrganization(context.Background(), "np-001", "org-001")
	require.NoError(t, err)
	require.NotNil(t, provider)
	assert.Equal(t, "org-001", provider.OrganizationID)
	assert.Contains(t, gotSQL, "organization_id = guliangiam.resolve_org_id($2)")
	require.Equal(t, []any{"np-001", "org-001"}, gotArgs)
}

func TestRepository_GetByIDForOrganization_NotFound(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryRowFn: func(context.Context, string, ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}

	repo := NewRepository(mockDB)
	_, err := repo.GetByIDForOrganization(context.Background(), "foreign", "org-001")
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
	_, err := repo.GetByIDForOrganization(context.Background(), "np-001", "org-001")
	require.Error(t, err)
}

func TestNotificationProviderTenantQueriesShape(t *testing.T) {
	assert.NotContains(t, queryGetNotificationProviderByID, "organization_id =")
	assert.Contains(t, queryGetNotificationProviderByIDForOrganization, "organization_id = guliangiam.resolve_org_id($2)")
	assert.Contains(t, queryUpdateNotificationProvider, "organization_id = guliangiam.resolve_org_id($10)")
	assert.Contains(t, queryDeleteNotificationProvider, "organization_id = guliangiam.resolve_org_id($2)")
}
