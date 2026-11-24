package auth

import (
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ---------------------------------------------------------------------------
// Additional OAuth2 authorize validation tests
// ---------------------------------------------------------------------------

func TestValidateAuthorizeCommand_EmptyOrgSlug(t *testing.T) {
	err := validateAuthorizeCommand(AuthorizeCommand{
		Username:     "alice",
		ClientID:     "app-1",
		RedirectURI:  "https://example.com/callback",
		ResponseType: "code",
	})
	if err == nil {
		t.Fatal("expected error for empty organizationSlug")
	}
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeUnauthorized {
		t.Errorf("expected unauthorized, got %s", appErr.Code)
	}
}

func TestValidateAuthorizeCommand_EmptyUsername(t *testing.T) {
	err := validateAuthorizeCommand(AuthorizeCommand{
		OrganizationSlug: "acme",
		ClientID:         "app-1",
		RedirectURI:      "https://example.com/callback",
		ResponseType:     "code",
	})
	if err == nil {
		t.Fatal("expected error for empty username")
	}
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeUnauthorized {
		t.Errorf("expected unauthorized, got %s", appErr.Code)
	}
}

// TestValidateAuthorizeCommand_RetiredResponseTypes pins the implicit-flow
// retirement: `token` and `id_token` are refused with the protocol code
// unsupported_response_type instead of being accepted or silently downgraded.
func TestValidateAuthorizeCommand_RetiredResponseTypes(t *testing.T) {
	for _, responseType := range []string{"token", "id_token", "token id_token"} {
		t.Run(responseType, func(t *testing.T) {
			err := validateAuthorizeCommand(AuthorizeCommand{
				OrganizationSlug: "acme",
				Username:         "alice",
				ClientID:         "app-1",
				RedirectURI:      "https://example.com/callback",
				ResponseType:     responseType,
			})
			if err == nil {
				t.Fatalf("expected rejection for responseType=%s", responseType)
			}
			protocolErr, ok := err.(apperrors.ProtocolCodeError)
			if !ok {
				t.Fatalf("expected a protocol-coded error, got %T: %v", err, err)
			}
			if protocolErr.ProtocolCode() != "unsupported_response_type" {
				t.Fatalf("expected unsupported_response_type, got %s", protocolErr.ProtocolCode())
			}
		})
	}
}

func TestValidateAuthorizeCommand_EmptyResponseType(t *testing.T) {
	err := validateAuthorizeCommand(AuthorizeCommand{
		OrganizationSlug: "acme",
		Username:         "alice",
		ClientID:         "app-1",
		RedirectURI:      "https://example.com/callback",
	})
	if err != nil {
		t.Errorf("expected valid for empty responseType, got error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Additional ExchangeAuthorizationCode validation tests
// ---------------------------------------------------------------------------

func TestValidateExchangeCommand_InvalidRedirectURI(t *testing.T) {
	err := validateExchangeAuthorizationCodeCommand(ExchangeAuthorizationCodeCommand{
		GrantType:   "authorization_code",
		ClientID:    "app-1",
		Code:        "auth-code-1",
		RedirectURI: "not-absolute",
	})
	if err == nil {
		t.Fatal("expected error for non-absolute redirectURI")
	}
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("expected invalid_argument, got %s", appErr.Code)
	}
}

func TestValidateExchangeCommand_EmptyClientID(t *testing.T) {
	err := validateExchangeAuthorizationCodeCommand(ExchangeAuthorizationCodeCommand{
		GrantType:   "authorization_code",
		Code:        "auth-code-1",
		RedirectURI: "https://example.com/callback",
	})
	if err == nil {
		t.Fatal("expected error for empty clientID")
	}
}

// ---------------------------------------------------------------------------
// Additional login validation edge case tests
// ---------------------------------------------------------------------------

func TestValidateLoginCommand_ChallengeWithRecoveryCode(t *testing.T) {
	err := validateLoginCommand(LoginCommand{
		ChallengeID:  "challenge-1",
		RecoveryCode: "ABCD-EFGH",
	})
	if err != nil {
		t.Errorf("expected valid for challenge with recoveryCode, got error: %v", err)
	}
}

func TestValidateLoginCommand_ChallengeWithPushMfa(t *testing.T) {
	err := validateLoginCommand(LoginCommand{
		ChallengeID: "challenge-1",
		MfaType:     "push",
	})
	if err != nil {
		t.Errorf("expected valid for challenge with push MFA, got error: %v", err)
	}
}

func TestValidateLoginCommand_MfaTypePushWithoutPasscode(t *testing.T) {
	err := validateLoginCommand(LoginCommand{
		OrganizationSlug: "acme",
		Username:         "alice",
		Password:         "P@ssw0rd",
		ApplicationID:    "app-1",
		MfaType:          "push",
	})
	if err != nil {
		t.Errorf("expected valid for push MFA without passcode, got error: %v", err)
	}
}

func TestValidateLoginCommand_MfaTypeTotpRequiresPasscode(t *testing.T) {
	err := validateLoginCommand(LoginCommand{
		OrganizationSlug: "acme",
		Username:         "alice",
		Password:         "P@ssw0rd",
		ApplicationID:    "app-1",
		MfaType:          "totp",
	})
	if err == nil {
		t.Fatal("expected error when mfaType=totp without passcode")
	}
}

func TestValidateLoginCommand_MfaTypeWithPasscode(t *testing.T) {
	err := validateLoginCommand(LoginCommand{
		OrganizationSlug: "acme",
		Username:         "alice",
		Password:         "P@ssw0rd",
		ApplicationID:    "app-1",
		MfaType:          "totp",
		Passcode:         "123456",
	})
	if err != nil {
		t.Errorf("expected valid for mfaType with passcode, got error: %v", err)
	}
}

func TestValidateLoginCommand_EnableMfaRememberWithPasscode(t *testing.T) {
	err := validateLoginCommand(LoginCommand{
		OrganizationSlug:  "acme",
		Username:          "alice",
		Password:          "P@ssw0rd",
		ApplicationID:     "app-1",
		Passcode:          "123456",
		EnableMfaRemember: true,
	})
	if err != nil {
		t.Errorf("expected valid for enableMfaRemember with passcode, got error: %v", err)
	}
}

func TestValidateLoginCommand_EnableMfaRememberWithRecoveryCode(t *testing.T) {
	err := validateLoginCommand(LoginCommand{
		OrganizationSlug:  "acme",
		Username:          "alice",
		Password:          "P@ssw0rd",
		ApplicationID:     "app-1",
		RecoveryCode:      "ABCD-EFGH",
		EnableMfaRemember: true,
	})
	if err != nil {
		t.Errorf("expected valid for enableMfaRemember with recoveryCode, got error: %v", err)
	}
}

func TestValidateLoginCommand_EnableMfaRememberWithPush(t *testing.T) {
	err := validateLoginCommand(LoginCommand{
		OrganizationSlug:  "acme",
		Username:          "alice",
		Password:          "P@ssw0rd",
		ApplicationID:     "app-1",
		MfaType:           "push",
		EnableMfaRemember: true,
	})
	if err != nil {
		t.Errorf("expected valid for enableMfaRemember with push MFA, got error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Account query validation tests
// ---------------------------------------------------------------------------

func TestValidateAccountQuery_BothMissing(t *testing.T) {
	err := validateAccountQuery(AccountQuery{})
	if err == nil {
		t.Fatal("expected error when both fields are missing")
	}
}

func TestValidateAccountQuery_WhitespaceFields(t *testing.T) {
	err := validateAccountQuery(AccountQuery{
		OrganizationSlug: "   ",
		Username:         "  ",
	})
	if err == nil {
		t.Fatal("expected error when fields are whitespace-only")
	}
}
