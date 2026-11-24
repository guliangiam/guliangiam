package syncers

import (
	"time"
)

type ListSyncersQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	IsEnabled               *bool
}
type CreateSyncerCommand struct {
	OrganizationID, Name, DisplayName, Type, DatabaseType, Host string
	Port                                                        int
	Database, TableName, Username, Password, SslMode            string
	SshType, SshHost                                            string
	SshPort                                                     int
	SshUsername, SshPassword, SshCertID                         string
	IsEnabled                                                   *bool
	SyncInterval                                                int
	FieldMapping                                                string
	ColumnMapping                                               string
	IsReadOnly                                                  bool
	Affinity, AffiliationTable, AvatarBaseUrl, ErrorText        string
}
type PatchSyncerCommand struct {
	ID                         string
	DisplayName, Host          *string
	Port                       *int
	Database, TableName        *string
	Username, Password         *string
	IsEnabled                  *bool
	SyncInterval               *int
	FieldMapping               *string
	DatabaseType               *string
	SslMode                    *string
	SshType, SshHost           *string
	SshPort                    *int
	SshUsername, SshPassword   *string
	SshCertID                  *string
	ColumnMapping              *string
	IsReadOnly                 *bool
	Affinity, AffiliationTable *string
	AvatarBaseUrl, ErrorText   *string
	Revision                   int64
}

type SyncerListItem struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	Name           string     `json:"name"`
	DisplayName    string     `json:"displayName"`
	Type           string     `json:"type"`
	Host           string     `json:"host"`
	Port           int        `json:"port"`
	IsEnabled      bool       `json:"isEnabled"`
	SyncInterval   int        `json:"syncInterval"`
	LastSyncAt     *time.Time `json:"lastSyncAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}
type SyncerDetail struct {
	SyncerListItem
	DatabaseType     string `json:"databaseType,omitempty"`
	Database         string `json:"database,omitempty"`
	TableName        string `json:"tableName,omitempty"`
	SslMode          string `json:"sslMode,omitempty"`
	SshType          string `json:"sshType,omitempty"`
	SshHost          string `json:"sshHost,omitempty"`
	SshPort          int    `json:"sshPort,omitempty"`
	SshUsername      string `json:"sshUsername,omitempty"`
	SshCertID        string `json:"sshCertId,omitempty"`
	FieldMapping     string `json:"fieldMapping,omitempty"`
	ColumnMapping    string `json:"columnMapping,omitempty"`
	IsReadOnly       bool   `json:"isReadOnly"`
	Affinity         string `json:"affinity,omitempty"`
	AffiliationTable string `json:"affiliationTable,omitempty"`
	AvatarBaseUrl    string `json:"avatarBaseUrl,omitempty"`
	ErrorText        string `json:"errorText,omitempty"`
	Revision         int64  `json:"revision"`
}
