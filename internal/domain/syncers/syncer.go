package syncers

import "time"

// SyncerType defines the type of external source for synchronization.
type SyncerType string

const (
	SyncerTypeDatabase        SyncerType = "database"
	SyncerTypeLDAP            SyncerType = "ldap"
	SyncerTypeActiveDirectory SyncerType = "active_directory"
	SyncerTypeAzureAD         SyncerType = "azure_ad"
	SyncerTypeGoogleWorkspace SyncerType = "google_workspace"
	SyncerTypeOkta            SyncerType = "okta"
	SyncerTypeKeycloak        SyncerType = "keycloak"
	SyncerTypeDingTalk        SyncerType = "dingtalk"
	SyncerTypeLark            SyncerType = "lark"
	SyncerTypeWeCom           SyncerType = "wecom"
	SyncerTypeSCIM            SyncerType = "scim"
	SyncerTypeAWSIAM          SyncerType = "aws_iam"
	SyncerTypePublicAPI       SyncerType = "public_api"
)

// TableColumn defines the column mapping between external source and GuliangIAM.
type TableColumn struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	GuliangIAMName string   `json:"guliangiamName"`
	IsKey          bool     `json:"isKey"`
	IsHashed       bool     `json:"isHashed"`
	Values         []string `json:"values"`
}

// Syncer represents a synchronization configuration for an external identity source.
type Syncer struct {
	ID               string
	OrganizationID   string
	Name             string
	DisplayName      string
	Type             SyncerType // database, ldap, active_directory, azure_ad, etc.
	DatabaseType     string     // postgres, mysql, mssql, etc. (for database type)
	Host             string
	Port             int
	Database         string
	TableName        string
	Username         string
	Password         string
	SslMode          string // disable, require, verify-ca, verify-full
	SshType          string // empty=disabled, password, key
	SshHost          string
	SshPort          int
	SshUsername      string
	SshPassword      string
	SshCertID        string
	IsEnabled        bool
	SyncInterval     int            // seconds, 0 = manual only
	FieldMapping     string         // JSON field mapping (deprecated, use TableColumns)
	TableColumns     []*TableColumn // column mapping between source and GuliangIAM
	ColumnMapping    string         // JSON column mapping (JSONB)
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
