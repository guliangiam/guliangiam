package tenantctx

import (
	"context"
	"fmt"
)

// 本文件是「请求/任务上下文里的租户作用域」的唯一事实源。
//
// 为什么放在 platform 而不是 infra/postgres：作用域是跨层共享的纯上下文值
// （app service、worker、platform/db 执行器都读它），与具体数据库驱动无关。
// Postgres 适配层通过 internal/infra/postgres/tenant_filter.go 的别名重导出
// 这些符号，29 个既有仓储无需改动；platform 包之间因此不再反向依赖 infra/
// （分层门禁 Rule 7）。

type contextKey string

const tenantScopeKey contextKey = "tenant_scope"

// TenantFilterMode 是租户过滤模式。
type TenantFilterMode string

const (
	// TenantFilterStrict 严格模式：必须有 organization_id，否则 panic。
	TenantFilterStrict TenantFilterMode = "strict"
	// TenantFilterOptional 可选模式：有 org_id 就过滤，没有就跳过。
	TenantFilterOptional TenantFilterMode = "optional"
	// TenantFilterSuperAdmin 业务层显式绕过：Rls 映射到 super_admin role，绕过所有租户隔离。
	TenantFilterSuperAdmin TenantFilterMode = "super_admin"
	// TenantFilterSystem 框架层系统任务：Worker/Cron/后台任务入口统一注入。
	TenantFilterSystem TenantFilterMode = "system"
	// TenantFilterCross 显式跨租户（ScopeAuthorizer 放行后的单一目标租户）：
	// RLS 固定 role=member + 非空 target 组织，只允许访问该租户，不再落入
	// system/super_admin 空组织全库旁路（OPEN-API-42 C1）。
	TenantFilterCross TenantFilterMode = "cross"
)

// TenantScope 是上下文中的租户作用域值。
type TenantScope struct {
	OrganizationID   string
	OrganizationName string
	Mode             TenantFilterMode
}

// WithTenantScope 把租户作用域写入 context。
func WithTenantScope(ctx context.Context, orgID, orgName string, mode TenantFilterMode) context.Context {
	return context.WithValue(ctx, tenantScopeKey, TenantScope{
		OrganizationID:   orgID,
		OrganizationName: orgName,
		Mode:             mode,
	})
}

// TenantScopeFromContext 读取 context 中的租户作用域（缺省为零值 = 可选模式）。
func TenantScopeFromContext(ctx context.Context) TenantScope {
	scope, ok := ctx.Value(tenantScopeKey).(TenantScope)
	if !ok {
		return TenantScope{}
	}
	return scope
}

// TenantWhereClause 生成 WHERE 子句片段与参数；super_admin/system 返回空。
func TenantWhereClause(ctx context.Context, column string) (clause string, arg interface{}) {
	scope := TenantScopeFromContext(ctx)

	switch scope.Mode {
	case TenantFilterSuperAdmin, TenantFilterSystem:
		return "", nil
	case TenantFilterStrict, TenantFilterCross:
		if scope.OrganizationID == "" && scope.OrganizationName == "" {
			panic(fmt.Sprintf("tenant_filter: strict/cross mode requires non-empty organization ID, column=%s", column))
		}
	case TenantFilterOptional:
		if scope.OrganizationID == "" && scope.OrganizationName == "" {
			return "", nil
		}
	default:
		if scope.OrganizationID == "" && scope.OrganizationName == "" {
			return "", nil
		}
	}

	if scope.OrganizationID != "" {
		return fmt.Sprintf("%s = $1", column), scope.OrganizationID
	}
	return fmt.Sprintf("%s = $1", column), scope.OrganizationName
}

// TenantAndClause 生成 AND 连接子句片段与参数；super_admin/system 返回空。
func TenantAndClause(ctx context.Context, column string, nextParamIndex int) (clause string, arg interface{}) {
	scope := TenantScopeFromContext(ctx)

	switch scope.Mode {
	case TenantFilterSuperAdmin, TenantFilterSystem:
		return "", nil
	case TenantFilterStrict, TenantFilterCross:
		if scope.OrganizationID == "" && scope.OrganizationName == "" {
			panic(fmt.Sprintf("tenant_filter: strict/cross mode requires non-empty organization ID, column=%s", column))
		}
	case TenantFilterOptional:
		if scope.OrganizationID == "" && scope.OrganizationName == "" {
			return "", nil
		}
	default:
		if scope.OrganizationID == "" && scope.OrganizationName == "" {
			return "", nil
		}
	}

	placeholder := fmt.Sprintf("$%d", nextParamIndex)
	if scope.OrganizationID != "" {
		return fmt.Sprintf("AND %s = %s", column, placeholder), scope.OrganizationID
	}
	return fmt.Sprintf("AND %s = %s", column, placeholder), scope.OrganizationName
}
