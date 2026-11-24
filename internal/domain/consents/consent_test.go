package consents

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConsent_Fields(t *testing.T) {
	now := time.Now()
	consent := Consent{
		ID:               "019test-consent-id",
		UserID:           "019test-user-id",
		Username:         "testuser",
		OrganizationID:   "019test-org-id",
		OrganizationSlug: "test-org",
		ApplicationID:    "019test-app-id",
		ApplicationSlug:  "test-app",
		GrantedScopes:    []string{"read", "write", "profile"},
		CreatedAt:        now,
		UpdatedAt:        now,
		Revision:         1,
	}

	assert.Equal(t, "019test-consent-id", consent.ID)
	assert.Equal(t, "019test-user-id", consent.UserID)
	assert.Len(t, consent.GrantedScopes, 3)
}

func TestConsent_Validate(t *testing.T) {
	t.Run("valid_consent", func(t *testing.T) {
		c := Consent{UserID: "user-001", ApplicationID: "app-001"}
		assert.NoError(t, c.Validate())
	})

	t.Run("missing_userId", func(t *testing.T) {
		c := Consent{ApplicationID: "app-001"}
		assert.Error(t, c.Validate())
	})

	t.Run("missing_applicationId", func(t *testing.T) {
		c := Consent{UserID: "user-001"}
		assert.Error(t, c.Validate())
	})
}

func TestConsent_IsRevoked(t *testing.T) {
	t.Run("not_revoked", func(t *testing.T) {
		c := Consent{UserID: "user-001"}
		assert.False(t, c.IsRevoked())
	})

	t.Run("revoked", func(t *testing.T) {
		now := time.Now()
		c := Consent{UserID: "user-001", RevokedAt: &now}
		assert.True(t, c.IsRevoked())
	})
}

func TestConsent_HasScope(t *testing.T) {
	c := Consent{
		UserID:        "user-001",
		ApplicationID: "app-001",
		GrantedScopes: []string{"openid", "profile", "email"},
	}

	t.Run("has_openid", func(t *testing.T) {
		assert.True(t, c.HasScope("openid"))
	})

	t.Run("has_profile", func(t *testing.T) {
		assert.True(t, c.HasScope("profile"))
	})

	t.Run("missing_scope", func(t *testing.T) {
		assert.False(t, c.HasScope("admin"))
	})

	t.Run("empty_scopes", func(t *testing.T) {
		emptyC := Consent{GrantedScopes: []string{}}
		assert.False(t, emptyC.HasScope("openid"))
	})
}

func TestConsent_OAuthConsentLifecycle(t *testing.T) {
	now := time.Now()

	// Grant consent
	consent := Consent{
		ID:             "consent-1",
		UserID:         "user-1",
		OrganizationID: "org-1",
		ApplicationID:  "app-1",
		GrantedScopes:  []string{"openid", "profile", "email"},
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	assert.False(t, consent.IsRevoked())
	assert.True(t, consent.HasScope("openid"))

	// Revoke
	revokeTime := now.Add(24 * time.Hour)
	consent.RevokedAt = &revokeTime
	assert.True(t, consent.IsRevoked())
}
