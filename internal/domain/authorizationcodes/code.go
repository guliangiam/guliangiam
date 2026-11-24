package authorizationcodes

import (
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type AuthorizationCode struct {
	ID string
	// Code is the plaintext credential. It is populated on the create path only:
	// the table stores just its SHA-256 hash (code_hash), so a code read back from
	// the database leaves this field empty.
	Code                string
	UserID              string
	Username            string
	OrganizationID      string
	OrganizationSlug    string
	ApplicationID       string
	ApplicationSlug     string
	RedirectURI         string
	Scopes              []string
	State               string
	Nonce               string
	CodeHash            string
	CodePrefix          string
	Status              string
	CodeChallenge       string
	CodeChallengeMethod string
	ChallengeMethod     string
	Resource            string
	// Sid is the OIDC session identifier of the session this code was issued for
	// (设计报告 14.12.3). It travels with the code so the token endpoint can put
	// the same value into the ID token: without it the logout token carries a
	// `sid` no relying party can match against anything.
	Sid string
	// AuthTime is when the end user actually completed authentication, including
	// MFA. It is not the code's issue time: a session may have been established
	// long before the code was minted, and OIDC's auth_time carries that earlier
	// moment (设计报告 14.8).
	AuthTime  time.Time
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// Validate checks the authorization code fields for business rule violations.
func (c *AuthorizationCode) Validate() error {
	if c.Code == "" {
		return apperrors.NewInvalidArgument("code is required")
	}
	if c.UserID == "" {
		return apperrors.NewInvalidArgument("userId is required")
	}
	if c.ApplicationID == "" {
		return apperrors.NewInvalidArgument("applicationId is required")
	}
	if c.RedirectURI == "" {
		return apperrors.NewInvalidArgument("redirectUri is required")
	}
	if c.ExpiresAt.IsZero() {
		return apperrors.NewInvalidArgument("expiresAt is required")
	}
	return nil
}

// IsExpired returns true if the authorization code has passed its expiration time.
func (c *AuthorizationCode) IsExpired(now time.Time) bool {
	return now.After(c.ExpiresAt)
}

// IsUsed returns true if the authorization code has already been consumed.
func (c *AuthorizationCode) IsUsed() bool {
	return c.UsedAt != nil
}

// HasPKCE returns true if the code was created with a PKCE code challenge.
func (c *AuthorizationCode) HasPKCE() bool {
	return c.CodeChallenge != "" && c.ChallengeMethod != ""
}

// IsPKCEMethodValid returns true if the PKCE challenge method is S256 (only S256 is allowed).
func (c *AuthorizationCode) IsPKCEMethodValid() bool {
	return c.ChallengeMethod == "S256"
}
