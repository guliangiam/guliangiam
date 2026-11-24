package config

import (
	"net/http"
	"os"
	"strings"
)

// IssuerURL resolves the OIDC issuer identifier.
//
// Every token claim that must agree with the discovery document (`iss` in the ID
// token, in the logout token, and the `issuer` field of
// `/.well-known/openid-configuration`) resolves through this one function. The
// defect it closes was a hardcoded constant in the token paths while discovery
// reported the deployment's real URL, so any deployment on a domain other than
// the literal string produced tokens no relying party would accept
// (设计报告 14.8).
//
// ISSUER_URL is authoritative. The request-derived fallback exists for local
// development where no external URL is configured; it must never be relied upon
// in a multi-domain deployment, because two hosts would then mint tokens with
// two different issuers.
func IssuerURL(r *http.Request) string {
	if configured := ConfiguredIssuerURL(); configured != "" {
		return configured
	}
	return RequestIssuerURL(r)
}

// ConfiguredIssuerURL returns the ISSUER_URL environment value, trimmed of any
// trailing slash so it compares byte-for-byte with the discovery `issuer`.
// It returns an empty string when unset, which is the signal for callers that
// have no request to fall back to (e.g. the background logout fan-out).
func ConfiguredIssuerURL() string {
	return strings.TrimRight(strings.TrimSpace(os.Getenv("ISSUER_URL")), "/")
}

// RequestIssuerURL derives the issuer from the request when ISSUER_URL is unset.
// Forwarded headers are honoured only when the direct peer is a configured
// trusted proxy: otherwise a client could choose the issuer that discovery
// advertises and that token `iss` claims repeat. Every request-derived issuer
// (`IssuerURL`, discovery, WebFinger, the DPoP `htu`) resolves through here.
func RequestIssuerURL(r *http.Request) string {
	if r == nil {
		return ""
	}

	scheme := TrustedForwardedProto(r)

	host := r.Host
	if forwarded := TrustedForwardedHost(r); forwarded != "" {
		host = forwarded
	}

	return strings.TrimRight(scheme+"://"+host, "/")
}

// TrustedForwardedHost returns X-Forwarded-Host only when the request came from
// a peer inside TRUSTED_PROXY_CIDRS — the same trust boundary the client-IP
// resolver uses (security-model S13). Multi-value headers take the first entry;
// an empty or syntactically unsafe value (path, credentials, query, fragment or
// whitespace) is discarded. A malformed CIDR entry is ignored, so it can never
// widen trust; with no configured CIDRs the header is never honoured.
func TrustedForwardedHost(r *http.Request) string {
	if !IsTrustedProxyRequest(r) {
		return ""
	}
	value := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Host"), ",")[0])
	if value == "" || strings.ContainsAny(value, "/\\?#@ \t") {
		return ""
	}
	return value
}

// TrustedForwardedProto is the scheme counterpart of TrustedForwardedHost. TLS
// terminated by this process is always authoritative; X-Forwarded-Proto is
// consulted only from a trusted proxy and only to upgrade to "https". Anything
// else falls back to "http", so a forged header cannot claim TLS.
func TrustedForwardedProto(r *http.Request) string {
	if r == nil {
		return "http"
	}
	if r.TLS != nil {
		return "https"
	}
	if !IsTrustedProxyRequest(r) {
		return "http"
	}
	value := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0])
	if strings.EqualFold(value, "https") {
		return "https"
	}
	return "http"
}

// AbsoluteRequestURL returns the absolute URL of r, i.e. the string a client
// that POSTed to this endpoint would have used. It exists for `htu`: a DPoP proof
// must name the URL the request was made to, and RFC 9449 §4.3 step 5 makes
// matching it mandatory, so the server has to reconstruct exactly what the
// client saw rather than compare against a path or an empty string.
//
// It deliberately shares the scheme/host derivation with RequestIssuerURL: if the
// two disagreed, a proof that is valid for one would fail for the other.
func AbsoluteRequestURL(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}
	base := RequestIssuerURL(r)
	if base == "" {
		return ""
	}
	path := r.URL.EscapedPath()
	if path == "" {
		return base
	}
	return base + path
}
