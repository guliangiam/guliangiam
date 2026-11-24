package idp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestHeaderInjectorAddsProviderHeaders(t *testing.T) {
	var seen *http.Request
	client := &http.Client{Transport: headerInjector{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			seen = req
			return &http.Response{StatusCode: 200, Body: io.NopCloser(nil)}, nil
		}),
		headers: map[string]string{"X-Tenant": "acme"},
	}}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", http.NoBody)
	require.NoError(t, err)

	_, err = client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, "acme", seen.Header.Get("X-Tenant"))
	assert.Empty(t, req.Header.Get("X-Tenant"), "the caller's request must not be mutated")
}

func TestProviderHTTPClientReturnsNilWithoutOptions(t *testing.T) {
	assert.Nil(t, providerHTTPClient(&providers.Provider{}))
	assert.Nil(t, providerHTTPClient(nil))
	assert.NotNil(t, providerHTTPClient(&providers.Provider{HttpHeaders: map[string]string{"X-Tenant": "acme"}}))
}

func TestBaseOAuthExchangeCodeSendsPKCEVerifier(t *testing.T) {
	var gotVerifier string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		gotVerifier = r.Form.Get("code_verifier")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token-1", "token_type": "Bearer"})
	}))
	defer server.Close()

	provider := newBaseOAuth(&OAuthConfig{
		ClientID:     "client",
		ClientSecret: "secret",
		TokenURL:     server.URL,
		CodeVerifier: "verifier-0123456789abcdef0123456789abcdef",
		HTTPClient:   ssrf.SafeHTTPClientWithAllowlist(5*time.Second, []string{"127.0.0.0/8"}),
	}, "", server.URL, "")

	token, err := provider.ExchangeCode(context.Background(), "auth-code")
	require.NoError(t, err)
	assert.Equal(t, "token-1", token.AccessToken)
	assert.Equal(t, "verifier-0123456789abcdef0123456789abcdef", gotVerifier)
}

func TestSAMLProviderPrefersCertColumn(t *testing.T) {
	config := &OAuthConfig{IdPCert: "cert-from-cert-column", ClientSecret: "legacy-secret"}
	provider := NewSAMLIdPProvider(config)
	assert.Equal(t, "cert-from-cert-column", provider.idpCert)
}

func TestSanitizeURLError(t *testing.T) {
	plain := errors.New("dial failed")
	assert.Equal(t, plain, sanitizeURLError(plain), "non-url errors pass through")

	urlErr := &url.Error{Op: "Get", URL: "https://example.com/cgi-bin/gettoken?corpid=c&corpsecret=s3cret", Err: plain}
	sanitized := sanitizeURLError(urlErr)
	assert.NotContains(t, sanitized.Error(), "s3cret")
	assert.Contains(t, sanitized.Error(), "https://example.com/cgi-bin/gettoken?redacted")
	assert.True(t, errors.Is(sanitized, plain), "unwrap chain must be preserved")
}

func TestWeComInternalExchangeCodeEncodesSecretAndRedactsTransportError(t *testing.T) {
	var gotRawQuery string
	var gotQuery url.Values
	failing := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotRawQuery = req.URL.RawQuery
		gotQuery = req.URL.Query()
		return nil, errors.New("dial tcp: connection refused")
	})
	provider := NewWeComInternalProvider(&OAuthConfig{
		ClientID:     "corp-id",
		ClientSecret: "secret+with&specials",
		HTTPClient:   &http.Client{Transport: failing},
	})

	_, err := provider.ExchangeCode(context.Background(), "code-1")
	require.Error(t, err)

	assert.Equal(t, "secret+with&specials", gotQuery.Get("corpsecret"), "query must round-trip through percent encoding")
	assert.Contains(t, gotRawQuery, "corpsecret=secret%2Bwith%26specials")
	assert.NotContains(t, err.Error(), "secret%2Bwith", "transport errors must not echo the query")
	assert.NotContains(t, err.Error(), "corpsecret")
	assert.Contains(t, err.Error(), "?redacted")
}

func TestWeChatExchangeCodeRedactsSecretInTransportError(t *testing.T) {
	failing := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("no such host")
	})
	provider := NewWeChatProvider(&OAuthConfig{
		ClientID:     "app-id",
		ClientSecret: "app-secret-value",
		HTTPClient:   &http.Client{Transport: failing},
	})

	_, err := provider.ExchangeCode(context.Background(), "code-1")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "app-secret-value")
	assert.Contains(t, err.Error(), "?redacted")
}
