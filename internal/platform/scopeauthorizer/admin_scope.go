package scopeauthorizer

import (
	"context"

	"github.com/google/uuid"
)

// AdminScopeClass 描述平台管理员请求期内的有效跨租户范围。
//
// 它用于让 TenantFilterMiddleware 与 ScopeAuthorizerMiddleware 对「无显式
// target 的 platform_admin 请求」采用同一套语义：只有 all 作用域才能提升为
// RLS super_admin；targeted / none / unknown 一律不提升（fail-closed）。
type AdminScopeClass string

const (
	// AdminScopeClassUnknown 表示未能完成作用域解析（仓库缺失或查询失败）。
	// 与 none 一样不得提升权限。
	AdminScopeClassUnknown AdminScopeClass = "unknown"
	// AdminScopeClassNone 表示管理员没有生效中的作用域。
	AdminScopeClassNone AdminScopeClass = "none"
	// AdminScopeClassTargeted 表示管理员只有定向租户作用域。
	AdminScopeClassTargeted AdminScopeClass = "targeted"
	// AdminScopeClassAll 表示管理员有已审批的 all 作用域。
	AdminScopeClassAll AdminScopeClass = "all"
)

// AdminScope 是平台管理员的有效作用域快照。
type AdminScope struct {
	Class AdminScopeClass
	// TargetTenants 仅在 Class == AdminScopeClassTargeted 时有值。
	TargetTenants []uuid.UUID
}

// GrantsAllTenants 报告该作用域是否允许跨全部租户。
func (s AdminScope) GrantsAllTenants() bool {
	return s.Class == AdminScopeClassAll
}

// Covers 报告作用域是否覆盖指定租户。
func (s AdminScope) Covers(tenantID uuid.UUID) bool {
	if s.GrantsAllTenants() {
		return true
	}
	if s.Class != AdminScopeClassTargeted {
		return false
	}
	for _, target := range s.TargetTenants {
		if target == tenantID {
			return true
		}
	}
	return false
}

// AdminScopeResolver 解析平台管理员的有效作用域。
//
// 它是 ScopeAuthorizer 的可选能力：未实现它的实现（如 DenyAll）不会获得
// 任何 RLS 提升，调用方必须按 fail-closed 处理。
type AdminScopeResolver interface {
	ResolveAdminScope(ctx context.Context, userID string) (AdminScope, error)
}

type adminScopeCtxKey struct{}

// WithAdminScope 把请求期内解析出的管理员作用域写入 context。
func WithAdminScope(ctx context.Context, scope AdminScope) context.Context {
	return context.WithValue(ctx, adminScopeCtxKey{}, scope)
}

// AdminScopeFromContext 读取请求期内解析出的管理员作用域。
// ok=false 表示该请求没有经过作用域分类（普通用户或未挂载 ScopeAuthorizerMiddleware 的链路）。
func AdminScopeFromContext(ctx context.Context) (AdminScope, bool) {
	scope, ok := ctx.Value(adminScopeCtxKey{}).(AdminScope)
	return scope, ok
}

type authorizedTargetCtxKey struct{}

// WithAuthorizedTargetTenant 记录本次请求经 ScopeAuthorizer 放行的显式目标租户。
// 仅目标为受控的 targeted 管理员使用；all 作用域保持原有的全量语义。
func WithAuthorizedTargetTenant(ctx context.Context, tenantID uuid.UUID) context.Context {
	return context.WithValue(ctx, authorizedTargetCtxKey{}, tenantID)
}

// AuthorizedTargetTenantFromContext 读取本次请求经授权的显式目标租户。
func AuthorizedTargetTenantFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(authorizedTargetCtxKey{}).(uuid.UUID)
	return id, ok
}
