package bootstrap

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"

	auth "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// rootPublicRoutes lists the routes registered directly on the root router
// (chi.go), which therefore have no routes.go declaration to carry an explicit
// Permission/Surface. They are enumerated here instead of being inferred from a
// prefix.
var rootPublicRoutes = map[string]struct{}{
	"GET /healthz":                          {},
	"GET /livez":                            {},
	"GET /.well-known/openid-configuration": {},
	"GET /uploads/*":                        {},
	"HEAD /uploads/*":                       {}, // 静态代理的读方法对（chi.go 按方法注册，不用 Handle 兜底）
	"GET /openapi/*":                        {},
}

// explicitPublicRouteExemptions lists the routes that must stay anonymous even
// though their declaration is not Permission="public" and not SurfaceProtocol.
// Each entry is method + chi pattern, so a new sibling route under the same
// path prefix is NOT covered.
var explicitPublicRouteExemptions = map[string]struct{}{
	// Pre-login organization discovery. The handler projects the aggregate onto
	// OrganizationPublicDetail (no credential/security fields); the route is
	// declared SurfaceManagement today, so it needs the explicit entry.
	"GET /api/v1/organizations/by-slug/{slug}": {},
}

// publicPrefixFamilies mirrors the prefix branches in
// internal/transport/http/api/v1/auth/auth_paths.go. It is used ONLY to name
// the rule that accidentally covered a route in the failure message; the
// pass/fail decision does not depend on it.
var publicPrefixFamilies = []string{
	"/.well-known/",
	"/openapi/",
	"/uploads/",
	"/cas/",
	"/oauth2/",
	"/kerberos/",
	"/scim/v2/",
	"/api/v1/saml/",
	"/api/v1/auth/saml/",
	"/api/v1/dcr/",
	"/api/v1/applications/by-client-id/",
	"/api/v1/organizations/by-slug/",
	"/api/v1/payments/notify/",
	"/api/v1/auth/captcha",
}

// validateStartupPublicWhitelist walks the real chi route tree and requires
// every anonymously reachable route to be explicitly registered, not merely
// swallowed by a prefix in the runtime whitelist.
func validateStartupPublicWhitelist(router chi.Router) {
	if err := validatePublicRouteWhitelist(router, allRouteDefinitions()); err != nil {
		panic("startup public whitelist contract violation: " + err.Error())
	}
}

// validatePublicRouteWhitelist is the testable core. A route is explicitly
// public when at least one of the following holds:
//
//  1. it is a root route enumerated in rootPublicRoutes;
//  2. it is enumerated in explicitPublicRouteExemptions;
//  3. the whitelist rule that matched it is a per-route rule (exact path or
//     single-segment template), not a namespace family;
//  4. it is covered by a family prefix, but its own route declaration says
//     Permission="public" or SurfaceProtocol (per-route declaration, not a
//     path-family inference).
func validatePublicRouteWhitelist(router chi.Router, definitions []route.Definition) error {
	byKey := make(map[string]route.Definition, len(definitions))
	for _, definition := range definitions {
		byKey[normalizePublicRouteKey(definition.Method, definition.Pattern)] = definition
	}

	violations := make(map[string]struct{})
	err := chi.Walk(router, func(method, pattern string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		pattern = normalizePublicRoutePattern(pattern)
		if !auth.IsPublicRequest(method, pattern) {
			return nil
		}
		key := method + " " + pattern
		if _, ok := rootPublicRoutes[key]; ok {
			return nil
		}
		if _, ok := explicitPublicRouteExemptions[key]; ok {
			return nil
		}
		rule, kind := auth.PublicRule(pattern)
		if kind != auth.PublicRuleFamily {
			return nil
		}
		if definition, ok := byKey[key]; ok &&
			(definition.Permission == "public" || definition.Surface == route.SurfaceProtocol) {
			return nil
		}
		violations[fmt.Sprintf(
			"%s 仅被白名单前缀 %q（规则 %s）顺带覆盖，未在路由声明（Permission=public / SurfaceProtocol）或显式清单中逐条登记",
			key, matchedPublicPrefix(pattern), rule)] = struct{}{}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk router for public whitelist check: %w", err)
	}
	if len(violations) == 0 {
		return nil
	}
	ordered := make([]string, 0, len(violations))
	for violation := range violations {
		ordered = append(ordered, violation)
	}
	sort.Strings(ordered)
	return fmt.Errorf("public whitelist contract violation:\n  %s", strings.Join(ordered, "\n  "))
}

// normalizePublicRoutePattern drops the trailing slash chi.Walk reports for a
// route registered with an explicit "/" child, so it matches routes.go patterns.
func normalizePublicRoutePattern(pattern string) string {
	if len(pattern) > 1 {
		return strings.TrimSuffix(pattern, "/")
	}
	return pattern
}

func normalizePublicRouteKey(method, pattern string) string {
	return method + " " + normalizePublicRoutePattern(pattern)
}

// matchedPublicPrefix returns the whitelist prefix that covers pattern, or
// "(unknown)" when no family matches (diagnostics only).
func matchedPublicPrefix(pattern string) string {
	for _, prefix := range publicPrefixFamilies {
		if strings.HasPrefix(pattern, prefix) {
			return prefix
		}
	}
	return "(unknown)"
}
