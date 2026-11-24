package enforcers

import "time"

// Enforcer represents a Casbin enforcer instance.
type Enforcer struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Description    string
	ModelID        string
	AdapterID      string
	IsEnabled      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
	// Phase 2: Extended fields
	Owner string
}
