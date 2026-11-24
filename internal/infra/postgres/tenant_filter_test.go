package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithTenantScope(t *testing.T) {
	t.Run("stores_scope_in_context", func(t *testing.T) {
		ctx := WithTenantScope(context.Background(), "org-001", "test-org", TenantFilterStrict)
		scope := TenantScopeFromContext(ctx)

		assert.Equal(t, "org-001", scope.OrganizationID)
		assert.Equal(t, "test-org", scope.OrganizationName)
		assert.Equal(t, TenantFilterStrict, scope.Mode)
	})
}

func TestTenantScopeFromContext(t *testing.T) {
	t.Run("missing_scope_returns_zero", func(t *testing.T) {
		scope := TenantScopeFromContext(context.Background())
		assert.Equal(t, "", scope.OrganizationID)
		assert.Equal(t, TenantFilterMode(""), scope.Mode)
	})
}

func TestTenantWhereClause(t *testing.T) {
	t.Run("strict_mode_with_org_id", func(t *testing.T) {
		ctx := WithTenantScope(context.Background(), "org-001", "", TenantFilterStrict)
		clause, arg := TenantWhereClause(ctx, "organization_id")

		assert.Equal(t, "organization_id = $1", clause)
		assert.Equal(t, "org-001", arg)
	})

	t.Run("strict_mode_with_org_name", func(t *testing.T) {
		ctx := WithTenantScope(context.Background(), "", "test-org", TenantFilterStrict)
		clause, arg := TenantWhereClause(ctx, "organization_id")

		assert.Equal(t, "organization_id = $1", clause)
		assert.Equal(t, "test-org", arg)
	})

	t.Run("super_admin_mode_returns_empty", func(t *testing.T) {
		ctx := WithTenantScope(context.Background(), "", "", TenantFilterSuperAdmin)
		clause, arg := TenantWhereClause(ctx, "organization_id")

		assert.Equal(t, "", clause)
		assert.Nil(t, arg)
	})

	t.Run("system_mode_returns_empty_same_as_super_admin", func(t *testing.T) {
		// TenantFilterSystem = 框架层 Worker/Cron 统一注入
		// 与 TenantFilterSuperAdmin RLS 效果相同（跳过租户过滤）
		ctx := WithTenantScope(context.Background(), "", "", TenantFilterSystem)
		clause, arg := TenantWhereClause(ctx, "organization_id")

		assert.Equal(t, "", clause)
		assert.Nil(t, arg)
	})

	t.Run("optional_mode_with_empty_org_returns_empty", func(t *testing.T) {
		ctx := WithTenantScope(context.Background(), "", "", TenantFilterOptional)
		clause, arg := TenantWhereClause(ctx, "organization_id")

		assert.Equal(t, "", clause)
		assert.Nil(t, arg)
	})

	t.Run("optional_mode_with_org_id", func(t *testing.T) {
		ctx := WithTenantScope(context.Background(), "org-001", "", TenantFilterOptional)
		clause, arg := TenantWhereClause(ctx, "organization_id")

		assert.Equal(t, "organization_id = $1", clause)
		assert.Equal(t, "org-001", arg)
	})
}

func TestTenantWhereClause_StrictModeEmptyOrg_Panics(t *testing.T) {
	ctx := WithTenantScope(context.Background(), "", "", TenantFilterStrict)

	assert.Panics(t, func() {
		TenantWhereClause(ctx, "organization_id")
	}, "strict mode with empty org should panic")
}

func TestTenantAndClause(t *testing.T) {
	t.Run("strict_mode_with_org_id", func(t *testing.T) {
		ctx := WithTenantScope(context.Background(), "org-001", "", TenantFilterStrict)
		clause, arg := TenantAndClause(ctx, "organization_id", 2)

		assert.Equal(t, "AND organization_id = $2", clause)
		assert.Equal(t, "org-001", arg)
	})

	t.Run("super_admin_mode_returns_empty", func(t *testing.T) {
		ctx := WithTenantScope(context.Background(), "", "", TenantFilterSuperAdmin)
		clause, arg := TenantAndClause(ctx, "organization_id", 1)

		assert.Equal(t, "", clause)
		assert.Nil(t, arg)
	})

	t.Run("system_mode_returns_empty_same_as_super_admin", func(t *testing.T) {
		ctx := WithTenantScope(context.Background(), "", "", TenantFilterSystem)
		clause, arg := TenantAndClause(ctx, "organization_id", 1)

		assert.Equal(t, "", clause)
		assert.Nil(t, arg)
	})

	t.Run("optional_mode_with_empty_org_returns_empty", func(t *testing.T) {
		ctx := WithTenantScope(context.Background(), "", "", TenantFilterOptional)
		clause, arg := TenantAndClause(ctx, "organization_id", 3)

		assert.Equal(t, "", clause)
		assert.Nil(t, arg)
	})

	t.Run("with_custom_param_index", func(t *testing.T) {
		ctx := WithTenantScope(context.Background(), "org-001", "", TenantFilterStrict)
		clause, arg := TenantAndClause(ctx, "organization_id", 5)

		assert.Equal(t, "AND organization_id = $5", clause)
		assert.Equal(t, "org-001", arg)
	})
}

func TestTenantAndClause_StrictModeEmptyOrg_Panics(t *testing.T) {
	ctx := WithTenantScope(context.Background(), "", "", TenantFilterStrict)

	assert.Panics(t, func() {
		TenantAndClause(ctx, "organization_id", 1)
	}, "strict mode with empty org should panic")
}
