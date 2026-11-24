package sessions

import (
	"database/sql"
	"time"
)

// sessionRow 是 MySQL sessions 表的扫描目标，
// 与 PG 版 internal/infra/postgres/sessions/sessionRow 字段一一对应（全链路契约同步）。
// NOT NULL 列用原生类型，可 NULL 列用 sql.Null* 处理。
type sessionRow struct {
	ID                  string
	UserID              string
	Username            string
	OrganizationID      string
	OrganizationSlug    string
	ApplicationID       string
	ApplicationSlug     string
	SessionState        string
	Sid                 sql.NullString // MySQL VARCHAR(100) 可 NULL
	DeviceInfo          []byte         // MySQL JSON NOT NULL
	IPAddress           sql.NullString // MySQL VARCHAR(45) 可 NULL
	UserAgent           sql.NullString // MySQL VARCHAR(100) 可 NULL
	ExpiresAt           time.Time      // MySQL TIMESTAMP NOT NULL
	RevokedAt           sql.NullTime   // MySQL TIMESTAMP 可 NULL
	CreatedAt           time.Time      // MySQL TIMESTAMP NOT NULL
	LastSeenAt          time.Time      // MySQL TIMESTAMP NOT NULL
	UpdatedAt           time.Time      // MySQL TIMESTAMP NOT NULL
	Revision            int64
	AuthenticationLevel string
	AssuranceLevel      string
	IsPrivileged        bool
	LifecycleID         string
	TicketID            string
}
