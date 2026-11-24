// ============================================================================
// Dashboard 领域端口接口与查询聚合 DTO — 归属 domain/dashboard
// ============================================================================
//
// internal/app/dashboard 仅保留 Service（应用层组合逻辑）和对外 JSON 输出 DTO
// （DashboardStats / UserStats / AuthStats / ResourceStats 等 service.go 中定义）。
//
// StatsRepository 端口接口和 UserCountResult / SessionCountResult 等查询聚合
// DTO 统一定义在 internal/domain/dashboard/repository.go（Clean Architecture
// 依赖反转：接口归属 domain，infra/postgres/dashboard 提供实现）。
//
// 本文件重导出 domain 层类型 + 别名，保持 app 层现有 Service 和测试代码
// 的向后兼容（mockStatsRepo 无需改动）。
// ============================================================================

package dashboard

import (
	domaindashboard "github.com/guliangiam/guliangiam/internal/domain/dashboard"
)

// StatsRepository 是查询聚合统计数据的端口接口。
// 归属 domain/dashboard；infra/postgres/dashboard 提供 SQL 实现。
type StatsRepository = domaindashboard.StatsRepository

// 查询聚合 DTO —— 归属 domain/dashboard。
type (
	UserCountResult    = domaindashboard.UserCountResult
	SessionCountResult = domaindashboard.SessionCountResult
	TokenCountResult   = domaindashboard.TokenCountResult
	MfaCoverageResult  = domaindashboard.MfaCoverageResult
	HeatmapPoint       = domaindashboard.HeatmapPoint
	ProviderDistPoint  = domaindashboard.ProviderDistPoint
	TrendPoint         = domaindashboard.TrendPoint
	ActivityPoint      = domaindashboard.ActivityPoint
	OrgUserDistPoint   = domaindashboard.OrgUserDistPoint
)
