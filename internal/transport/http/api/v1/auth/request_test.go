package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateOptionalString(t *testing.T) {
	t.Run("empty_string_passes", func(t *testing.T) {
		err := validateOptionalString("test", "")
		assert.NoError(t, err)
	})

	t.Run("normal_string_passes", func(t *testing.T) {
		err := validateOptionalString("test", "hello")
		assert.NoError(t, err)
	})

	t.Run("exceeds_max_length_fails", func(t *testing.T) {
		longString := strings.Repeat("a", maxStringLength+1)
		err := validateOptionalString("test", longString)
		assert.Error(t, err)
	})

	t.Run("at_max_length_passes", func(t *testing.T) {
		atMax := strings.Repeat("a", maxStringLength)
		err := validateOptionalString("test", atMax)
		assert.NoError(t, err)
	})
}

func TestValidateStringSlice(t *testing.T) {
	t.Run("empty_slice_passes", func(t *testing.T) {
		err := validateStringSlice("test", []string{})
		assert.NoError(t, err)
	})

	t.Run("nil_slice_passes", func(t *testing.T) {
		err := validateStringSlice("test", nil)
		assert.NoError(t, err)
	})

	t.Run("normal_slice_passes", func(t *testing.T) {
		err := validateStringSlice("test", []string{"a", "b", "c"})
		assert.NoError(t, err)
	})

	t.Run("exceeds_max_entries_fails", func(t *testing.T) {
		longSlice := make([]string, maxSliceLength+1)
		for i := range longSlice {
			longSlice[i] = "a"
		}
		err := validateStringSlice("test", longSlice)
		assert.Error(t, err)
	})

	t.Run("entry_exceeds_max_length_fails", func(t *testing.T) {
		longString := strings.Repeat("a", maxStringLength+1)
		err := validateStringSlice("test", []string{longString})
		assert.Error(t, err)
	})
}

func TestLoginRequest_Validate(t *testing.T) {
	t.Run("valid_request", func(t *testing.T) {
		r := LoginRequest{
			OrganizationSlug: "test-org",
			Username:         "testuser",
			Password:         "password123",
			ApplicationID:    "app-001",
		}
		assert.NoError(t, r.Validate())
	})

	t.Run("password_exceeds_max_length", func(t *testing.T) {
		r := LoginRequest{
			Username: "testuser",
			Password: strings.Repeat("p", maxPasswordLen+1),
		}
		assert.Error(t, r.Validate())
	})

	t.Run("username_exceeds_max_length", func(t *testing.T) {
		r := LoginRequest{
			Username: strings.Repeat("u", maxStringLength+1),
			Password: "password123",
		}
		assert.Error(t, r.Validate())
	})

	t.Run("organization_slug_exceeds_max_length", func(t *testing.T) {
		r := LoginRequest{
			OrganizationSlug: strings.Repeat("o", maxStringLength+1),
			Username:         "testuser",
			Password:         "password123",
		}
		assert.Error(t, r.Validate())
	})

	t.Run("scope_exceeds_max_entries", func(t *testing.T) {
		r := LoginRequest{
			Username: "testuser",
			Password: "password123",
			Scope:    make([]string, maxSliceLength+1),
		}
		assert.Error(t, r.Validate())
	})

	t.Run("empty_fields_valid", func(t *testing.T) {
		r := LoginRequest{}
		assert.NoError(t, r.Validate())
	})

	t.Run("passcode_exceeds_max_length", func(t *testing.T) {
		r := LoginRequest{
			Passcode: strings.Repeat("p", maxStringLength+1),
		}
		assert.Error(t, r.Validate())
	})

	t.Run("recovery_code_exceeds_max_length", func(t *testing.T) {
		r := LoginRequest{
			RecoveryCode: strings.Repeat("r", maxStringLength+1),
		}
		assert.Error(t, r.Validate())
	})
}

func TestResetPasswordRequest_Validate(t *testing.T) {
	t.Run("valid_request", func(t *testing.T) {
		r := ResetPasswordRequest{
			OrganizationSlug: "test-org",
			Username:         "testuser",
			VerificationCode: "123456",
			NewPassword:      "newpass123",
		}
		assert.NoError(t, r.Validate())
	})

	t.Run("password_exceeds_max_length", func(t *testing.T) {
		r := ResetPasswordRequest{
			NewPassword: strings.Repeat("p", maxPasswordLen+1),
		}
		assert.Error(t, r.Validate())
	})

	t.Run("empty_fields_valid", func(t *testing.T) {
		r := ResetPasswordRequest{}
		assert.NoError(t, r.Validate())
	})
}

func TestCheckPasswordRequest_Validate(t *testing.T) {
	t.Run("valid_request", func(t *testing.T) {
		r := CheckPasswordRequest{
			OrganizationSlug: "test-org",
			Username:         "testuser",
			Password:         "password123",
		}
		assert.NoError(t, r.Validate())
	})

	t.Run("password_exceeds_max_length", func(t *testing.T) {
		r := CheckPasswordRequest{
			Password: strings.Repeat("p", maxPasswordLen+1),
		}
		assert.Error(t, r.Validate())
	})
}

func TestSignupRequest_Validate(t *testing.T) {
	t.Run("valid_request", func(t *testing.T) {
		r := SignupRequest{
			OrganizationSlug: "test-org",
			Username:         "newuser",
			Password:         "password123",
			Email:            "user@example.com",
		}
		assert.NoError(t, r.Validate())
	})

	t.Run("password_exceeds_max_length", func(t *testing.T) {
		r := SignupRequest{
			Password: strings.Repeat("p", maxPasswordLen+1),
		}
		assert.Error(t, r.Validate())
	})

	t.Run("username_exceeds_max_length", func(t *testing.T) {
		r := SignupRequest{
			Username: strings.Repeat("u", maxStringLength+1),
		}
		assert.Error(t, r.Validate())
	})
}

func TestLoginRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := LoginRequest{
			OrganizationSlug:  "test-org",
			Username:          "testuser",
			Password:          "password123",
			ApplicationID:     "app-001",
			ChallengeID:       "challenge-001",
			MfaType:           "totp",
			Passcode:          "123456",
			EnableMfaRemember: true,
			Scope:             []string{"openid", "profile"},
		}

		req := testHTTPRequest()
		cmd := r.toCommand(req)

		assert.Equal(t, "test-org", cmd.OrganizationSlug)
		assert.Equal(t, "testuser", cmd.Username)
		assert.Equal(t, "password123", cmd.Password)
		assert.Equal(t, "app-001", cmd.ApplicationID)
		assert.Equal(t, "challenge-001", cmd.ChallengeID)
		assert.Equal(t, "totp", cmd.MfaType)
		assert.Equal(t, "123456", cmd.Passcode)
		assert.True(t, cmd.EnableMfaRemember)
		assert.Equal(t, []string{"openid", "profile"}, cmd.Scope)
	})
}

func TestGetAccountRequest_ToQuery(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := GetAccountRequest{
			OrganizationSlug: "test-org",
			Username:         "testuser",
			AccessToken:      "at-12345",
		}
		q := r.toQuery()
		assert.Equal(t, "test-org", q.OrganizationSlug)
		assert.Equal(t, "testuser", q.Username)
		assert.Equal(t, "at-12345", q.AccessToken)
	})

	t.Run("empty_fields", func(t *testing.T) {
		r := GetAccountRequest{}
		q := r.toQuery()
		assert.Equal(t, "", q.OrganizationSlug)
		assert.Equal(t, "", q.Username)
		assert.Equal(t, "", q.AccessToken)
	})
}

func TestExchangeAuthorizationCodeRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := ExchangeAuthorizationCodeRequest{
			GrantType:    "authorization_code",
			ClientID:     "client-001",
			ClientSecret: "secret-001",
			Code:         "auth-code-001",
			RedirectURI:  "https://example.com/callback",
			CodeVerifier: "verifier-001",
			Resource:     "https://api.example.com",
		}
		cmd := r.toCommand(httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/exchange", nil))
		assert.Equal(t, "authorization_code", cmd.GrantType)
		assert.Equal(t, "client-001", cmd.ClientID)
		assert.Equal(t, "secret-001", cmd.ClientSecret)
		assert.Equal(t, "auth-code-001", cmd.Code)
		assert.Equal(t, "https://example.com/callback", cmd.RedirectURI)
		assert.Equal(t, "verifier-001", cmd.CodeVerifier)
		assert.Equal(t, "https://api.example.com", cmd.Resource)
	})

	t.Run("empty_fields", func(t *testing.T) {
		r := ExchangeAuthorizationCodeRequest{}
		cmd := r.toCommand(httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/exchange", nil))
		assert.Equal(t, "", cmd.GrantType)
		assert.Equal(t, "", cmd.ClientID)
	})
}

func TestResetPasswordRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := ResetPasswordRequest{
			OrganizationSlug: "test-org",
			Username:         "testuser",
			VerificationCode: "123456",
			NewPassword:      "newpass123",
		}
		cmd := r.toCommand()
		assert.Equal(t, "test-org", cmd.OrganizationSlug)
		assert.Equal(t, "testuser", cmd.Username)
		assert.Equal(t, "123456", cmd.VerificationCode)
		assert.Equal(t, "newpass123", cmd.NewPassword)
	})
}

func TestCheckPasswordRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := CheckPasswordRequest{
			OrganizationSlug: "test-org",
			Username:         "testuser",
			Password:         "password123",
		}
		cmd := r.toCommand()
		assert.Equal(t, "test-org", cmd.OrganizationSlug)
		assert.Equal(t, "testuser", cmd.Username)
		assert.Equal(t, "password123", cmd.Password)
	})
}

func TestSignupRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := SignupRequest{
			OrganizationSlug: "test-org",
			Username:         "newuser",
			Password:         "password123",
			Email:            "user@example.com",
			Phone:            "+8613800138000",
			SignupAppID:      "app-001",
			VerificationCode: "123456",
			InvitationCode:   "invite-001",
		}
		cmd := r.toCommand()
		assert.Equal(t, "test-org", cmd.OrganizationSlug)
		assert.Equal(t, "newuser", cmd.Username)
		assert.Equal(t, "password123", cmd.Password)
		assert.Equal(t, "user@example.com", cmd.Email)
		assert.Equal(t, "+8613800138000", cmd.Phone)
		assert.Equal(t, "app-001", cmd.SignupAppID)
		assert.Equal(t, "123456", cmd.VerificationCode)
		assert.Equal(t, "invite-001", cmd.InvitationCode)
	})
}

func TestDeviceAuthorizeRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := DeviceAuthorizeRequest{
			ClientID: "client-001",
			Scope:    "openid profile",
		}
		cmd := r.toCommand(httptest.NewRequest(http.MethodPost, "/api/v1/auth/device/authorize", nil))
		assert.Equal(t, "client-001", cmd.ClientID)
		assert.Equal(t, "openid profile", cmd.Scope)
	})
}

func TestDeviceTokenRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := DeviceTokenRequest{
			ClientID:     "client-001",
			ClientSecret: "secret-001",
			DeviceCode:   "device-code-001",
		}
		cmd := r.toCommand()
		assert.Equal(t, "client-001", cmd.ClientID)
		assert.Equal(t, "secret-001", cmd.ClientSecret)
		assert.Equal(t, "device-code-001", cmd.DeviceCode)
	})
}

func TestDeviceCancelRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := DeviceCancelRequest{
			DeviceCode: "device-code-001",
			ClientID:   "client-001",
		}
		cmd := r.toCommand()
		assert.Equal(t, "device-code-001", cmd.DeviceCode)
		assert.Equal(t, "client-001", cmd.ClientID)
	})
}

func TestJwtBearerTokenRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := JwtBearerTokenRequest{
			GrantType:           "urn/ietf/params/oauth/grant-type/jwt-bearer",
			ClientAssertionType: "urn/ietf/params/oauth/client-assertion-type/jwt-bearer",
			ClientAssertion:     "jwt-token-001",
			Scope:               "openid",
		}
		cmd := r.toCommand(httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", nil))
		assert.Equal(t, "urn/ietf/params/oauth/client-assertion-type/jwt-bearer", cmd.ClientAssertionType)
		assert.Equal(t, "jwt-token-001", cmd.ClientAssertion)
		assert.Equal(t, "openid", cmd.Scope)
	})
}

func TestTokenExchangeRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := TokenExchangeRequest{
			GrantType:          "urn/ietf/params/oauth/grant-type/token-exchange",
			SubjectToken:       "subject-token-001",
			SubjectTokenType:   "urn/ietf/params/oauth/token-type/access_token",
			ActorToken:         "actor-token-001",
			ActorTokenType:     "urn/ietf/params/oauth/token-type/access_token",
			Resource:           "https://api.example.com",
			Audience:           "client-001",
			Scope:              "openid profile",
			ClientID:           "client-001",
			ClientSecret:       "secret-001",
			RequestedTokenType: "urn/ietf/params/oauth/token-type/access_token",
		}
		cmd := r.toCommand()
		assert.Equal(t, "urn/ietf/params/oauth/grant-type/token-exchange", cmd.GrantType)
		assert.Equal(t, "subject-token-001", cmd.SubjectToken)
		assert.Equal(t, "urn/ietf/params/oauth/token-type/access_token", cmd.SubjectTokenType)
		assert.Equal(t, "actor-token-001", cmd.ActorToken)
		assert.Equal(t, "https://api.example.com", cmd.Resource)
		assert.Equal(t, "client-001", cmd.Audience)
		assert.Equal(t, "openid profile", cmd.Scope)
		assert.Equal(t, "client-001", cmd.ClientID)
		assert.Equal(t, "secret-001", cmd.ClientSecret)
		assert.Equal(t, "urn/ietf/params/oauth/token-type/access_token", cmd.RequestedTokenType)
	})
}

func TestTokenIntrospectionRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := TokenIntrospectionRequest{
			Token:     "access-token-001",
			TokenType: "access_token",
			ClientID:  "client-001",
		}
		cmd := r.toCommand()
		assert.Equal(t, "access-token-001", cmd.Token)
		assert.Equal(t, "access_token", cmd.TokenType)
		assert.Equal(t, "client-001", cmd.ClientID)
	})
}

func TestBackchannelLogoutRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := BackchannelLogoutRequest{
			UserID:    "user-001",
			SessionID: "session-001",
			ClientID:  "client-001",
		}
		httpRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/backchannel-logout", nil)
		httpRequest = httpRequest.WithContext(WithSessionContext(httpRequest.Context(), SessionContext{
			UserID:         "user-001",
			OrganizationID: "org-001",
			PrincipalType:  "user",
		}))
		cmd := r.toCommand(httpRequest)
		assert.Equal(t, "user-001", cmd.UserID)
		assert.Equal(t, "user-001", cmd.ActorID)
		assert.Equal(t, "user", cmd.ActorType)
		assert.Equal(t, "org-001", cmd.OrganizationID)
		assert.Equal(t, "session-001", cmd.SessionID)
		assert.Equal(t, "client-001", cmd.ClientID)
	})
}

func TestDPoPProofValidationRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := DPoPProofValidationRequest{
			DPoPHeader: "dpop-jwt-001",
			Method:     "POST",
			URL:        "https://api.example.com/token",
		}
		cmd := r.toCommand()
		assert.Equal(t, "dpop-jwt-001", cmd.DPoPHeader)
		assert.Equal(t, "POST", cmd.Method)
		assert.Equal(t, "https://api.example.com/token", cmd.URL)
	})
}

func TestClientAuthRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := ClientAuthRequest{
			ClientID:            "client-001",
			ClientSecret:        "secret-001",
			ClientAssertion:     "jwt-assertion-001",
			ClientAssertionType: "urn/ietf/params/oauth/client-assertion-type/jwt-bearer",
		}
		cmd := r.toCommand(httptest.NewRequest(http.MethodPost, "/api/v1/auth/client/authenticate", nil))
		assert.Equal(t, "client-001", cmd.ClientID)
		assert.Equal(t, "secret-001", cmd.ClientSecret)
		assert.Equal(t, "jwt-assertion-001", cmd.ClientAssertion)
		assert.Equal(t, "urn/ietf/params/oauth/client-assertion-type/jwt-bearer", cmd.ClientAssertionType)
	})
}

func TestClientCredentialsRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := ClientCredentialsRequest{
			ClientID:     "client-001",
			ClientSecret: "secret-001",
			Scope:        "openid profile",
		}
		cmd := r.toCommand(httptest.NewRequest(http.MethodPost, "/tokens/client-credentials", nil))
		assert.Equal(t, "client-001", cmd.ClientID)
		assert.Equal(t, "secret-001", cmd.ClientSecret)
		assert.Equal(t, "openid profile", cmd.Scope)
	})
}

func TestCreateQRCodeRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := CreateQRCodeRequest{
			OrganizationID: "org-001",
			ApplicationID:  "app-001",
		}
		cmd := r.toCommand()
		assert.Equal(t, "org-001", cmd.OrganizationID)
		assert.Equal(t, "app-001", cmd.ApplicationID)
	})
}

func TestScanQRCodeRequest_ToCommand(t *testing.T) {
	t.Run("anonymous_registration_maps_code_and_profile", func(t *testing.T) {
		r := ScanQRCodeRequest{
			Code:        "qr-payload",
			Username:    "testuser",
			DisplayName: "Test User",
			IPAddress:   "127.0.0.1",
			UserAgent:   "Mozilla/5.0",
		}
		cmd := r.toCommand("", "")
		assert.Equal(t, "qr-payload", cmd.Code)
		assert.Equal(t, "testuser", cmd.Username)
		assert.Equal(t, "Test User", cmd.DisplayName)
		assert.Empty(t, cmd.ScannerUsername)
		assert.Empty(t, cmd.ScannerOrganizationID)
		assert.Equal(t, "127.0.0.1", cmd.IPAddress)
		assert.Equal(t, "Mozilla/5.0", cmd.UserAgent)
	})

	t.Run("session_identity_is_carried_separately_from_the_body", func(t *testing.T) {
		r := ScanQRCodeRequest{Code: "qr-payload", Username: "body-supplied"}
		cmd := r.toCommand("alice", "org-uuid")
		assert.Equal(t, "alice", cmd.ScannerUsername)
		assert.Equal(t, "org-uuid", cmd.ScannerOrganizationID)
	})
}

func TestConfirmQRCodeRequest_ToCommand(t *testing.T) {
	t.Run("maps_code_and_session_subject", func(t *testing.T) {
		r := ConfirmQRCodeRequest{
			Code:     "qr-payload",
			Approved: true,
		}
		cmd := r.toCommand("alice", "org-uuid")
		assert.Equal(t, "qr-payload", cmd.Code)
		assert.Equal(t, "alice", cmd.ScannerUsername)
		assert.Equal(t, "org-uuid", cmd.ScannerOrganizationID)
		assert.True(t, cmd.Approved)
	})

	t.Run("rejected", func(t *testing.T) {
		r := ConfirmQRCodeRequest{
			Code:     "qr-payload-2",
			Approved: false,
		}
		cmd := r.toCommand("bob", "org-uuid")
		assert.False(t, cmd.Approved)
	})
}

func TestExchangeQRCodeRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := ExchangeQRCodeRequest{CodeID: "qr-001"}
		cmd := r.toCommand()
		assert.Equal(t, "qr-001", cmd.CodeID)
	})

	t.Run("carries_the_protocol_context_like_faceid_complete", func(t *testing.T) {
		r := ExchangeQRCodeRequest{
			CodeID:       "qr-002",
			ResponseType: "code",
			RedirectURI:  "https://rp.example.com/cb",
			State:        "state-1",
			Nonce:        "nonce-1",
			Scope:        []string{"openid", "profile"},
		}
		cmd := r.toCommand()
		assert.Equal(t, "code", cmd.ResponseType)
		assert.Equal(t, "https://rp.example.com/cb", cmd.RedirectURI)
		assert.Equal(t, "state-1", cmd.State)
		assert.Equal(t, "nonce-1", cmd.Nonce)
		assert.Equal(t, []string{"openid", "profile"}, cmd.Scope)
	})
}

func TestInitiateFaceIDRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := InitiateFaceIDRequest{
			OrganizationSlug: "test-org",
			Username:         "testuser",
			ApplicationID:    "app-001",
		}
		cmd := r.toCommand()
		assert.Equal(t, "test-org", cmd.OrganizationSlug)
		assert.Equal(t, "testuser", cmd.Username)
		assert.Equal(t, "app-001", cmd.ApplicationID)
	})
}

func TestVerifyFaceIDRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := VerifyFaceIDRequest{
			SessionID:  "face-session-001",
			ImageData:  "base64-image-data",
			DeviceInfo: "iPhone 15",
		}
		cmd := r.toCommand()
		assert.Equal(t, "face-session-001", cmd.SessionID)
		assert.Equal(t, "base64-image-data", cmd.ImageData)
		assert.Equal(t, "iPhone 15", cmd.DeviceInfo)
	})
}

func TestCompleteFaceIDLoginRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := CompleteFaceIDLoginRequest{
			SessionID:     "face-session-001",
			ApplicationID: "app-001",
			ResponseType:  "code",
			RedirectURI:   "https://example.com/callback",
			State:         "state-001",
			Nonce:         "nonce-001",
			Scope:         []string{"openid", "profile"},
		}
		cmd := r.toCommand()
		assert.Equal(t, "face-session-001", cmd.SessionID)
		assert.Equal(t, "app-001", cmd.ApplicationID, "complete 必须把协议上下文的目标应用传下去")
		assert.Equal(t, "code", cmd.ResponseType)
		assert.Equal(t, "https://example.com/callback", cmd.RedirectURI)
		assert.Equal(t, "state-001", cmd.State)
		assert.Equal(t, "nonce-001", cmd.Nonce)
		assert.Equal(t, []string{"openid", "profile"}, cmd.Scope)
	})
}

func TestInitiatePushMfaRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := InitiatePushMfaRequest{
			OrganizationID: "org-001",
			UserID:         "user-001",
			ApplicationID:  "app-001",
		}
		cmd := r.toCommand()
		assert.Equal(t, "org-001", cmd.OrganizationID)
		assert.Equal(t, "user-001", cmd.UserID)
		assert.Equal(t, "app-001", cmd.ApplicationID)
	})
}

func TestVerifyPushMfaRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := VerifyPushMfaRequest{ChallengeID: "challenge-001"}
		cmd := r.toCommand()
		assert.Equal(t, "challenge-001", cmd.ChallengeID)
	})
}

func TestEnablePushMfaRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := EnablePushMfaRequest{
			UserID:          "user-001",
			MfaPushReceiver: "push-receiver-001",
			MfaPushProvider: "fcm",
		}
		cmd := r.toCommand()
		assert.Equal(t, "user-001", cmd.UserID)
		assert.Equal(t, "push-receiver-001", cmd.MfaPushReceiver)
		assert.Equal(t, "fcm", cmd.MfaPushProvider)
	})
}

func TestDisablePushMfaRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := DisablePushMfaRequest{UserID: "user-001"}
		cmd := r.toCommand()
		assert.Equal(t, "user-001", cmd.UserID)
	})
}

func TestSocialLoginRequest_Validate(t *testing.T) {
	t.Run("valid_request", func(t *testing.T) {
		r := SocialLoginRequest{
			ProviderID:    "github",
			Code:          "auth-code-001",
			ApplicationID: "app-001",
		}
		assert.NoError(t, r.Validate())
	})

	t.Run("missing_provider_id", func(t *testing.T) {
		r := SocialLoginRequest{Code: "auth-code-001"}
		assert.Error(t, r.Validate())
	})

	t.Run("missing_code", func(t *testing.T) {
		r := SocialLoginRequest{ProviderID: "github"}
		assert.Error(t, r.Validate())
	})

	t.Run("scope_exceeds_max_entries", func(t *testing.T) {
		r := SocialLoginRequest{
			ProviderID: "github",
			Code:       "auth-code-001",
			Scope:      make([]string, maxSliceLength+1),
		}
		assert.Error(t, r.Validate())
	})

	t.Run("valid_pkce_verifier", func(t *testing.T) {
		r := SocialLoginRequest{
			ProviderID:   "github",
			Code:         "auth-code-001",
			CodeVerifier: "0123456789abcdef0123456789abcdef0123456789a",
		}
		assert.NoError(t, r.Validate())
	})

	t.Run("pkce_verifier_too_short", func(t *testing.T) {
		r := SocialLoginRequest{
			ProviderID:   "github",
			Code:         "auth-code-001",
			CodeVerifier: "short",
		}
		assert.Error(t, r.Validate())
	})

	t.Run("pkce_verifier_too_long", func(t *testing.T) {
		r := SocialLoginRequest{
			ProviderID:   "github",
			Code:         "auth-code-001",
			CodeVerifier: strings.Repeat("a", 129),
		}
		assert.Error(t, r.Validate())
	})
}

func TestSocialAuthorizeRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := SocialAuthorizeRequest{
			ProviderID:       "github",
			OrganizationSlug: "test-org",
			ApplicationID:    "app-001",
			ResponseType:     "code",
			RedirectURI:      "https://example.com/callback",
			State:            "state-001",
			Nonce:            "nonce-001",
			Scope:            []string{"openid", "profile"},
		}
		cmd := r.toCommand()
		assert.Equal(t, "github", cmd.ProviderID)
		assert.Equal(t, "test-org", cmd.OrganizationSlug)
		assert.Equal(t, "app-001", cmd.ApplicationID)
		assert.Equal(t, "code", cmd.ResponseType)
		assert.Equal(t, "https://example.com/callback", cmd.RedirectURI)
		assert.Equal(t, "state-001", cmd.State)
		assert.Equal(t, "nonce-001", cmd.Nonce)
		assert.Equal(t, []string{"openid", "profile"}, cmd.Scope)
	})
}

func TestLdapLoginRequest_Validate(t *testing.T) {
	t.Run("valid_request", func(t *testing.T) {
		r := LdapLoginRequest{
			OrganizationSlug: "test-org",
			Username:         "testuser",
			Password:         "password123",
			ApplicationID:    "app-001",
		}
		assert.NoError(t, r.Validate())
	})

	t.Run("missing_organization_slug", func(t *testing.T) {
		r := LdapLoginRequest{Username: "testuser", Password: "password123"}
		assert.Error(t, r.Validate())
	})

	t.Run("missing_username", func(t *testing.T) {
		r := LdapLoginRequest{OrganizationSlug: "test-org", Password: "password123"}
		assert.Error(t, r.Validate())
	})

	t.Run("password_exceeds_max_length", func(t *testing.T) {
		r := LdapLoginRequest{
			OrganizationSlug: "test-org",
			Username:         "testuser",
			Password:         strings.Repeat("p", maxPasswordLen+1),
		}
		assert.Error(t, r.Validate())
	})
}

func TestWebAuthnRegistrationBeginRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := WebAuthnRegistrationBeginRequest{
			OrganizationSlug: "test-org",
			Username:         "testuser",
		}
		cmd := r.toCommand()
		assert.Equal(t, "test-org", cmd.OrganizationSlug)
		assert.Equal(t, "testuser", cmd.Username)
	})
}

func TestWebAuthnRegistrationFinishRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := WebAuthnRegistrationFinishRequest{
			OrganizationSlug: "test-org",
			Username:         "testuser",
			Challenge:        "challenge-001",
			CredentialID:     "cred-001",
			PublicKey:        "pubkey-001",
			AttestationType:  "none",
			Transports:       []string{"usb", "nfc"},
		}
		cmd := r.toCommand()
		assert.Equal(t, "test-org", cmd.OrganizationSlug)
		assert.Equal(t, "testuser", cmd.Username)
		assert.Equal(t, "challenge-001", cmd.Challenge)
		assert.Equal(t, "cred-001", cmd.CredentialID)
		assert.Equal(t, "pubkey-001", cmd.PublicKey)
		assert.Equal(t, "none", cmd.AttestationType)
		assert.Equal(t, []string{"usb", "nfc"}, cmd.Transports)
	})
}

func TestWebAuthnAuthenticationBeginRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := WebAuthnAuthenticationBeginRequest{
			OrganizationSlug: "test-org",
			Username:         "testuser",
		}
		cmd := r.toCommand()
		assert.Equal(t, "test-org", cmd.OrganizationSlug)
		assert.Equal(t, "testuser", cmd.Username)
	})
}

func TestWebAuthnAuthenticationFinishRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := WebAuthnAuthenticationFinishRequest{
			OrganizationSlug:  "test-org",
			Username:          "testuser",
			Challenge:         "challenge-001",
			CredentialID:      "cred-001",
			Signature:         "sig-001",
			AuthenticatorData: "auth-data-001",
			ClientDataJSON:    "client-data-001",
		}
		cmd := r.toCommand()
		assert.Equal(t, "test-org", cmd.OrganizationSlug)
		assert.Equal(t, "testuser", cmd.Username)
		assert.Equal(t, "challenge-001", cmd.Challenge)
		assert.Equal(t, "cred-001", cmd.CredentialID)
		assert.Equal(t, "sig-001", cmd.Signature)
		assert.Equal(t, "auth-data-001", cmd.AuthenticatorData)
		assert.Equal(t, "client-data-001", cmd.ClientDataJSON)
	})
}

func TestVerifyCodeRequest_Validate(t *testing.T) {
	t.Run("valid_email_type", func(t *testing.T) {
		r := VerifyCodeRequest{Type: "email", Dest: "user@example.com", Code: "123456"}
		assert.NoError(t, r.Validate())
	})

	t.Run("valid_sms_type", func(t *testing.T) {
		r := VerifyCodeRequest{Type: "sms", Phone: "+8613800138000", Code: "123456"}
		assert.NoError(t, r.Validate())
	})

	t.Run("invalid_type", func(t *testing.T) {
		r := VerifyCodeRequest{Type: "invalid", Code: "123456"}
		assert.Error(t, r.Validate())
	})

	t.Run("missing_code", func(t *testing.T) {
		r := VerifyCodeRequest{Type: "email", Dest: "user@example.com"}
		assert.Error(t, r.Validate())
	})

	t.Run("email_missing_dest", func(t *testing.T) {
		r := VerifyCodeRequest{Type: "email", Code: "123456"}
		assert.Error(t, r.Validate())
	})

	t.Run("sms_missing_phone", func(t *testing.T) {
		r := VerifyCodeRequest{Type: "sms", Code: "123456"}
		assert.Error(t, r.Validate())
	})
}

func TestVerifyCodeRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields_correctly", func(t *testing.T) {
		r := VerifyCodeRequest{
			Dest:        "user@example.com",
			Code:        "123456",
			Type:        "email",
			Phone:       "+8613800138000",
			CountryCode: "CN",
		}
		cmd := r.toCommand()
		assert.Equal(t, "user@example.com", cmd.Dest)
		assert.Equal(t, "123456", cmd.Code)
		assert.Equal(t, "email", cmd.Type)
		assert.Equal(t, "+8613800138000", cmd.Phone)
		assert.Equal(t, "CN", cmd.CountryCode)
	})
}

func TestNativeSsoCompleteRequest_Fields(t *testing.T) {
	t.Run("fields_mapped", func(t *testing.T) {
		r := NativeSsoCompleteRequest{
			AccessToken:   "at-001",
			ApplicationID: "app-001",
		}
		assert.Equal(t, "at-001", r.AccessToken)
		assert.Equal(t, "app-001", r.ApplicationID)
	})
}

// testHTTPRequest 与 handler_test.go 共享 — 避免重复定义。
