package records

import (
	"context"
	"time"
)

// AuditLogCategory 是审计中心对日志的业务分类。
type AuditLogCategory string

const (
	AuditCategoryLogin     AuditLogCategory = "login"
	AuditCategoryOperation AuditLogCategory = "operation"
	AuditCategoryError     AuditLogCategory = "error"
)

// AuditLogResult 是审计结果。
type AuditLogResult string

const (
	AuditResultSuccess AuditLogResult = "success"
	AuditResultFailure AuditLogResult = "failure"
)

// AuditLogQuery 是审计中心分类日志查询条件。
//
// 分类与结果通过 action 约定推导，不新增表列：
//   - 登录：action LIKE 'auth.login.%'
//   - 错误：action LIKE '%.failed' OR action LIKE 'error.%'
//   - 操作：其余
type AuditLogQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Category       AuditLogCategory
	Result         AuditLogResult
	ApplicationID  string
	Keyword        string
	StartTime      *time.Time
	EndTime        *time.Time
}

// LoginLogEntry 是登录日志视图：成功来自 sessions，失败来自审计记录。
type LoginLogEntry struct {
	ID             string
	Result         AuditLogResult
	Username       string
	OrganizationID string
	ApplicationID  string
	IP             string
	UserAgent      string
	Reason         string
	CreatedAt      time.Time
}

// LoginLogQuery 是登录日志查询条件。
type LoginLogQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	ApplicationID  string
	Result         AuditLogResult
	Keyword        string
	StartTime      *time.Time
	EndTime        *time.Time
}

// AuditLogReader 由支持分类审计查询的仓储实现。
//
// 单独定义而不扩展 Repository，避免所有仓储实现与 mock 被动升级；
// 应用层通过类型断言探测能力，不支持时返回明确错误。
type AuditLogReader interface {
	ListAuditLogs(ctx context.Context, query AuditLogQuery) ([]*Record, int64, error)
	ListLoginLogs(ctx context.Context, query LoginLogQuery) ([]*LoginLogEntry, int64, error)
}
