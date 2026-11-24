package providers

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

func providerTenantContext(orgID string) context.Context {
	return tenantctx.WithTenantScope(context.Background(), orgID, "", tenantctx.TenantFilterStrict)
}

func requireProviderNotFound(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// TestRepository_GetByID_TenantScoped pins that the by-ID read carries the
// authenticated organization into the SQL predicate. PostgreSQL has RLS today,
// but the predicate keeps the adapter independent of the session role.
func TestRepository_GetByID_TenantScoped(t *testing.T) {
	var captured []any
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			assert.Contains(t, sql, "organization_id = guliangiam.resolve_org_id($2)")
			captured = append([]any(nil), args...)
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}

	_, err := NewRepository(db).GetByID(providerTenantContext("org-1"), "prov-1")
	requireProviderNotFound(t, err)
	require.Len(t, captured, 2)
	assert.Equal(t, "prov-1", captured[0])
	assert.Equal(t, "org-1", captured[1])
}

// TestRepository_GetByID_PlatformScopeSkipsPredicate keeps the explicit
// platform-admin/system path working: those scopes carry no organization, so
// the SQL filter must stay disabled rather than matching an empty value.
func TestRepository_GetByID_PlatformScopeSkipsPredicate(t *testing.T) {
	var captured []any
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
			captured = append([]any(nil), args...)
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}

	_, err := NewRepository(db).GetByID(tenantctx.SuperAdmin(context.Background()), "prov-1")
	requireProviderNotFound(t, err)
	require.Len(t, captured, 2)
	assert.Equal(t, "", captured[1])
}

// TestRepository_GetByIDForOrganization_ExplicitPredicate pins the management
// by-ID read: the trusted tenant arrives as an argument and the SQL carries a
// strict predicate instead of an ambient tenantctx-derived CASE.
func TestRepository_GetByIDForOrganization_ExplicitPredicate(t *testing.T) {
	var captured []any
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			assert.Contains(t, sql, "organization_id = guliangiam.resolve_org_id($2)")
			assert.NotContains(t, sql, "CASE WHEN $2 = ''")
			captured = append([]any(nil), args...)
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}

	_, err := NewRepository(db).GetByIDForOrganization(context.Background(), "prov-1", "org-1")
	requireProviderNotFound(t, err)
	require.Len(t, captured, 2)
	assert.Equal(t, "prov-1", captured[0])
	assert.Equal(t, "org-1", captured[1])
}

// TestRepository_Update_TenantScoped pins the organization argument on the
// optimistic-lock update; the app layer decides ownership before calling it.
func TestRepository_Update_TenantScoped(t *testing.T) {
	var captured []any
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			assert.Contains(t, sql, "organization_id = guliangiam.resolve_org_id($47)")
			captured = append([]any(nil), args...)
			return shared.NewMockRow([]any{int64(2)}, nil)
		},
	}

	provider := &domain.Provider{
		ID: "prov-1", Name: "renamed", Revision: 2,
		Family: domain.FamilyPayment, Kind: domain.Kind("alipay"), Variant: domain.VariantSandbox,
	}
	require.NoError(t, NewRepository(db).Update(providerTenantContext("org-1"), provider))
	require.Len(t, captured, 47)
	assert.Equal(t, "org-1", captured[46])
}

func TestRepository_Delete_TenantScoped(t *testing.T) {
	var captured []any
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			assert.Contains(t, sql, "organization_id = guliangiam.resolve_org_id($2)")
			captured = append([]any(nil), args...)
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}

	require.NoError(t, NewRepository(db).Delete(providerTenantContext("org-1"), "prov-1"))
	require.Len(t, captured, 2)
	assert.Equal(t, "prov-1", captured[0])
	assert.Equal(t, "org-1", captured[1])
}

// TestRepository_Delete_ZeroRowsIsNotFound closes the soft-delete idempotency
// loophole: a row outside the tenant scope must look missing, not silently
// succeed.
func TestRepository_Delete_ZeroRowsIsNotFound(t *testing.T) {
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}

	err := NewRepository(db).Delete(providerTenantContext("org-1"), "prov-1")
	requireProviderNotFound(t, err)
}

func TestRepository_GetConfig_TenantScoped(t *testing.T) {
	var captured []any
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			assert.Contains(t, sql, "EXISTS")
			assert.Contains(t, sql, "organization_id = guliangiam.resolve_org_id($2)")
			captured = append([]any(nil), args...)
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}

	_, err := NewRepository(db).GetConfig(providerTenantContext("org-1"), "prov-1")
	require.NoError(t, err)
	require.Len(t, captured, 2)
	assert.Equal(t, "org-1", captured[1])
}

func TestRepository_ListSecretStatuses_TenantScoped(t *testing.T) {
	var captured []any
	db := &shared.MockDBTX{
		QueryFn: func(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
			assert.Contains(t, sql, "EXISTS")
			assert.Contains(t, sql, "organization_id = guliangiam.resolve_org_id($2)")
			captured = append([]any(nil), args...)
			return shared.NewMockRows(nil), nil
		},
	}

	_, err := NewRepository(db).ListSecretStatuses(providerTenantContext("org-1"), "prov-1")
	require.NoError(t, err)
	require.Len(t, captured, 2)
	assert.Equal(t, "org-1", captured[1])
}
