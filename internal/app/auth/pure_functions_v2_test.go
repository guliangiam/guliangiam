package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	authorizationcodedomain "github.com/guliangiam/guliangiam/internal/domain/authorizationcodes"
	mfachallengedomain "github.com/guliangiam/guliangiam/internal/domain/mfachallenges"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Token Exchange Pure Functions ---

func TestShouldIssueAuthorizationCodeRefreshToken(t *testing.T) {
	assert.True(t, shouldIssueAuthorizationCodeRefreshToken([]string{"refresh_token"}, []string{"offline_access"}))
	assert.False(t, shouldIssueAuthorizationCodeRefreshToken([]string{"refresh_token"}, []string{"openid"}))
	assert.False(t, shouldIssueAuthorizationCodeRefreshToken([]string{"authorization_code"}, []string{"offline_access"}))
	assert.False(t, shouldIssueAuthorizationCodeRefreshToken(nil, nil))
	assert.False(t, shouldIssueAuthorizationCodeRefreshToken([]string{}, []string{}))
}

func TestBuildPKCEChallenge(t *testing.T) {
	// Known test vector from RFC 7636
	challenge := buildPKCEChallenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")
	assert.Equal(t, "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", challenge)

	// Empty verifier
	challenge = buildPKCEChallenge("")
	sum := sha256.Sum256([]byte(""))
	expected := base64.RawURLEncoding.EncodeToString(sum[:])
	assert.Equal(t, expected, challenge)
}

func TestMakeAudience(t *testing.T) {
	assert.Equal(t, []string{}, makeAudience(""))
	assert.Equal(t, []string{}, makeAudience("  "))
	assert.Equal(t, []string{"https://api.example.com"}, makeAudience("https://api.example.com"))
}

func TestNewAuthorizationCodeTokenMetadata(t *testing.T) {
	code := &authorizationcodedomain.AuthorizationCode{
		ID:       "code-1",
		Nonce:    "",
		Resource: "",
	}
	metadata := newAuthorizationCodeTokenMetadata(code)
	assert.Equal(t, "authorization_code", metadata["grantType"])
	assert.Equal(t, "code-1", metadata["authorizationCodeId"])
	_, hasNonce := metadata["nonce"]
	assert.False(t, hasNonce)

	code.Nonce = "nonce-123"
	code.Resource = "https://api.example.com"
	metadata = newAuthorizationCodeTokenMetadata(code)
	assert.Equal(t, "nonce-123", metadata["nonce"])
	assert.Equal(t, "https://api.example.com", metadata["resource"])
}

func TestIsAuthorizationCodeNotFound(t *testing.T) {
	assert.False(t, isAuthorizationCodeNotFound(nil))
	assert.False(t, isAuthorizationCodeNotFound(fmt.Errorf("some error")))
	err := apperrors.NewNotFound("not found")
	assert.True(t, isAuthorizationCodeNotFound(err))
}

func TestIsAppNotFound(t *testing.T) {
	assert.False(t, isAppNotFound(nil))
	assert.False(t, isAppNotFound(fmt.Errorf("some error")))
	err := apperrors.NewNotFound("not found")
	assert.True(t, isAppNotFound(err))
}

func TestValidateAuthorizationCodeClientAuthentication_NilApp(t *testing.T) {
	cmd := ExchangeAuthorizationCodeCommand{ClientSecret: "secret"}
	_, err := validateClientAuthenticationShape(clientAuthMaterialFromExchange(cmd), nil)
	require.Error(t, err)
}

func TestValidateAuthorizationCodeClientAuthentication_NoSecretRequired(t *testing.T) {
	app := &applicationdomain.Application{TokenEndpointAuthMethod: "none"}
	cmd := ExchangeAuthorizationCodeCommand{ClientSecret: ""}
	_, err := validateClientAuthenticationShape(clientAuthMaterialFromExchange(cmd), app)
	require.NoError(t, err)
}

func TestValidateAuthorizationCodeClientAuthentication_NoSecretRequiredButProvided(t *testing.T) {
	app := &applicationdomain.Application{TokenEndpointAuthMethod: "none"}
	cmd := ExchangeAuthorizationCodeCommand{ClientSecret: "secret"}
	_, err := validateClientAuthenticationShape(clientAuthMaterialFromExchange(cmd), app)
	require.Error(t, err)
}

func TestValidateAuthorizationCodeClientAuthentication_SecretRequiredButMissing(t *testing.T) {
	app := &applicationdomain.Application{TokenEndpointAuthMethod: "client_secret_post", ClientSecretHash: "somehash"}
	cmd := ExchangeAuthorizationCodeCommand{ClientSecret: ""}
	_, err := validateClientAuthenticationShape(clientAuthMaterialFromExchange(cmd), app)
	require.Error(t, err)
}

func TestValidateAuthorizationCodeExchange_WrongAppID(t *testing.T) {
	cmd := ExchangeAuthorizationCodeCommand{RedirectURI: "https://example.com/callback"}
	app := &applicationdomain.Application{ID: "app-1"}
	code := &authorizationcodedomain.AuthorizationCode{ApplicationID: "app-2", ExpiresAt: time.Now().Add(time.Hour)}
	err := validateAuthorizationCodeExchange(cmd, app, code, time.Now())
	require.Error(t, err)
}

func TestValidateAuthorizationCodeExchange_UsedCode(t *testing.T) {
	now := time.Now()
	usedAt := now.Add(-time.Minute)
	cmd := ExchangeAuthorizationCodeCommand{RedirectURI: "https://example.com/callback"}
	app := &applicationdomain.Application{ID: "app-1"}
	code := &authorizationcodedomain.AuthorizationCode{ApplicationID: "app-1", UsedAt: &usedAt, ExpiresAt: now.Add(time.Hour)}
	err := validateAuthorizationCodeExchange(cmd, app, code, now)
	require.Error(t, err)
}

func TestValidateAuthorizationCodeExchange_ExpiredCode(t *testing.T) {
	now := time.Now()
	cmd := ExchangeAuthorizationCodeCommand{RedirectURI: "https://example.com/callback"}
	app := &applicationdomain.Application{ID: "app-1"}
	code := &authorizationcodedomain.AuthorizationCode{ApplicationID: "app-1", ExpiresAt: now.Add(-time.Hour)}
	err := validateAuthorizationCodeExchange(cmd, app, code, now)
	require.Error(t, err)
}

func TestValidateAuthorizationCodeExchange_ValidCode(t *testing.T) {
	now := time.Now()
	verifier := "verifier-value"
	cmd := ExchangeAuthorizationCodeCommand{RedirectURI: "https://example.com/callback", CodeVerifier: verifier}
	app := &applicationdomain.Application{ID: "app-1"}
	code := &authorizationcodedomain.AuthorizationCode{
		ApplicationID: "app-1", RedirectURI: "https://example.com/callback",
		ExpiresAt: now.Add(time.Hour), CodeChallenge: buildPKCEChallenge(verifier), ChallengeMethod: "S256",
	}
	err := validateAuthorizationCodeExchange(cmd, app, code, now)
	require.NoError(t, err)
}

// TestValidateAuthorizationCodeExchange_CodeWithoutPKCE pins the rule: a
// code that carries no challenge can only come from a path that ignored PKCE, so
// it must never be exchangeable.
func TestValidateAuthorizationCodeExchange_CodeWithoutPKCE(t *testing.T) {
	now := time.Now()
	cmd := ExchangeAuthorizationCodeCommand{RedirectURI: "https://example.com/callback", CodeVerifier: "verifier-value"}
	app := &applicationdomain.Application{ID: "app-1"}
	code := &authorizationcodedomain.AuthorizationCode{
		ApplicationID: "app-1", RedirectURI: "https://example.com/callback",
		ExpiresAt: now.Add(time.Hour),
	}
	err := validateAuthorizationCodeExchange(cmd, app, code, now)
	require.Error(t, err)
}

func TestValidateAuthorizationCodeExchange_PKCEMissingVerifier(t *testing.T) {
	now := time.Now()
	cmd := ExchangeAuthorizationCodeCommand{RedirectURI: "https://example.com/callback", CodeVerifier: ""}
	app := &applicationdomain.Application{ID: "app-1"}
	code := &authorizationcodedomain.AuthorizationCode{
		ApplicationID: "app-1", RedirectURI: "https://example.com/callback",
		ExpiresAt: now.Add(time.Hour), CodeChallenge: "abc123", ChallengeMethod: "S256",
	}
	err := validateAuthorizationCodeExchange(cmd, app, code, now)
	require.Error(t, err)
}

func TestValidateAuthorizationCodeExchange_PKCEUnsupportedMethod(t *testing.T) {
	now := time.Now()
	cmd := ExchangeAuthorizationCodeCommand{RedirectURI: "https://example.com/callback", CodeVerifier: "verifier"}
	app := &applicationdomain.Application{ID: "app-1"}
	code := &authorizationcodedomain.AuthorizationCode{
		ApplicationID: "app-1", RedirectURI: "https://example.com/callback",
		ExpiresAt: now.Add(time.Hour), CodeChallenge: "abc123", ChallengeMethod: "plain",
	}
	err := validateAuthorizationCodeExchange(cmd, app, code, now)
	require.Error(t, err)
}

func TestValidateAuthorizationCodeExchange_PKCEInvalidVerifier(t *testing.T) {
	now := time.Now()
	cmd := ExchangeAuthorizationCodeCommand{RedirectURI: "https://example.com/callback", CodeVerifier: "wrong-verifier"}
	app := &applicationdomain.Application{ID: "app-1"}
	code := &authorizationcodedomain.AuthorizationCode{
		ApplicationID: "app-1", RedirectURI: "https://example.com/callback",
		ExpiresAt: now.Add(time.Hour), CodeChallenge: "abc123", ChallengeMethod: "S256",
	}
	err := validateAuthorizationCodeExchange(cmd, app, code, now)
	require.Error(t, err)
}

func TestValidateAuthorizationCodeExchange_ResourceMismatch(t *testing.T) {
	now := time.Now()
	cmd := ExchangeAuthorizationCodeCommand{RedirectURI: "https://example.com/callback", Resource: "https://api2.example.com"}
	app := &applicationdomain.Application{ID: "app-1"}
	code := &authorizationcodedomain.AuthorizationCode{
		ApplicationID: "app-1", RedirectURI: "https://example.com/callback",
		ExpiresAt: now.Add(time.Hour), Resource: "https://api1.example.com",
	}
	err := validateAuthorizationCodeExchange(cmd, app, code, now)
	require.Error(t, err)
}

// --- Social Login Pure Functions ---

func TestGenerateSocialUsername_Google(t *testing.T) {
	username := generateSocialUsername("google", &SocialUserInfo{ID: "12345", Email: "test@example.com"})
	assert.NotEmpty(t, username)

}

func TestMinLen_V2(t *testing.T) {
	assert.Equal(t, 3, minLen(3, 5))
	assert.Equal(t, 3, minLen(5, 3))
	assert.Equal(t, 0, minLen(0, 5))
}

func TestGenerateSocialState_V2(t *testing.T) {
	state, err := generateSocialState()
	require.NoError(t, err)
	assert.NotEmpty(t, state)
}

func TestSetSocialLoginAdapters_Nil(t *testing.T) {
	SetSocialLoginAdapters(nil, nil)
}

// --- Face ID Pure Functions ---

func TestSetFaceIdAdapters_Nil(t *testing.T) {
	SetFaceIdAdapters(nil, nil)
}

// --- Kerberos Pure Functions ---

func TestWireKerberosAuthenticator_Nil(t *testing.T) {
	WireKerberosAuthenticator(nil)
}

// --- Token Reuse Detection ---

func TestWireTokenFamilyRevocation_Nil(t *testing.T) {
	WireTokenFamilyRevocation(nil)
}

// --- Verification Adapters ---

func TestWireVerificationAdapters_Nil(t *testing.T) {
	WireVerificationAdapters(nil)
}

// --- Push Notification ---

func TestNewMfaPushNotificationSender_Nil(t *testing.T) {
	sender := NewMfaPushNotificationSender(nil)
	assert.NotNil(t, sender)
}

// --- Test Hooks ---

func TestSetSendPushAuthenticationForTest_Nil(t *testing.T) {
	SetSendPushAuthenticationForTest(nil)
}

// --- WebAuthn ---

func TestFormatWebAuthnError_Registration(t *testing.T) {
	result := FormatWebAuthnError("registration", fmt.Errorf("test error"))
	assert.Contains(t, result, "registration")
	assert.Contains(t, result, "test error")
}

// --- Token Introspection ---

func TestHashTokenForIntrospection_Consistency(t *testing.T) {
	hash := hashTokenForIntrospection("test-token")
	assert.NotEmpty(t, hash)
	hash2 := hashTokenForIntrospection("test-token")
	assert.Equal(t, hash, hash2)
	// Different input should produce different hash
	hash3 := hashTokenForIntrospection("different-token")
	assert.NotEqual(t, hash, hash3)
}

// --- LDAP Login ---

func TestSetLdapLoginAdapter_Nil(t *testing.T) {
	SetLdapLoginAdapter(nil)
}

// --- Login Challenge ---

func TestToLoginChallengeResponse_V2_Nil(t *testing.T) {
	resp := toLoginChallengeResponse(nil)
	assert.Nil(t, resp)
}

func TestToLoginChallengeResponse_V2_Valid(t *testing.T) {
	now := time.Now()
	challenge := &mfachallengedomain.Challenge{
		ID:         "challenge-1",
		State:      "pending",
		MethodType: "totp",
		ExpiresAt:  now,
	}
	resp := toLoginChallengeResponse(challenge)
	require.NotNil(t, resp)
	assert.Equal(t, "challenge-1", resp.ID)
	assert.Equal(t, "pending", resp.State)
	assert.Equal(t, "totp", resp.MethodType)
}

// --- SAML SP ---

func TestBuildIdPSSOURL_EmptySlug(t *testing.T) {
	url := buildIdPSSOURL("", nil)
	// Even with empty slug, the function returns a base path
	assert.NotEmpty(t, url)
}
