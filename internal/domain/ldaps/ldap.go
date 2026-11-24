package ldaps

import "time"

type Ldap struct {
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
	Type                string // OpenLDAP, ActiveDirectory, FreeIPA
	AttributeName       string
	PasswordType        string // Plain, SSHA, MD5
	Filter              string
	FilterFields        []string
	EnableSsl           bool
	AllowSelfSignedCert bool
	EnableGroups        bool
	DefaultGroups       []string
	CustomAttributes    map[string]string
	IsEnabled           bool
	SyncInterval        int // seconds, 0 = manual only
	LastSyncAt          *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Revision            int64
}
