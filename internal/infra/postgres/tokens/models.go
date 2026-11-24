package tokens

import "time"

type tokenRow struct {
	ID                    string
	UserID                string
	Username              string
	OrganizationID        string
	OrganizationSlug      string
	ApplicationID         string
	ApplicationSlug       string
	TokenPrefix           string
	TokenHash             string
	DeviceSecretHash      string
	DeviceSecretExpiresIn int
	Scopes                []string
	Audience              []string
	Metadata              []byte
	ExpiresAt             time.Time
	RevokedAt             *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
	Revision              int64
}

type refreshTokenRow struct {
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
