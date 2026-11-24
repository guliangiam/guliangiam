package adapters

import "time"

// Adapter represents a Casbin adapter configuration.
type Adapter struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Description    string
	Type           string // database, file, etc.
	Server         string
	Database       string
	TableName      string
	Username       string
	Password       string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
	// Phase 2: Extended fields
	Owner        string
	UseSameDb    bool
	DatabaseType string
	Host         string
	Port         *int
	DbUser       string
	DbPassword   string
	DatabaseName string
	Policy       string
	IsEnabled    bool
}
