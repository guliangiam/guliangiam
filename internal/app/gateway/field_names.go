package gateway

// 审计 details 与结构化日志里复用的键名。审计记录与前端按这些键读取，改名
// 会让历史记录的展示与检索断链，因此集中成常量。
const (
	keyServerID = "serverId"
	keySiteID   = "siteId"
	keyOrgID    = "orgId"
	keyName     = "name"
)

// 站点健康检查状态。写入 site.LastHealthStatus 并出现在 API 响应与告警判断里。
const (
	healthStatusHealthy   = "healthy"
	healthStatusUnhealthy = "unhealthy"
	healthStatusUnknown   = "unknown"
)
