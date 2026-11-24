package records

import "time"

// recordRow 是 MySQL records 表的扫描目标。
// 与 PG 版 internal/infra/postgres/records/models.go 同构，
// metadata JSONB 列统一为 []byte（MySQL JSON）。
type recordRow struct {
	ID             string
	OrganizationID string
	ActorID        *string
	Action         string
	ResourceType   string
	ResourceID     *string
	IP             *string
	UserAgent      string
	Surface        string
	RequestID      string
	TraceID        string
	Metadata       []byte
	CreatedAt      time.Time
}
