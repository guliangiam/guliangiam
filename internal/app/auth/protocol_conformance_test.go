package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	authorizationcodedomain "github.com/guliangiam/guliangiam/internal/domain/authorizationcodes"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// These tests pin the authorization-endpoint baseline of 设计报告 14.6. They are
// the executable form of the : each assertion
// names the rule it protects, so a regression fails with the rule, not just with
// an unexpected value.

var authorizationCodeValuePattern = regexp.MustCompile(`^gl_ac_[A-Za-z0-9_-]+$`)

func TestNewAuthorizationCodeSecret_Has256BitEntropy(t *testing.T) {
	const samples = 512
	seen := make(map[string]struct{}, samples)

	for i := 0; i < samples; i++ {
		value, err := newAuthorizationCodeSecret()
		if err != nil {
			t.Fatalf("newAuthorizationCodeSecret() error = %v", err)
		}
		if !authorizationCodeValuePattern.MatchString(value) {
			t.Fatalf("code %q does not match the expected base64url shape", value)
		}
		if _, duplicate := seen[value]; duplicate {
			t.Fatalf("code %q was issued twice in %d samples", value, samples)
		}
		seen[value] = struct{}{}

		randomPart := strings.TrimPrefix(value, authorizationCodeSecretPrefix)
		decoded, err := base64.RawURLEncoding.DecodeString(randomPart)
		if err != nil {
			t.Fatalf("code %q is not valid unpadded base64url: %v", value, err)
		}
		if len(decoded) != authorizationCodeEntropyBytes {
			t.Fatalf("code entropy = %d bytes, want %d", len(decoded), authorizationCodeEntropyBytes)
		}
	}
}

// TestNewAuthorizationCodeSecret_IsNotAUUID guards the specific regression that
// recorded: a UUIDv7 value carries only 74 random bits, so a code must
// never look like one. The prefix and the 43-character base64url body already
// make the shapes disjoint; the assertion keeps it that way.
func TestNewAuthorizationCodeSecret_IsNotAUUID(t *testing.T) {
	uuidShape := regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[0-9a-f]{4}-[0-9a-f]{12}$`)

	value, err := newAuthorizationCodeSecret()
	if err != nil {
		t.Fatalf("newAuthorizationCodeSecret() error = %v", err)
	}
	randomPart := strings.TrimPrefix(value, authorizationCodeSecretPrefix)
	if uuidShape.MatchString(value) || uuidShape.MatchString(randomPart) {
		t.Fatalf("authorization code %q looks like a UUIDv7 value", value)
	}
	if len(randomPart) != base64.RawURLEncoding.EncodedLen(authorizationCodeEntropyBytes) {
		t.Fatalf("random part length = %d, want %d", len(randomPart), base64.RawURLEncoding.EncodedLen(authorizationCodeEntropyBytes))
	}
}

func TestNewAuthorizationCode_BindsPKCEStateAndResource(t *testing.T) {
	service := Service{UUIDGenerator: fakeUUIDGenerator{fixedID: "code-id-1"}}
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

	code, err := service.newAuthorizationCode(authorizationCodeIssuance{
		OrganizationID:   "org-1",
		OrganizationSlug: "built-in",
		UserID:           "user-1",
		Username:         "jane",
		ApplicationID:    "app-1",
		ApplicationSlug:  "portal",
		RedirectURI:      "https://portal.example.com/callback",
		Scopes:           []string{"openid", "profile"},
		State:            "state-1",
		Nonce:            "nonce-1",
		CodeChallenge:    "challenge-1",
		ChallengeMethod:  "S256",
		Resource:         "https://api.example.com",
	}, now)
	if err != nil {
		t.Fatalf("newAuthorizationCode() error = %v", err)
	}

	if code.ID != "code-id-1" {
		t.Fatalf("code id = %q, want code-id-1", code.ID)
	}
	if code.CodeChallenge != "challenge-1" || code.ChallengeMethod != "S256" {
		t.Fatalf("PKCE binding lost: challenge=%q method=%q", code.CodeChallenge, code.ChallengeMethod)
	}
	if code.Resource != "https://api.example.com" {
		t.Fatalf("resource binding lost: %q", code.Resource)
	}
	if code.State != "state-1" || code.Nonce != "nonce-1" {
		t.Fatalf("state/nonce binding lost: state=%q nonce=%q", code.State, code.Nonce)
	}
	if want := now.Add(authorizationCodeTTL); !code.ExpiresAt.Equal(want) {
		t.Fatalf("expiresAt = %v, want %v", code.ExpiresAt, want)
	}
}

func TestNewAuthorizationCode_RequiresPKCE(t *testing.T) {
	service := Service{UUIDGenerator: fakeUUIDGenerator{fixedID: "code-id-1"}}
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

	cases := map[string]authorizationCodeIssuance{
		"missing challenge": {CodeChallenge: "", ChallengeMethod: "S256"},
		"missing method":    {CodeChallenge: "challenge-1", ChallengeMethod: ""},
		"plain method":      {CodeChallenge: "challenge-1", ChallengeMethod: "plain"},
	}
	for name, issuance := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := service.newAuthorizationCode(issuance, now); err == nil {
				t.Fatal("expected a PKCE binding error")
			}
		})
	}
}

// TestHandleLoggedIn_RetiresImplicitResponseTypes covers the server half of
// 设计报告 14.6.1「禁用 response_type=token」: no flow may answer `token` or `id_token` any more.
func TestHandleLoggedIn_RetiresImplicitResponseTypes(t *testing.T) {
	service := Service{}

	for _, responseType := range []string{"token", "id_token", "token id_token"} {
		t.Run(responseType, func(t *testing.T) {
			_, err := service.handleLoggedIn(context.Background(), nil, LoginCommand{ResponseType: responseType}, nil, nil, nil, "", time.Now())
			if err == nil {
				t.Fatalf("expected rejection for responseType=%s", responseType)
			}
			protocolErr, ok := err.(apperrors.ProtocolCodeError)
			if !ok {
				t.Fatalf("expected a protocol-coded error, got %T: %v", err, err)
			}
			if protocolErr.ProtocolCode() != "unsupported_response_type" {
				t.Fatalf("protocol code = %q, want unsupported_response_type", protocolErr.ProtocolCode())
			}
		})
	}
}

// TestGetOIDCDiscovery_AdvertisesOnlyCode keeps the discovery document in step
// with the server: an advertised response type that is refused would be a
// capability lie (设计报告 14.13.2「response_types_supported」).
func TestGetOIDCDiscovery_AdvertisesOnlyCode(t *testing.T) {
	discovery, err := (&Service{}).GetOIDCDiscovery(context.Background(), "https://iam.example.com")
	if err != nil {
		t.Fatalf("GetOIDCDiscovery() error = %v", err)
	}
	if len(discovery.ResponseTypesSupported) != 1 || discovery.ResponseTypesSupported[0] != "code" {
		t.Fatalf("response_types_supported = %v, want [code]", discovery.ResponseTypesSupported)
	}
	if len(discovery.CodeChallengeMethodsSupported) != 1 || discovery.CodeChallengeMethodsSupported[0] != "S256" {
		t.Fatalf("code_challenge_methods_supported = %v, want [S256]", discovery.CodeChallengeMethodsSupported)
	}
}

// TestValidateAuthorizationCodeExchange_EnforcesPKCE covers from the token
// endpoint side: `plain`, a missing challenge, a missing verifier and a wrong
// verifier must all be refused.
func TestValidateAuthorizationCodeExchange_EnforcesPKCE(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	verifier := "correct-verifier"
	application := &applicationdomain.Application{ID: "app-1"}

	base := func() *authorizationcodedomain.AuthorizationCode {
		return &authorizationcodedomain.AuthorizationCode{
			ApplicationID:   "app-1",
			RedirectURI:     "https://portal.example.com/callback",
			ExpiresAt:       now.Add(time.Minute),
			CodeChallenge:   buildPKCEChallenge(verifier),
			ChallengeMethod: "S256",
		}
	}

	cases := []struct {
		name    string
		command ExchangeAuthorizationCodeCommand
		mutate  func(*authorizationcodedomain.AuthorizationCode)
		wantErr bool
	}{
		{name: "valid", command: ExchangeAuthorizationCodeCommand{RedirectURI: "https://portal.example.com/callback", CodeVerifier: verifier}},
		{name: "missing verifier", command: ExchangeAuthorizationCodeCommand{RedirectURI: "https://portal.example.com/callback"}, wantErr: true},
		{name: "wrong verifier", command: ExchangeAuthorizationCodeCommand{RedirectURI: "https://portal.example.com/callback", CodeVerifier: "other"}, wantErr: true},
		{
			name:    "code bound without challenge",
			command: ExchangeAuthorizationCodeCommand{RedirectURI: "https://portal.example.com/callback", CodeVerifier: verifier},
			mutate:  func(c *authorizationcodedomain.AuthorizationCode) { c.CodeChallenge = ""; c.ChallengeMethod = "" },
			wantErr: true,
		},
		{
			name:    "plain challenge method",
			command: ExchangeAuthorizationCodeCommand{RedirectURI: "https://portal.example.com/callback", CodeVerifier: verifier},
			mutate:  func(c *authorizationcodedomain.AuthorizationCode) { c.ChallengeMethod = "plain" },
			wantErr: true,
		},
		{
			name:    "redirect uri mismatch",
			command: ExchangeAuthorizationCodeCommand{RedirectURI: "https://portal.example.com/other", CodeVerifier: verifier},
			wantErr: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			code := base()
			if testCase.mutate != nil {
				testCase.mutate(code)
			}
			err := validateAuthorizationCodeExchange(testCase.command, application, code, now)
			if testCase.wantErr && err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !testCase.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestUnsupportedResponseTypeError_CarriesProtocolCode(t *testing.T) {
	err := newUnsupportedResponseTypeError("token")

	var protocolErr apperrors.ProtocolCodeError
	if !errors.As(err, &protocolErr) {
		t.Fatalf("error %T does not implement apperrors.ProtocolCodeError", err)
	}
	if protocolErr.ProtocolCode() != "unsupported_response_type" {
		t.Fatalf("protocol code = %q", protocolErr.ProtocolCode())
	}
	if status := err.(interface{ HTTPStatus() int }).HTTPStatus(); status != 400 {
		t.Fatalf("HTTP status = %d, want 400", status)
	}
}
