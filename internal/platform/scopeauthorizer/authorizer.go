// package scopeauthorizer — Phase A-D5: 跨租户作用域授权接口定义。
//
// 设计文档：docs/design/iam/permissions.md §3（双层授权架构）+ §4（跨租户触发条件）
// 评审修订：§8.3 C1-C6 + H1-H7
//
// Phase A 状态：
//
//	本包仅定义接口 + deny-all 默认实现，**尚未接入中间件链**。
//	Phase B 才实现真正的 scope 校验（读 platform_admin_scopes + cross_tenant_operations）。
//	这样做的原因：Casbin matcher 修订（D6）和 impersonate 纳入（D8）都在 D5 之后
//	才落地，deny-all 提前启用会误杀所有请求。
//
// 双层授权架构：
//
//	Layer 1: Casbin（ResourceAuthMiddleware）— resource_type × action 基础授权
//	Layer 2: ScopeAuthorizer — 租户作用域授权（跨租户校验 + 审计）
//
// 请求中的角色类型：
//   - user:              普通组织级用户 → scope_tenants = [session.organization_id]
//   - platform_admin:    平台管理员 → scope_tenants = platform_admin_scopes.target_tenants（或 ['*']）
//   - service_account:   服务账号 → 类似 user，但可能绑定多个 org
//   - system:            系统任务（Worker/Cron）→ skip ScopeAuthorizer，直接 TenantFilterSystem
package scopeauthorizer

import (
	"context"

	"github.com/google/uuid"
)

// PrincipalType 主体类型
type PrincipalType string

const (
	PrincipalUser           PrincipalType = "user"
	PrincipalPlatformAdmin  PrincipalType = "platform_admin"
	PrincipalServiceAccount PrincipalType = "service_account"
	PrincipalSystem         PrincipalType = "system"
)

// RiskLevel 风险级别（跨租户操作需不需要 ticket）
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"      // 只读查询
	RiskMedium   RiskLevel = "medium"   // 单次写操作
	RiskHigh     RiskLevel = "high"     // 删除操作
	RiskCritical RiskLevel = "critical" // 批量操作 / impersonate
)

// AuthorizationRequest ScopeAuthorizer 授权请求
type AuthorizationRequest struct {
	// 主体
	ActorID   uuid.UUID
	ActorType PrincipalType

	// 目标租户（跨租户时有值）
	SourceTenant uuid.UUID // Actor 所属组织（user/platform_admin 可能不属于任何 org）
	TargetTenant uuid.UUID // 要操作的组织

	// 资源 + 动作
	ResourceType string
	Action       string
	RiskLevel    RiskLevel

	// 审批票据（高风险操作必需）
	TicketID uuid.UUID
}

// AuthorizationResult ScopeAuthorizer 授权结果
type AuthorizationResult struct {
	Allowed    bool
	Reason     string   // 拒绝原因 / 允许说明
	ScopeIDs   []string // 本次用到的 platform_admin_scopes.id（审计用）
	TicketUsed string   // 本次用到的 cross_tenant_operations.id（审计用）
}

// TenantContext ScopeAuthorizer 写入 context 的租户上下文
type TenantContext struct {
	Mode         string // "single" | "cross" | "system"
	ScopeTenants []uuid.UUID
	TicketID     uuid.UUID
}

// ScopeAuthorizer 跨租户作用域授权接口
//
// Layer 2 of the dual-layer authorization architecture (docs §2):
//
//	Layer 1: Casbin  Enforce(principal, resource_type, action)
//	Layer 2: ScopeAuthorizer.Authorize(req) → allow + reason
//
// 调用时机：Casbin 放行后、Service 层执行前。
// 作用：校验 platform_admin 的 scope 是否覆盖 target_tenant + 高风险操作是否有 ticket。
// TenantBypassAuditor 记录 RLS 旁路决策（无显式 target 的 all-scope 平台管理员）。
// 实现可选：未实现时调用方跳过审计，但旁路语义不变。
type TenantBypassAuditor interface {
	AuditTenantBypass(ctx context.Context, actorID uuid.UUID, actorType PrincipalType, sourceTenant uuid.UUID, reason string)
}

type ScopeAuthorizer interface {
	// Authorize 执行跨租户作用域校验。
	// 返回 AuthorizationResult（Allowed=true/false + Reason）和 error（系统故障）。
	// 系统故障 → 调用方应 fail-closed（拒绝请求）。
	Authorize(ctx context.Context, req AuthorizationRequest) (AuthorizationResult, error)
}

// DenyAllScopeAuthorizer Phase A 默认实现 — deny-all + audit log-only。
// Phase B 才替换为真正的 scope 校验实现。
//
// 为什么 deny-all？因为 Phase A 还没启用 Casbin matcher 修订（D6），
// 跨租户请求在 Casbin 层就会被 403。ScopeAuthorizer 提前启用没意义。
// Phase B 完成 D6 + D8 后切换为 AllowIfScopedScopeAuthorizer。
type DenyAllScopeAuthorizer struct{}

func NewDenyAllScopeAuthorizer() *DenyAllScopeAuthorizer {
	return &DenyAllScopeAuthorizer{}
}

func (d *DenyAllScopeAuthorizer) Authorize(ctx context.Context, req AuthorizationRequest) (AuthorizationResult, error) {
	// system 主体（Worker/Cron）跳过 ScopeAuthorizer — TenantFilterSystem 已处理
	if req.ActorType == PrincipalSystem {
		return AuthorizationResult{Allowed: true, Reason: "system principal — skip scope check"}, nil
	}

	// Phase A: deny-all + reason 明确说明未启用
	// Phase B 替换为真正的 scope 校验实现
	return AuthorizationResult{
		Allowed: false,
		Reason:  "scope authorization not enabled (Phase A placeholder)",
	}, nil
}
