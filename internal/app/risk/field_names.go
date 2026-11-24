package risk

// 审计 details、告警通知模板变量与风险因子 JSON 共用的键名。
// 前端、日志检索、已配置的通知模板与历史因子记录都按同名键取值，改名会让
// 它们断链。
const (
	keyDecisionID = "decisionId"
	keyOrgID      = "orgId"
	keyChannel    = "channel"
	keyScore      = "score"
	keyRevision   = "revision"
)
