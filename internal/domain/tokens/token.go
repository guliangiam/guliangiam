package tokens

import (
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Valid grant types for token creation
var validGrantTypes = map[string]bool{
	"authorization_code": true,
	"client_credentials": true,
	"refresh_token":      true,
	"password":           true,
	"urn:ietf:params:oauth:grant-type:device_code": true,
	"urn:ietf:params:oauth:grant-type:jwt-bearer":  true,
}

type Token struct {
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
	Metadata              map[string]any
	ExpiresAt             time.Time
	RevokedAt             *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
	Revision              int64
}

// Validate checks the token fields for business rule violations.
func (t *Token) Validate() error {
	if t.UserID == "" {
		return apperrors.NewInvalidArgument("userId is required")
	}
	if t.TokenHash == "" {
		return apperrors.NewInvalidArgument("tokenHash is required")
	}
	if t.ApplicationID == "" {
		return apperrors.NewInvalidArgument("applicationId is required")
	}
	if t.ExpiresAt.IsZero() {
		return apperrors.NewInvalidArgument("expiresAt is required")
	}
	// Scopes format validation: each scope should be non-empty
	for _, scope := range t.Scopes {
		if scope == "" {
			return apperrors.NewNotFound("scope value cannot be empty")
		}
	}
	return nil
}

// IsExpired returns true if the token has passed its expiration time.
func (t *Token) IsExpired(now time.Time) bool {
	return now.After(t.ExpiresAt)
}

// IsRevoked returns true if the token has been revoked.
func (t *Token) IsRevoked() bool {
	return t.RevokedAt != nil
}
