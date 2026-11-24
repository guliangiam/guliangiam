// Package tenantctx 平台级租户上下文 helper（跨 app 包共享）。
//
// 作用域原语（TenantScope/TenantFilterMode/WithTenantScope/TenantWhereClause…）
// 定义在 scope.go，是「请求/任务上下文里的租户作用域」唯一事实源。
// Postgres 适配层通过 infra/postgres/tenant_filter.go 的别名重导出，仓储不受影响。
//
// ⚠️  LEGACY — 下列函数直接提升 RLS 角色，绕过 ScopeAuthorizer 中间件的跨租户 scope 校验。
// 正确的跨租户路径是：ScopeAuthorizerMiddleware（Casbin 后、TenantFilterMiddleware 前）
// → repository 内部 TenantFilterSystem 自动注入。
//
// 保留本包的 SuperAdmin/System 函数用于：
//  1. Worker / Cron / Outbox handler — 无 HTTP 请求上下文，需要系统级 RLS
//  2. Service 内需要 super_admin RLS 的**本租户**操作（不是跨租户）
//  3. 迁移已完成（B1 批次 1bdc7ab6：Service 层调用点 166→0），函数仅作保留出口；
//     新增业务调用需架构评审，不得用于跨租户请求
package tenantctx

import "context"

// SuperAdmin 将 RLS 上下文提升为 super_admin（绕过所有租户隔离）。
//
// 🔴 LEGACY 用途：service 内部跨租户查询/写入绕过了 ScopeAuthorizer 的作用域校验。
// 保留的合理场景：本租户内需要 super_admin RLS 的操作、Worker/Cron 后台任务、
// Outbox handler 的系统级重试。
func SuperAdmin(ctx context.Context) context.Context {
	return WithTenantScope(ctx, "", "", TenantFilterSuperAdmin)
}

// System 将 RLS 上下文提升为 system（TenantFilterSystem）。
// 语义上区分于 SuperAdmin：框架层系统任务 vs 业务层显式绕过。
func System(ctx context.Context) context.Context {
	return WithTenantScope(ctx, "", "", TenantFilterSystem)
}
