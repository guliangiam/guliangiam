package outbox

import (
	"database/sql"
	"time"
)

// outboxRow 是 outbox 表的扫描目标。
// NULL 列（organization_id, locked_by, locked_at, processed_at, last_error）用 sql.Null*。
type outboxRow struct {
	ID             string
	OrganizationID sql.NullString // MySQL CHAR(36) 可 NULL
	AggregateType  string
	AggregateID    string
	EventType      string
	EventVersion   int
	Payload        []byte // MySQL JSON NOT NULL
	Headers        []byte // MySQL JSON NOT NULL
	Status         string
	Attempts       int
	MaxAttempts    int
	AvailableAt    time.Time // MySQL TIMESTAMP NOT NULL
	LockedBy       sql.NullString
	LockedAt       sql.NullTime
	ProcessedAt    sql.NullTime
	LastError      sql.NullString
	CreatedAt      time.Time // MySQL TIMESTAMP NOT NULL
	UpdatedAt      time.Time // MySQL TIMESTAMP NOT NULL
	Revision       int64
}
