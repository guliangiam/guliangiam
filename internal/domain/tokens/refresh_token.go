package tokens

import "time"

type RefreshToken struct {
	ID               string
	AccessTokenID    string
	UserID           string
	Username         string
	OrganizationID   string
	OrganizationSlug string
	ApplicationID    string
	ApplicationSlug  string
	RefreshTokenHash string
	FamilyID         string
	Scopes           []string
	Audience         []string
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Revision         int64
}
