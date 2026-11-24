package applications

import (
	"net/url"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	// fieldValue 是重定向 URI 校验错误元数据的原始值键。
	fieldValue = "value"
	// schemeHTTP 是 RFC 8252 §7 允许的回环例外使用的明文协议。
	schemeHTTP = "http"
)

// loopbackLiterals is the RFC 8252 §7 native-application redirect exception, and
// the only place a plain-http redirect URI is acceptable. It is deliberately the
// two IP literals and nothing else: `localhost` and `*.localhost` resolve
// through the resolver (and through /etc/hosts, which any local process can
// rewrite), so neither is the loopback interface by construction.
var loopbackLiterals = map[string]struct{}{
	"127.0.0.1": {},
	"::1":       {},
}

// MatchRedirectURI reports whether requested is one of the URIs the application
// registered (设计报告 14.6.3). It is the single implementation for every authorize
// path: it replaced isRedirectURIValid and isRedirectURIAllowed, which disagreed
// with each other — one admitted a `*.example.com` registration for *any* depth
// of subdomain and ignored the query entirely, the other admitted every URI
// whenever the registered set happened to be empty, which is an open redirect.
//
// The rules, in order:
//
//  1. requested must be an absolute URI with no fragment, no userinfo, and
//     either an https scheme or the RFC 8252 §7 loopback exception.
//  2. full-string equality with a registered value. `/cb` is not `/cb/`, and a
//     registered value that carries no query admits no query.
//  3. the loopback exception: when both sides are the same loopback literal and
//     the registered value carries no port, the port is ignored. Path and query
//     still compare exactly.
//  4. everything else is false. There is no wildcard, no regex, no
//     suffix/substring matching, and an empty registered set admits nothing.
func MatchRedirectURI(requested string, registered []string) bool {
	requested = strings.TrimSpace(requested)
	if requested == "" || len(registered) == 0 {
		return false
	}

	parsed, ok := parseRedirectURI(requested)
	if !ok {
		return false
	}

	for _, candidate := range registered {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if candidate == requested {
			return true
		}
		if matchesLoopbackException(parsed, candidate) {
			return true
		}
	}

	return false
}

// ValidateRedirectURI rejects a redirect URI that could never match, or that
// could match more than the operator intended. Each branch names the rule it
// enforces, because the realistic failure is an operator who registered
// `http://localhost:7101/callback` for local development and now needs to know
// what to type instead.
func ValidateRedirectURI(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "redirectUris must not contain an empty value", map[string]any{"field": "redirectUris", "expected": "non-empty value"})
	}
	if strings.Contains(trimmed, "*") {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"redirectUris must not contain a wildcard: registration is exact-match, "+
				"so `https://*.example.com/cb` could never be matched and would only "+
				"become a hole if the matcher were ever relaxed",
			map[string]any{"field": "redirectUris", fieldValue: trimmed, "expected": "no_wildcard"},
		)
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"redirectUris must be an absolute URL",
			map[string]any{"field": "redirectUris", fieldValue: trimmed, "format": "uri"},
		)
	}
	if parsed.Fragment != "" || parsed.RawFragment != "" {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"redirectUris must not contain a fragment",
			map[string]any{"field": "redirectUris", fieldValue: trimmed, "expected": "no_fragment"},
		)
	}
	if parsed.User != nil {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"redirectUris must not contain userinfo",
			map[string]any{"field": "redirectUris", fieldValue: trimmed, "expected": "no_userinfo"},
		)
	}

	switch {
	case parsed.Scheme == "https":
		return nil
	case parsed.Scheme == schemeHTTP && isLoopbackLiteral(parsed.Hostname()):
		// RFC 8252 §7: a native app may use plain http, but only on the loopback
		// interface, and only by IP literal.
		return nil
	default:
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"redirectUris must use https; plain http is accepted only for the loopback "+
				"IP literals 127.0.0.1 and [::1] (RFC 8252 §7), not for a hostname such "+
				"as localhost",
			map[string]any{"field": "redirectUris", fieldValue: trimmed, "format": "uri", "expected": "https_or_loopback_http"},
		)
	}
}

// parseRedirectURI reports whether value is a redirect URI this server is
// willing to compare at all, and returns it parsed.
func parseRedirectURI(value string) (*url.URL, bool) {
	parsed, err := url.Parse(value)
	if err != nil {
		return nil, false
	}
	if !parsed.IsAbs() || parsed.Host == "" {
		return nil, false
	}
	if parsed.Fragment != "" || parsed.RawFragment != "" {
		return nil, false
	}
	if parsed.User != nil {
		return nil, false
	}
	if parsed.Scheme == "https" {
		return parsed, true
	}
	// The authorization code travels in this URI, so plain http is only
	// acceptable when it cannot leave the machine (RFC 8252 §7).
	if parsed.Scheme == schemeHTTP && isLoopbackLiteral(parsed.Hostname()) {
		return parsed, true
	}
	return nil, false
}

// matchesLoopbackException applies the port-variable rule: a registration of
// `http://127.0.0.1/cb` admits `http://127.0.0.1:54321/cb` because a native app
// cannot know its ephemeral port in advance (RFC 8252 §7).
func matchesLoopbackException(requested *url.URL, registered string) bool {
	if requested.Scheme != schemeHTTP || !isLoopbackLiteral(requested.Hostname()) {
		return false
	}

	parsed, err := url.Parse(registered)
	if err != nil {
		return false
	}
	if parsed.Scheme != schemeHTTP || !isLoopbackLiteral(parsed.Hostname()) {
		return false
	}
	if parsed.Fragment != "" || parsed.RawFragment != "" || parsed.User != nil {
		return false
	}
	// Same literal only: `127.0.0.1` must not admit `::1`. They are different
	// bindings, and conflating them would let a registration for one interface
	// receive a code redirected to the other.
	if parsed.Hostname() != requested.Hostname() {
		return false
	}
	// A registered port is an exact value, and the whole-string comparison above
	// already covered it. Only a port-less registration makes the port variable.
	if parsed.Port() != "" {
		return false
	}
	if parsed.EscapedPath() != requested.EscapedPath() {
		return false
	}
	// The registered value carrying no query is what makes a query-less request
	// the match; a query on either side must agree exactly.
	return parsed.RawQuery == requested.RawQuery
}

func isLoopbackLiteral(hostname string) bool {
	_, ok := loopbackLiterals[hostname]
	return ok
}
