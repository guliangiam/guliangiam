package wellknown

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	platformconfig "github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBearerTokenFromRequest(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"empty", "", ""},
		{"no bearer", "Basic abc123", ""},
		{"bearer token", "Bearer mytoken123", "mytoken123"},
		{"bearer with extra spaces", "Bearer   mytoken123  ", "mytoken123"},
		{"lowercase bearer", "bearer mytoken123", "mytoken123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				r.Header.Set("Authorization", tt.header)
			}
			got := bearerTokenFromRequest(r)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIssuerBase_WithIssuerURL(t *testing.T) {
	h := Handler{IssuerURL: "https://auth.example.com/"}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Host = "other.example.com"
	base, err := h.issuerBase(r)
	require.NoError(t, err)
	assert.Equal(t, "https://auth.example.com", base)
}

func TestIssuerBase_WithTrailingSlash(t *testing.T) {
	h := Handler{IssuerURL: "https://auth.example.com///"}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	base, err := h.issuerBase(r)
	require.NoError(t, err)
	assert.Equal(t, "https://auth.example.com", base)
}

func TestIssuerBase_FromRequest(t *testing.T) {
	h := Handler{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Host = "localhost:8080"
	base, err := h.issuerBase(r)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080", base)
}

// TestIssuerBase_FromRequestHTTPS: the forwarded scheme is honoured only from a
// configured trusted proxy, so the request has to name one.
func TestIssuerBase_FromRequestHTTPS(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "192.0.2.1/32")
	h := Handler{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Host = "auth.example.com"
	r.RemoteAddr = "192.0.2.1:1234"
	r.Header.Set("X-Forwarded-Proto", "https")
	base, err := h.issuerBase(r)
	require.NoError(t, err)
	assert.Equal(t, "https://auth.example.com", base)
}

func TestIssuerBase_TrustedProxyForwardedHost(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "192.0.2.1/32")
	h := Handler{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.0.2.1:1234"
	r.Host = "internal:8080"
	r.Header.Set("X-Forwarded-Host", "public.example.com")
	base, err := h.issuerBase(r)
	require.NoError(t, err)
	assert.Equal(t, "http://public.example.com", base)
}

// GAP-010: a forged X-Forwarded-Host must not rewrite the issuer when the
// caller did not configure a trusted proxy.
func TestIssuerBase_UntrustedForwardedHostIsIgnored(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "")
	h := Handler{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Host = "internal:8080"
	r.Header.Set("X-Forwarded-Host", "evil.example.com")
	base, err := h.issuerBase(r)
	require.NoError(t, err)
	assert.Equal(t, "http://internal:8080", base)
}

func TestIssuerBase_TrustedProxyStillRejectsMalformedHost(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "192.0.2.1/32")
	h := Handler{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.0.2.1:1234"
	r.Host = "internal:8080"
	r.Header.Set("X-Forwarded-Host", "evil.example.com/path")
	base, err := h.issuerBase(r)
	require.NoError(t, err)
	assert.Equal(t, "http://internal:8080", base)
}

func TestIssuerBase_ProductionRequiresIssuerURL(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("ISSUER_URL", "")
	t.Setenv("TRUSTED_PROXY_CIDRS", "192.0.2.1/32")
	h := Handler{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Host = "internal:8080"
	r.RemoteAddr = "192.0.2.1:1234"
	r.Header.Set("X-Forwarded-Host", "evil.example.com")
	_, err := h.issuerBase(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ISSUER_URL")
}

func TestIssuerBase_ConfiguredIssuerURLWinsOverForwardedHost(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("ISSUER_URL", "https://auth.example.com")
	t.Setenv("TRUSTED_PROXY_CIDRS", "192.0.2.1/32")
	h := Handler{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.0.2.1:1234"
	r.Host = "internal:8080"
	r.Header.Set("X-Forwarded-Host", "evil.example.com")
	base, err := h.issuerBase(r)
	require.NoError(t, err)
	assert.Equal(t, "https://auth.example.com", base)
}

func TestIssuerBase_TLS(t *testing.T) {
	h := Handler{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Host = "auth.example.com"
	r.TLS = &tls.ConnectionState{} // non-nil TLS
	base, err := h.issuerBase(r)
	require.NoError(t, err)
	assert.Contains(t, base, "https://")
}

func TestOpenIDConfiguration_ProductionWithoutIssuerURLIs500(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("ISSUER_URL", "")
	// Even a trusted proxy must not turn the production requirement into a
	// request-derived fallback.
	t.Setenv("TRUSTED_PROXY_CIDRS", "192.0.2.1/32")
	router := testRouter()
	RegisterChiRoutes(router, Handler{OIDCDiscoveryFetch: &mockOIDCDiscoveryFetcher{
		response: &appauth.OIDCDiscoveryResponse{Issuer: "https://evil.example.com"},
	}})
	r := httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", nil)
	r.Host = "internal:8080"
	r.RemoteAddr = "192.0.2.1:1234"
	r.Header.Set("X-Forwarded-Host", "evil.example.com")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, r)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// TestDiscoveryIssuerMatchesRequestDerivation is the cross-layer lock for B2:
// the issuer published by discovery and the issuer derived by
// config.RequestIssuerURL (which also feeds the DPoP `htu`) must be byte-equal
// for the same request, trusted peer or not. This is the drift class the batch
// removes; a second trust implementation would resurface it here.
func TestDiscoveryIssuerMatchesRequestDerivation(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("ISSUER_URL", "")
	t.Setenv("TRUSTED_PROXY_CIDRS", "192.0.2.1/32")

	tests := []struct {
		name       string
		remoteAddr string
		want       string
	}{
		{"trusted proxy forwarded host and proto", "192.0.2.1:1234", "https://public.example.com"},
		{"forged headers from untrusted peer", "203.0.113.9:1234", "http://internal:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := testRouter()
			RegisterChiRoutes(router, Handler{})

			r := httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", nil)
			r.Host = "internal:8080"
			r.RemoteAddr = tt.remoteAddr
			r.Header.Set("X-Forwarded-Host", "public.example.com")
			r.Header.Set("X-Forwarded-Proto", "https")

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, r)
			require.Equal(t, http.StatusOK, rec.Code)

			var doc struct {
				Issuer string `json:"issuer"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &doc))
			assert.Equal(t, tt.want, doc.Issuer)
			assert.Equal(t, platformconfig.RequestIssuerURL(r), doc.Issuer)
		})
	}
}

func TestOidcDiscoveryFetch_WithFetcher(t *testing.T) {
	mockFetcher := &mockOIDCDiscoveryFetcher{
		response: &appauth.OIDCDiscoveryResponse{Issuer: "https://example.com"},
		err:      nil,
	}
	h := Handler{OIDCDiscoveryFetch: mockFetcher}
	resp, err := h.oidcDiscoveryFetch(context.Background(), "https://example.com")
	assert.NoError(t, err)
	assert.Equal(t, "https://example.com", resp.Issuer)
}

func TestOidcDiscoveryFetch_Fallback(t *testing.T) {
	h := Handler{}
	resp, err := h.oidcDiscoveryFetch(context.Background(), "https://example.com")
	assert.NoError(t, err)
	assert.Equal(t, "https://example.com", resp.Issuer)
	assert.Equal(t, "https://example.com/api/v1/auth/authorize", resp.AuthorizationEndpoint)
	assert.Equal(t, "https://example.com/api/v1/auth/tokens/exchange", resp.TokenEndpoint)
	assert.Equal(t, "https://example.com/.well-known/jwks.json", resp.JWKSEndpoint)
	assert.Equal(t, "https://example.com/oauth2/userinfo", resp.UserinfoEndpoint)
	assert.Contains(t, resp.ScopesSupported, "openid")
	assert.Contains(t, resp.GrantTypesSupported, "authorization_code")
}

func TestOIDCDiscoverySigningAlgorithmsShareSingleSource(t *testing.T) {
	appResp, err := (&appauth.Service{}).GetOIDCDiscovery(context.Background(), "https://example.com")
	require.NoError(t, err)

	h := Handler{}
	fallback, err := h.oidcDiscoveryFetch(context.Background(), "https://example.com")
	require.NoError(t, err)

	assert.Equal(t, platformjwks.IDTokenSigningAlgorithms, appResp.IDTokenSigningAlgValuesSupported)
	assert.Equal(t, platformjwks.IDTokenSigningAlgorithms, fallback.IDTokenSigningAlgValuesSupported)
	assert.Equal(t, appResp.IDTokenSigningAlgValuesSupported, fallback.IDTokenSigningAlgValuesSupported)

	// Response slices are copies: mutating one must not change the source or the other path.
	fallback.IDTokenSigningAlgValuesSupported[0] = "BROKEN"
	assert.Equal(t, "RS256", platformjwks.IDTokenSigningAlgorithms[0])
	assert.Equal(t, "RS256", appResp.IDTokenSigningAlgValuesSupported[0])
}

func TestOidcDiscoveryByApplication_WithFetcher(t *testing.T) {
	mockFetcher := &mockOIDCDiscoveryFetcher{
		response: &appauth.OIDCDiscoveryResponse{Issuer: "https://app.example.com"},
	}
	h := Handler{OIDCDiscoveryFetch: mockFetcher}
	resp, err := h.oidcDiscoveryByApplication(context.Background(), "https://example.com", "app1")
	assert.NoError(t, err)
	assert.Equal(t, "https://app.example.com", resp.Issuer)
}

func TestOidcDiscoveryByApplication_Fallback(t *testing.T) {
	h := Handler{}
	resp, err := h.oidcDiscoveryByApplication(context.Background(), "https://example.com", "app1")
	assert.NoError(t, err)
	assert.Equal(t, "https://example.com", resp.Issuer)
}

// GAP-009: the per-application route must surface an unknown slug/client_id as
// 404 instead of serving the global document.
func TestPerApplicationDiscovery_NotFoundIs404(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, Handler{OIDCDiscoveryFetch: &mockOIDCDiscoveryFetcher{
		err: apperrors.NewNotFound("application not found"),
	}})
	resp := doRequest(t, router, http.MethodGet, "/.well-known/missing/openid-configuration", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestOauthAuthorizationServer(t *testing.T) {
	h := Handler{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Host = "auth.example.com"
	// Forwarded proto from an untrusted peer is no longer honoured; local TLS is
	// the authoritative scheme signal for this document-shape test.
	r.TLS = &tls.ConnectionState{}

	doc, err := h.oauthAuthorizationServer(r)
	assert.NoError(t, err)
	assert.Equal(t, "https://auth.example.com", doc.Issuer)
	assert.Equal(t, "https://auth.example.com/api/v1/auth/authorize", doc.AuthorizationEndpoint)
	assert.Equal(t, "https://auth.example.com/api/v1/auth/tokens/exchange", doc.TokenEndpoint)
	assert.Equal(t, "https://auth.example.com/.well-known/jwks.json", doc.JWKSEndpoint)
	assert.Contains(t, doc.ScopesSupported, "openid")
	assert.Contains(t, doc.GrantTypesSupported, "authorization_code")
}

func TestWebFinger_MissingResource(t *testing.T) {
	h := Handler{}
	r := httptest.NewRequest(http.MethodGet, "/.well-known/webfinger", nil)
	_, err := h.webFinger(r)
	assert.Error(t, err)
}

func TestWebFinger_UnsupportedRel(t *testing.T) {
	h := Handler{}
	r := httptest.NewRequest(http.MethodGet, "/.well-known/webfinger?resource=acct/user@example.com&rel=something", nil)
	_, err := h.webFinger(r)
	assert.Error(t, err)
}

func TestWebFinger_Success(t *testing.T) {
	h := Handler{IssuerURL: "https://auth.example.com"}
	r := httptest.NewRequest(http.MethodGet, "/.well-known/webfinger?resource=acct/user@example.com", nil)
	resp, err := h.webFinger(r)
	assert.NoError(t, err)
	assert.Equal(t, "acct/user@example.com", resp.Subject)
	assert.Len(t, resp.Links, 1)
	assert.Equal(t, "http://openid.net/specs/connect/1.0/issuer", resp.Links[0].Rel)
	assert.Equal(t, "https://auth.example.com", resp.Links[0].Href)
}

func TestWebFinger_WithValidRel(t *testing.T) {
	h := Handler{IssuerURL: "https://auth.example.com"}
	r := httptest.NewRequest(http.MethodGet, "/.well-known/webfinger?resource=acct/user@example.com&rel=http://openid.net/specs/connect/1.0/issuer", nil)
	resp, err := h.webFinger(r)
	assert.NoError(t, err)
	assert.Equal(t, "acct/user@example.com", resp.Subject)
}

func TestProtectedResourceMetadata(t *testing.T) {
	h := Handler{IssuerURL: "https://auth.example.com"}
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	meta, err := h.protectedResourceMetadata(r)
	require.NoError(t, err)
	assert.Equal(t, "https://auth.example.com/oauth2/userinfo", meta.Resource)
	assert.Contains(t, meta.AuthorizationServers, "https://auth.example.com")
	assert.Contains(t, meta.ScopesSupported, "openid")
	assert.Contains(t, meta.BearerMethodsSupported, "header")
}

func TestParseRSAPublicKeyPEM_InvalidPEM(t *testing.T) {
	_, err := parseRSAPublicKeyPEM("not a PEM block")
	assert.Error(t, err)
}

func TestParseRSAPublicKeyPEM_ValidKey(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)

	pemStr := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	}))

	parsed, err := parseRSAPublicKeyPEM(pemStr)
	assert.NoError(t, err)
	assert.NotNil(t, parsed)
	assert.Equal(t, key.N, parsed.N)
}

func TestErrBadRequest(t *testing.T) {
	err := errBadRequest("test error")
	assert.Equal(t, "test error", err.Error())
}

// mock OIDCDiscoveryFetcher

type mockOIDCDiscoveryFetcher struct {
	response    *appauth.OIDCDiscoveryResponse
	err         error
	application *appauth.OIDCApplication
	resolveErr  error
}

func (m *mockOIDCDiscoveryFetcher) GetOIDCDiscovery(ctx context.Context, issuerURL string) (*appauth.OIDCDiscoveryResponse, error) {
	return m.response, m.err
}

func (m *mockOIDCDiscoveryFetcher) GetOIDCDiscoveryByApplication(ctx context.Context, issuerURL string, applicationID string) (*appauth.OIDCDiscoveryResponse, error) {
	return m.response, m.err
}

func (m *mockOIDCDiscoveryFetcher) ResolveOIDCApplication(ctx context.Context, reference string) (*appauth.OIDCApplication, error) {
	return m.application, m.resolveErr
}
