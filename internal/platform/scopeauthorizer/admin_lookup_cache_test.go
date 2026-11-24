package scopeauthorizer

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	crosstenantops "github.com/guliangiam/guliangiam/internal/domain/cross_tenant_operations"
	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// BE-E-14 回归：Authorize 与 ResolveAdminScope 在请求级缓存内共享 membership 查询。

type coveringScopeRepo struct {
	fakeScopeRepo
}

func (r *coveringScopeRepo) HasActiveScopeForTenant(context.Context, string, string) (bool, error) {
	return true, nil
}

// RiskLow 路径不会触达 ticket 查询；嵌入 nil 接口仅为满足 Gate 4 非 nil 校验。
type stubTicketRepo struct {
	crosstenantops.TicketRepository
}

func (stubTicketRepo) HasApprovedTicket(context.Context, string, string, string, string) (bool, error) {
	return false, nil
}

func crossTenantRequest() AuthorizationRequest {
	return AuthorizationRequest{
		ActorID:      uuid.New(),
		ActorType:    PrincipalPlatformAdmin,
		SourceTenant: uuid.New(),
		TargetTenant: uuid.New(),
		ResourceType: "users",
		Action:       "get",
		RiskLevel:    RiskLow,
	}
}

func newCacheTestAuthorizer(adminRepo *mutablePlatformAdminRepo) *AllowIfScopedScopeAuthorizer {
	approver := "approver-id"
	return NewAllowIfScopedScopeAuthorizer(adminRepo, &coveringScopeRepo{fakeScopeRepo{
		scopes: []*platformadmins.PlatformAdminScope{
			{ScopeType: platformadmins.ScopeTypeAll, ApprovedBy: &approver, ExpiresAt: time.Now().Add(time.Hour)},
		},
	}}, stubTicketRepo{}, nil, nil)
}

func TestAdminLookupCache_SharedBetweenAuthorizeAndResolve(t *testing.T) {
	adminRepo := &mutablePlatformAdminRepo{admin: &platformadmins.PlatformAdmin{ID: "admin-1", IsActive: true}}
	auth := newCacheTestAuthorizer(adminRepo)
	req := crossTenantRequest()

	ctx := WithAdminLookupCache(context.Background())
	result, err := auth.Authorize(ctx, req)
	require.NoError(t, err)
	assert.True(t, result.Allowed)

	scope, err := auth.ResolveAdminScope(ctx, req.ActorID.String())
	require.NoError(t, err)
	assert.Equal(t, AdminScopeClassAll, scope.Class)

	adminRepo.mu.Lock()
	defer adminRepo.mu.Unlock()
	assert.Equal(t, 1, adminRepo.queries, "同一请求内 membership 只查一次")
}

func TestAdminLookupCache_WithoutCacheQueriesTwice(t *testing.T) {
	adminRepo := &mutablePlatformAdminRepo{admin: &platformadmins.PlatformAdmin{ID: "admin-1", IsActive: true}}
	auth := newCacheTestAuthorizer(adminRepo)
	req := crossTenantRequest()

	ctx := context.Background()
	_, err := auth.Authorize(ctx, req)
	require.NoError(t, err)
	_, err = auth.ResolveAdminScope(ctx, req.ActorID.String())
	require.NoError(t, err)

	adminRepo.mu.Lock()
	defer adminRepo.mu.Unlock()
	assert.Equal(t, 2, adminRepo.queries, "未安装缓存时保持原行为")
}

func TestAdminLookupCache_ErrorCachedFailClosed(t *testing.T) {
	adminRepo := &mutablePlatformAdminRepo{admin: nil} // GetByUserID → NotFound
	auth := newCacheTestAuthorizer(adminRepo)
	req := crossTenantRequest()

	ctx := WithAdminLookupCache(context.Background())
	result, err := auth.Authorize(ctx, req)
	require.NoError(t, err)
	assert.False(t, result.Allowed, "membership 查询失败必须 fail-closed")

	_, err = auth.ResolveAdminScope(ctx, req.ActorID.String())
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInternal, apperrors.FromError(err).Code)

	adminRepo.mu.Lock()
	defer adminRepo.mu.Unlock()
	assert.Equal(t, 1, adminRepo.queries, "错误结果同样缓存，请求内口径一致")
}

func TestAdminLookupCache_IdempotentInstall(t *testing.T) {
	ctx := WithAdminLookupCache(context.Background())
	assert.Equal(t, ctx, WithAdminLookupCache(ctx))
}
