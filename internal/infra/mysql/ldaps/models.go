package ldaps

import "time"

// ldapRow 是 MySQL ldaps 表的扫描目标。27 列。
// FilterFields/DefaultGroups 用 []byte 存 JSON（MySQL JSON 列返回 []byte）。
// CustomAttributes 用 []byte 存 JSON。
type ldapRow struct {
	ID                  string
	OrganizationID      string
	Name                string
	DisplayName         string
	ServerName          string
	Host                string
	Port                int
	Admin               string
	BindDN              string
	BindPassword        string
	BaseDN              string
	Type                string
	AttributeName       string
	PasswordType        string
	Filter              string
	FilterFields        []byte // JSON
	EnableSsl           bool
	AllowSelfSignedCert bool
	EnableGroups        bool
	DefaultGroups       []byte // JSON
	CustomAttributes    []byte // JSON
	IsEnabled           bool
	SyncInterval        int
	LastSyncAt          *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Revision            int64
}
