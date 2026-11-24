package config

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsTrustedProxyIP(t *testing.T) {
	tests := []struct {
		name            string
		cidrs, remoteIP string
		want            bool
	}{
		{"empty list", "", "192.0.2.10", false},
		{"single match", "192.0.2.0/24", "192.0.2.10", true},
		{"single miss", "192.0.2.0/24", "198.51.100.10", false},
		{"multiple match", "10.0.0.0/8,192.0.2.0/24", "192.0.2.10", true},
		{"mixed whitespace and trailing comma", " 10.0.0.0/8 , 192.0.2.0/24 , ", "192.0.2.10", true},
		{"invalid entries skipped", "not-a-cidr,192.0.2.0/24", "192.0.2.10", true},
		{"only invalid entries fail closed", "not-a-cidr,", "192.0.2.10", false},
		{"blank remote ip", "192.0.2.0/24", " ", false},
		{"malformed remote ip", "192.0.2.0/24", "not-an-ip", false},
		{"ipv4 in cidr", "192.0.2.0/24", "192.0.2.1", true},
		{"ipv4 outside cidr", "192.0.2.0/24", "198.51.100.1", false},
		{"ipv6 in cidr", "2001:db8::/32", "2001:db8::1", true},
		{"ipv6 outside cidr", "2001:db8::/32", "2001:db9::1", false},
		{"ipv6 in mixed list", "10.0.0.0/8, 2001:db8::/32", "2001:db8::1", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TRUSTED_PROXY_CIDRS", tt.cidrs)
			assert.Equal(t, tt.want, IsTrustedProxyIP(tt.remoteIP))
		})
	}
}

func TestIsTrustedProxyRequestUsesRemoteHost(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "192.0.2.0/24")
	assert.True(t, IsTrustedProxyRequest(&http.Request{RemoteAddr: "192.0.2.10:1234"}))
	assert.False(t, IsTrustedProxyRequest(&http.Request{RemoteAddr: "198.51.100.10:1234"}))
	assert.True(t, IsTrustedProxyRequest(&http.Request{RemoteAddr: "192.0.2.10"}))
	assert.False(t, IsTrustedProxyRequest(&http.Request{RemoteAddr: "not-an-ip:1234"}))
	assert.False(t, IsTrustedProxyRequest(nil))
}

func TestValidHeaderName(t *testing.T) {
	for _, name := range []string{"X-Forwarded-Client-Cert", "X-Client-Cert", "x_ssl_client_cert"} {
		assert.True(t, ValidHeaderName(name), name)
	}
	for _, name := range []string{"", "X Forwarded", "X-Forwarded-Client-Cert\r\n", "X:Y", "证书"} {
		assert.False(t, ValidHeaderName(name), name)
	}
}

func TestMTLSProxyClientCertHeaderTrims(t *testing.T) {
	t.Setenv("MTLS_PROXY_CLIENT_CERT_HEADER", "  X-Client-Cert  ")
	assert.Equal(t, "X-Client-Cert", MTLSProxyClientCertHeader())
}
