package roles

import "time"

type roleRow struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Description    string
	Users          []byte
	GroupIDs       []byte
	RoleIDs        []byte
	Domains        []byte
	IsEnabled      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
