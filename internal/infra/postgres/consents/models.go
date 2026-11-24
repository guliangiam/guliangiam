package consents

import "time"

type consentRow struct {
	ID               string
	UserID           string
	Username         string
	OrganizationID   string
	OrganizationSlug string
	ApplicationID    string
	ApplicationSlug  string
	GrantedScopes    []byte
	CreatedAt        time.Time
	UpdatedAt        time.Time
	RevokedAt        *time.Time
	Revision         int64
}
