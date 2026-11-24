// Package aiupstream implements the infrastructure layer that talks to
// upstream LLM providers. All adapters in this package talk to the Core
// exclusively through the domain.UpstreamAdapter Port.
package aiupstream

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

// safeHTTPClient wraps net/http.Client with a DialContext interceptor that
// rejects outbound connections to private/local IPs not on the whitelist.
// This protects us against SSRF-style exfiltration when the base_url or a
// redirect points somewhere unintended.
var (
	errSSRFRejected = errors.New("ssrf_client: connection to disallowed address rejected")
)

// lookupIPAddr 是 DNS 解析入口（变量便于测试注入，模拟 DNS rebinding）。
var lookupIPAddr = net.DefaultResolver.LookupIPAddr

// NewSSRFClient returns an *http.Client whose DialContext only allows
// connections to:
//   - public (globally routable) IPs, or
//   - private IPs on the local whitelist (loopback + RFC1918 ranges).
//
// All other destinations — link-local, multicast, unspecified, or private
// ranges outside the whitelist — are refused at dial time.
func NewSSRFClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DialContext:           ssrfDialContext,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// ssrfDialContext is the Transport.DialContext replacement. It resolves the
// target host, rejects connections whose resolved IP is non-public and not
// on the whitelist, then delegates to a real net.Dialer.
func ssrfDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	ips, err := lookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, &net.OpError{Op: "dial", Net: network, Err: errors.New("no resolved addresses")}
	}

	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	for _, ip := range ips {
		if !isAllowedIP(ip.IP) {
			continue
		}
		// 直接按已校验的 numeric IP 拨号，避免“校验后再解析”的 DNS rebinding 窗口。
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		return nil, err
	}

	return nil, errSSRFRejected
}

// isAllowedIP returns true when ip is either globally routable or falls into
// the local whitelist: 127.0.0.0/8, 10.0.0.0/8, 192.168.0.0/16, 172.16.0.0/12.
func isAllowedIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	// Loopback — always allowed (local models).
	if ip.IsLoopback() {
		return true
	}
	// RFC1918 private ranges explicitly whitelisted.
	if ip.IsPrivate() {
		// IsPrivate covers 10/8, 172.16/12, 192.168/16 — exactly our whitelist.
		return true
	}
	// Globally routable public address — allowed.
	if ip.IsGlobalUnicast() {
		return true
	}
	// Link-local, multicast, unspecified, IPv4-mapped IPv6 variants, etc. — rejected.
	return false
}
