package auth

import (
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestValidateAuthorizeCommand_EmptyClientID(t *testing.T) {
	cmd := AuthorizeCommand{
		OrganizationSlug: "org-1",
		Username:         "admin",
		ClientID:         "",
		RedirectURI:      "http://localhost/callback",
		ResponseType:     "code",
		RequestedScopes:  []string{"openid"},
	}
	err := validateAuthorizeCommand(cmd)
	if err == nil {
		t.Fatal("expected error for empty clientID")
	}
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("expected invalid_argument, got %s", appErr.Code)
	}
}

func TestValidateAuthorizeCommand_EmptyRedirectURI(t *testing.T) {
	cmd := AuthorizeCommand{
		OrganizationSlug: "org-1",
		Username:         "admin",
		ClientID:         "app-1",
		RedirectURI:      "",
		ResponseType:     "code",
		RequestedScopes:  []string{"openid"},
	}
	err := validateAuthorizeCommand(cmd)
	if err == nil {
		t.Fatal("expected error for empty redirectURI")
	}
}

func TestDefaultOAuthScopes(t *testing.T) {
	expected := []string{"openid", "profile", "email", "phone", "address", "offline_access"}
	for _, scope := range expected {
		if _, ok := defaultOAuthScopes[scope]; !ok {
			t.Errorf("expected scope %q in defaultOAuthScopes", scope)
		}
	}
}

func TestAuthorizeCommand_PKCEFields(t *testing.T) {
	cmd := AuthorizeCommand{
		OrganizationSlug: "org-1",
		Username:         "admin",
		ClientID:         "app-1",
		RedirectURI:      "http://localhost/callback",
		ResponseType:     "code",
		RequestedScopes:  []string{"openid"},
		CodeChallenge:    "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuG8stw9M10",
		ChallengeMethod:  "S256",
	}
	if cmd.CodeChallenge == "" {
		t.Error("expected CodeChallenge to be set")
	}
	if cmd.ChallengeMethod != "S256" {
		t.Errorf("expected S256, got %s", cmd.ChallengeMethod)
	}
}

// ---------------------------------------------------------------------------
// Additional OAuth2 test scenarios
// ---------------------------------------------------------------------------

func TestOAuth2_AuthorizeCommand_RelativeRedirectURI(t *testing.T) {
	cmd := AuthorizeCommand{
		OrganizationSlug: "org-1",
		Username:         "admin",
		ClientID:         "app-1",
		RedirectURI:      "/callback",
		ResponseType:     "code",
		RequestedScopes:  []string{"openid"},
	}
	err := validateAuthorizeCommand(cmd)
	if err == nil {
		t.Fatal("expected error for relative redirectURI")
	}
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("expected invalid_argument, got %s", appErr.Code)
	}
}

func TestOAuth2_ExchangeCode_MissingClientID(t *testing.T) {
	cmd := ExchangeAuthorizationCodeCommand{
		GrantType:   "authorization_code",
		ClientID:    "",
		Code:        "auth-code-1",
		RedirectURI: "http://localhost/callback",
	}
	err := validateExchangeAuthorizationCodeCommand(cmd)
	if err == nil {
		t.Fatal("expected error for missing client ID")
	}
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("expected invalid_argument, got %s", appErr.Code)
	}
}

func TestOAuth2_ExchangeCode_MissingRedirectURI(t *testing.T) {
	cmd := ExchangeAuthorizationCodeCommand{
		GrantType:   "authorization_code",
		ClientID:    "app-1",
		Code:        "auth-code-1",
		RedirectURI: "",
	}
	err := validateExchangeAuthorizationCodeCommand(cmd)
	if err == nil {
		t.Fatal("expected error for missing redirect URI")
	}
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("expected invalid_argument, got %s", appErr.Code)
	}
}

func TestOAuth2_PKCE_VerifierField(t *testing.T) {
	cmd := ExchangeAuthorizationCodeCommand{
		GrantType:    "authorization_code",
		ClientID:     "app-1",
		Code:         "auth-code-1",
		RedirectURI:  "http://localhost/callback",
		CodeVerifier: "dBjftJeZ4CVP-mB92K27uhbU47Ukw4Sl3aOqOYmr0Qw",
	}
	if cmd.CodeVerifier == "" {
		t.Error("expected CodeVerifier to be set")
	}
}

func TestOAuth2_BuildAuthorizationRedirectURL(t *testing.T) {
	redirectURL := buildAuthorizationRedirectURL("http://localhost/callback", "auth-code-123", "state-abc")
	if redirectURL == "" {
		t.Fatal("expected non-empty redirect URL")
	}
	if redirectURL == "http://localhost/callback" {
		t.Error("expected query parameters to be appended")
	}
}

func TestOAuth2_ValidateAuthorizeScopes_EmptyScopes(t *testing.T) {
	scopes, err := validateAuthorizeScopes([]string{}, nil)
	if err != nil {
		t.Fatalf("expected no error for empty scopes, got %v", err)
	}
	if len(scopes) != 0 {
		t.Errorf("expected empty scopes, got %v", scopes)
	}
}

func TestOAuth2_ValidateAuthorizeScopes_DefaultScopes(t *testing.T) {
	scopes, err := validateAuthorizeScopes([]string{"openid", "profile"}, nil)
	if err != nil {
		t.Fatalf("expected no error for default scopes, got %v", err)
	}
	if len(scopes) != 2 {
		t.Errorf("expected 2 scopes, got %d", len(scopes))
	}
}

func TestOAuth2_ValidateAuthorizeScopes_InvalidScope(t *testing.T) {
	_, err := validateAuthorizeScopes([]string{"openid", "invalid_scope"}, nil)
	if err == nil {
		t.Fatal("expected error for invalid scope")
	}
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("expected invalid_argument, got %s", appErr.Code)
	}
}

func TestOAuth2_ExchangeCodeResponse_Structure(t *testing.T) {
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
	if resp.TokenType != "Bearer" {
		t.Errorf("expected Bearer, got %s", resp.TokenType)
	}
	if resp.ExpiresIn <= 0 {
		t.Errorf("expected positive expires_in, got %d", resp.ExpiresIn)
	}
}
