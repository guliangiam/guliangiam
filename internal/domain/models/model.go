package models

import "time"

// Model represents a Casbin model definition.
type Model struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Description    string
	ModelText      string // The Casbin model definition text (INI format)
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
	// Phase 2: Extended fields
	Owner     string
	IsEnabled bool
}
