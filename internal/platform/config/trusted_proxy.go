package config

import (
	"net"
	"net/http"
	"strings"
)

// IsTrustedProxyIP reports whether remoteIP belongs to TRUSTED_PROXY_CIDRS.
// It is the single matcher for callers deciding whether forwarded headers may
// be trusted. Blank configuration trusts nobody; malformed entries are skipped
// fail-closed, without panicking or logging.
func IsTrustedProxyIP(remoteIP string) bool {
	raw := TrustedProxyCIDRs()
	if raw == "" {
		return false
	}
	ip := net.ParseIP(strings.TrimSpace(remoteIP))
	if ip == nil {
		return false
	}
	for _, entry := range strings.Split(raw, ",") {
		_, network, err := net.ParseCIDR(strings.TrimSpace(entry))
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// IsTrustedProxyRequest reports whether r's direct peer belongs to
// TRUSTED_PROXY_CIDRS. It is the request-level counterpart of
// IsTrustedProxyIP and the single matcher for every forwarded-header trust
// decision, so the issuer and client-certificate paths cannot drift.
func IsTrustedProxyRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	remote := strings.TrimSpace(r.RemoteAddr)
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	return IsTrustedProxyIP(strings.TrimSpace(host))
}
