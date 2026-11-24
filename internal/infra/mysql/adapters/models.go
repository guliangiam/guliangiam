package adapters

import "time"

// adapterRow 是 MySQL adapters 表的扫描目标。
// 列顺序必须与 sql.go 各 SELECT 严格一致（24 列）。
// Port 列在 PG 版是 *int  nullable，MySQL 版也用 *int。
type adapterRow struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Description    string
	Type           string
	Server         string
	Database       string
	TableName      string
	Username       string
	Password       string
	Owner          string
	UseSameDb      bool
	DatabaseType   string
	Host           string
	Port           *int
	DbUser         string
	DbPassword     string
	DatabaseName   string
	Policy         string
	IsEnabled      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
