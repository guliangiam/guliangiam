package invitations

import "time"

type invitationRow struct {
	ID             string
	OrganizationID string
	Email          string
	Code           string
	RoleIDs        []string
	GroupIDs       []string
	Status         string
	ExpiresAt      time.Time
	AcceptedAt     *time.Time
	InvitedBy      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
