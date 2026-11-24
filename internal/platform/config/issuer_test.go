package config

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// These tests cover the single issuer resolver that both the discovery document
// and every `iss` claim use (设计报告 14.8). Their point is not the string
// formatting but the invariant: one function, therefore one value.

func TestIssuerURL_ConfiguredValueWins(t *testing.T) {
	t.Setenv("ISSUER_URL", "https://auth.example.com")

	r := httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", http.NoBody)
	assert.Equal(t, "https://auth.example.com", IssuerURL(r))
}

func TestIssuerURL_TrimsTrailingSlash(t *testing.T) {
	t.Setenv("ISSUER_URL", "https://auth.example.com/")

	r := httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", http.NoBody)
	assert.Equal(t, "https://auth.example.com", IssuerURL(r))
}

func TestIssuerURL_FallsBackToRequest(t *testing.T) {
	t.Setenv("ISSUER_URL", "")

	r := httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", http.NoBody)
	r.Host = "localhost:8080"
	assert.Equal(t, "http://localhost:8080", IssuerURL(r))
}

func TestIssuerURL_HonoursForwardedProto(t *testing.T) {
	t.Setenv("ISSUER_URL", "")
	t.Setenv("TRUSTED_PROXY_CIDRS", "192.0.2.1/32")

	r := httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", http.NoBody)
	r.Host = "api.example.com"
	r.RemoteAddr = "192.0.2.1:1234"
	r.Header.Set("X-Forwarded-Proto", "https")
	assert.Equal(t, "https://api.example.com", IssuerURL(r))
}

func TestIssuerURL_HonoursForwardedHost(t *testing.T) {
	t.Setenv("ISSUER_URL", "")
	t.Setenv("TRUSTED_PROXY_CIDRS", "192.0.2.1/32")

	r := httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", http.NoBody)
	r.Host = "internal:8080"
	r.RemoteAddr = "192.0.2.1:1234"
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("X-Forwarded-Host", "auth.example.com")
	assert.Equal(t, "https://auth.example.com", IssuerURL(r))
}

// TestRequestIssuerURL_IgnoresForwardedHeadersFromUntrustedPeer is the B2
// invariant at the lowest layer: the forwarded host is not an input unless the
// direct peer is a configured trusted proxy. A forged value, an unsafe value
// from a trusted peer, and a blank/ malformed CIDR configuration must all fall
// back to r.Host.
func TestRequestIssuerURL_IgnoresForwardedHeadersFromUntrustedPeer(t *testing.T) {
	tests := []struct {
		name       string
		cidrs      string
		remoteAddr string
		xfh        string
		want       string
	}{
		{
			name:       "forged host from untrusted peer",
			cidrs:      "192.0.2.1/32",
			remoteAddr: "203.0.113.9:1234",
			xfh:        "evil.example.com",
			want:       "http://internal:8080",
		},
		{
			name:       "path in host from trusted peer",
			cidrs:      "192.0.2.1/32",
			remoteAddr: "192.0.2.1:1234",
			xfh:        "evil.example.com/path",
			// The host is discarded, but the same trusted peer may still claim https.
			want: "https://internal:8080",
		},
		{
			name:       "no trusted proxies configured",
			cidrs:      "",
			remoteAddr: "192.0.2.1:1234",
			xfh:        "evil.example.com",
			want:       "http://internal:8080",
		},
		{
			name:       "malformed cidr fails closed",
			cidrs:      "not-a-cidr",
			remoteAddr: "192.0.2.1:1234",
			xfh:        "evil.example.com",
			want:       "http://internal:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TRUSTED_PROXY_CIDRS", tt.cidrs)

			r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/exchange", http.NoBody)
			r.RemoteAddr = tt.remoteAddr
			r.Host = "internal:8080"
			r.Header.Set("X-Forwarded-Host", tt.xfh)
			r.Header.Set("X-Forwarded-Proto", "https")

			assert.Equal(t, tt.want, RequestIssuerURL(r))
		})
	}
}

func TestTrustedForwardedHost(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "192.0.2.1/32")

	tests := []struct {
		name       string
		remoteAddr string
		xfh        string
		want       string
	}{
		{"multi-value takes the first entry", "192.0.2.1:1234", "auth.example.com, evil.example.com", "auth.example.com"},
		{"userinfo is rejected", "192.0.2.1:1234", "user@auth.example.com", ""},
		{"query is rejected", "192.0.2.1:1234", "auth.example.com?x=1", ""},
		{"blank value is rejected", "192.0.2.1:1234", " ", ""},
		{"untrusted peer is rejected", "203.0.113.9:1234", "auth.example.com", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			r.RemoteAddr = tt.remoteAddr
			r.Header.Set("X-Forwarded-Host", tt.xfh)
			assert.Equal(t, tt.want, TrustedForwardedHost(r))
		})
	}
}

func TestTrustedForwardedProto(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "192.0.2.1/32")

	tests := []struct {
		name       string
		remoteAddr string
		tls        bool
		xfp        string
		want       string
	}{
		{"tls wins over the header", "203.0.113.9:1234", true, "http", "https"},
		{"trusted proxy may claim https", "192.0.2.1:1234", false, "https", "https"},
		{"untrusted peer cannot claim https", "203.0.113.9:1234", false, "https", "http"},
		{"trusted proxy cannot downgrade tls", "192.0.2.1:1234", true, "http", "https"},
		{"multi-value takes the first entry", "192.0.2.1:1234", false, "https, http", "https"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			r.RemoteAddr = tt.remoteAddr
			r.Header.Set("X-Forwarded-Proto", tt.xfp)
			if tt.tls {
				r.TLS = &tls.ConnectionState{}
			}
			assert.Equal(t, tt.want, TrustedForwardedProto(r))
		})
	}
}

// TestAbsoluteRequestURL_SharesIssuerDerivation pins the DPoP `htu` to the same
// base as RequestIssuerURL: when a forged X-Forwarded-Host is ignored for the
// issuer it must be ignored for the proof URL too, or a valid proof would be
// checked against a value the client never used.
func TestAbsoluteRequestURL_SharesIssuerDerivation(t *testing.T) {
	t.Setenv("ISSUER_URL", "")
	t.Setenv("TRUSTED_PROXY_CIDRS", "192.0.2.1/32")

	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/exchange", http.NoBody)
	r.RemoteAddr = "203.0.113.9:1234"
	r.Host = "internal:8080"
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("X-Forwarded-Host", "evil.example.com")

	assert.Equal(t, "http://internal:8080", RequestIssuerURL(r))
	assert.Equal(t, "http://internal:8080/api/v1/auth/tokens/exchange", AbsoluteRequestURL(r))
}

func TestIssuerURL_NilRequestWithoutConfig(t *testing.T) {
	t.Setenv("ISSUER_URL", "")
	assert.Empty(t, IssuerURL(nil))
}

// TestIssuerURL_DoesNotDriftBetweenCallSites is the executable form of the
// defect: the configured value must be what discovery reports AND what the token
// paths embed, with no second code path that could disagree.
func TestIssuerURL_DoesNotDriftBetweenCallSites(t *testing.T) {
	t.Setenv("ISSUER_URL", "https://iam.acme.example/")

	discoveryRequest := httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", http.NoBody)
	discoveryRequest.Host = "internal-service:8080"

	tokenRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/exchange", http.NoBody)
	tokenRequest.Host = "another-host:9090"

	assert.Equal(t, ConfiguredIssuerURL(), IssuerURL(discoveryRequest))
	assert.Equal(t, IssuerURL(discoveryRequest), IssuerURL(tokenRequest))
	assert.Equal(t, "https://iam.acme.example", IssuerURL(tokenRequest))
}
