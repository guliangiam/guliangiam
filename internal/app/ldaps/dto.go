package ldaps

import (
	"time"
)

type ListLdapsQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	IsEnabled               *bool
}

type CreateLdapCommand struct {
	OrganizationID, Name, DisplayName, Host string
	Port                                    int
	ServerName, Admin                       string
	BindDN, BindPassword, BaseDN, Filter    string
	Type, AttributeName, PasswordType       string
	FilterFields                            []string
	EnableSsl, AllowSelfSignedCert          bool
	EnableGroups                            bool
	DefaultGroups                           []string
	CustomAttributes                        map[string]string
	IsEnabled                               *bool
	SyncInterval                            int
}

type PatchLdapCommand struct {
	ID                                string
	DisplayName, Host                 *string
	Port                              *int
	ServerName, Admin                 *string
	BindDN, BindPassword, BaseDN      *string
	Filter                            *string
	Type, AttributeName, PasswordType *string
	FilterFields                      []string
	EnableSsl, AllowSelfSignedCert    *bool
	EnableGroups                      *bool
	DefaultGroups                     []string
	CustomAttributes                  map[string]string
	IsEnabled                         *bool
	SyncInterval                      *int
	Revision                          int64
}

type LdapListItem struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	Name           string     `json:"name"`
	DisplayName    string     `json:"displayName"`
	Host           string     `json:"host"`
	Port           int        `json:"port"`
	BaseDN         string     `json:"baseDn"`
	IsEnabled      bool       `json:"isEnabled"`
	LastSyncAt     *time.Time `json:"lastSyncAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type LdapDetail struct {
	LdapListItem
	ServerName          string            `json:"serverName,omitempty"`
	Admin               string            `json:"admin,omitempty"`
	Type                string            `json:"type,omitempty"`
	AttributeName       string            `json:"attributeName,omitempty"`
	PasswordType        string            `json:"passwordType,omitempty"`
	Filter              string            `json:"filter,omitempty"`
	FilterFields        []string          `json:"filterFields,omitempty"`
	EnableSsl           bool              `json:"enableSsl"`
	AllowSelfSignedCert bool              `json:"allowSelfSignedCert"`
	EnableGroups        bool              `json:"enableGroups"`
	DefaultGroups       []string          `json:"defaultGroups,omitempty"`
	CustomAttributes    map[string]string `json:"customAttributes,omitempty"`
	SyncInterval        int               `json:"syncInterval"`
	Revision            int64             `json:"revision"`
}
