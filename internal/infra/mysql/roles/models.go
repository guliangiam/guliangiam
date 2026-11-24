package roles

import "time"

// roleRow 是 MySQL roles 表的扫描目标。
// DDL 所有列均 NOT NULL（参照 db/mysql/V1__guliangiam_schema_init.sql 的 roles 表定义），
// 因此不需要 sql.Null* 类型，保持与 PG 版 internal/infra/postgres/roles/models.go 同构。
type roleRow struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Description    string
	Users          []byte // JSON
	GroupIDs       []byte // JSON
	RoleIDs        []byte // JSON
	Domains        []byte // JSON
	IsEnabled      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
