package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/guliangiam/guliangiam/internal/platform/config"
)

// isHTTPSRequest also honors X-Forwarded-Proto=https from a trusted proxy:
// TLS is commonly terminated at the proxy, leaving r.TLS nil at the backend
// (BE-E-18). Untrusted callers cannot spoof the header into forcing HSTS.
func isHTTPSRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return config.IsTrustedProxyIP(host) && strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

type SecurityHeadersConfig struct {
	ContentTypeNosniff bool
	FrameDeny          bool
	HSTSMaxAge         int
	HSTSIncludeSubs    bool
	HSTSPreload        bool
	CSPPolicy          string
	// CSPReportOnly emits the policy on `Content-Security-Policy-Report-Only`
	// instead of `Content-Security-Policy`.
	//
	// The policy below is only safe to *enforce* once every HTML response this
	// server emits satisfies it. It does not today: postSamlResponse
	// (internal/transport/http/api/v1/saml/chi.go) auto-submits the SP form via
	// an inline `onload=` handler, which `script-src 'self'` blocks. Enforcing
	// the policy therefore breaks the SAML POST binding outright. Report-only
	// keeps the violations observable in the browser console without taking the
	// flow down; enforcement becomes available as soon as the SAML response
	// carries a nonce (or a hash) and the policy lists it.
	CSPReportOnly  bool
	XSSProtection  bool
	ReferrerPolicy string
	// PermissionsPolicy is emitted as `Permissions-Policy`. An empty string
	// omits the header entirely.
	PermissionsPolicy string
	// ReportToURL, when non-empty, is where CSP violations are POSTed. It is
	// resolved from CSP_REPORT_TO_URL. When set, the policy gains both the
	// legacy `report-uri` directive and the `report-to csp-endpoint` directive
	// (手册 4.17.1), and a matching `Report-To` header declares the group.
	// Reports are meant for an internal endpoint only; never point this at a
	// third party.
	ReportToURL string
}

// DefaultPermissionsPolicy denies the powerful browser features this server has
// no use for, while keeping the two the product actually depends on:
//   - camera=(self): face recognition / ID-card capture (getUserMedia) runs on
//     the console, which may be served same-origin in a BFF deployment;
//   - publickey-credentials-{get,create}=(self): WebAuthn registration and
//     login ceremonies.
//
// Denying the rest is what makes the header worth sending: an absent policy
// leaves every one of these features at the browser default.
const DefaultPermissionsPolicy = "camera=(self), " +
	"microphone=(), geolocation=(), payment=(), usb=(), serial=(), bluetooth=(), " +
	"midi=(), display-capture=(), idle-detection=(), magnetometer=(), gyroscope=(), " +
	"accelerometer=(), publickey-credentials-get=(self), publickey-credentials-create=(self)"

func DefaultSecurityHeadersConfig() SecurityHeadersConfig {
	return SecurityHeadersConfig{
		ContentTypeNosniff: true,
		FrameDeny:          true,
		HSTSMaxAge:         31536000,
		HSTSIncludeSubs:    true,
		HSTSPreload:        true,
		CSPPolicy:          "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'",
		CSPReportOnly:      true,
		ReportToURL:        config.CSPReportToURL(),
		XSSProtection:      true,
		ReferrerPolicy:     "strict-origin-when-cross-origin",
		PermissionsPolicy:  DefaultPermissionsPolicy,
	}
}

func SecurityHeaders(cfg SecurityHeadersConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.ContentTypeNosniff {
				w.Header().Set("X-Content-Type-Options", "nosniff")
			}
			if cfg.FrameDeny {
				w.Header().Set("X-Frame-Options", "DENY")
			}
			if cfg.HSTSMaxAge > 0 && isHTTPSRequest(r) {
				val := fmt.Sprintf("max-age=%d", cfg.HSTSMaxAge)
				if cfg.HSTSIncludeSubs {
					val += "; includeSubDomains"
				}
				if cfg.HSTSPreload {
					val += "; preload"
				}
				w.Header().Set("Strict-Transport-Security", val)
			}
			if cfg.CSPPolicy != "" {
				policy := cfg.CSPPolicy
				if cfg.ReportToURL != "" {
					policy += "; report-uri " + cfg.ReportToURL + "; report-to csp-endpoint"
				}
				if cfg.CSPReportOnly {
					w.Header().Set("Content-Security-Policy-Report-Only", policy)
				} else {
					w.Header().Set("Content-Security-Policy", policy)
				}
			}
			if cfg.ReportToURL != "" {
				w.Header().Set("Report-To", fmt.Sprintf(
					`{"group":"csp-endpoint","max_age":10886400,"endpoints":[{"url":%q}]}`,
					cfg.ReportToURL))
			}
			if cfg.XSSProtection {
				w.Header().Set("X-XSS-Protection", "0")
			}
			if cfg.ReferrerPolicy != "" {
				w.Header().Set("Referrer-Policy", cfg.ReferrerPolicy)
			}
			if cfg.PermissionsPolicy != "" {
				w.Header().Set("Permissions-Policy", cfg.PermissionsPolicy)
			}
			next.ServeHTTP(w, r)
		})
	}
}
