package scopeauthorizer

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
)

type fakeScopeRepo struct {
	scopes []*platformadmins.PlatformAdminScope
	err    error
}

func (r *fakeScopeRepo) GetByID(context.Context, string) (*platformadmins.PlatformAdminScope, error) {
	return nil, errResolverTestUnused
}

// ListByPlatformAdmin 模拟仓储契约：OnlyActive=true 时只返回满足
// sql.go scopeValidity（未过期 && (is_bootstrap || approved_by 非空)）的行。
// 真实过滤在 SQL 层；此处仅让分类逻辑的单测拿到与生产一致的输入。
func (r *fakeScopeRepo) ListByPlatformAdmin(_ context.Context, _ string, opts platformadmins.ScopeListOptions) ([]*platformadmins.PlatformAdminScope, error) {
	if r.err != nil || !opts.OnlyActive {
		return r.scopes, r.err
	}
	now := time.Now()
	active := make([]*platformadmins.PlatformAdminScope, 0, len(r.scopes))
	for _, s := range r.scopes {
		if s == nil || !s.ExpiresAt.After(now) || (!s.IsBootstrap && s.ApprovedBy == nil) {
			continue
		}
		active = append(active, s)
	}
	return active, nil
}

func (r *fakeScopeRepo) Create(context.Context, *platformadmins.PlatformAdminScope) error {
	return errResolverTestUnused
}

func (r *fakeScopeRepo) Approve(context.Context, string, string) error {
	return errResolverTestUnused
}

func (r *fakeScopeRepo) Delete(context.Context, string) error {
	return errResolverTestUnused
}

func (r *fakeScopeRepo) HasActiveScopeForTenant(context.Context, string, string) (bool, error) {
	return false, errResolverTestUnused
}

func TestAllowIfScoped_ResolveAdminScope(t *testing.T) {
	future := time.Now().Add(time.Hour)
	approver := "approver-id"
	targetA, targetB := uuid.NewString(), uuid.NewString()

	adminRepo := &mutablePlatformAdminRepo{admin: &platformadmins.PlatformAdmin{
		ID: "admin-1", IsActive: true,
	}}

	t.Run("all_scope_grants_all_tenants", func(t *testing.T) {
		auth := NewAllowIfScopedScopeAuthorizer(adminRepo, &fakeScopeRepo{scopes: []*platformadmins.PlatformAdminScope{
			{ScopeType: platformadmins.ScopeTypeAll, ApprovedBy: &approver, ExpiresAt: future},
		}}, nil, nil, nil)
		scope, err := auth.ResolveAdminScope(context.Background(), "user-1")
		require.NoError(t, err)
		assert.Equal(t, AdminScopeClassAll, scope.Class)
		assert.True(t, scope.GrantsAllTenants())
	})

	t.Run("targeted_scope_lists_targets_only", func(t *testing.T) {
		auth := NewAllowIfScopedScopeAuthorizer(adminRepo, &fakeScopeRepo{scopes: []*platformadmins.PlatformAdminScope{
			{ScopeType: platformadmins.ScopeTypeTargeted, TargetTenants: []string{targetA, targetB}, ApprovedBy: &approver, ExpiresAt: future},
		}}, nil, nil, nil)
		scope, err := auth.ResolveAdminScope(context.Background(), "user-1")
		require.NoError(t, err)
		assert.Equal(t, AdminScopeClassTargeted, scope.Class)
		require.Len(t, scope.TargetTenants, 2)
		assert.True(t, scope.Covers(uuid.MustParse(targetA)))
		assert.False(t, scope.Covers(uuid.New()))
	})

	t.Run("bootstrap_all_scope_without_approver_is_all", func(t *testing.T) {
		// is_bootstrap 是种子内置 admin 的唯一免审批来源。
		auth := NewAllowIfScopedScopeAuthorizer(adminRepo, &fakeScopeRepo{scopes: []*platformadmins.PlatformAdminScope{
			{ScopeType: platformadmins.ScopeTypeAll, IsBootstrap: true, ExpiresAt: future},
		}}, nil, nil, nil)
		scope, err := auth.ResolveAdminScope(context.Background(), "user-1")
		require.NoError(t, err)
		assert.Equal(t, AdminScopeClassAll, scope.Class)
		assert.True(t, scope.GrantsAllTenants())
	})

	t.Run("unapproved_non_bootstrap_all_scope_is_none", func(t *testing.T) {
		// fail-closed：非 bootstrap 且未审批的 all scope 不得解析为 all。
		auth := NewAllowIfScopedScopeAuthorizer(adminRepo, &fakeScopeRepo{scopes: []*platformadmins.PlatformAdminScope{
			{ScopeType: platformadmins.ScopeTypeAll, ExpiresAt: future},
		}}, nil, nil, nil)
		scope, err := auth.ResolveAdminScope(context.Background(), "user-1")
		require.NoError(t, err)
		assert.Equal(t, AdminScopeClassNone, scope.Class)
		assert.False(t, scope.GrantsAllTenants())
	})

	t.Run("all_scope_without_approver_expired_is_none", func(t *testing.T) {
		// bootstrap 也受「未过期」约束。
		auth := NewAllowIfScopedScopeAuthorizer(adminRepo, &fakeScopeRepo{scopes: []*platformadmins.PlatformAdminScope{
			{ScopeType: platformadmins.ScopeTypeAll, IsBootstrap: true, ExpiresAt: time.Now().Add(-time.Hour)},
		}}, nil, nil, nil)
		scope, err := auth.ResolveAdminScope(context.Background(), "user-1")
		require.NoError(t, err)
		assert.Equal(t, AdminScopeClassNone, scope.Class)
		assert.False(t, scope.GrantsAllTenants())
	})

	t.Run("expired_scope_is_none", func(t *testing.T) {
		auth := NewAllowIfScopedScopeAuthorizer(adminRepo, &fakeScopeRepo{scopes: []*platformadmins.PlatformAdminScope{
			{ScopeType: platformadmins.ScopeTypeTargeted, TargetTenants: []string{targetA}, ApprovedBy: &approver, ExpiresAt: time.Now().Add(-time.Hour)},
		}}, nil, nil, nil)
		scope, err := auth.ResolveAdminScope(context.Background(), "user-1")
		require.NoError(t, err)
		assert.Equal(t, AdminScopeClassNone, scope.Class)
	})

	t.Run("missing_repository_fails_closed", func(t *testing.T) {
		auth := NewAllowIfScopedScopeAuthorizer(nil, nil, nil, nil, nil)
		_, err := auth.ResolveAdminScope(context.Background(), "user-1")
		assert.ErrorIs(t, err, ErrNoRepository)
	})
}
