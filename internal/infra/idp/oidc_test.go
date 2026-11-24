package idp

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

func useOIDCTestClient(t *testing.T) {
	t.Helper()
	previous := oidcHTTPClient
	oidcHTTPClient = ssrf.SafeHTTPClientWithAllowlist(5*time.Second, []string{"127.0.0.0/8"})
	t.Cleanup(func() { oidcHTTPClient = previous })
}

func oidcDiscoveryHandler(serverURL string, discoveredIssuer string, requests *atomic.Int32) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != oidcWellKnownPath {
			http.NotFound(w, r)
			return
		}
		if requests != nil {
			requests.Add(1)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                discoveredIssuer,
			"authorization_endpoint":                serverURL + "/oauth2/authorize",
			"token_endpoint":                        serverURL + "/oauth2/token",
			"userinfo_endpoint":                     serverURL + "/oauth2/userinfo",
			"jwks_uri":                              serverURL + "/jwks",
			"scopes_supported":                      []string{"openid", "profile", "email"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	}
}

func TestOIDCProvider_DerivesEndpointsFromDiscovery(t *testing.T) {
	useOIDCTestClient(t)
	var requests atomic.Int32
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		oidcDiscoveryHandler(serverURL, serverURL, &requests)(w, r)
	}))
	defer server.Close()
	serverURL = server.URL

	provider := NewOIDCProvider(&OAuthConfig{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURI:  "https://app.example.com/callback",
		Scopes:       []string{"openid", "profile"},
		Host:         server.URL + "/",
	})

	redirect, err := provider.GetRedirectURL("state-1")
	require.NoError(t, err)
	assert.Contains(t, redirect, server.URL+"/oauth2/authorize?")
	assert.Contains(t, redirect, "client_id=client-id")
	assert.Contains(t, redirect, "state=state-1")
	assert.Contains(t, redirect, "scope=openid+profile")

	assert.Equal(t, server.URL+"/oauth2/token", provider.tokenURL)
	assert.Equal(t, server.URL+"/oauth2/userinfo", provider.userURL)
	assert.Equal(t, int32(1), requests.Load())

	// A second provider instance reuses the cached discovery document.
	second := NewOIDCProvider(&OAuthConfig{ClientID: "client-id", Host: server.URL})
	_, err = second.GetRedirectURL("state-2")
	require.NoError(t, err)
	assert.Equal(t, int32(1), requests.Load())
}

func TestOIDCProvider_ExplicitURLsSkipDiscovery(t *testing.T) {
	provider := NewOIDCProvider(&OAuthConfig{
		ClientID:    "client-id",
		RedirectURI: "https://app.example.com/callback",
		AuthURL:     "https://sso.example.com/authorize",
		TokenURL:    "https://sso.example.com/token",
		UserInfoURL: "https://sso.example.com/userinfo",
		Scopes:      []string{"openid"},
	})

	redirect, err := provider.GetRedirectURL("state")
	require.NoError(t, err)
	assert.Contains(t, redirect, "https://sso.example.com/authorize?")
}

func TestOIDCProvider_RejectsDiscoveryIssuerMismatch(t *testing.T) {
	useOIDCTestClient(t)
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		oidcDiscoveryHandler(serverURL, "https://evil.example.com", nil)(w, r)
	}))
	defer server.Close()
	serverURL = server.URL

	provider := NewOIDCProvider(&OAuthConfig{ClientID: "client-id", Host: server.URL})
	_, err := provider.GetRedirectURL("state")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "issuer does not match")
}

func TestOIDCProvider_RequiresIssuerWithoutExplicitURLs(t *testing.T) {
	provider := NewOIDCProvider(&OAuthConfig{ClientID: "client-id"})
	_, err := provider.GetRedirectURL("state")
	require.Error(t, err)
	_, err = provider.ExchangeCode(t.Context(), "code")
	require.Error(t, err)
}

func TestVerifyOIDCIDToken_AcceptsValidToken(t *testing.T) {
	key := generateRSAKey(t)
	claims := map[string]any{
		"iss":                "https://issuer.example.com",
		"aud":                "client-id",
		"sub":                "user-1",
		"preferred_username": "alice",
		"name":               "Alice",
		"email":              "alice@example.com",
		"phone_number":       "+10000000000",
		"picture":            "https://cdn.example.com/alice.png",
		"exp":                time.Now().Add(time.Hour).Unix(),
	}

	idToken := signTestJWT(t, key, "k1", claims)
	verified, err := verifyOIDCIDToken(t.Context(), idToken, "https://issuer.example.com", "client-id", "",
		func(_ context.Context, kid string) (*rsa.PublicKey, error) {
			require.Equal(t, "k1", kid)
			return &key.PublicKey, nil
		})
	require.NoError(t, err)

	info, err := userInfoFromOIDCClaims(verified)
	require.NoError(t, err)
	assert.Equal(t, "user-1", info.ID)
	assert.Equal(t, "alice", info.Username)
	assert.Equal(t, "Alice", info.DisplayName)
	assert.Equal(t, "alice@example.com", info.Email)
	assert.Equal(t, "+10000000000", info.Phone)
	assert.Equal(t, "https://cdn.example.com/alice.png", info.AvatarURL)
}

func TestVerifyOIDCIDToken_AudienceArray(t *testing.T) {
	key := generateRSAKey(t)
	idToken := signTestJWT(t, key, "k1", map[string]any{
		"iss": "https://issuer.example.com",
		"aud": []string{"other-client", "client-id"},
		"sub": "user-1",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	verified, err := verifyOIDCIDToken(t.Context(), idToken, "https://issuer.example.com", "client-id", "",
		func(context.Context, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil })
	require.NoError(t, err)
	assert.Equal(t, "user-1", stringClaim(verified, "sub"))
}

func TestVerifyOIDCIDToken_RejectsInvalidTokens(t *testing.T) {
	key := generateRSAKey(t)
	otherKey := generateRSAKey(t)
	validClaims := map[string]any{
		"iss": "https://issuer.example.com",
		"aud": "client-id",
		"sub": "user-1",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	lookup := func(context.Context, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }

	tests := []struct {
		name     string
		token    string
		audience string
	}{
		{"wrong signature", signTestJWT(t, otherKey, "k1", validClaims), "client-id"},
		{"wrong issuer", signTestJWT(t, key, "k1", mergeClaims(validClaims, "iss", "https://other.example.com")), "client-id"},
		{"wrong audience", signTestJWT(t, key, "k1", mergeClaims(validClaims, "aud", "other-client")), "client-id"},
		{"expired", signTestJWT(t, key, "k1", mergeClaims(validClaims, "exp", time.Now().Add(-time.Hour).Unix())), "client-id"},
		{"missing exp", signTestJWT(t, key, "k1", withoutClaim(validClaims, "exp")), "client-id"},
		{"missing sub", signTestJWT(t, key, "k1", withoutClaim(validClaims, "sub")), "client-id"},
		{"not a jwt", "not-a-jwt", "client-id"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verified, err := verifyOIDCIDToken(t.Context(), test.token, "https://issuer.example.com", test.audience, "", lookup)
			if err == nil {
				_, err = userInfoFromOIDCClaims(verified)
			}
			require.Error(t, err)
		})
	}

	t.Run("rejects non RS256", func(t *testing.T) {
		token := signTestJWTWithAlg(t, key, "k1", "HS256", validClaims)
		_, err := verifyOIDCIDToken(t.Context(), token, "https://issuer.example.com", "client-id", "", lookup)
		require.Error(t, err)
	})
}

func TestOIDCProvider_VerifiesIDTokenAgainstJWKS(t *testing.T) {
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
				"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
			}},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	serverURL = server.URL

	idToken := signTestJWT(t, key, "k1", map[string]any{
		"iss":                server.URL,
		"aud":                "client-id",
		"sub":                "casdoor-user",
		"preferred_username": "carol",
		"email":              "carol@example.com",
		"exp":                time.Now().Add(10 * time.Minute).Unix(),
	})

	provider := NewOIDCProvider(&OAuthConfig{ClientID: "client-id", Host: server.URL})
	info, err := provider.GetUserInfo(t.Context(), &OAuthToken{AccessToken: "access", IDToken: idToken})
	require.NoError(t, err)
	assert.Equal(t, "casdoor-user", info.ID)
	assert.Equal(t, "carol", info.Username)
	assert.Equal(t, "carol@example.com", info.Email)
}

func generateRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func signTestJWT(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	return signTestJWTWithAlg(t, key, kid, "RS256", claims)
}

func signTestJWTWithAlg(t *testing.T, key *rsa.PrivateKey, kid, alg string, claims map[string]any) string {
	t.Helper()
	header, err := json.Marshal(map[string]any{"alg": alg, "kid": kid, "typ": "JWT"})
	require.NoError(t, err)
	payload, err := json.Marshal(claims)
	require.NoError(t, err)

	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	require.NoError(t, err)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func mergeClaims(base map[string]any, key string, value any) map[string]any {
	merged := make(map[string]any, len(base))
	for k, v := range base {
		merged[k] = v
	}
	merged[key] = value
	return merged
}

func withoutClaim(base map[string]any, key string) map[string]any {
	merged := make(map[string]any, len(base))
	for k, v := range base {
		if k == key {
			continue
		}
		merged[k] = v
	}
	return merged
}
