package rememberme

import "time"

// Family is a rotating remember-me token family. It pins the tenant, the user
// and the optional application a login may be resumed for. Revocation applies
// to the whole family, never to a single token.
type Family struct {
	ID             string
	OrganizationID string
	UserID         string
	ApplicationID  *string
	ExpiresAt      time.Time
	RevokedAt      *time.Time
	RevokeReason   *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

// Token is one generation inside a family. Only the SHA-256 hex of the
// plaintext is stored; the plaintext is returned to the client exactly once.
type Token struct {
	ID             string
	FamilyID       string
	OrganizationID string
	TokenHash      string
	TokenPrefix    string
	ConsumedAt     *time.Time
	ExpiresAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
