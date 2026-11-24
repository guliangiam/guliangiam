package consents

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToConsent(t *testing.T) {
	now := time.Now()

	t.Run("basic_consent", func(t *testing.T) {
		row := consentRow{
			ID:               "consent-001",
			UserID:           "user-001",
			Username:         "testuser",
			OrganizationID:   "org-001",
			OrganizationSlug: "test-org",
			ApplicationID:    "app-001",
			ApplicationSlug:  "test-app",
			GrantedScopes:    []byte(`["openid","profile","email"]`),
			CreatedAt:        now,
			UpdatedAt:        now,
			Revision:         1,
		}

		consent, err := toConsent(row)
		require.NoError(t, err)

		assert.Equal(t, "consent-001", consent.ID)
		assert.Equal(t, "user-001", consent.UserID)
		assert.Equal(t, "org-001", consent.OrganizationID)
		assert.Equal(t, "app-001", consent.ApplicationID)
		assert.Equal(t, int64(1), consent.Revision)
	})

	t.Run("revoked_consent", func(t *testing.T) {
		revokedAt := now.Add(-time.Hour)
		row := consentRow{
			ID:            "consent-002",
			UserID:        "user-001",
			GrantedScopes: []byte(`["openid"]`),
			RevokedAt:     &revokedAt,
			CreatedAt:     now,
			UpdatedAt:     now,
			Revision:      1,
		}

		consent, err := toConsent(row)
		require.NoError(t, err)
		assert.NotNil(t, consent.RevokedAt)
	})

	t.Run("nil_granted_scopes", func(t *testing.T) {
		row := consentRow{
			ID:            "consent-003",
			UserID:        "user-001",
			GrantedScopes: nil,
			CreatedAt:     now,
			UpdatedAt:     now,
			Revision:      1,
		}

		consent, err := toConsent(row)
		require.NoError(t, err)
		assert.Equal(t, "consent-003", consent.ID)
	})
}
