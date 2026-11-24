package authorizationcodes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestToAuthorizationCode(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		usedAt := now.Add(time.Minute)

		row := authorizationCodeRow{
			ID:               "ac-001",
			UserID:           "user-001",
			Username:         "john",
			OrganizationID:   "org-001",
			OrganizationSlug: "my-org",
			ApplicationID:    "app-001",
			ApplicationSlug:  "my-app",
			RedirectURI:      "https://app.example.com/callback",
			Scopes:           []string{"openid", "profile", "email"},
			State:            "random-state",
			Nonce:            "random-nonce",
			CodeChallenge:    "challenge-value",
			ChallengeMethod:  "S256",
			Resource:         "https://api.example.com",
			Sid:              "gl_sid_from-session",
			ExpiresAt:        now.Add(10 * time.Minute),
			UsedAt:           &usedAt,
			CreatedAt:        now,
		}

		result := toAuthorizationCode(row)

		assert.Equal(t, "ac-001", result.ID)
		assert.Equal(t, "user-001", result.UserID)
		assert.Equal(t, "john", result.Username)
		assert.Equal(t, "org-001", result.OrganizationID)
		assert.Equal(t, "my-org", result.OrganizationSlug)
		assert.Equal(t, "app-001", result.ApplicationID)
		// The token endpoint reads the sid back off the redeemed code to build the
		// ID token, so losing it here would drop the claim silently.
		assert.Equal(t, "gl_sid_from-session", result.Sid)
		assert.Equal(t, []string{"openid", "profile", "email"}, result.Scopes)
		assert.Equal(t, "S256", result.ChallengeMethod)
		assert.Equal(t, "random-nonce", result.Nonce)
		assert.Equal(t, "https://api.example.com", result.Resource)
		assert.Equal(t, &usedAt, result.UsedAt)
		// The plaintext code is never stored, so a row read back must not carry it.
		assert.Empty(t, result.Code)
	})

	t.Run("empty_scopes", func(t *testing.T) {
		now := time.Now()
		row := authorizationCodeRow{
			ID:        "ac-002",
			Scopes:    nil,
			CreatedAt: now,
			ExpiresAt: now,
		}

		result := toAuthorizationCode(row)

		assert.Equal(t, "ac-002", result.ID)
		assert.Nil(t, result.Scopes)
	})
}
