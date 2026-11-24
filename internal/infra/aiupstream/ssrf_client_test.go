package aiupstream

import (
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── isAllowedIP boundary cases ─────────────────────────────────────────

func TestIsAllowedIP_LoopbackV4(t *testing.T) {
	for _, ipStr := range []string{"127.0.0.1", "127.0.0.0", "127.255.255.255", "127.1.2.3"} {
		ip := net.ParseIP(ipStr)
		require.NotNil(t, ip)
		assert.True(t, isAllowedIP(ip), "loopback %s must be allowed", ipStr)
	}
}

func TestIsAllowedIP_LoopbackV6(t *testing.T) {
	// ::1 — IPv6 loopback
	ip := net.ParseIP("::1")
	require.NotNil(t, ip)
	assert.True(t, isAllowedIP(ip), "IPv6 loopback ::1 must be allowed")
}

func TestIsAllowedIP_RFC1918_10x(t *testing.T) {
	for _, ipStr := range []string{"10.0.0.1", "10.255.255.254", "10.1.2.3"} {
		ip := net.ParseIP(ipStr)
		require.NotNil(t, ip)
		assert.True(t, isAllowedIP(ip), "RFC1918 10/8 %s must be allowed", ipStr)
	}
}

func TestIsAllowedIP_RFC1918_17216_17231(t *testing.T) {
	// 172.16.0.0/12 → 172.16.0.0 – 172.31.255.255
	for _, ipStr := range []string{"172.16.0.1", "172.31.255.254", "172.20.5.5"} {
		ip := net.ParseIP(ipStr)
		require.NotNil(t, ip)
		assert.True(t, isAllowedIP(ip), "RFC1918 172.16/12 %s must be allowed", ipStr)
	}
}

func TestIsAllowedIP_RFC1918_192168(t *testing.T) {
	for _, ipStr := range []string{"192.168.0.1", "192.168.255.254", "192.168.10.20"} {
		ip := net.ParseIP(ipStr)
		require.NotNil(t, ip)
		assert.True(t, isAllowedIP(ip), "RFC1918 192.168/16 %s must be allowed", ipStr)
	}
}

func TestIsAllowedIP_PublicGlobalUnicast(t *testing.T) {
	// Globally routable addresses
	for _, ipStr := range []string{
		"8.8.8.8",              // Google DNS
		"1.1.1.1",              // Cloudflare DNS
		"93.184.216.34",        // example.com
		"2606:4700:4700::1111", // Cloudflare IPv6
	} {
		ip := net.ParseIP(ipStr)
		require.NotNil(t, ip)
		assert.True(t, isAllowedIP(ip), "public %s must be allowed", ipStr)
	}
}

func TestIsAllowedIP_LinkLocal_Rejected(t *testing.T) {
	// 169.254.0.0/16 — link-local, NOT on whitelist
	for _, ipStr := range []string{"169.254.1.1", "169.254.255.254"} {
		ip := net.ParseIP(ipStr)
		require.NotNil(t, ip)
		assert.False(t, isAllowedIP(ip), "link-local %s must be rejected", ipStr)
	}
	// IPv6 link-local fe80::
	ip := net.ParseIP("fe80::1")
	require.NotNil(t, ip)
	assert.False(t, isAllowedIP(ip), "IPv6 link-local fe80 must be rejected")
}

func TestIsAllowedIP_Multicast_Rejected(t *testing.T) {
	// 224.0.0.0/4 — multicast
	for _, ipStr := range []string{"224.0.0.1", "239.255.255.255"} {
		ip := net.ParseIP(ipStr)
		require.NotNil(t, ip)
		assert.False(t, isAllowedIP(ip), "multicast %s must be rejected", ipStr)
	}
}

func TestIsAllowedIP_Unspecified_Rejected(t *testing.T) {
	for _, ipStr := range []string{"0.0.0.0", "::"} {
		ip := net.ParseIP(ipStr)
		require.NotNil(t, ip)
		assert.False(t, isAllowedIP(ip), "unspecified %s must be rejected", ipStr)
	}
}

func TestIsAllowedIP_Nil_Rejected(t *testing.T) {
	assert.False(t, isAllowedIP(nil), "nil IP must be rejected")
}

func TestIsAllowedIP_Reserved_CGNandDocumentationAllowedAsGlobalUnicast(t *testing.T) {
	// Go's net.IP.IsGlobalUnicast() returns true for 100.64.0.0/10 (CGN)
	// and 192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24 (documentation ranges).
	// These are technically globally routable (they just won't route anywhere
	// useful). Our SSRF guard allows them per the current implementation.
	for _, ipStr := range []string{
		"100.64.0.1",
		"192.0.2.1",
		"198.51.100.1",
		"203.0.113.1",
	} {
		ip := net.ParseIP(ipStr)
		require.NotNil(t, ip)
		assert.True(t, isAllowedIP(ip), "Go's IsGlobalUnicast allows %s — our guard follows", ipStr)
	}
}

// ── NewSSRFClient wiring ────────────────────────────────────────────────

func TestNewSSRFClient_ReturnsHTTPClient(t *testing.T) {
	client := NewSSRFClient(5 * time.Second)
	require.NotNil(t, client)
	assert.Equal(t, 5*time.Second, client.Timeout)

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok, "Transport must be *http.Transport")
	assert.NotNil(t, transport.DialContext, "DialContext must be set")
	assert.NotNil(t, transport.Proxy, "Proxy should be set (env-based)")
}

func TestNewSSRFClient_WithZeroTimeout_SetsTransportTimeouts(t *testing.T) {
	client := NewSSRFClient(0)
	require.NotNil(t, client)

	transport := client.Transport.(*http.Transport)
	assert.Equal(t, 10*time.Second, transport.TLSHandshakeTimeout)
	assert.Equal(t, 90*time.Second, transport.IdleConnTimeout)
	assert.Equal(t, 1*time.Second, transport.ExpectContinueTimeout)
}

// ── ssrfDialContext error when all IPs rejected (integration-ish) ──────
//
// We don't test ssrfDialContext end-to-end because it does real DNS resolution,
// but isAllowedIP covers the actual decision logic. The DialContext wiring in
// NewSSRFClient is verified above.

// ── Edge: IPv4-mapped IPv6 ──────────────────────────────────────────────

func TestIsAllowedIP_IPv4MappedIPv6(t *testing.T) {
	// ::ffff:169.254.1.1 — IPv4-mapped link-local; Go's IsPrivate/IsLoopback
	// treat mapped addresses as their IPv4 equivalent, so this should be
	// rejected since 169.254 is link-local.
	ip := net.ParseIP("::ffff:169.254.1.1")
	require.NotNil(t, ip)
	assert.False(t, isAllowedIP(ip), "IPv4-mapped link-local ::ffff:169.254.1.1 must be rejected")

	// ::ffff:10.0.0.1 — IPv4-mapped RFC1918; should be allowed
	ip = net.ParseIP("::ffff:10.0.0.1")
	require.NotNil(t, ip)
	assert.True(t, isAllowedIP(ip), "IPv4-mapped RFC1918 ::ffff:10.0.0.1 must be allowed")
}

// ── Confirm SSRF rejection constant is exported via error wrapping ──────

func TestSSRFRejectedError_Value(t *testing.T) {
	// Just verify the internal sentinel error exists and is not nil
	require.NotNil(t, errSSRFRejected)
	assert.Contains(t, errSSRFRejected.Error(), "ssrf_client")
}
