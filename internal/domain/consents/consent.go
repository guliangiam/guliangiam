package consents

import (
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type Consent struct {
	ID               string
	UserID           string
	Username         string
	OrganizationID   string
	OrganizationSlug string
	ApplicationID    string
	ApplicationSlug  string
	GrantedScopes    []string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	RevokedAt        *time.Time
	Revision         int64
}

// Validate checks the consent fields for business rule violations.
func (c *Consent) Validate() error {
	if c.UserID == "" {
		return apperrors.NewInvalidArgument("userId is required")
	}
	if c.ApplicationID == "" {
		return apperrors.NewInvalidArgument("applicationId is required")
	}
	return nil
}

// IsRevoked returns true if the consent has been revoked.
func (c *Consent) IsRevoked() bool {
	return c.RevokedAt != nil
}

// HasScope returns true if the consent includes the given scope.
func (c *Consent) HasScope(scope string) bool {
	for _, s := range c.GrantedScopes {
		if s == scope {
			return true
		}
	}
	return false
}
