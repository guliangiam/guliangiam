package bootstrap

import (
	"sort"
	"testing"

	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// TestSurfaceContract enforces the API three-surface contract of 手册 4.1.3:
// every declared route must carry an explicit Surface, the value must be one of
// the three declared faces, and it must agree with the central classifier
// route.SurfaceFor.
//
// The last assertion is what makes the field trustworthy: without it a stale or
// copy-pasted Surface would silently mis-tag OpenAPI tags, metrics labels and
// audit rows.
func TestSurfaceContract(t *testing.T) {
	counts := map[route.Surface]int{}

	for _, mod := range declaredRouteModules {
		for _, d := range mod.routes() {
			key := d.Method + " " + d.Pattern

			if d.Surface == "" {
				t.Errorf("route %q (%s) declares no Surface; every declaration must state its API face (手册 4.1.3)", key, mod.name)
				continue
			}
			if !d.Surface.Valid() {
				t.Errorf("route %q (%s) declares invalid Surface %q; valid values are %v", key, mod.name, d.Surface, route.AllSurfaces)
				continue
			}
			if want := route.SurfaceFor(d.Pattern); d.Surface != want {
				t.Errorf("route %q (%s) declares Surface %q but the path classifier says %q; fix the declaration or the classifier rule",
					key, mod.name, d.Surface, want)
			}
			counts[d.Surface]++
		}
	}

	// A classifier that collapsed every path into one face would make the
	// per-surface assertions vacuous, so require all three faces to be used.
	for _, surface := range route.AllSurfaces {
		if counts[surface] == 0 {
			t.Errorf("surface %q has no declared route; the three faces must all be populated", surface)
		}
	}

	surfaces := make([]string, 0, len(counts))
	total := 0
	for surface, n := range counts {
		surfaces = append(surfaces, string(surface))
		total += n
	}
	sort.Strings(surfaces)
	for _, surface := range surfaces {
		t.Logf("surface %-14s %3d routes", surface, counts[route.Surface(surface)])
	}
	t.Logf("surface contract holds for %d declared routes", total)
}

// TestSurfaceForKnownPaths pins the classifier against hand-checked examples
// from 手册 4.1.3, so a rule edit that fixes one family cannot silently break
// another.
func TestSurfaceForKnownPaths(t *testing.T) {
	cases := []struct {
		pattern string
		want    route.Surface
	}{
		// ① root /oauth2/*
		{"/oauth2/register", route.SurfaceProtocol},
		{"/oauth2/register/{clientID}", route.SurfaceProtocol},
		{"/oauth2/userinfo", route.SurfaceProtocol},
		// ② root /.well-known/*
		{"/.well-known/openid-configuration", route.SurfaceProtocol},
		{"/.well-known/{application}/jwks.json", route.SurfaceProtocol},
		// ③ root /cas/{org}/{app}/*
		{"/cas/{organizationSlug}/{applicationSlug}/serviceValidate", route.SurfaceProtocol},
		// ④ root /scim/v2/*
		{"/scim/v2/Users", route.SurfaceProtocol},
		// ⑤ protocol endpoints under /api/v1/auth/*
		{"/api/v1/auth/authorize", route.SurfaceProtocol},
		{"/api/v1/auth/tokens/exchange", route.SurfaceProtocol},
		{"/api/v1/auth/device/authorize", route.SurfaceProtocol},
		{"/api/v1/auth/device/verify", route.SurfaceProtocol},
		{"/api/v1/auth/oauth2/revoke", route.SurfaceProtocol},
		{"/api/v1/auth/webauthn/registration/begin", route.SurfaceProtocol},
		{"/api/v1/auth/cas/serviceValidate", route.SurfaceProtocol},
		{"/api/v1/auth/kerberos/login", route.SurfaceProtocol},
		{"/api/v1/auth/saml/acs", route.SurfaceProtocol},
		{"/api/v1/auth/saml/metadata/{orgSlug}/{appSlug}", route.SurfaceProtocol},
		{"/api/v1/auth/client/authenticate", route.SurfaceProtocol},
		{"/api/v1/auth/dpop/validate", route.SurfaceProtocol},
		{"/api/v1/auth/backchannel-logout", route.SurfaceProtocol},
		{"/api/v1/auth/consents/grant", route.SurfaceProtocol},
		// ⑥ root /api/v1/saml/*
		{"/api/v1/saml/sso/{applicationId}", route.SurfaceProtocol},
		{"/api/v1/saml/metadata/{applicationId}", route.SurfaceProtocol},
		// authentication face: page and session endpoints under /api/v1/auth/*
		{"/api/v1/auth/login", route.SurfaceAuthentication},
		{"/api/v1/auth/signup", route.SurfaceAuthentication},
		{"/api/v1/auth/logout", route.SurfaceAuthentication},
		{"/api/v1/auth/account", route.SurfaceAuthentication},
		{"/api/v1/auth/permissions", route.SurfaceAuthentication},
		{"/api/v1/auth/mfa/push/verify", route.SurfaceAuthentication},
		{"/api/v1/auth/password/reset", route.SurfaceAuthentication},
		{"/api/v1/auth/captcha", route.SurfaceAuthentication},
		{"/api/v1/auth/social/login", route.SurfaceAuthentication},
		{"/api/v1/auth/web3/verify", route.SurfaceAuthentication},
		{"/api/v1/auth/login-challenges/{challengeId}", route.SurfaceAuthentication},
		// management face: everything else under /api/v1/*
		{"/api/v1/users", route.SurfaceManagement},
		{"/api/v1/orders/{id}/pay", route.SurfaceManagement},
		{"/api/v1/ldap-sync-configs/{id}", route.SurfaceManagement},
		{"/api/v1/analytics/web-vitals", route.SurfaceManagement},
		// /api/v1/auth/* subpaths that are not protocol families are
		// authentication (the SPA calls them), never management — 手册 4.1.3
		// defines the management face as /api/v1/* minus the other two.
		{"/api/v1/auth/authorization", route.SurfaceAuthentication},
	}

	for _, c := range cases {
		if got := route.SurfaceFor(c.pattern); got != c.want {
			t.Errorf("SurfaceFor(%q) = %q, want %q", c.pattern, got, c.want)
		}
	}
}
