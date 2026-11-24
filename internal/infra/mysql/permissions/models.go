package permissions

import "time"

// permissionRow 是 MySQL permissions 表的扫描目标。
// 与 PG 版 internal/infra/postgres/permissions/models.go 同构，
// JSONB 列统一为 []byte（MySQL JSON），ApproveTime 保持 *time.Time（可空 TIMESTAMP）。
type permissionRow struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Description    string
	Users          []byte
	Groups         []byte
	Roles          []byte
	Domains        []byte
	ResourceType   string
	Resources      []byte
	Actions        []byte
	Effect         string
	IsEnabled      bool
	Submitter      string
	Approver       string
	State          string
	Model          string
	Adapter        string
	Sorter         string
	Application    string
	ApproveTime    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
