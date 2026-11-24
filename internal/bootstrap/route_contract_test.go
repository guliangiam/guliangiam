package bootstrap

import (
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"

	contractmod58 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/actions"
	contractmod0 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/adapters"
	contractmod1 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/agents"
	contractmodaigateway "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/aigateway"
	contractmod2 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/analytics"
	contractmod3 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/applications"
	contractmod4 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	contractmodb2b "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/b2b"
	contractmod5 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/batch"
	contractmod6 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/captchaproviders"
	contractmod7 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/cas"
	contractmod8 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/certs"
	contractmod9 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/coupons"
	contractmod10 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/dashboard"
	contractmod11 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/dcr"
	contractmod12 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/enforcers"
	contractmod13 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/entries"
	contractmod14 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/faceidproviders"
	contractmod15 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/forms"
	contractmod16 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/gateway"
	contractmod17 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/groups"
	contractmod18 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/i18n"
	contractmod63_identitypools "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/identitypools"
	contractmod65 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/idv"
	contractmod61 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/iga"
	contractmod19 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/invitations"
	contractmod20 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/kerberos"
	contractmod21 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/keys"
	contractmod22 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/ldaps"
	contractmod23 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/ldapsync"
	contractmod24 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/mcpservers"
	contractmod25 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/messages"
	contractmod62 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/metering"
	contractmod26 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/models"
	contractmod27 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/notificationproviders"
	contractmod28 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/notifications"
	contractmod29 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/orders"
	contractmod30 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/organizations"
	contractmod31 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/payments"
	contractmod32 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/permissions"
	contractmod33 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/plans"
	contractmodplatformadmins "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/platformadmins"
	contractmod34 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/pricings"
	contractmodprivacy "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/privacy"
	contractmod35 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/products"
	contractmod36 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/providers"
	contractmod37 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/radius"
	contractmod39 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/radiusaccountings"
	contractmod38 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/radiusconfigs"
	contractmod59 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/rebac"
	contractmod66 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/reconciliation"
	contractmod40 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/records"
	contractmod41 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/resources"
	contractmod60 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/risk"
	contractmod42 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/roles"
	contractmod43 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/rules"
	contractmod44 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/saml"
	contractmod45 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/scim"
	contractmod67 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/search"
	contractmod46 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/sessions"
	contractmod47 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/subscriptions"
	contractmod48 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/syncers"
	contractmod49 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/system"
	contractmod50 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/tickets"
	contractmod51 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/tokens"
	contractmod52 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/transactions"
	contractmodusernotifications "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/usernotifications"
	contractmod53 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/users"
	contractmod54 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/verifications"
	contractmod55 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/webauthn"
	contractmod64 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/webhookevents"
	contractmod56 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/webhooks"
	contractmod57 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/wellknown"
)

// declaredRouteModules lists every api/v1 module that publishes a route contract
// through Routes(). Each entry pairs the module directory name with the function
// returning its declarations.
//
// The contract has two consumers that must agree with the runtime router: the
// OpenAPI generator (scripts/gen_openapi.py) and the gate below.
var declaredRouteModules = []struct {
	name   string
	routes func() []route.Definition
}{
	{name: "adapters", routes: contractmod0.Routes},
	{name: "agents", routes: contractmod1.Routes},
	{name: "analytics", routes: contractmod2.Routes},
	{name: "aigateway", routes: contractmodaigateway.Routes},
	{name: "applications", routes: contractmod3.Routes},
	{name: "auth", routes: contractmod4.Routes},
	{name: "batch", routes: contractmod5.Routes},
	{name: "b2b", routes: contractmodb2b.Routes},
	{name: "captchaproviders", routes: contractmod6.Routes},
	{name: "cas", routes: contractmod7.Routes},
	{name: "certs", routes: func() []route.Definition {
		return append(contractmod8.Routes(), contractmod8.GlobalRoutes()...)
	}},
	{name: "coupons", routes: contractmod9.Routes},
	{name: "dashboard", routes: contractmod10.Routes},
	{name: "dcr", routes: contractmod11.Routes},
	{name: "enforcers", routes: contractmod12.Routes},
	{name: "entries", routes: contractmod13.Routes},
	{name: "faceidproviders", routes: contractmod14.Routes},
	{name: "forms", routes: contractmod15.Routes},
	{name: "gateway", routes: contractmod16.Routes},
	{name: "groups", routes: contractmod17.Routes},
	{name: "i18n", routes: contractmod18.Routes},
	{name: "invitations", routes: contractmod19.Routes},
	{name: "kerberos", routes: contractmod20.Routes},
	{name: "keys", routes: contractmod21.Routes},
	{name: "ldaps", routes: contractmod22.Routes},
	{name: "ldapsync", routes: contractmod23.Routes},
	{name: "mcpservers", routes: contractmod24.Routes},
	{name: "messages", routes: contractmod25.Routes},
	{name: "models", routes: contractmod26.Routes},
	{name: "notificationproviders", routes: contractmod27.Routes},
	{name: "notifications", routes: contractmod28.Routes},
	{name: "orders", routes: contractmod29.Routes},
	{name: "organizations", routes: contractmod30.Routes},
	{name: "payments", routes: contractmod31.Routes},
	{name: "permissions", routes: contractmod32.Routes},
	{name: "plans", routes: contractmod33.Routes},
	{name: "platformadmins", routes: contractmodplatformadmins.Routes},
	{name: "pricings", routes: contractmod34.Routes},
	{name: "privacy", routes: contractmodprivacy.Routes},
	{name: "products", routes: contractmod35.Routes},
	{name: "providers", routes: contractmod36.Routes},
	{name: "radius", routes: contractmod37.Routes},
	{name: "radiusconfigs", routes: contractmod38.Routes},
	{name: "radiusaccountings", routes: contractmod39.Routes},
	{name: "records", routes: contractmod40.Routes},
	{name: "reconciliation", routes: contractmod66.Routes},
	{name: "resources", routes: contractmod41.Routes},
	{name: "roles", routes: contractmod42.Routes},
	{name: "rules", routes: contractmod43.Routes},
	{name: "saml", routes: contractmod44.Routes},
	{name: "scim", routes: contractmod45.Routes},
	{name: "search", routes: contractmod67.Routes},
	{name: "sessions", routes: contractmod46.Routes},
	{name: "subscriptions", routes: contractmod47.Routes},
	{name: "syncers", routes: contractmod48.Routes},
	{name: "system", routes: contractmod49.Routes},
	{name: "tickets", routes: contractmod50.Routes},
	{name: "tokens", routes: contractmod51.Routes},
	{name: "transactions", routes: contractmod52.Routes},
	{name: "users", routes: contractmod53.Routes},
	{name: "verifications", routes: contractmod54.Routes},
	{name: "webauthn", routes: contractmod55.Routes},
	{name: "webhooks", routes: contractmod56.Routes},
	{name: "webhookevents", routes: contractmod64.Routes},
	{name: "wellknown", routes: contractmod57.Routes},
	{name: "actions", routes: contractmod58.Routes},
	{name: "rebac", routes: contractmod59.Routes},
	{name: "risk", routes: contractmod60.Routes},
	{name: "idv", routes: contractmod65.Routes},
	{name: "iga", routes: contractmod61.Routes},
	{name: "metering", routes: contractmod62.Routes},
	{name: "identitypools", routes: contractmod63_identitypools.Routes},
	{name: "usernotifications", routes: contractmodusernotifications.Routes},
}

// infraRouteAllowlist holds routes intentionally registered by the server
// bootstrap rather than by a product API module.
var infraRouteAllowlist = map[string]bool{
	"GET /healthz": true,
	"GET /livez":   true,
	"GET /metrics": true,
	"GET /readyz":  true,
	// pprof 端点（与 /metrics /readyz 同属 ops token-protected 基础设施组）
	"GET /debug/pprof":         true,
	"GET /debug/pprof/":        true,
	"GET /debug/pprof/cmdline": true,
	"GET /debug/pprof/profile": true,
	"GET /debug/pprof/symbol":  true,
	"GET /debug/pprof/trace":   true,
}

// normalizeRouteKey makes declaration and runtime keys comparable:
//   - chi reports the collection route of a sub-router with a trailing slash
//     ("/api/v1/orders/") while declarations omit it, and a trailing slash is
//     not significant for these routes.
func normalizeRouteKey(key string) string {
	for strings.HasSuffix(key, "/") && len(key) > 1 {
		key = strings.TrimSuffix(key, "/")
	}
	return key
}

// TestRouteContractMatchesRuntimeRouter enforces DB-055: every route declared in
// a module's Routes() must be registered on the runtime chi router, and every
// registered route must be declared. The comparison is a bidirectional set diff;
// a non-empty result in either direction fails the test.
func TestRouteContractMatchesRuntimeRouter(t *testing.T) {
	declared := map[string]string{}
	for _, mod := range declaredRouteModules {
		for _, d := range mod.routes() {
			key := normalizeRouteKey(d.Method + " " + d.Pattern)
			if prev, ok := declared[key]; ok {
				t.Errorf("duplicate declaration %q in modules %q and %q", key, prev, mod.name)
			}
			declared[key] = mod.name
		}
	}

	registered := map[string]bool{}
	// Reconciliation is the one management module the router registers
	// conditionally (nil handler means "storage not configured"). The contract
	// still declares its route, so the test must supply a handler or the
	// declaration would be reported as phantom.
	runtime := &Runtime{}
	runtime.Handlers.ReconciliationHandler = &contractmod66.Handler{}
	router := NewChiRouter(runtime)
	if err := chi.Walk(router, func(method string, r string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		key := normalizeRouteKey(method + " " + r)
		if infraRouteAllowlist[key] {
			return nil
		}
		registered[key] = true
		return nil
	}); err != nil {
		t.Fatalf("chi.Walk() error = %v", err)
	}

	var declaredNotRegistered []string
	for key := range declared {
		if !registered[key] {
			declaredNotRegistered = append(declaredNotRegistered, key+"  ["+declared[key]+"]")
		}
	}
	var registeredNotDeclared []string
	for key := range registered {
		if _, ok := declared[key]; !ok {
			registeredNotDeclared = append(registeredNotDeclared, key)
		}
	}
	sort.Strings(declaredNotRegistered)
	sort.Strings(registeredNotDeclared)

	for _, key := range declaredNotRegistered {
		t.Errorf("declared but not registered on the router (phantom OpenAPI path): %s", key)
	}
	for _, key := range registeredNotDeclared {
		t.Errorf("registered but not declared in any Routes() (missing contract): %s", key)
	}
	if len(declared) != len(registered) {
		t.Errorf("route contract size mismatch: declared=%d registered=%d", len(declared), len(registered))
	}
	if len(declaredNotRegistered) == 0 && len(registeredNotDeclared) == 0 {
		t.Logf("route contract holds for %d declared routes (%d registered, %d infra-excluded)",
			len(declared), len(registered), len(infraRouteAllowlist))
	}
}
