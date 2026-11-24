package auth

import (
	"testing"
)

func TestValidateAccountQuery_Valid(t *testing.T) {
	err := validateAccountQuery(AccountQuery{
		OrganizationSlug: "acme",
		Username:         "alice",
	})
	if err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}
}

func TestValidateAccountQuery_MissingOrg(t *testing.T) {
	err := validateAccountQuery(AccountQuery{
		Username: "alice",
	})
	if err == nil {
		t.Error("expected error for missing organization")
	}
}

func TestValidateAccountQuery_MissingUsername(t *testing.T) {
	err := validateAccountQuery(AccountQuery{
		OrganizationSlug: "acme",
	})
	if err == nil {
		t.Error("expected error for missing username")
	}
}

func TestValidateAuthorizeCommand_Valid(t *testing.T) {
	err := validateAuthorizeCommand(AuthorizeCommand{
		OrganizationSlug: "acme",
		Username:         "alice",
		ClientID:         "app-1",
		RedirectURI:      "https://example.com/callback",
		ResponseType:     "code",
	})
	if err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}
}

func TestValidateAuthorizeCommand_MissingClientID(t *testing.T) {
	err := validateAuthorizeCommand(AuthorizeCommand{
		OrganizationSlug: "acme",
		Username:         "alice",
		RedirectURI:      "https://example.com/callback",
	})
	if err == nil {
		t.Error("expected error for missing clientID")
	}
}

func TestValidateAuthorizeCommand_MissingRedirectURI(t *testing.T) {
	err := validateAuthorizeCommand(AuthorizeCommand{
		OrganizationSlug: "acme",
		Username:         "alice",
		ClientID:         "app-1",
	})
	if err == nil {
		t.Error("expected error for missing redirectURI")
	}
}

func TestValidateAuthorizeCommand_InvalidRedirectURI(t *testing.T) {
	err := validateAuthorizeCommand(AuthorizeCommand{
		OrganizationSlug: "acme",
		Username:         "alice",
		ClientID:         "app-1",
		RedirectURI:      "not-absolute",
	})
	if err == nil {
		t.Error("expected error for non-absolute redirectURI")
	}
}

func TestValidateAuthorizeCommand_InvalidResponseType(t *testing.T) {
	err := validateAuthorizeCommand(AuthorizeCommand{
		OrganizationSlug: "acme",
		Username:         "alice",
		ClientID:         "app-1",
		RedirectURI:      "https://example.com/callback",
		ResponseType:     "invalid",
	})
	if err == nil {
		t.Error("expected error for invalid responseType")
	}
}

func TestValidateAuthorizeCommand_PKCE_Valid(t *testing.T) {
	err := validateAuthorizeCommand(AuthorizeCommand{
		OrganizationSlug: "acme",
		Username:         "alice",
		ClientID:         "app-1",
		RedirectURI:      "https://example.com/callback",
		ResponseType:     "code",
		CodeChallenge:    "abc123",
		ChallengeMethod:  "S256",
	})
	if err != nil {
		t.Errorf("expected valid PKCE, got error: %v", err)
	}
}

func TestValidateAuthorizeCommand_PKCE_MissingChallenge(t *testing.T) {
	err := validateAuthorizeCommand(AuthorizeCommand{
		OrganizationSlug: "acme",
		Username:         "alice",
		ClientID:         "app-1",
		RedirectURI:      "https://example.com/callback",
		ChallengeMethod:  "S256",
	})
	if err == nil {
		t.Error("expected error for missing codeChallenge")
	}
}

func TestValidateAuthorizeCommand_PKCE_MissingMethod(t *testing.T) {
	err := validateAuthorizeCommand(AuthorizeCommand{
		OrganizationSlug: "acme",
		Username:         "alice",
		ClientID:         "app-1",
		RedirectURI:      "https://example.com/callback",
		CodeChallenge:    "abc123",
	})
	if err == nil {
		t.Error("expected error for missing challengeMethod")
	}
}

func TestValidateAuthorizeCommand_PKCE_InvalidMethod(t *testing.T) {
	err := validateAuthorizeCommand(AuthorizeCommand{
		OrganizationSlug: "acme",
		Username:         "alice",
		ClientID:         "app-1",
		RedirectURI:      "https://example.com/callback",
		CodeChallenge:    "abc123",
		ChallengeMethod:  "plain",
	})
	if err == nil {
		t.Error("expected error for non-S256 challengeMethod")
	}
}

func TestValidateExchangeAuthorizationCodeCommand_Valid(t *testing.T) {
	err := validateExchangeAuthorizationCodeCommand(ExchangeAuthorizationCodeCommand{
		GrantType:   "authorization_code",
		ClientID:    "app-1",
		Code:        "auth-code-1",
		RedirectURI: "https://example.com/callback",
	})
	if err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}
}

func TestValidateExchangeAuthorizationCodeCommand_DefaultGrantType(t *testing.T) {
	err := validateExchangeAuthorizationCodeCommand(ExchangeAuthorizationCodeCommand{
		ClientID:    "app-1",
		Code:        "auth-code-1",
		RedirectURI: "https://example.com/callback",
	})
	if err != nil {
		t.Errorf("expected valid with default grantType, got error: %v", err)
	}
}

func TestValidateExchangeAuthorizationCodeCommand_InvalidGrantType(t *testing.T) {
	err := validateExchangeAuthorizationCodeCommand(ExchangeAuthorizationCodeCommand{
		GrantType:   "client_credentials",
		ClientID:    "app-1",
		Code:        "auth-code-1",
		RedirectURI: "https://example.com/callback",
	})
	if err == nil {
		t.Error("expected error for invalid grantType")
	}
}

func TestValidateExchangeAuthorizationCodeCommand_MissingCode(t *testing.T) {
	err := validateExchangeAuthorizationCodeCommand(ExchangeAuthorizationCodeCommand{
		GrantType:   "authorization_code",
		ClientID:    "app-1",
		RedirectURI: "https://example.com/callback",
	})
	if err == nil {
		t.Error("expected error for missing code")
	}
}
