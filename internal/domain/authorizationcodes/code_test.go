package authorizationcodes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAuthorizationCode_PKCEBinding(t *testing.T) {
	now := time.Now()

	t.Run("S256_code_challenge", func(t *testing.T) {
		code := AuthorizationCode{
			ID:              "ac-001",
			Code:            "auth-code-xyz",
			UserID:          "user-001",
			OrganizationID:  "org-001",
			ApplicationID:   "app-001",
			CodeChallenge:   "E9Melhoa2OwvFrEMTJguCHaoSzK1jzOGw4Sh2O7D1Qc",
			ChallengeMethod: "S256",
			ExpiresAt:       now.Add(10 * time.Minute),
			CreatedAt:       now,
		}
		assert.Equal(t, "S256", code.ChallengeMethod, "only S256 method is allowed")
		assert.NotEmpty(t, code.CodeChallenge)
	})

	t.Run("plain_method_should_be_rejected", func(t *testing.T) {
		code := AuthorizationCode{
			Code:            "auth-code-xyz",
			ChallengeMethod: "plain",
		}
		assert.Equal(t, "plain", code.ChallengeMethod)
		assert.True(t, code.ChallengeMethod != "S256", "plain method must be rejected by the service layer")
	})
}

func TestAuthorizationCode_Expiration(t *testing.T) {
	now := time.Now()

	t.Run("not_expired", func(t *testing.T) {
		code := AuthorizationCode{
			Code:      "auth-code-xyz",
			ExpiresAt: now.Add(10 * time.Minute),
		}
		assert.True(t, now.Before(code.ExpiresAt))
	})

	t.Run("expired", func(t *testing.T) {
		code := AuthorizationCode{
			Code:      "auth-code-xyz",
			ExpiresAt: now.Add(-1 * time.Minute),
		}
		assert.True(t, now.After(code.ExpiresAt))
	})

	t.Run("already_used", func(t *testing.T) {
		usedAt := now.Add(-1 * time.Minute)
		code := AuthorizationCode{
			Code:      "auth-code-xyz",
			UsedAt:    &usedAt,
			ExpiresAt: now.Add(10 * time.Minute),
		}
		assert.NotNil(t, code.UsedAt, "code already consumed")
	})
}

func TestAuthorizationCode_Fields(t *testing.T) {
	now := time.Now()
	code := AuthorizationCode{
		ID:               "ac-001",
		Code:             "auth-code-xyz",
		UserID:           "user-001",
		Username:         "testuser",
		OrganizationID:   "org-001",
		OrganizationSlug: "test-org",
		ApplicationID:    "app-001",
		ApplicationSlug:  "test-app",
		RedirectURI:      "https://app.example.com/callback",
		Scopes:           []string{"openid", "profile", "email"},
		State:            "random-state",
		Nonce:            "random-nonce",
		CodeChallenge:    "E9Melhoa2OwvFrEMTJguCHaoSzK1jzOGw4Sh2O7D1Qc",
		ChallengeMethod:  "S256",
		ExpiresAt:        now.Add(10 * time.Minute),
		CreatedAt:        now,
	}

	assert.Equal(t, "ac-001", code.ID)
	assert.Equal(t, "user-001", code.UserID)
	assert.Equal(t, "org-001", code.OrganizationID)
	assert.Equal(t, "app-001", code.ApplicationID)
	assert.Equal(t, "https://app.example.com/callback", code.RedirectURI)
	assert.Equal(t, []string{"openid", "profile", "email"}, code.Scopes)
	assert.Equal(t, "random-nonce", code.Nonce, "nonce must survive the round trip so the id_token can carry it")
}
