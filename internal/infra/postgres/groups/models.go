package groups

import "time"

type groupRow struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Manager        string
	ContactEmail   string
	Type           string
	ParentID       string
	IsTopGroup     bool
	IsEnabled      bool
	Homepage       string
	Users          []byte
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
