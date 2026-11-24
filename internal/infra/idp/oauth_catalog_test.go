package idp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGothCatalog_P4IncrementalProviders pins the OPEN-API-75 P4 adjudication:
// Kakao/Naver/PayPal/Zoom/Yandex/Dropbox stay served by the Goth catalog
// (kind=goth + provider_type routing, no dedicated kind). The expected hosts
// are the vendors' official authorize endpoints as of 2026-11-14. Known
// deviation: goth's PayPal adapter still targets the legacy
// /webapps/auth/protocol/openidconnect/v1/authorize endpoint instead of the
// documented https://www.paypal.com/signin/authorize; the test pins current
// library behaviour and the host prefix keeps the vendor domain governed.
func TestGothCatalog_P4IncrementalProviders(t *testing.T) {
	cases := map[string]string{
		"kakao":   "https://kauth.kakao.com/oauth/authorize",
		"naver":   "https://nid.naver.com/oauth2.0/authorize",
		"paypal":  "https://www.paypal.com/webapps/auth/protocol/openidconnect/v1/authorize",
		"zoom":    "https://zoom.us/oauth/authorize",
		"yandex":  "https://oauth.yandex.ru/authorize",
		"dropbox": "https://www.dropbox.com/oauth2/authorize",
	}
	for providerType, authHost := range cases {
		provider, err := NewGothProvider(providerType, &OAuthConfig{
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			RedirectURI:  "https://app.example.com/callback",
		})
		require.NoError(t, err, "provider %s", providerType)
		authURL, err := provider.GetRedirectURL("state-123")
		require.NoError(t, err, "provider %s", providerType)
		assert.True(t, strings.HasPrefix(authURL, authHost),
			"provider %s authorize URL %q must start with official endpoint %q", providerType, authURL, authHost)
	}
}

func TestCreateGothProvider_BuildsEverySupportedType(t *testing.T) {
	for _, providerType := range SupportedGothTypes() {
		provider, session, err := createGothProvider(providerType, "client-id", "client-secret", "https://app.example.com/callback", "https://tenant.example.com")
		require.NoError(t, err, "provider %s", providerType)
		assert.NotNil(t, provider, "provider %s", providerType)
		assert.NotNil(t, session, "provider %s", providerType)
	}
}

func TestCreateGothProvider_MicrosoftOnlineAndPayPal(t *testing.T) {
	for _, providerType := range []string{"microsoftonline", "MicrosoftOnline", "paypal", "PayPal"} {
		provider, _, err := createGothProvider(providerType, "client-id", "client-secret", "https://app.example.com/callback", "")
		require.NoError(t, err, "provider %s", providerType)
		assert.NotNil(t, provider, "provider %s", providerType)
	}
}

func TestNewCasdoorProvider_DerivesEndpointsFromHost(t *testing.T) {
	config := &OAuthConfig{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURI:  "https://app.example.com/callback",
		Host:         "https://casdoor.example.com/",
	}
	provider := NewCasdoorProvider(config)

	assert.Equal(t, "https://casdoor.example.com/login/oauth/authorize", provider.authURL)
	assert.Equal(t, "https://casdoor.example.com/api/login/oauth/access_token", provider.tokenURL)
	assert.Equal(t, "https://casdoor.example.com/api/userinfo", provider.userURL)

	redirect, err := provider.GetRedirectURL("state-1")
	require.NoError(t, err)
	assert.Contains(t, redirect, "https://casdoor.example.com/login/oauth/authorize?")
	assert.Contains(t, redirect, "client_id=client-id")
	assert.Contains(t, redirect, "state=state-1")
}

func TestNewCasdoorProvider_ExplicitURLsWin(t *testing.T) {
	config := &OAuthConfig{
		ClientID:    "client-id",
		RedirectURI: "https://app.example.com/callback",
		Host:        "https://casdoor.example.com",
		AuthURL:     "https://sso.example.com/authorize",
		TokenURL:    "https://sso.example.com/token",
		UserInfoURL: "https://sso.example.com/userinfo",
	}
	provider := NewCasdoorProvider(config)

	assert.Equal(t, "https://sso.example.com/authorize", provider.authURL)
	assert.Equal(t, "https://sso.example.com/token", provider.tokenURL)
	assert.Equal(t, "https://sso.example.com/userinfo", provider.userURL)
}

func TestCasdoorProvider_RequiresHostOrExplicitURLs(t *testing.T) {
	provider := NewCasdoorProvider(&OAuthConfig{ClientID: "client-id"})

	_, err := provider.GetRedirectURL("state")
	require.Error(t, err)
	_, err = provider.ExchangeCode(t.Context(), "code")
	require.Error(t, err)
}

func TestDecodeTokenPayload_SnakeCase(t *testing.T) {
	token, err := decodeTokenPayload([]byte(`{"access_token":"tok","token_type":"Bearer","expires_in":3600,"refresh_token":"refresh","id_token":"jwt"}`))
	require.NoError(t, err)
	assert.Equal(t, "tok", token.AccessToken)
	assert.Equal(t, "Bearer", token.TokenType)
	assert.Equal(t, int64(3600), token.ExpiresIn)
	assert.Equal(t, "refresh", token.RefreshToken)
	assert.Equal(t, "jwt", token.IDToken)
}

func TestDecodeTokenPayload_CamelCase(t *testing.T) {
	token, err := decodeTokenPayload([]byte(`{"accessToken":"tok","tokenType":"Bearer","expiresIn":3600,"refreshToken":"refresh"}`))
	require.NoError(t, err)
	assert.Equal(t, "tok", token.AccessToken)
	assert.Equal(t, "Bearer", token.TokenType)
	assert.Equal(t, int64(3600), token.ExpiresIn)
	assert.Equal(t, "refresh", token.RefreshToken)
}

func TestDecodeTokenPayload_RejectsErrorAndMissingToken(t *testing.T) {
	_, err := decodeTokenPayload([]byte(`{"error":"invalid_grant","error_description":"bad code"}`))
	require.Error(t, err)

	_, err = decodeTokenPayload([]byte(`{"token_type":"Bearer"}`))
	require.Error(t, err)
}

func TestParseUserInfo_CasdoorOIDCShape(t *testing.T) {
	provider := &BaseOAuthProvider{}
	info, err := provider.parseUserInfo([]byte(`{
		"sub": "user-1",
		"preferred_username": "alice",
		"name": "Alice",
		"email": "alice@example.com",
		"picture": "https://example.com/alice.png",
		"phone_number": "+10086"
	}`))
	require.NoError(t, err)
	assert.Equal(t, "user-1", info.ID)
	assert.Equal(t, "alice", info.Username)
	assert.Equal(t, "Alice", info.DisplayName)
	assert.Equal(t, "alice@example.com", info.Email)
	assert.Equal(t, "https://example.com/alice.png", info.AvatarURL)
	assert.Equal(t, "+10086", info.Phone)
}
