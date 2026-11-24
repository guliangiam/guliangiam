package http

import (
	"net/http/httptest"
	"testing"

	"github.com/guliangiam/guliangiam/internal/platform/config"
	"github.com/stretchr/testify/assert"
)

func TestIsTrustedProxyMatchesConfigMatcher(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "192.0.2.0/24, 2001:db8::/32")
	for _, ip := range []string{"192.0.2.10", "198.51.100.10", "2001:db8::1", "2001:db9::1", ""} {
		assert.Equal(t, config.IsTrustedProxyIP(ip), isTrustedProxy(ip), ip)
	}
}

func TestClientIP_TrustedProxyStripsXFFRightToLeft(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "192.0.2.0/24,10.0.0.5/32")
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.0.2.10:1234"
	r.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.5")
	assert.Equal(t, "203.0.113.7", ClientIP(r))
}

func TestClientIP_UntrustedPeerIgnoresForwardedHeaders(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "192.0.2.0/24")
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "198.51.100.10:1234"
	r.Header.Set("X-Forwarded-For", "203.0.113.7")
	assert.Equal(t, "198.51.100.10", ClientIP(r))
}
