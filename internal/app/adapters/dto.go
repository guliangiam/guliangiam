package adapters

import "time"

type AdapterListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	Type           string    `json:"type"`
	Server         string    `json:"server,omitempty"`
	Database       string    `json:"database,omitempty"`
	Owner          string    `json:"owner,omitempty"`
	UseSameDb      bool      `json:"useSameDb"`
	IsEnabled      bool      `json:"isEnabled"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type AdapterDetail struct {
	AdapterListItem
	Description      string `json:"description,omitempty"`
	TableName        string `json:"tableName,omitempty"`
	DatabaseType     string `json:"databaseType,omitempty"`
	Host             string `json:"host,omitempty"`
	Port             *int   `json:"port,omitempty"`
	DbUser           string `json:"dbUser,omitempty"`
	DbPasswordSet    bool   `json:"dbPasswordSet"`
	DbPasswordMasked string `json:"dbPasswordMasked,omitempty"`
	DatabaseName     string `json:"databaseName,omitempty"`
	Policy           string `json:"policy,omitempty"`
	Revision         int64  `json:"revision"`
}
