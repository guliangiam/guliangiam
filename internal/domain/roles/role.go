package roles

import (
	"context"
	"time"
)

type Role struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Description    string
	Users          []string
	GroupIDs       []string
	RoleIDs        []string
	Domains        []string
	IsEnabled      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

type PolicyManager interface {
	SyncRole(ctx context.Context, orgID string, role *Role) error
	RemoveRole(ctx context.Context, orgID string, roleID string) error
}
