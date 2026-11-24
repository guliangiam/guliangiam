package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	platformhttp "github.com/guliangiam/guliangiam/internal/platform/http"
	httpmiddleware "github.com/guliangiam/guliangiam/internal/transport/http/middleware"
)

func TestNewChiRouterRegistersV1APIRoutes(t *testing.T) {
	router := NewChiRouter(&Runtime{})

	wantRoutes := map[string]bool{
		"POST /api/v1/auth/login":                         false,
		"POST /api/v1/auth/authorize":                     false,
		"POST /api/v1/auth/tokens/exchange":               false,
		"POST /api/v1/auth/consents/check":                false,
		"POST /api/v1/auth/consents/grant":                false,
		"POST /api/v1/auth/consents/revoke":               false,
		"GET /api/v1/auth/account":                        false,
		"POST /api/v1/auth/logout":                        false,
		"GET /api/v1/applications":                        false,
		"POST /api/v1/applications":                       false,
		"GET /api/v1/applications/{id}":                   false,
		"PATCH /api/v1/applications/{id}":                 false,
		"GET /api/v1/applications/{id}/provider-bindings": false,
		"PUT /api/v1/applications/{id}/provider-bindings": false,
		"GET /api/v1/organizations":                       false,
		"POST /api/v1/organizations":                      false,
		"GET /api/v1/organizations/{id}":                  false,
		"GET /api/v1/organizations/by-slug/{slug}":        false,
		"PATCH /api/v1/organizations/{id}":                false,
		"PUT /api/v1/organizations/{id}/settings":         false,
		"PUT /api/v1/organizations/{id}/theme":            false,
		"PUT /api/v1/organizations/{id}/security-policy":  false,
		"GET /api/v1/permissions":                         false,
		"POST /api/v1/permissions":                        false,
		"GET /api/v1/permissions/{id}":                    false,
		"PATCH /api/v1/permissions/{id}":                  false,
		"GET /api/v1/providers":                           false,
		"POST /api/v1/providers":                          false,
		"GET /api/v1/providers/{id}":                      false,
		"PATCH /api/v1/providers/{id}":                    false,
		"GET /api/v1/providers/{id}/config":               false,
		"PUT /api/v1/providers/{id}/config":               false,
		"GET /api/v1/providers/{id}/secret-status":        false,
		"GET /api/v1/roles":                               false,
		"POST /api/v1/roles":                              false,
		"GET /api/v1/roles/{id}":                          false,
		"PATCH /api/v1/roles/{id}":                        false,
		"GET /api/v1/sessions/current":                    false,
		"POST /api/v1/sessions/current/revoke-others":     false,
		"GET /api/v1/sessions":                            false,
		"GET /api/v1/sessions/{id}":                       false,
		"DELETE /api/v1/sessions/{id}":                    false,
		"POST /api/v1/sessions/{id}/revoke":               false,
		"GET /api/v1/tokens":                              false,
		"POST /api/v1/tokens":                             false,
		"POST /api/v1/tokens/revoke":                      false,
		"POST /api/v1/tokens/introspect":                  false,
		"POST /api/v1/tokens/refresh":                     false,
		"GET /api/v1/tokens/{id}":                         false,
		"POST /api/v1/tokens/{id}/revoke":                 false,
		"DELETE /api/v1/tokens/{id}":                      false,
		"GET /api/v1/users":                               false,
		"POST /api/v1/users":                              false,
		"GET /api/v1/users/{id}":                          false,
		"PATCH /api/v1/users/{id}":                        false,
		"PUT /api/v1/users/{id}/profile":                  false,
		"PUT /api/v1/users/{id}/role-bindings":            false,
		"PUT /api/v1/users/{id}/group-memberships":        false,
		"PUT /api/v1/users/{id}/credentials":              false,
		"PUT /api/v1/users/{id}/identities":               false,
		"PUT /api/v1/users/{id}/mfa-methods":              false,
		"PUT /api/v1/users/{id}/preferred-mfa":            false,
		"POST /api/v1/users/{id}/mfa-setup/initiate":      false,
		"POST /api/v1/users/{id}/mfa-setup/verify":        false,
		"POST /api/v1/users/{id}/mfa-setup/enable":        false,
	}

	if err := chi.Walk(router, func(method, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		key := method + " " + route
		if _, ok := wantRoutes[key]; ok {
			wantRoutes[key] = true
		}
		return nil
	}); err != nil {
		t.Fatalf("chi.Walk() error = %v", err)
	}

	for routeKey, found := range wantRoutes {
		if !found {
			t.Fatalf("route %q not registered", routeKey)
		}
	}
}

func TestNewChiRouterPreservesSocketPeerForClientIP(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "")
	router := NewChiRouter(&Runtime{Middleware: MiddlewareConfig{
		WAFMiddleware:    &httpmiddleware.WAFMiddleware{},
		ServiceIPLimiter: &httpmiddleware.ServiceIPLimiter{},
	}})

	var remoteAddr, clientIP string
	router.Get("/_test/client-ip", func(w http.ResponseWriter, r *http.Request) {
		remoteAddr = r.RemoteAddr
		clientIP = platformhttp.ClientIP(r)
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/_test/client-ip", http.NoBody)
	req.RemoteAddr = "198.51.100.10:1234"
	req.Header.Set("X-Real-IP", "203.0.113.20")
	req.Header.Set("X-Forwarded-For", "203.0.113.21")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if remoteAddr != "198.51.100.10:1234" {
		t.Fatalf("RemoteAddr = %q, middleware rewrote the socket peer", remoteAddr)
	}
	if clientIP != "198.51.100.10" {
		t.Fatalf("ClientIP = %q, want untrusted socket peer", clientIP)
	}
}
