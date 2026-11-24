package authorizationcodes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAuthorizationCode_Validate(t *testing.T) {
	now := time.Now().Add(10 * time.Minute)

	t.Run("valid_code", func(t *testing.T) {
		c := AuthorizationCode{
			Code:          "auth-code-xyz",
			UserID:        "user-001",
			ApplicationID: "app-001",
			RedirectURI:   "https://app.example.com/callback",
			ExpiresAt:     now,
		}
		assert.NoError(t, c.Validate())
	})

	t.Run("missing_code", func(t *testing.T) {
		c := AuthorizationCode{UserID: "user", ApplicationID: "app", RedirectURI: "https://example.com", ExpiresAt: now}
		assert.Error(t, c.Validate())
	})

	t.Run("missing_userId", func(t *testing.T) {
		c := AuthorizationCode{Code: "code", ApplicationID: "app", RedirectURI: "https://example.com", ExpiresAt: now}
		assert.Error(t, c.Validate())
	})

	t.Run("missing_applicationId", func(t *testing.T) {
		c := AuthorizationCode{Code: "code", UserID: "user", RedirectURI: "https://example.com", ExpiresAt: now}
		assert.Error(t, c.Validate())
	})

	t.Run("missing_redirectUri", func(t *testing.T) {
		c := AuthorizationCode{Code: "code", UserID: "user", ApplicationID: "app", ExpiresAt: now}
		assert.Error(t, c.Validate())
	})

	t.Run("missing_expiresAt", func(t *testing.T) {
		c := AuthorizationCode{Code: "code", UserID: "user", ApplicationID: "app", RedirectURI: "https://example.com"}
		assert.Error(t, c.Validate())
	})
}

func TestAuthorizationCode_IsExpired(t *testing.T) {
	now := time.Now()

	t.Run("not_expired", func(t *testing.T) {
		c := AuthorizationCode{ExpiresAt: now.Add(10 * time.Minute)}
		assert.False(t, c.IsExpired(now))
	})

	t.Run("expired", func(t *testing.T) {
		c := AuthorizationCode{ExpiresAt: now.Add(-1 * time.Minute)}
		assert.True(t, c.IsExpired(now))
	})
}

func TestAuthorizationCode_IsUsed(t *testing.T) {
	t.Run("unused", func(t *testing.T) {
		c := AuthorizationCode{Code: "code"}
		assert.False(t, c.IsUsed())
	})

	t.Run("used", func(t *testing.T) {
		now := time.Now()
		c := AuthorizationCode{Code: "code", UsedAt: &now}
		assert.True(t, c.IsUsed())
	})
}

func TestAuthorizationCode_HasPKCE(t *testing.T) {
	t.Run("with_pkce", func(t *testing.T) {
		c := AuthorizationCode{
			CodeChallenge:   "E9Melhoa2OwvFrEMTJguCHaoSzK1jzOGw4Sh2O7D1Qc",
			ChallengeMethod: "S256",
		}
		assert.True(t, c.HasPKCE())
	})

	t.Run("without_pkce", func(t *testing.T) {
		c := AuthorizationCode{}
		assert.False(t, c.HasPKCE())
	})

	t.Run("only_challenge_no_method", func(t *testing.T) {
		c := AuthorizationCode{CodeChallenge: "some-challenge"}
		assert.False(t, c.HasPKCE())
	})
}

func TestAuthorizationCode_IsPKCEMethodValid(t *testing.T) {
	t.Run("S256_valid", func(t *testing.T) {
		c := AuthorizationCode{ChallengeMethod: "S256"}
		assert.True(t, c.IsPKCEMethodValid())
	})

	t.Run("plain_invalid", func(t *testing.T) {
		c := AuthorizationCode{ChallengeMethod: "plain"}
		assert.False(t, c.IsPKCEMethodValid())
	})

	t.Run("empty_invalid", func(t *testing.T) {
		c := AuthorizationCode{ChallengeMethod: ""}
		assert.False(t, c.IsPKCEMethodValid())
	})
}
