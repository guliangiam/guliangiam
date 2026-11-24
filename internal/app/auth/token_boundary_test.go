package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ---------------------------------------------------------------------------
// Token boundary scenario tests
// ---------------------------------------------------------------------------

func TestTokenIntrospection_Boundary(t *testing.T) {
	t.Run("active token with full scope", func(t *testing.T) {
		resp := TokenIntrospectionResponse{
			Active:    true,
			TokenType: "Bearer",
			Scope:     "openid profile email address phone",
			ClientID:  "app-1",
			Sub:       "user-1",
			Iss:       "https://iam.example.com",
		}
		assert.True(t, resp.Active)
		assert.Equal(t, "Bearer", resp.TokenType)
		assert.NotEmpty(t, resp.Scope)
		assert.NotEmpty(t, resp.ClientID)
		assert.NotEmpty(t, resp.Sub)
	})

	t.Run("inactive token", func(t *testing.T) {
		resp := TokenIntrospectionResponse{Active: false}
		assert.False(t, resp.Active)
	})

	t.Run("token with empty scope", func(t *testing.T) {
		resp := TokenIntrospectionResponse{
			Active:    true,
			TokenType: "Bearer",
			Scope:     "",
			ClientID:  "app-1",
		}
		assert.True(t, resp.Active)
		assert.Empty(t, resp.Scope)
	})

	t.Run("token with minimal scope", func(t *testing.T) {
		resp := TokenIntrospectionResponse{
			Active:    true,
			TokenType: "Bearer",
			Scope:     "openid",
			ClientID:  "app-1",
		}
		assert.True(t, resp.Active)
		assert.Equal(t, "openid", resp.Scope)
	})
}

func TestToken_ErrorCodes_Boundary(t *testing.T) {
	tests := []struct {
		name       string
		code       apperrors.Code
		wantStatus int
	}{
		{"expired token - unauthorized", apperrors.CodeUnauthorized, 401},
		{"invalid token - bad request", apperrors.CodeInvalidArgument, 400},
		{"mfa required - forbidden", apperrors.CodeMfaRequired, 403},
		{"account locked - 423", apperrors.CodeAccountLocked, 423},
		{"password expired - forbidden", apperrors.CodePasswordExpired, 403},
		{"not found", apperrors.CodeNotFound, 404},
		{"conflict", apperrors.CodeConflict, 409},
		{"too many requests", apperrors.CodeTooManyRequests, 429},
		{"internal error", apperrors.CodeInternal, 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := apperrors.New(tt.code, "test error")
			appErr := apperrors.FromError(err)
			assert.Equal(t, tt.code, appErr.Code)
			assert.Equal(t, tt.wantStatus, appErr.HTTPStatus())
		})
	}
}

func TestExchangeAuthorizationCodeResponse_Boundary(t *testing.T) {
	t.Run("successful exchange", func(t *testing.T) {
		resp := ExchangeAuthorizationCodeResponse{
			Status:       "success",
			AccessToken:  "access-token-value",
			RefreshToken: "refresh-token-value",
			TokenType:    "Bearer",
			ExpiresIn:    3600,
			Scope:        "openid profile email",
		}
		assert.Equal(t, "success", resp.Status)
		assert.NotEmpty(t, resp.AccessToken)
		assert.NotEmpty(t, resp.RefreshToken)
		assert.Equal(t, "Bearer", resp.TokenType)
		assert.Positive(t, resp.ExpiresIn)
	})

	t.Run("exchange without refresh token", func(t *testing.T) {
		resp := ExchangeAuthorizationCodeResponse{
			Status:      "success",
			AccessToken: "access-token-value",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		}
		assert.Empty(t, resp.RefreshToken)
	})

	t.Run("exchange with zero expiry", func(t *testing.T) {
		resp := ExchangeAuthorizationCodeResponse{
			Status:      "success",
			AccessToken: "access-token-value",
			TokenType:   "Bearer",
			ExpiresIn:   0,
		}
		assert.Zero(t, resp.ExpiresIn)
	})
}

func TestRevokeTokenCommand_Boundary(t *testing.T) {
	t.Run("access token hint", func(t *testing.T) {
		cmd := RevokeTokenCommand{
			Token:         "access-token-123",
			TokenTypeHint: "access_token",
			ClientID:      "app-1",
			ClientSecret:  "secret-1",
		}
		assert.Equal(t, "access_token", cmd.TokenTypeHint)
		assert.NotEmpty(t, cmd.Token)
	})

	t.Run("refresh token hint", func(t *testing.T) {
		cmd := RevokeTokenCommand{
			Token:         "refresh-token-123",
			TokenTypeHint: "refresh_token",
			ClientID:      "app-1",
		}
		assert.Equal(t, "refresh_token", cmd.TokenTypeHint)
	})

	t.Run("no token type hint", func(t *testing.T) {
		cmd := RevokeTokenCommand{
			Token:    "some-token",
			ClientID: "app-1",
		}
		assert.Empty(t, cmd.TokenTypeHint)
	})

	t.Run("empty token", func(t *testing.T) {
		cmd := RevokeTokenCommand{
			Token:    "",
			ClientID: "app-1",
		}
		assert.Empty(t, cmd.Token)
	})
}

func TestToken_HashTokenValueForRevocation_Boundary(t *testing.T) {
	t.Run("deterministic hashing", func(t *testing.T) {
		token := "test-token-value"
		hash1 := hashTokenValueForRevocation(token)
		hash2 := hashTokenValueForRevocation(token)
		assert.Equal(t, hash1, hash2)
	})

	t.Run("different tokens different hashes", func(t *testing.T) {
		hash1 := hashTokenValueForRevocation("token-1")
		hash2 := hashTokenValueForRevocation("token-2")
		assert.NotEqual(t, hash1, hash2)
	})

	t.Run("empty token produces hash", func(t *testing.T) {
		hash := hashTokenValueForRevocation("")
		assert.NotEmpty(t, hash)
	})

	t.Run("long token produces hash", func(t *testing.T) {
		longToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyLTEiLCJhdWQiOiJhcHAtMSIsImlzcyI6Imh0dHBzOi8vaWFtLmV4YW1wbGUuY29tIiwiZXhwIjoxNjk5MDAwMDAwLCJpYXQiOjE2OTkwMDAwMDB9.signature"
		hash := hashTokenValueForRevocation(longToken)
		assert.NotEmpty(t, hash)
	})
}

func TestTokenFamily_Boundary(t *testing.T) {
	t.Run("complete family", func(t *testing.T) {
		family := tokenFamily{
			FamilyID:       "family-1",
			AccessTokenID:  "access-1",
			RefreshTokenID: "refresh-1",
		}
		assert.NotEmpty(t, family.FamilyID)
		assert.NotEmpty(t, family.AccessTokenID)
		assert.NotEmpty(t, family.RefreshTokenID)
	})

	t.Run("family with empty refresh", func(t *testing.T) {
		family := tokenFamily{
			FamilyID:      "family-2",
			AccessTokenID: "access-2",
		}
		assert.Empty(t, family.RefreshTokenID)
	})
}

func TestAuthorizationCodeTTL_Boundary(t *testing.T) {
	t.Run("TTL is 5 minutes", func(t *testing.T) {
		assert.Equal(t, 5*time.Minute, authorizationCodeTTL)
	})
}

func TestAppErrors_SpecializedErrors(t *testing.T) {
	t.Run("MfaRequired error", func(t *testing.T) {
		err := apperrors.NewMfaRequired("MFA is required")
		appErr := apperrors.FromError(err)
		assert.Equal(t, apperrors.CodeMfaRequired, appErr.Code)
		assert.Equal(t, 403, appErr.HTTPStatus())
	})

	t.Run("AccountLocked error", func(t *testing.T) {
		err := apperrors.NewAccountLocked("Account is locked")
		appErr := apperrors.FromError(err)
		assert.Equal(t, apperrors.CodeAccountLocked, appErr.Code)
		assert.Equal(t, 423, appErr.HTTPStatus())
	})

	t.Run("PasswordExpired error", func(t *testing.T) {
		err := apperrors.NewPasswordExpired("Password has expired")
		appErr := apperrors.FromError(err)
		assert.Equal(t, apperrors.CodePasswordExpired, appErr.Code)
		assert.Equal(t, 403, appErr.HTTPStatus())
	})
}
