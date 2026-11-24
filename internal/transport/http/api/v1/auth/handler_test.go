package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformconfig "github.com/guliangiam/guliangiam/internal/platform/config"
)

// ---------------------------------------------------------------------------
// Auth Handler request validation and response format tests
// ---------------------------------------------------------------------------

func TestHandler_LoginRequest_Validate_Boundary(t *testing.T) {
	tests := []struct {
		name    string
		req     LoginRequest
		wantErr bool
	}{
		{"valid login", LoginRequest{
			OrganizationSlug: "acme", Username: "alice", Password: "password123", ApplicationID: "app-1",
		}, false},
		{"empty request", LoginRequest{}, false},
		{"password exceeds max", LoginRequest{
			Password: strings.Repeat("p", maxPasswordLen+1),
		}, true},
		{"username exceeds max", LoginRequest{
			Username: strings.Repeat("u", maxStringLength+1),
		}, true},
		{"org slug exceeds max", LoginRequest{
			OrganizationSlug: strings.Repeat("o", maxStringLength+1),
		}, true},
		{"application ID exceeds max", LoginRequest{
			ApplicationID: strings.Repeat("a", maxStringLength+1),
		}, true},
		{"passcode exceeds max", LoginRequest{
			Passcode: strings.Repeat("p", maxStringLength+1),
		}, true},
		{"recovery code exceeds max", LoginRequest{
			RecoveryCode: strings.Repeat("r", maxStringLength+1),
		}, true},
		{"scope exceeds max entries", LoginRequest{
			Scope: make([]string, maxSliceLength+1),
		}, true},
		{"scope entry exceeds max length", LoginRequest{
			Scope: []string{strings.Repeat("s", maxStringLength+1)},
		}, true},
		{"MFA login with passcode", LoginRequest{
			OrganizationSlug: "acme", Username: "alice", MfaType: "totp", Passcode: "123456",
		}, false},
		{"login with SAML request", LoginRequest{
			OrganizationSlug: "acme", Username: "alice", SAMLRequest: "base64-saml-request", RelayState: "relay",
		}, false},
		{"login with CAS service", LoginRequest{
			OrganizationSlug: "acme", Username: "alice", CASService: "https://cas.example.com/service",
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandler_ResetPasswordRequest_Validate_Boundary(t *testing.T) {
	tests := []struct {
		name    string
		req     ResetPasswordRequest
		wantErr bool
	}{
		{"valid reset", ResetPasswordRequest{
			OrganizationSlug: "acme", Username: "alice", VerificationCode: "123456", NewPassword: "newpass123",
		}, false},
		{"empty request", ResetPasswordRequest{}, false},
		{"new password exceeds max", ResetPasswordRequest{
			NewPassword: strings.Repeat("p", maxPasswordLen+1),
		}, true},
		{"org slug exceeds max", ResetPasswordRequest{
			OrganizationSlug: strings.Repeat("o", maxStringLength+1),
		}, true},
		{"username exceeds max", ResetPasswordRequest{
			Username: strings.Repeat("u", maxStringLength+1),
		}, true},
		{"verification code exceeds max", ResetPasswordRequest{
			VerificationCode: strings.Repeat("v", maxStringLength+1),
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandler_SignupRequest_Validate_Boundary(t *testing.T) {
	tests := []struct {
		name    string
		req     SignupRequest
		wantErr bool
	}{
		{"valid signup", SignupRequest{
			OrganizationSlug: "acme", Username: "newuser", Password: "password123", Email: "user@example.com",
		}, false},
		{"empty request", SignupRequest{}, false},
		{"password exceeds max", SignupRequest{
			Password: strings.Repeat("p", maxPasswordLen+1),
		}, true},
		{"username exceeds max", SignupRequest{
			Username: strings.Repeat("u", maxStringLength+1),
		}, true},
		{"email exceeds max", SignupRequest{
			Email: strings.Repeat("e", maxStringLength+1),
		}, true},
		{"phone exceeds max", SignupRequest{
			Phone: strings.Repeat("p", maxStringLength+1),
		}, true},
		{"signup app ID exceeds max", SignupRequest{
			SignupAppID: strings.Repeat("a", maxStringLength+1),
		}, true},
		{"verification code exceeds max", SignupRequest{
			VerificationCode: strings.Repeat("v", maxStringLength+1),
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandler_CheckPasswordRequest_Validate_Boundary(t *testing.T) {
	tests := []struct {
		name    string
		req     CheckPasswordRequest
		wantErr bool
	}{
		{"valid check", CheckPasswordRequest{
			OrganizationSlug: "acme", Username: "alice", Password: "password123",
		}, false},
		{"empty request", CheckPasswordRequest{}, false},
		{"password exceeds max", CheckPasswordRequest{
			Password: strings.Repeat("p", maxPasswordLen+1),
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Request-to-Command mapping tests
// ---------------------------------------------------------------------------

func TestHandler_LoginRequest_ToCommand(t *testing.T) {
	req := LoginRequest{
		OrganizationSlug:  "acme",
		Username:          "alice",
		Password:          "password123",
		ApplicationID:     "app-1",
		ChallengeID:       "challenge-1",
		MfaType:           "totp",
		Passcode:          "123456",
		EnableMfaRemember: true,
		ResponseType:      "code",
		RedirectURI:       "https://example.com/callback",
		State:             "state-123",
		Nonce:             "nonce-456",
		Scope:             []string{"openid", "profile"},
		SAMLRequest:       "saml-req",
		RelayState:        "relay-state",
		CASService:        "https://cas.example.com",
	}

	httpReq := testHTTPRequest()
	cmd := req.toCommand(httpReq)

	assert.Equal(t, "acme", cmd.OrganizationSlug)
	assert.Equal(t, "alice", cmd.Username)
	assert.Equal(t, "password123", cmd.Password)
	assert.Equal(t, "app-1", cmd.ApplicationID)
	assert.Equal(t, "challenge-1", cmd.ChallengeID)
	assert.Equal(t, "totp", cmd.MfaType)
	assert.Equal(t, "123456", cmd.Passcode)
	assert.True(t, cmd.EnableMfaRemember)
	assert.Equal(t, "code", cmd.ResponseType)
	assert.Equal(t, "https://example.com/callback", cmd.RedirectURI)
	assert.Equal(t, "state-123", cmd.State)
	assert.Equal(t, "nonce-456", cmd.Nonce)
	assert.Equal(t, []string{"openid", "profile"}, cmd.Scope)
	assert.Equal(t, "saml-req", cmd.SAMLRequest)
	assert.Equal(t, "relay-state", cmd.RelayState)
	assert.Equal(t, "https://cas.example.com", cmd.CASService)
}

func TestHandler_AuthorizeRequest_ToCommand(t *testing.T) {
	req := AuthorizeRequest{
		ClientID:        "app-1",
		RedirectURI:     "https://example.com/callback",
		ResponseType:    "code",
		RequestedScopes: []string{"openid", "profile"},
		State:           "state-123",
		Nonce:           "nonce-456",
		CodeChallenge:   "challenge-abc",
		ChallengeMethod: "S256",
		Resource:        "https://api.example.com",
	}

	httpReq := testHTTPRequest()
	cmd := req.toCommand(httpReq)

	assert.Equal(t, "app-1", cmd.ClientID)
	assert.Equal(t, "https://example.com/callback", cmd.RedirectURI)
	assert.Equal(t, "code", cmd.ResponseType)
	assert.Equal(t, []string{"openid", "profile"}, cmd.RequestedScopes)
	assert.Equal(t, "state-123", cmd.State)
	assert.Equal(t, "challenge-abc", cmd.CodeChallenge)
	assert.Equal(t, "S256", cmd.ChallengeMethod)
	assert.Equal(t, "https://api.example.com", cmd.Resource)
}

func TestHandler_ExchangeAuthorizationCodeRequest_ToCommand(t *testing.T) {
	req := ExchangeAuthorizationCodeRequest{
		GrantType:    "authorization_code",
		ClientID:     "app-1",
		ClientSecret: "secret-1",
		Code:         "auth-code-1",
		RedirectURI:  "https://example.com/callback",
		CodeVerifier: "verifier-abc",
		Resource:     "https://api.example.com",
	}

	cmd := req.toCommand(httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/exchange", nil))
	assert.Equal(t, "authorization_code", cmd.GrantType)
	assert.Equal(t, "app-1", cmd.ClientID)
	assert.Equal(t, "secret-1", cmd.ClientSecret)
	assert.Equal(t, "auth-code-1", cmd.Code)
	assert.Equal(t, "https://example.com/callback", cmd.RedirectURI)
	assert.Equal(t, "verifier-abc", cmd.CodeVerifier)
	assert.Equal(t, "https://api.example.com", cmd.Resource)
}

func TestHandler_GrantConsentRequest_ToCommand(t *testing.T) {
	req := GrantConsentRequest{
		ApplicationID: "app-1",
		GrantedScopes: []string{"openid", "profile", "email"},
	}

	httpReq := testHTTPRequest()
	cmd := req.toCommand(httpReq)
	assert.Equal(t, "app-1", cmd.ApplicationID)
	assert.Equal(t, []string{"openid", "profile", "email"}, cmd.GrantedScopes)
}

func TestHandler_ResetPasswordRequest_ToCommand(t *testing.T) {
	req := ResetPasswordRequest{
		OrganizationSlug: "acme",
		Username:         "alice",
		VerificationCode: "123456",
		NewPassword:      "newpass123",
	}

	cmd := req.toCommand()
	assert.Equal(t, "acme", cmd.OrganizationSlug)
	assert.Equal(t, "alice", cmd.Username)
	assert.Equal(t, "123456", cmd.VerificationCode)
	assert.Equal(t, "newpass123", cmd.NewPassword)
}

func TestHandler_SignupRequest_ToCommand(t *testing.T) {
	req := SignupRequest{
		OrganizationSlug: "acme",
		Username:         "newuser",
		Password:         "password123",
		Email:            "user@example.com",
		Phone:            "+8613800138000",
		SignupAppID:      "app-1",
		VerificationCode: "654321",
		InvitationCode:   "invite-abc",
	}

	cmd := req.toCommand()
	assert.Equal(t, "acme", cmd.OrganizationSlug)
	assert.Equal(t, "newuser", cmd.Username)
	assert.Equal(t, "password123", cmd.Password)
	assert.Equal(t, "user@example.com", cmd.Email)
	assert.Equal(t, "+8613800138000", cmd.Phone)
	assert.Equal(t, "app-1", cmd.SignupAppID)
	assert.Equal(t, "654321", cmd.VerificationCode)
	assert.Equal(t, "invite-abc", cmd.InvitationCode)
}

func TestHandler_DeviceAuthorizeRequest_ToCommand(t *testing.T) {
	req := DeviceAuthorizeRequest{ClientID: "app-1", Scope: "openid profile"}
	cmd := req.toCommand(httptest.NewRequest(http.MethodPost, "/api/v1/auth/device/authorize", nil))
	assert.Equal(t, "app-1", cmd.ClientID)
	assert.Equal(t, "openid profile", cmd.Scope)
}

func TestHandler_DeviceTokenRequest_ToCommand(t *testing.T) {
	req := DeviceTokenRequest{ClientID: "app-1", ClientSecret: "secret", DeviceCode: "device-abc"}
	cmd := req.toCommand()
	assert.Equal(t, "app-1", cmd.ClientID)
	assert.Equal(t, "secret", cmd.ClientSecret)
	assert.Equal(t, "device-abc", cmd.DeviceCode)
}

func TestHandler_JwtBearerTokenRequest_ToCommand(t *testing.T) {
	req := JwtBearerTokenRequest{
		GrantType:           "urn/ietf/params/oauth/grant-type/jwt-bearer",
		ClientAssertionType: "urn/ietf/params/oauth/client-assertion-type/jwt-bearer",
		ClientAssertion:     "eyJhbGciOiJSUzI1NiJ9...",
		Scope:               "openid",
	}
	httpRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", strings.NewReader(""))
	cmd := req.toCommand(httpRequest)
	assert.Equal(t, "urn/ietf/params/oauth/client-assertion-type/jwt-bearer", cmd.ClientAssertionType)
	assert.NotEmpty(t, cmd.ClientAssertion)
	assert.Equal(t, "openid", cmd.Scope)
	assert.Equal(t, platformconfig.AbsoluteRequestURL(httpRequest), cmd.TokenEndpointURL)
}

func TestHandler_TokenExchangeRequest_ToCommand(t *testing.T) {
	req := TokenExchangeRequest{
		GrantType:          "urn/ietf/params/oauth/grant-type/token-exchange",
		SubjectToken:       "subject-token",
		SubjectTokenType:   "urn/ietf/params/oauth/token-type/access_token",
		ActorToken:         "actor-token",
		ActorTokenType:     "urn/ietf/params/oauth/token-type/access_token",
		Resource:           "https://api.example.com",
		Audience:           "app-1",
		Scope:              "openid",
		ClientID:           "app-1",
		ClientSecret:       "secret",
		RequestedTokenType: "urn/ietf/params/oauth/token-type/access_token",
	}
	cmd := req.toCommand()
	assert.Equal(t, "subject-token", cmd.SubjectToken)
	assert.Equal(t, "actor-token", cmd.ActorToken)
	assert.Equal(t, "https://api.example.com", cmd.Resource)
}

func TestHandler_TokenIntrospectionRequest_ToCommand(t *testing.T) {
	req := TokenIntrospectionRequest{
		Token:     "access-token-123",
		TokenType: "access_token",
		ClientID:  "app-1",
	}
	cmd := req.toCommand()
	assert.Equal(t, "access-token-123", cmd.Token)
	assert.Equal(t, "access_token", cmd.TokenType)
	assert.Equal(t, "app-1", cmd.ClientID)
}

func TestHandler_ClientAuthRequest_ToCommand(t *testing.T) {
	req := ClientAuthRequest{
		ClientID:            "app-1",
		ClientSecret:        "secret",
		ClientAssertion:     "assertion",
		ClientAssertionType: "urn/ietf/params/oauth/client-assertion-type/jwt-bearer",
	}
	httpRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/client/authenticate", strings.NewReader(""))
	cmd := req.toCommand(httpRequest)
	assert.Equal(t, "app-1", cmd.ClientID)
	assert.Equal(t, "secret", cmd.ClientSecret)
	assert.Equal(t, "assertion", cmd.ClientAssertion)
	assert.Equal(t, platformconfig.AbsoluteRequestURL(httpRequest), cmd.TokenEndpointURL)
}

// ---------------------------------------------------------------------------
// QR Code login request mapping tests
// ---------------------------------------------------------------------------

func TestHandler_CreateQRCodeRequest_ToCommand(t *testing.T) {
	req := CreateQRCodeRequest{OrganizationID: "org-1", ApplicationID: "app-1"}
	cmd := req.toCommand()
	assert.Equal(t, "org-1", cmd.OrganizationID)
	assert.Equal(t, "app-1", cmd.ApplicationID)
}

func TestHandler_ScanQRCodeRequest_ToCommand(t *testing.T) {
	req := ScanQRCodeRequest{
		Code: "qr-payload", Username: "alice",
		IPAddress: "127.0.0.1", UserAgent: "TestAgent",
	}
	cmd := req.toCommand("", "")
	assert.Equal(t, "qr-payload", cmd.Code)
	assert.Equal(t, "alice", cmd.Username)
	assert.Equal(t, "127.0.0.1", cmd.IPAddress)
}

func TestHandler_ConfirmQRCodeRequest_ToCommand(t *testing.T) {
	req := ConfirmQRCodeRequest{Code: "qr-payload", Approved: true}
	cmd := req.toCommand("alice", "org-uuid")
	assert.Equal(t, "qr-payload", cmd.Code)
	assert.Equal(t, "alice", cmd.ScannerUsername)
	assert.Equal(t, "org-uuid", cmd.ScannerOrganizationID)
	assert.True(t, cmd.Approved)
}

func TestHandler_ExchangeQRCodeRequest_ToCommand(t *testing.T) {
	req := ExchangeQRCodeRequest{CodeID: "code-1", ResponseType: "code", State: "st"}
	cmd := req.toCommand()
	assert.Equal(t, "code-1", cmd.CodeID)
	assert.Equal(t, "code", cmd.ResponseType)
	assert.Equal(t, "st", cmd.State)
}

// ---------------------------------------------------------------------------
// Face ID login request mapping tests
// ---------------------------------------------------------------------------

func TestHandler_InitiateFaceIDRequest_ToCommand(t *testing.T) {
	req := InitiateFaceIDRequest{
		OrganizationSlug: "acme", Username: "alice", ApplicationID: "app-1",
	}
	cmd := req.toCommand()
	assert.Equal(t, "acme", cmd.OrganizationSlug)
	assert.Equal(t, "alice", cmd.Username)
}

func TestHandler_VerifyFaceIDRequest_ToCommand(t *testing.T) {
	req := VerifyFaceIDRequest{
		SessionID: "session-1", ImageData: "base64-image", DeviceInfo: "iPhone",
	}
	cmd := req.toCommand()
	assert.Equal(t, "session-1", cmd.SessionID)
	assert.Equal(t, "base64-image", cmd.ImageData)
}

func TestHandler_CompleteFaceIDLoginRequest_ToCommand(t *testing.T) {
	req := CompleteFaceIDLoginRequest{
		SessionID: "session-1", ApplicationID: "app-1", Scope: []string{"openid", "profile"},
	}
	cmd := req.toCommand()
	assert.Equal(t, "session-1", cmd.SessionID)
	assert.Equal(t, "app-1", cmd.ApplicationID)
	assert.Equal(t, []string{"openid", "profile"}, cmd.Scope)
}

// ---------------------------------------------------------------------------
// GetAccount request mapping tests
// ---------------------------------------------------------------------------

func TestHandler_GetAccountRequest_ToQuery(t *testing.T) {
	req := GetAccountRequest{
		OrganizationSlug: "acme",
		Username:         "alice",
		AccessToken:      "access-token-123",
	}
	query := req.toQuery()
	assert.Equal(t, "acme", query.OrganizationSlug)
	assert.Equal(t, "alice", query.Username)
	assert.Equal(t, "access-token-123", query.AccessToken)
}

// ---------------------------------------------------------------------------
// HTTP request decoding tests
// ---------------------------------------------------------------------------

func TestHandler_DecodeExchangeAuthorizationCodeRequest_FormEncoded(t *testing.T) {
	form := "grant_type=authorization_code&client_id=app-1&code=auth-code&redirect_uri=https%3A%2F%2Fexample.com%2Fcallback"
	req, err := http.NewRequest("POST", "/token", strings.NewReader(form))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	decoded, err := decodeExchangeAuthorizationCodeRequest(req)
	assert.NoError(t, err)
	assert.Equal(t, "authorization_code", decoded.GrantType)
	assert.Equal(t, "app-1", decoded.ClientID)
	assert.Equal(t, "auth-code", decoded.Code)
	assert.Equal(t, "https://example.com/callback", decoded.RedirectURI)
}

func TestHandler_DecodeExchangeAuthorizationCodeRequest_JSON(t *testing.T) {
	jsonBody := `{"grantType":"authorization_code","clientId":"app-1","code":"auth-code","redirectUri":"https://example.com/callback"}`
	req, err := http.NewRequest("POST", "/token", strings.NewReader(jsonBody))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	decoded, err := decodeExchangeAuthorizationCodeRequest(req)
	assert.NoError(t, err)
	assert.Equal(t, "authorization_code", decoded.GrantType)
	assert.Equal(t, "app-1", decoded.ClientID)
	assert.Equal(t, "auth-code", decoded.Code)
}

func TestHandler_DecodeExchangeAuthorizationCodeRequest_BasicAuth(t *testing.T) {
	form := "grant_type=authorization_code&code=auth-code&redirect_uri=https%3A%2F%2Fexample.com%2Fcallback"
	req, err := http.NewRequest("POST", "/token", strings.NewReader(form))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("app-1", "secret-1")

	decoded, err := decodeExchangeAuthorizationCodeRequest(req)
	assert.NoError(t, err)
	assert.Equal(t, "app-1", decoded.ClientID)
	assert.Equal(t, "secret-1", decoded.ClientSecret)
}

// ---------------------------------------------------------------------------
// Client Credentials request mapping tests
// ---------------------------------------------------------------------------

func TestHandler_ClientCredentialsRequest_ToCommand(t *testing.T) {
	req := ClientCredentialsRequest{
		ClientID:     "app-1",
		ClientSecret: "secret",
		Scope:        "openid profile",
	}
	cmd := req.toCommand(httptest.NewRequest(http.MethodPost, "/tokens/client-credentials", nil))
	assert.Equal(t, "app-1", cmd.ClientID)
	assert.Equal(t, "secret", cmd.ClientSecret)
	assert.Equal(t, "openid profile", cmd.Scope)
}

func TestHandler_ClientCredentialsRequestValidation(t *testing.T) {
	t.Run("dedicated endpoint request", func(t *testing.T) {
		request := ClientCredentialsRequest{ClientSecret: strings.Repeat("s", maxStringLength+1)}
		assert.Error(t, request.Validate())
	})

	t.Run("standard token endpoint form", func(t *testing.T) {
		body := "grant_type=client_credentials&client_id=sa-client&client_secret=" + strings.Repeat("s", maxStringLength+1)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/exchange", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		_, err := decodeExchangeAuthorizationCodeRequest(req)
		assert.Error(t, err)
	})
}

func TestHandler_TokenEndpointRoutesClientCredentialsGrant(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/exchange", strings.NewReader("grant_type=client_credentials&scope=orders%3Aread"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("sa-client", "sa-secret")

	decoded, err := decodeExchangeAuthorizationCodeRequest(req)
	require.NoError(t, err)
	assert.True(t, decoded.isClientCredentialsGrant())
	command := decoded.toClientCredentialsCommand(req)
	assert.Equal(t, "sa-client", command.ClientID)
	assert.Equal(t, "sa-secret", command.ClientSecret)
	assert.Equal(t, "orders:read", command.Scope)
	assert.Equal(t, platformconfig.AbsoluteRequestURL(req), command.TokenEndpointURL)
}

// ---------------------------------------------------------------------------
// DPoP and Backchannel Logout request mapping tests
// ---------------------------------------------------------------------------

func TestHandler_DPoPProofValidationRequest_ToCommand(t *testing.T) {
	req := DPoPProofValidationRequest{
		DPoPHeader: "dpop-header-value",
		Method:     "POST",
		URL:        "https://api.example.com/resource",
	}
	cmd := req.toCommand()
	assert.Equal(t, "dpop-header-value", cmd.DPoPHeader)
	assert.Equal(t, "POST", cmd.Method)
}

func TestHandler_BackchannelLogoutRequest_ToCommand(t *testing.T) {
	req := BackchannelLogoutRequest{
		UserID:    "user-1",
		SessionID: "session-1",
		ClientID:  "app-1",
	}
	cmd := req.toCommand(httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/exchange", nil))
	assert.Equal(t, "user-1", cmd.UserID)
	assert.Equal(t, "session-1", cmd.SessionID)
	assert.Equal(t, "app-1", cmd.ClientID)
}

// ---------------------------------------------------------------------------
// Session context resolution tests
// ---------------------------------------------------------------------------

func TestHandler_ResolveSessionContext(t *testing.T) {
	// Verify the SessionContext struct mapping
	ctx := SessionContext{
		OrganizationSlug: "acme",
		Username:         "alice",
		AccessToken:      "access-token-123",
		Source:           "repository",
	}
	assert.Equal(t, "acme", ctx.OrganizationSlug)
	assert.Equal(t, "alice", ctx.Username)
	assert.Equal(t, "access-token-123", ctx.AccessToken)
	assert.Equal(t, "repository", ctx.Source)
}

// ---------------------------------------------------------------------------
// Boundary scenario tests for auth
// ---------------------------------------------------------------------------

func TestHandler_LoginRequest_Boundary(t *testing.T) {
	t.Run("empty scope array", func(t *testing.T) {
		req := LoginRequest{
			OrganizationSlug: "acme",
			Username:         "alice",
			Password:         "password123",
			Scope:            []string{},
		}
		assert.NoError(t, req.Validate())
	})

	t.Run("nil scope array", func(t *testing.T) {
		req := LoginRequest{
			OrganizationSlug: "acme",
			Username:         "alice",
			Password:         "password123",
			Scope:            nil,
		}
		assert.NoError(t, req.Validate())
	})

	t.Run("max length password", func(t *testing.T) {
		req := LoginRequest{
			Password: strings.Repeat("p", maxPasswordLen),
		}
		assert.NoError(t, req.Validate())
	})

	t.Run("max length username", func(t *testing.T) {
		req := LoginRequest{
			Username: strings.Repeat("u", maxStringLength),
		}
		assert.NoError(t, req.Validate())
	})

	t.Run("max scope entries", func(t *testing.T) {
		scopes := make([]string, maxSliceLength)
		for i := range scopes {
			scopes[i] = "scope"
		}
		req := LoginRequest{Scope: scopes}
		assert.NoError(t, req.Validate())
	})
}

func TestHandler_SignupRequest_Boundary(t *testing.T) {
	t.Run("minimal signup", func(t *testing.T) {
		req := SignupRequest{
			OrganizationSlug: "acme",
			Username:         "newuser",
			Password:         "password123",
		}
		assert.NoError(t, req.Validate())
	})

	t.Run("signup with invitation code", func(t *testing.T) {
		req := SignupRequest{
			OrganizationSlug: "acme",
			Username:         "newuser",
			Password:         "password123",
			InvitationCode:   "invite-abc",
		}
		assert.NoError(t, req.Validate())
	})
}

// testHTTPRequest 构造一个最小的 http.Request，供 toCommand 方法读取 client IP / 协议等。
func testHTTPRequest() *http.Request {
	return httptest.NewRequest("POST", "/test", nil)
}
