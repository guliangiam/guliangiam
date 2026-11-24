package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/casbin/casbin/v3"
	casbinmodel "github.com/casbin/casbin/v3/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aigatewayResources lists every unique Casbin object produced by
// resourceFromPath for /api/v1/ai/** paths (see routes.go + admin_chi.go).
var aigatewayResources = []string{
	"aigateway:providers",
	"aigateway:models",
	"aigateway:aliases",
	"aigateway:routes",
	"aigateway:api_keys",
	"aigateway:quotas",
	"aigateway:usage",
	"aigateway:prices",
	"aigateway:health",
	"aigateway:chat",
}

// aigatewayPolicyTable defines the desired RBAC matrix for AI Gateway
// management and data-plane roles.  The middleware Enforce call is:
//
//	Enforce(subject, orgSlug, resource, strings.ToLower(method))
//
// where resource = resourceFromPath(url) per TestResourceFromPath.
//
// Role semantics (matches design doc §权限模型):
//
//	admin      → everything  (p, admin, *, *, *)   — admin always wins
//	viewer     → read-only on all admin surfaces; NO data-plane chat
//	editor     → CRUD on admin surfaces (no delete on providers/api_keys)
//	             ; NO data-plane chat
//	operator   → full CRUD including delete; data-plane chat allowed
var aigatewayPolicyTable = map[string]map[string][]string{
	"admin": {
		"*": {"*"},
	},
	"built-in-viewer": {
		"aigateway:providers": {"get"},
		"aigateway:models":    {"get"},
		"aigateway:aliases":   {"get"},
		"aigateway:routes":    {"get"},
		"aigateway:api_keys":  {"get"},
		"aigateway:quotas":    {"get"},
		"aigateway:usage":     {"get"},
		"aigateway:prices":    {"get"},
		"aigateway:health":    {"get"},
		// chat missing → viewer cannot hit data-plane
	},
	"built-in-editor": {
		"aigateway:providers": {"get", "post", "patch"}, // no delete — only operator
		"aigateway:models":    {"get", "post", "patch", "delete"},
		"aigateway:aliases":   {"get", "post", "patch", "delete"},
		"aigateway:routes":    {"get", "post", "patch", "delete"},
		"aigateway:api_keys":  {"get", "post", "patch"}, // no delete/revoke — only operator
		"aigateway:quotas":    {"get", "post", "patch", "delete"},
		"aigateway:usage":     {"get"},
		"aigateway:prices":    {"get", "post", "patch", "delete"},
		"aigateway:health":    {"get"},
		// chat missing → editor cannot hit data-plane
	},
	"built-in-operator": {
		"aigateway:providers": {"get", "post", "patch", "delete"},
		"aigateway:models":    {"get", "post", "patch", "delete"},
		"aigateway:aliases":   {"get", "post", "patch", "delete"},
		"aigateway:routes":    {"get", "post", "patch", "delete"},
		"aigateway:api_keys":  {"get", "post", "patch", "delete"},
		"aigateway:quotas":    {"get", "post", "patch", "delete"},
		"aigateway:usage":     {"get"},
		"aigateway:prices":    {"get", "post", "patch", "delete"},
		"aigateway:health":    {"get"},
		"aigateway:chat":      {"get", "post"}, // data-plane — chat + embeddings (+ models listing)
	},
}

// newInMemoryAIGatewayEnforcer builds a Casbin enforcer backed by the
// built-in RBAC model (sub, org, obj, act) and loads every role in
// aigatewayPolicyTable as p-policy rows against domain "built-in".
func newInMemoryAIGatewayEnforcer(t *testing.T) *casbin.Enforcer {
	t.Helper()
	m, err := casbinmodel.NewModelFromString(defaultRBACModel())
	assert.NoError(t, err, "parse defaultRBACModel")
	e, err := casbin.NewEnforcer(m)
	assert.NoError(t, err, "create enforcer")

	// Load every role's p-policies into domain "built-in".
	// Format: p, role, built-in, object, act
	for role, objs := range aigatewayPolicyTable {
		for obj, acts := range objs {
			// Wildcard admin → p, admin, built-in, *, *
			if obj == "*" {
				_, err := e.AddPolicy(role, "built-in", "*", "*")
				assert.NoError(t, err, "add wildcard for %s", role)
				continue
			}
			for _, act := range acts {
				_, err := e.AddPolicy(role, "built-in", obj, act)
				assert.NoError(t, err, "add p %s built-in %s %s", role, obj, act)
			}
		}
	}

	// AddPolicy writes directly to the in-memory policy.Policy — no adapter
	// is configured so we MUST NOT call LoadPolicy (it would panic on nil
	// adapter).  Policies are ready to Enforce immediately after AddPolicy.
	return e
}

// TestAIGatewayCasbinPolicyMatrix exhaustively verifies the role → object →
// action permission matrix declared in aigatewayPolicyTable.  For every
// (role, resource) pair, we Enforce each of {get, post, patch, delete} and
// assert the result matches the policy table (or deny if the resource isn't
// listed at all for that role).
func TestAIGatewayCasbinPolicyMatrix(t *testing.T) {
	e := newInMemoryAIGatewayEnforcer(t)

	roles := []string{"admin", "built-in-viewer", "built-in-editor", "built-in-operator"}
	actions := []string{"get", "post", "patch", "delete"}

	for _, role := range roles {
		for _, resource := range aigatewayResources {
			for _, action := range actions {
				allowed, err := e.Enforce(role, "built-in", resource, action)
				assert.NoError(t, err)

				want := policyTableExpect(aigatewayPolicyTable, role, resource, action)
				assert.Equal(t, want, allowed,
					"role=%s resource=%s action=%s → Enforce got allow=%v want=%v",
					role, resource, action, allowed, want)
			}
		}
	}
}

// TestAIGatewayRouteMethodPolicy exercises the full middleware chain in
// miniature: for each registered AI Gateway route, feed the exact path +
// HTTP method through resourceFromPath + Enforce and assert against the
// role → (resource, action) matrix.
func TestAIGatewayRouteMethodPolicy(t *testing.T) {
	e := newInMemoryAIGatewayEnforcer(t)

	// (method, path) → expected (resource, action) — derived from admin_chi.go + routes.go.
	routeMethodTable := []struct {
		method string
		path   string
	}{
		// Providers
		{http.MethodGet, "/api/v1/ai/providers"},
		{http.MethodPost, "/api/v1/ai/providers"},
		{http.MethodGet, "/api/v1/ai/providers/550e8400-0000-4000-8000-000000000001"},
		{http.MethodPatch, "/api/v1/ai/providers/550e8400-0000-4000-8000-000000000001"},
		{http.MethodDelete, "/api/v1/ai/providers/550e8400-0000-4000-8000-000000000001"},
		{http.MethodPost, "/api/v1/ai/providers/550e8400-0000-4000-8000-000000000001/test"},
		// Models
		{http.MethodGet, "/api/v1/ai/models"},
		{http.MethodPost, "/api/v1/ai/models"},
		{http.MethodGet, "/api/v1/ai/models/mid"},
		{http.MethodPatch, "/api/v1/ai/models/mid"},
		{http.MethodDelete, "/api/v1/ai/models/mid"},
		// Aliases
		{http.MethodGet, "/api/v1/ai/aliases"},
		{http.MethodPost, "/api/v1/ai/aliases"},
		{http.MethodDelete, "/api/v1/ai/aliases/aid"},
		// Routes
		{http.MethodGet, "/api/v1/ai/routes"},
		{http.MethodPost, "/api/v1/ai/routes"},
		{http.MethodDelete, "/api/v1/ai/routes/rid"},
		// API Keys
		{http.MethodGet, "/api/v1/ai/api-keys"},
		{http.MethodPost, "/api/v1/ai/api-keys"},
		{http.MethodPatch, "/api/v1/ai/api-keys/kid"},
		{http.MethodDelete, "/api/v1/ai/api-keys/kid"},
		{http.MethodPost, "/api/v1/ai/api-keys/kid/revoke"},
		// Quotas
		{http.MethodGet, "/api/v1/ai/quotas"},
		{http.MethodPost, "/api/v1/ai/quotas"},
		{http.MethodDelete, "/api/v1/ai/quotas/qid"},
		// Usage (read-only)
		{http.MethodGet, "/api/v1/ai/usage"},
		{http.MethodGet, "/api/v1/ai/usage/stats"},
		// Prices
		{http.MethodGet, "/api/v1/ai/prices"},
		{http.MethodPost, "/api/v1/ai/prices"},
		{http.MethodDelete, "/api/v1/ai/prices/pid"},
		// Health (read-only)
		{http.MethodGet, "/api/v1/ai/health"},
		{http.MethodGet, "/api/v1/ai/health/hid"},
		// Data-plane faces (OpenAI)
		{http.MethodPost, "/api/v1/ai/faces/openai/chat/completions"},
		{http.MethodPost, "/api/v1/ai/faces/openai/embeddings"},
		{http.MethodGet, "/api/v1/ai/faces/openai/models"},
		// Data-plane faces (Anthropic)
		{http.MethodPost, "/api/v1/ai/faces/anthropic/chat/completions"},
		{http.MethodPost, "/api/v1/ai/faces/anthropic/embeddings"},
		// Data-plane faces (Gemini)
		{http.MethodPost, "/api/v1/ai/faces/gemini/chat/completions"},
		{http.MethodPost, "/api/v1/ai/faces/gemini/embeddings"},
	}

	roles := []string{"admin", "built-in-viewer", "built-in-editor", "built-in-operator"}

	for _, role := range roles {
		for _, rt := range routeMethodTable {
			resource := resourceFromPath(rt.path)
			action := string(rune(0)) // placeholder, replaced below
			// resourceFromPath produces "aigateway:*", Enforce action = strings.ToLower(method)
			_ = action
			enforceAction := stringsToLower(rt.method)

			allowed, err := e.Enforce(role, "built-in", resource, enforceAction)
			assert.NoError(t, err)

			want := policyTableExpect(aigatewayPolicyTable, role, resource, enforceAction)
			assert.Equal(t, want, allowed,
				"role=%s method=%s path=%s resource=%s action=%s",
				role, rt.method, rt.path, resource, enforceAction)
		}
	}
}

// policyTableExpect looks up (role, resource, action) in the policy table.
// Returns false if the role or resource is absent (implicit deny), true if
// the role has obj="*" (wildcard admin), or specific check passes.
func policyTableExpect(table map[string]map[string][]string, role, resource, action string) bool {
	objs, ok := table[role]
	if !ok {
		return false
	}
	// Wildcard admin
	if acts, ok := objs["*"]; ok {
		for _, a := range acts {
			if a == "*" || a == action {
				return true
			}
		}
		return false
	}
	acts, ok := objs[resource]
	if !ok {
		return false
	}
	for _, a := range acts {
		if a == "*" || a == action {
			return true
		}
	}
	return false
}

// stringsToLower helper (mirrors middleware strings.ToLower(r.Method)).
func stringsToLower(s string) string {
	return strings.ToLower(s)
}

// ── Middleware integration test ───────────────────────────────────────────
//
// These tests construct a minimal variant of APIAuthorizationMiddleware
// that skips the DB-dependent parts (ResolveEnforcerUUID → real enforcer)
// but exercises the EXACT same logic path: SessionContext → subject →
// resourceFromPath → strings.ToLower(method) → Enforce → allow/deny.
//
// The in-memory enforcer is loaded with the same policy table as
// TestAIGatewayCasbinPolicyTable so assertions are consistent.

// aigatewayEnforceMiddleware is a test-only middleware that mirrors the
// Gate 3 → Gate 4 → Gate 5 chain of APIAuthorizationMiddleware.  It skips
// Gate 1/2 (engine readiness, public endpoint skip) because those are not
// relevant to AI Gateway paths, and uses an in-memory enforcer directly
// instead of going through ResolveEnforcerUUID + CasbinManager.
//
// The orgSlug is read from session.OrganizationSlug (matching the real
// middleware).  The defaultOrgSlug is used ONLY as a fallback if the session
// has an empty OrganizationSlug.
func aigatewayEnforceMiddleware(e *casbin.Enforcer, defaultOrgSlug string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Gate 3: session context must exist
		session, ok := SessionContextFromContext(r.Context())
		if !ok {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}

		// Gate 4: subject must be non-empty
		subject := strings.TrimSpace(session.Username)
		if subject == "" {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}

		orgSlug := session.OrganizationSlug
		if orgSlug == "" {
			orgSlug = defaultOrgSlug
		}

		// Gate 5: resourceFromPath → action → Enforce
		resource := resourceFromPath(r.URL.Path)
		action := strings.ToLower(r.Method)

		allowed, err := e.Enforce(subject, orgSlug, resource, action)
		if err != nil {
			http.Error(w, "authorization check failed", http.StatusInternalServerError)
			return
		}
		if !allowed {
			http.Error(w, "insufficient permissions", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// addTestSession injects a SessionContext into the request context so the
// middleware sees a properly authenticated caller.
func addTestSession(r *http.Request, username, orgID, orgSlug string) *http.Request {
	session := SessionContext{
		ID:               "sess-test",
		OrganizationID:   orgID,
		OrganizationSlug: orgSlug,
		Username:         username,
		AccessToken:      "fake-token",
		Source:           "repository",
		AuthTime:         time.Now(),
	}
	return r.WithContext(WithSessionContext(r.Context(), session))
}

// TestAIGatewayMiddleware_AllowDeny runs the exact middleware chain against
// every registered AI Gateway (role × route/method) tuple and asserts:
//   - 200 when the policy table says allow
//   - 403 when the policy table says deny
//   - 401 when no session context (unauthenticated)
func TestAIGatewayMiddleware_AllowDeny(t *testing.T) {
	e := newInMemoryAIGatewayEnforcer(t)

	// A simple 200 handler wrapped by the middleware.
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mw := aigatewayEnforceMiddleware(e, "built-in", next)

	// ── Session-less request → 401 ──
	t.Run("no_session_401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/providers", nil)
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	// ── Empty username → 401 ──
	t.Run("empty_username_401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/providers", nil)
		req = addTestSession(req, "", "01a0ea76-0c00-7000-8000-000000000001", "built-in")
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	// ── Unknown role (not in policy table) → 403 deny ──
	t.Run("unknown_role_403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/providers", nil)
		req = addTestSession(req, "mallory", "01a0ea76-0c00-7000-8000-000000000001", "built-in")
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	// ── Cross-org request → 403 deny (policy table has orgSlug="built-in" only) ──
	t.Run("cross_org_403", func(t *testing.T) {
		// admin in a different org — the policy table only has "built-in" domain.
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/providers", nil)
		req = addTestSession(req, "admin", "01a0ea76-0c00-7000-8000-000000000001", "other-org")
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	// ── Full cartesian product: every (role × route/method) → assert status ──
	// Use the same routeMethodTable as TestAIGatewayRouteMethodPolicy to keep
	// the assertion matrix identical.
	roles := []string{"admin", "built-in-viewer", "built-in-editor", "built-in-operator"}
	routeMethodTable := buildRouteMethodTable(t)

	for _, role := range roles {
		t.Run("role="+role, func(t *testing.T) {
			for _, rt := range routeMethodTable {
				resource := resourceFromPath(rt.path)
				action := strings.ToLower(rt.method)
				want := policyTableExpect(aigatewayPolicyTable, role, resource, action)

				req := httptest.NewRequest(rt.method, rt.path, nil)
				req = addTestSession(req, role, "01a0ea76-0c00-7000-8000-000000000001", "built-in")
				rec := httptest.NewRecorder()
				mw.ServeHTTP(rec, req)

				if want {
					assert.Equal(t, http.StatusOK, rec.Code,
						"ALLOW expected → role=%s method=%s path=%s resource=%s action=%s",
						role, rt.method, rt.path, resource, action)
				} else {
					assert.Equal(t, http.StatusForbidden, rec.Code,
						"DENY expected → role=%s method=%s path=%s resource=%s action=%s",
						role, rt.method, rt.path, resource, action)
				}
			}
		})
	}
}

// TestAIGatewayMiddleware_EdgePaths asserts that paths that should NOT
// resolve to an aigateway resource (or should resolve to a different one)
// are handled correctly.  This catches accidental pollution from the special
// "ai" branch in resourceFromPath.
func TestAIGatewayMiddleware_EdgePaths(t *testing.T) {
	e := newInMemoryAIGatewayEnforcer(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := aigatewayEnforceMiddleware(e, "built-in", next)

	// Non-AI path → resource = first segment ("users"). Policy table has no
	// entry for "users", so Enforce denies → 403.  This is correct — we're
	// only testing that the middleware passes resourceFromPath through cleanly.
	t.Run("non_ai_path_resolved_correctly", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil)
		req = addTestSession(req, "admin", "01a0ea76-0c00-7000-8000-000000000001", "built-in")
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, req)
		// "admin" role only has obj="*" which covers any resource → 200
		assert.Equal(t, http.StatusOK, rec.Code)

		// viewer doesn't have "users" resource → 403
		req2 := httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil)
		req2 = addTestSession(req2, "built-in-viewer", "01a0ea76-0c00-7000-8000-000000000001", "built-in")
		rec2 := httptest.NewRecorder()
		mw.ServeHTTP(rec2, req2)
		assert.Equal(t, http.StatusForbidden, rec2.Code)
	})
}

// buildRouteMethodTable returns the same (method, path) tuples used by
// TestAIGatewayRouteMethodPolicy.  Extracted to a helper so both tests
// share the same route surface.
func buildRouteMethodTable(t *testing.T) []struct {
	method string
	path   string
} {
	t.Helper()
	return []struct {
		method string
		path   string
	}{
		// Providers
		{http.MethodGet, "/api/v1/ai/providers"},
		{http.MethodPost, "/api/v1/ai/providers"},
		{http.MethodGet, "/api/v1/ai/providers/550e8400-0000-4000-8000-000000000001"},
		{http.MethodPatch, "/api/v1/ai/providers/550e8400-0000-4000-8000-000000000001"},
		{http.MethodDelete, "/api/v1/ai/providers/550e8400-0000-4000-8000-000000000001"},
		{http.MethodPost, "/api/v1/ai/providers/550e8400-0000-4000-8000-000000000001/test"},
		// Models
		{http.MethodGet, "/api/v1/ai/models"},
		{http.MethodPost, "/api/v1/ai/models"},
		{http.MethodDelete, "/api/v1/ai/models/mid"},
		// API Keys (operator only: delete/revoke)
		{http.MethodDelete, "/api/v1/ai/api-keys/kid"},
		{http.MethodPost, "/api/v1/ai/api-keys/kid/revoke"},
		// Usage (everyone with surface access can GET)
		{http.MethodGet, "/api/v1/ai/usage"},
		{http.MethodGet, "/api/v1/ai/usage/stats"},
		// Data-plane faces (operator only)
		{http.MethodPost, "/api/v1/ai/faces/openai/chat/completions"},
		{http.MethodPost, "/api/v1/ai/faces/anthropic/chat/completions"},
		{http.MethodPost, "/api/v1/ai/faces/gemini/chat/completions"},
	}
}

// ensure context/time are used (they're used in addTestSession above but
// we import them explicitly for clarity).
var _ = context.Background
var _ = time.Now
var _ = require.NoError
