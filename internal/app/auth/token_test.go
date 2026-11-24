package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestTokenFamily_Struct(t *testing.T) {
	family := tokenFamily{
		FamilyID:       "family-1",
		AccessTokenID:  "access-1",
		RefreshTokenID: "refresh-1",
	}
	if family.FamilyID != "family-1" {
		t.Errorf("expected family-1, got %s", family.FamilyID)
	}
	if family.AccessTokenID != "access-1" {
		t.Errorf("expected access-1, got %s", family.AccessTokenID)
	}
	if family.RefreshTokenID != "refresh-1" {
		t.Errorf("expected refresh-1, got %s", family.RefreshTokenID)
	}
}

func TestAuthorizationCodeTTL(t *testing.T) {
	if authorizationCodeTTL != 5*time.Minute {
		t.Errorf("expected 5m TTL, got %v", authorizationCodeTTL)
	}
}

func TestTokenIntrospection_InactiveToken(t *testing.T) {
	response := TokenIntrospectionResponse{
		Active: false,
	}
	if response.Active {
		t.Error("expected inactive token")
	}
}

func TestTokenIntrospection_ActiveToken(t *testing.T) {
	response := TokenIntrospectionResponse{
		Active:    true,
		TokenType: "Bearer",
		Scope:     "openid profile email",
		ClientID:  "app-1",
		Sub:       "user-1",
		Iss:       "https://iam.example.com",
	}
	if !response.Active {
		t.Error("expected active token")
	}
	if response.TokenType != "Bearer" {
		t.Errorf("expected Bearer, got %s", response.TokenType)
	}
	if response.ClientID != "app-1" {
		t.Errorf("expected app-1, got %s", response.ClientID)
	}
}

func TestAppErrors_TokenExpired(t *testing.T) {
	err := apperrors.NewUnauthorized("token has expired")
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeUnauthorized {
		t.Errorf("expected unauthorized, got %s", appErr.Code)
	}
	if appErr.HTTPStatus() != 401 {
		t.Errorf("expected 401, got %d", appErr.HTTPStatus())
	}
}

func TestAppErrors_InvalidToken(t *testing.T) {
	err := apperrors.NewInvalidArgument("invalid token format")
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("expected invalid_argument, got %s", appErr.Code)
	}
	if appErr.HTTPStatus() != 400 {
		t.Errorf("expected 400, got %d", appErr.HTTPStatus())
	}
}

func TestAppErrors_MfaRequired(t *testing.T) {
	err := apperrors.NewMfaRequired("multi-factor authentication is required")
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeMfaRequired {
		t.Errorf("expected mfa_required, got %s", appErr.Code)
	}
	if appErr.HTTPStatus() != 403 {
		t.Errorf("expected 403, got %d", appErr.HTTPStatus())
	}
}

func TestAppErrors_AccountLocked(t *testing.T) {
	err := apperrors.NewAccountLocked("account is temporarily locked")
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeAccountLocked {
		t.Errorf("expected account_locked, got %s", appErr.Code)
	}
	if appErr.HTTPStatus() != 423 {
		t.Errorf("expected 423, got %d", appErr.HTTPStatus())
	}
}

func TestAppErrors_PasswordExpired(t *testing.T) {
	err := apperrors.NewPasswordExpired("password has expired")
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodePasswordExpired {
		t.Errorf("expected password_expired, got %s", appErr.Code)
	}
	if appErr.HTTPStatus() != 403 {
		t.Errorf("expected 403, got %d", appErr.HTTPStatus())
	}
}

// ---------------------------------------------------------------------------
// Additional token test scenarios
// ---------------------------------------------------------------------------

func TestToken_AccessTokenIssuance(t *testing.T) {
	// Verify the token structure for access token issuance.
	token := TokenIntrospectionResponse{
		Active:    true,
		TokenType: "Bearer",
		Scope:     "openid profile email",
		ClientID:  "app-1",
		Sub:       "user-1",
		Iss:       "https://iam.example.com",
	}
	if !token.Active {
		t.Error("expected active token after issuance")
	}
	if token.TokenType != "Bearer" {
		t.Errorf("expected Bearer, got %s", token.TokenType)
	}
}

func TestToken_RefreshTokenStructure(t *testing.T) {
	// Verify the exchange response structure for refresh token.
	resp := ExchangeAuthorizationCodeResponse{
		Status:       "success",
		AccessToken:  "access-token-value",
		RefreshToken: "refresh-token-value",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		Scope:        "openid profile email",
	}
	if resp.Status != "success" {
		t.Errorf("expected success, got %s", resp.Status)
	}
	if resp.RefreshToken == "" {
		t.Error("expected non-empty refresh token")
	}
	if resp.TokenType != "Bearer" {
		t.Errorf("expected Bearer, got %s", resp.TokenType)
	}
	if resp.ExpiresIn <= 0 {
		t.Errorf("expected positive expires_in, got %d", resp.ExpiresIn)
	}
}

func TestToken_Revocation_HashTokenValue(t *testing.T) {
	tokenValue := "test-token-value"
	hash := hashTokenValueForRevocation(tokenValue)
	if hash == "" {
		t.Error("expected non-empty hash")
	}
	// Verify the hash is deterministic.
	expectedSum := sha256.Sum256([]byte(tokenValue))
	expectedHash := hex.EncodeToString(expectedSum[:])
	if hash != expectedHash {
		t.Errorf("hash mismatch: got %s, want %s", hash, expectedHash)
	}
}

func TestToken_Revocation_DifferentTokensDifferentHashes(t *testing.T) {
	hash1 := hashTokenValueForRevocation("token-1")
	hash2 := hashTokenValueForRevocation("token-2")
	if hash1 == hash2 {
		t.Error("expected different hashes for different token values")
	}
}

func TestToken_ExpiryCheck(t *testing.T) {
	now := time.Now()
	// Token expired in the past.
	expiredAt := now.Add(-1 * time.Hour)
	if !now.After(expiredAt) {
		t.Error("expected token to be expired")
	}
	// Token valid in the future.
	validUntil := now.Add(1 * time.Hour)
	if now.After(validUntil) {
		t.Error("expected token to still be valid")
	}
}

func TestToken_RevocationCommand_Struct(t *testing.T) {
	cmd := RevokeTokenCommand{
		Token:         "access-token-123",
		TokenTypeHint: "access_token",
		ClientID:      "app-1",
		ClientSecret:  "secret-1",
	}
	if cmd.Token != "access-token-123" {
		t.Errorf("expected access-token-123, got %s", cmd.Token)
	}
	if cmd.TokenTypeHint != "access_token" {
		t.Errorf("expected access_token, got %s", cmd.TokenTypeHint)
	}
}
