package syncers

import "time"

// syncerRow 是 MySQL syncers 表的扫描目标。32 列。
// ColumnMapping 是 JSON 列，存为 []byte。
type syncerRow struct {
	ID               string
	OrganizationID   string
	Name             string
	DisplayName      string
	Type             string
	DatabaseType     string
	Host             string
	Port             int
	Database         string
	TableName        string
	Username         string
	Password         string
	SslMode          string
	SshType          string
	SshHost          string
	SshPort          int
	SshUsername      string
	SshPassword      string
	SshCertID        string
	IsEnabled        bool
	SyncInterval     int
	FieldMapping     string
	ColumnMapping    []byte // JSON
	IsReadOnly       bool
	Affinity         string
	AffiliationTable string
	AvatarBaseUrl    string
	ErrorText        string
	LastSyncAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Revision         int64
}
