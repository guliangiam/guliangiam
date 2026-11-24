package scopeauthorizer

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDenyAllScopeAuthorizer(t *testing.T) {
	auth := NewDenyAllScopeAuthorizer()

	t.Run("system_principal_allowed", func(t *testing.T) {
		req := AuthorizationRequest{
			ActorID:   uuid.New(),
			ActorType: PrincipalSystem,
		}
		result, err := auth.Authorize(context.Background(), req)
		assert.NoError(t, err)
		assert.True(t, result.Allowed)
	})

	t.Run("user_principal_denied", func(t *testing.T) {
		req := AuthorizationRequest{
			ActorID:      uuid.New(),
			ActorType:    PrincipalUser,
			TargetTenant: uuid.New(),
			ResourceType: "users",
			Action:       "read",
		}
		result, err := auth.Authorize(context.Background(), req)
		assert.NoError(t, err)
		assert.False(t, result.Allowed)
		assert.Contains(t, result.Reason, "not enabled")
	})

	t.Run("platform_admin_cross_tenant_denied_until_phase_b", func(t *testing.T) {
		req := AuthorizationRequest{
			ActorID:      uuid.New(),
			ActorType:    PrincipalPlatformAdmin,
			TargetTenant: uuid.New(),
			ResourceType: "applications",
			Action:       "delete",
			RiskLevel:    RiskHigh,
		}
		result, err := auth.Authorize(context.Background(), req)
		assert.NoError(t, err)
		assert.False(t, result.Allowed)
		assert.Contains(t, result.Reason, "not enabled")
	})
}

// TestAllowIfScoped_Gate1ToGate3 测试不依赖 DB 的 Gate（无 repo 也能提前 return）。
// Phase D-Step 3 Gate 顺序调整：same-tenant allow 移到 platform_admins membership 之后，
// 所有 membership 检查需要 pool → 无 repo 时 same-tenant 也 fail-closed（ErrNoRepository）。
// Gate 4+ 需要真实 DB — 留给集成测试。
func TestAllowIfScoped_Gate1ToGate3(t *testing.T) {
	// pool 传 nil — 只有 Gate 1（system bypass）和 Gate 3（非 platform_admin deny）能提前 return
	auth := NewAllowIfScopedScopeAuthorizer(nil, nil, nil, nil, nil)

	t.Run("gate1_system_principal_bypass", func(t *testing.T) {
		req := AuthorizationRequest{ActorType: PrincipalSystem}
		result, err := auth.Authorize(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, result.Allowed)
	})

	t.Run("gate2_riskcritical_nil_pool_errors", func(t *testing.T) {
		// RiskCritical 在 Gate 2，需要 pool 查 ticket → ErrNoRepository
		req := AuthorizationRequest{
			ActorType: PrincipalPlatformAdmin,
			ActorID:   uuid.New(),
			RiskLevel: RiskCritical,
		}
		_, err := auth.Authorize(context.Background(), req)
		assert.ErrorIs(t, err, ErrNoRepository)
	})

	t.Run("gate3_non_platform_admin_cannot_cross_tenant", func(t *testing.T) {
		target := uuid.New()
		for _, principalType := range []PrincipalType{PrincipalUser, PrincipalServiceAccount} {
			req := AuthorizationRequest{
				ActorType:    principalType,
				ActorID:      uuid.New(),
				TargetTenant: target,
			}
			result, err := auth.Authorize(context.Background(), req)
			require.NoError(t, err, "principal type %s should not error", principalType)
			assert.False(t, result.Allowed, "principal type %s should be denied cross-tenant", principalType)
			assert.Contains(t, result.Reason, "cannot cross-tenant")
		}
	})

	t.Run("gate3b_non_platform_admin_same_tenant_also_denied", func(t *testing.T) {
		// 非 platform_admin + same-tenant 也被 Gate 3 拦截（永远不能跨租户 + same-tenant allow 在 membership 之后）
		source := uuid.New()
		req := AuthorizationRequest{
			ActorType:    PrincipalUser,
			ActorID:      uuid.New(),
			SourceTenant: source,
			TargetTenant: source,
		}
		result, err := auth.Authorize(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, result.Allowed)
		assert.Contains(t, result.Reason, "cannot cross-tenant")
	})

	t.Run("gate4_nil_pool_errors_for_platform_admin", func(t *testing.T) {
		// platform_admin + 非 RiskCritical → Gate 3 通过 → Gate 4 pool check → ErrNoRepository
		req := AuthorizationRequest{
			ActorType: PrincipalPlatformAdmin,
			ActorID:   uuid.New(),
			RiskLevel: RiskLow,
		}
		_, err := auth.Authorize(context.Background(), req)
		assert.ErrorIs(t, err, ErrNoRepository)
	})
}
