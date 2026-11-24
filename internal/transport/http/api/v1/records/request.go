package records

import "time"

type CreateRecordRequest struct {
	// OrganizationID, ActorID, IP and UserAgent are accepted for wire
	// compatibility only. The handler overwrites them with the authenticated
	// tenant, the session actor and the observed request metadata; a client
	// cannot supply audit evidence (GAP-011/GAP-012).
	OrganizationID string         `json:"organizationId"`
	ActorID        string         `json:"actorId"`
	Action         string         `json:"action"`
	ResourceType   string         `json:"resourceType"`
	ResourceID     string         `json:"resourceId"`
	IP             string         `json:"ip"`
	UserAgent      string         `json:"userAgent"`
	Metadata       map[string]any `json:"metadata"`
}

type ListRecordsRequest struct {
	Page           int    `json:"page"`
	PageSize       int    `json:"pageSize"`
	OrganizationID string `json:"organizationId"`
	ActorID        string `json:"actorId"`
	Action         string `json:"action"`
	ResourceType   string `json:"resourceType"`
	RequestID      string `json:"requestId"`
	Keyword        string `json:"keyword"`
	SortField      string `json:"sortField"`
	SortOrder      string `json:"sortOrder"`
}

// AuditLogListRequest 是审计中心分类日志 / 登录日志的查询参数。
type AuditLogListRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Category       string
	Result         string
	ApplicationID  string
	Keyword        string
	StartTime      *time.Time
	EndTime        *time.Time
}

type SearchRecordsRequest struct {
	Page           int        `json:"page"`
	PageSize       int        `json:"pageSize"`
	OrganizationID string     `json:"organizationId"`
	ActorID        string     `json:"actorId"`
	Action         string     `json:"action"`
	ResourceType   string     `json:"resourceType"`
	ResourceID     string     `json:"resourceId"`
	RequestID      string     `json:"requestId"`
	Keyword        string     `json:"keyword"`
	SortField      string     `json:"sortField"`
	SortOrder      string     `json:"sortOrder"`
	StartTime      *time.Time `json:"startTime"`
	EndTime        *time.Time `json:"endTime"`
}
