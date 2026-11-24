package idp

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOIDCProviderAuthorizationURLCarriesNonceAndPKCE(t *testing.T) {
	provider := NewOIDCProvider(&OAuthConfig{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURI:  "https://app.example.com/callback",
		Scopes:       []string{"openid", "profile"},
		AuthURL:      "https://sso.example.com/authorize",
		TokenURL:     "https://sso.example.com/token",
		UserInfoURL:  "https://sso.example.com/userinfo",
	})

	challenge := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	redirect, err := provider.GetRedirectURLWithOptions("state-1", AuthorizationRequestOptions{
		Nonce:               "nonce-1",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)

	parsed, err := url.Parse(redirect)
	require.NoError(t, err)
	query := parsed.Query()
	assert.Equal(t, "nonce-1", query.Get("nonce"))
	assert.Equal(t, challenge, query.Get("code_challenge"))
	assert.Equal(t, "S256", query.Get("code_challenge_method"))
	assert.Equal(t, "state-1", query.Get("state"))
	assert.Equal(t, "code", query.Get("response_type"))
	assert.Equal(t, "openid profile", query.Get("scope"))
}

func TestOIDCProviderAuthorizationURLWithoutOptionsMatchesOAuth2(t *testing.T) {
	provider := NewOIDCProvider(&OAuthConfig{
		ClientID:    "client-id",
		RedirectURI: "https://app.example.com/callback",
		Scopes:      []string{"openid"},
		AuthURL:     "https://sso.example.com/authorize",
		TokenURL:    "https://sso.example.com/token",
		UserInfoURL: "https://sso.example.com/userinfo",
	})

	plain, err := provider.GetRedirectURL("state-1")
	require.NoError(t, err)
	withDefaults, err := provider.GetRedirectURLWithOptions("state-1", AuthorizationRequestOptions{})
	require.NoError(t, err)

	assert.Equal(t, plain, withDefaults)
	assert.NotContains(t, plain, "nonce=")
	assert.NotContains(t, plain, "code_challenge")
}

func TestOIDCProviderAuthorizationURLDefaultsChallengeMethodToS256(t *testing.T) {
	provider := NewOIDCProvider(&OAuthConfig{
		ClientID:    "client-id",
		RedirectURI: "https://app.example.com/callback",
		AuthURL:     "https://sso.example.com/authorize",
		TokenURL:    "https://sso.example.com/token",
		UserInfoURL: "https://sso.example.com/userinfo",
	})

	redirect, err := provider.GetRedirectURLWithOptions("state-1", AuthorizationRequestOptions{
		CodeChallenge: "challenge-1",
	})
	require.NoError(t, err)
	parsed, err := url.Parse(redirect)
	require.NoError(t, err)
	assert.Equal(t, "S256", parsed.Query().Get("code_challenge_method"))
}

func TestOIDCProviderAuthorizationURLRejectsNonS256ChallengeMethod(t *testing.T) {
	provider := NewOIDCProvider(&OAuthConfig{
		ClientID:    "client-id",
		RedirectURI: "https://app.example.com/callback",
		AuthURL:     "https://sso.example.com/authorize",
		TokenURL:    "https://sso.example.com/token",
		UserInfoURL: "https://sso.example.com/userinfo",
	})

	_, err := provider.GetRedirectURLWithOptions("state-1", AuthorizationRequestOptions{
		CodeChallenge:       "challenge-1",
		CodeChallengeMethod: "plain",
	})
	require.Error(t, err)
}

func TestVerifyOIDCIDTokenNonce(t *testing.T) {
	key := generateRSAKey(t)
	lookup := func(context.Context, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }
	baseClaims := map[string]any{
		"iss":   "https://issuer.example.com",
		"aud":   "client-id",
		"sub":   "user-1",
		"nonce": "nonce-1",
		"exp":   time.Now().Add(time.Hour).Unix(),
	}

	t.Run("matching nonce", func(t *testing.T) {
		verified, err := verifyOIDCIDToken(t.Context(), signTestJWT(t, key, "k1", baseClaims),
			"https://issuer.example.com", "client-id", "nonce-1", lookup)
		require.NoError(t, err)
		assert.Equal(t, "nonce-1", stringClaim(verified, "nonce"))
	})

	t.Run("missing nonce", func(t *testing.T) {
		_, err := verifyOIDCIDToken(t.Context(), signTestJWT(t, key, "k1", withoutClaim(baseClaims, "nonce")),
			"https://issuer.example.com", "client-id", "nonce-1", lookup)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing nonce")
	})

	t.Run("mismatched nonce", func(t *testing.T) {
		_, err := verifyOIDCIDToken(t.Context(), signTestJWT(t, key, "k1", mergeClaims(baseClaims, "nonce", "nonce-2")),
			"https://issuer.example.com", "client-id", "nonce-1", lookup)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nonce does not match")
	})

	t.Run("no expected nonce keeps existing behaviour", func(t *testing.T) {
		_, err := verifyOIDCIDToken(t.Context(), signTestJWT(t, key, "k1", baseClaims),
			"https://issuer.example.com", "client-id", "", lookup)
		require.NoError(t, err)
	})
}

func TestOIDCProviderGetUserInfoEnforcesNonce(t *testing.T) {
	useOIDCTestClient(t)
	key := generateRSAKey(t)
	var serverURL string
	mux := http.NewServeMux()
	mux.HandleFunc(oidcWellKnownPath, func(w http.ResponseWriter, r *http.Request) {
		oidcDiscoveryHandler(serverURL, serverURL, nil)(w, r)
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{{
				"kty": "RSA",
				"kid": "k1",
				"use": "sig",
				"alg": "RS256",
				"n":   base64RawURL(key.PublicKey.N.Bytes()),
				"e":   base64RawURL(big.NewInt(int64(key.PublicKey.E)).Bytes()),
			}},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	serverURL = server.URL

	claims := map[string]any{
		"iss":   server.URL,
		"aud":   "client-id",
		"sub":   "user-1",
		"nonce": "nonce-1",
		"exp":   time.Now().Add(10 * time.Minute).Unix(),
	}
	validToken := signTestJWT(t, key, "k1", claims)

	t.Run("matching nonce", func(t *testing.T) {
		provider := NewOIDCProvider(&OAuthConfig{ClientID: "client-id", Host: server.URL})
		provider.SetExpectedNonce("nonce-1")
		info, err := provider.GetUserInfo(t.Context(), &OAuthToken{AccessToken: "access", IDToken: validToken})
		require.NoError(t, err)
		assert.Equal(t, "user-1", info.ID)
	})

	t.Run("missing nonce claim", func(t *testing.T) {
		provider := NewOIDCProvider(&OAuthConfig{ClientID: "client-id", Host: server.URL})
		provider.SetExpectedNonce("nonce-1")
		_, err := provider.GetUserInfo(t.Context(), &OAuthToken{
			AccessToken: "access",
			IDToken:     signTestJWT(t, key, "k1", withoutClaim(claims, "nonce")),
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing nonce")
	})

	t.Run("mismatched nonce claim", func(t *testing.T) {
		provider := NewOIDCProvider(&OAuthConfig{ClientID: "client-id", Host: server.URL})
		provider.SetExpectedNonce("nonce-1")
		_, err := provider.GetUserInfo(t.Context(), &OAuthToken{
			AccessToken: "access",
			IDToken:     signTestJWT(t, key, "k1", mergeClaims(claims, "nonce", "nonce-2")),
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nonce does not match")
	})

	t.Run("missing id_token cannot skip nonce verification", func(t *testing.T) {
		provider := NewOIDCProvider(&OAuthConfig{ClientID: "client-id", Host: server.URL})
		provider.SetExpectedNonce("nonce-1")
		_, err := provider.GetUserInfo(t.Context(), &OAuthToken{AccessToken: "access"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "id_token is required")
	})
}

func TestOIDCProviderGetUserInfoRejectsNonceWithoutIssuer(t *testing.T) {
	provider := NewOIDCProvider(&OAuthConfig{
		ClientID:    "client-id",
		AuthURL:     "https://sso.example.com/authorize",
		TokenURL:    "https://sso.example.com/token",
		UserInfoURL: "https://sso.example.com/userinfo",
	})
	provider.SetExpectedNonce("nonce-1")

	_, err := provider.GetUserInfo(t.Context(), &OAuthToken{AccessToken: "access", IDToken: "a.b.c"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "issuer is required")
}

// base64RawURL encodes JWKS members the same way the production cache expects.
func base64RawURL(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}
