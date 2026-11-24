package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/guliangiam/guliangiam/internal/infra/postgres"
	"github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
)

func TestTenantFilterMiddleware_NonStrictWithSession(t *testing.T) {
	cfg := TenantFilterConfig{StrictMode: false}

	var capturedCtx context.Context
	handler := TenantFilterMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	ctx := auth.WithSessionContext(req.Context(), auth.SessionContext{
		OrganizationSlug: "acme",
		Username:         "alice",
		AccessToken:      "token-123",
	})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}

	// Verify tenant scope was set in context
	scope := postgres.TenantScopeFromContext(capturedCtx)
	if scope.OrganizationID != "acme" {
		t.Errorf("OrganizationID = %q, want %q", scope.OrganizationID, "acme")
	}
	if scope.Mode != postgres.TenantFilterOptional {
		t.Errorf("Mode = %v, want TenantFilterOptional", scope.Mode)
	}
}

func TestTenantFilterMiddleware_StrictWithSession(t *testing.T) {
	cfg := TenantFilterConfig{StrictMode: true}

	var capturedCtx context.Context
	handler := TenantFilterMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	ctx := auth.WithSessionContext(req.Context(), auth.SessionContext{
		OrganizationSlug: "acme",
		Username:         "alice",
		AccessToken:      "token-123",
	})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}

	scope := postgres.TenantScopeFromContext(capturedCtx)
	if scope.OrganizationID != "acme" {
		t.Errorf("OrganizationID = %q, want %q", scope.OrganizationID, "acme")
	}
	if scope.Mode != postgres.TenantFilterStrict {
		t.Errorf("Mode = %v, want TenantFilterStrict", scope.Mode)
	}
}

func TestTenantFilterMiddleware_StrictNoSession(t *testing.T) {
	cfg := TenantFilterConfig{StrictMode: true}

	handler := TenantFilterMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected %d for strict mode without session, got %d", http.StatusUnauthorized, rec.Code)
	}
}

// 回归：会话里 organizations.id（UUID）与 slug 同时存在时，作用域必须携带 UUID。
// 下游 handler（rebac / idv / records 等）会把作用域值直接写进 uuid 列，
// 传 slug 会让 PostgreSQL 抛 22P02 并以 500 暴露。
func TestTenantFilterMiddleware_PrefersSessionOrganizationUUID(t *testing.T) {
	const orgUUID = "01a0ea76-0c00-7000-8000-000000000001"
	cfg := TenantFilterConfig{StrictMode: false}

	var capturedCtx context.Context
	handler := TenantFilterMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rebac/namespaces", http.NoBody)
	ctx := auth.WithSessionContext(req.Context(), auth.SessionContext{
		OrganizationID:   orgUUID,
		OrganizationSlug: "built-in",
		Username:         "admin",
		AccessToken:      "token-123",
	})
	req = req.WithContext(ctx)

	handler.ServeHTTP(httptest.NewRecorder(), req)

	scope := postgres.TenantScopeFromContext(capturedCtx)
	if scope.OrganizationID != orgUUID {
		t.Errorf("OrganizationID = %q, want %q", scope.OrganizationID, orgUUID)
	}
	if scope.OrganizationName != "built-in" {
		t.Errorf("OrganizationName = %q, want %q", scope.OrganizationName, "built-in")
	}
}

func TestTenantFilterMiddleware_NonStrictNoSession(t *testing.T) {
	cfg := TenantFilterConfig{StrictMode: false}

	handler := TenantFilterMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Non-strict mode should pass through even without session
	if rec.Code != http.StatusOK {
		t.Errorf("expected %d for non-strict mode without session, got %d", http.StatusOK, rec.Code)
	}
}

func TestTenantFilterMiddleware_EmptyOrgSlug(t *testing.T) {
	cfg := TenantFilterConfig{StrictMode: false}

	handler := TenantFilterMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	ctx := auth.WithSessionContext(req.Context(), auth.SessionContext{
		OrganizationSlug: "",
		Username:         "alice",
		AccessToken:      "token-123",
	})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Empty org slug should be treated as no session context
	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rec.Code)
	}
}

// 预期变更（R2 硬化）：context 里没有任何 AdminScope 分类时，
// platform_admin 不再凭 PrincipalType 提升 RLS，而是按 ClassNone 处理。
// 旧断言（GetsSuperAdminMode）编码的是已移除的隐式提权行为。
func TestTenantFilterMiddleware_PlatformAdminNoScopeDoesNotEscalate(t *testing.T) {
	cfg := TenantFilterConfig{StrictMode: false}

	var capturedCtx context.Context
	handler := TenantFilterMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	ctx := auth.WithSessionContext(req.Context(), auth.SessionContext{
		OrganizationSlug: "built-in",
		Username:         "admin",
		PrincipalType:    "platform_admin",
	})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}

	scope := postgres.TenantScopeFromContext(capturedCtx)
	if scope.Mode == postgres.TenantFilterSuperAdmin {
		t.Fatalf("Mode = %v; platform_admin without AdminScope classification must not escalate", scope.Mode)
	}
	if scope.OrganizationID != "built-in" {
		t.Errorf("OrganizationID = %q, want session organization (fail-closed)", scope.OrganizationID)
	}
}

// P1 回归：targeted 管理员无显式 target 时不得提升为 RLS super_admin。
func TestTenantFilterMiddleware_PlatformAdminTargetedScopeDoesNotElevate(t *testing.T) {
	cfg := TenantFilterConfig{StrictMode: false}

	var capturedCtx context.Context
	handler := TenantFilterMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	ctx := auth.WithSessionContext(req.Context(), auth.SessionContext{
		OrganizationID:   "01a0ea76-0c00-7000-8000-000000000001",
		OrganizationSlug: "built-in",
		Username:         "admin",
		PrincipalType:    "platform_admin",
	})
	ctx = scopeauthorizer.WithAdminScope(ctx, scopeauthorizer.AdminScope{
		Class: scopeauthorizer.AdminScopeClassTargeted,
	})
	req = req.WithContext(ctx)

	handler.ServeHTTP(httptest.NewRecorder(), req)

	scope := postgres.TenantScopeFromContext(capturedCtx)
	if scope.Mode == postgres.TenantFilterSuperAdmin {
		t.Fatalf("Mode = %v; targeted admin without target must not elevate to RLS super_admin", scope.Mode)
	}
	if scope.OrganizationID != "01a0ea76-0c00-7000-8000-000000000001" {
		t.Errorf("OrganizationID = %q, want session organization (fail-closed)", scope.OrganizationID)
	}
}

// P1 回归：all 作用域保持现状（RLS super_admin）。
func TestTenantFilterMiddleware_PlatformAdminAllScopeElevates(t *testing.T) {
	cfg := TenantFilterConfig{StrictMode: false}

	var capturedCtx context.Context
	handler := TenantFilterMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	ctx := auth.WithSessionContext(req.Context(), auth.SessionContext{
		OrganizationSlug: "built-in",
		Username:         "admin",
		PrincipalType:    "platform_admin",
	})
	ctx = scopeauthorizer.WithAdminScope(ctx, scopeauthorizer.AdminScope{Class: scopeauthorizer.AdminScopeClassAll})
	req = req.WithContext(ctx)

	handler.ServeHTTP(httptest.NewRecorder(), req)

	scope := postgres.TenantScopeFromContext(capturedCtx)
	if scope.Mode != postgres.TenantFilterSuperAdmin {
		t.Errorf("Mode = %v, want TenantFilterSuperAdmin for all-scope platform_admin", scope.Mode)
	}
}

// P1 回归：作用域解析失败（unknown）也必须 fail-closed。
func TestTenantFilterMiddleware_PlatformAdminUnknownScopeDoesNotElevate(t *testing.T) {
	cfg := TenantFilterConfig{StrictMode: false}

	var capturedCtx context.Context
	handler := TenantFilterMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	ctx := auth.WithSessionContext(req.Context(), auth.SessionContext{
		OrganizationID:   "01a0ea76-0c00-7000-8000-000000000001",
		OrganizationSlug: "built-in",
		Username:         "admin",
		PrincipalType:    "platform_admin",
	})
	ctx = scopeauthorizer.WithAdminScope(ctx, scopeauthorizer.AdminScope{Class: scopeauthorizer.AdminScopeClassUnknown})
	req = req.WithContext(ctx)

	handler.ServeHTTP(httptest.NewRecorder(), req)

	scope := postgres.TenantScopeFromContext(capturedCtx)
	if scope.Mode == postgres.TenantFilterSuperAdmin {
		t.Fatalf("Mode = %v; unknown scope must not elevate", scope.Mode)
	}
	if scope.OrganizationID != "01a0ea76-0c00-7000-8000-000000000001" {
		t.Errorf("OrganizationID = %q, want session organization (fail-closed)", scope.OrganizationID)
	}
}

// none 与 unknown 是同一类 fail-closed，必须同样保留会话本租户。
func TestTenantFilterMiddleware_PlatformAdminNoneScopeDoesNotElevate(t *testing.T) {
	cfg := TenantFilterConfig{StrictMode: false}

	var capturedCtx context.Context
	handler := TenantFilterMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	ctx := auth.WithSessionContext(req.Context(), auth.SessionContext{
		OrganizationID:   "01a0ea76-0c00-7000-8000-000000000001",
		OrganizationSlug: "built-in",
		Username:         "admin",
		PrincipalType:    "platform_admin",
	})
	ctx = scopeauthorizer.WithAdminScope(ctx, scopeauthorizer.AdminScope{Class: scopeauthorizer.AdminScopeClassNone})
	req = req.WithContext(ctx)

	handler.ServeHTTP(httptest.NewRecorder(), req)

	scope := postgres.TenantScopeFromContext(capturedCtx)
	if scope.Mode == postgres.TenantFilterSuperAdmin {
		t.Fatalf("Mode = %v; none scope must not elevate", scope.Mode)
	}
	if scope.OrganizationID != "01a0ea76-0c00-7000-8000-000000000001" {
		t.Errorf("OrganizationID = %q, want session organization (fail-closed)", scope.OrganizationID)
	}
}

func TestTenantFilterMiddleware_RegularUserStaysOptional(t *testing.T) {
	cfg := TenantFilterConfig{StrictMode: false}

	var capturedCtx context.Context
	handler := TenantFilterMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	ctx := auth.WithSessionContext(req.Context(), auth.SessionContext{
		OrganizationSlug: "guliangtech",
		Username:         "wangwu",
		PrincipalType:    "user", // default, empty also means user
	})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	scope := postgres.TenantScopeFromContext(capturedCtx)
	if scope.Mode != postgres.TenantFilterOptional {
		t.Errorf("Mode = %v, want TenantFilterOptional for regular user", scope.Mode)
	}
}

func TestTenantFilterMiddleware_PassThroughWhenScopeAuthorizerSet(t *testing.T) {
	cfg := TenantFilterConfig{StrictMode: false}

	var capturedCtx context.Context
	handler := TenantFilterMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	ctx := auth.WithSessionContext(req.Context(), auth.SessionContext{
		OrganizationSlug: "guliangtech",
		Username:         "lisi",
		PrincipalType:    "user",
	})
	// ScopeAuthorizer already injected TenantFilterSuperAdmin → pass-through
	ctx = postgres.WithTenantScope(ctx, "guliangtech", "guliangtech", postgres.TenantFilterSuperAdmin)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	scope := postgres.TenantScopeFromContext(capturedCtx)
	if scope.Mode != postgres.TenantFilterSuperAdmin {
		t.Errorf("Mode = %v, want TenantFilterSuperAdmin (pass-through, no overwrite)", scope.Mode)
	}
}

func TestTenantFilterMiddleware_PassThroughCrossScope(t *testing.T) {
	nextCalled := false
	handler := TenantFilterMiddleware(TenantFilterConfig{StrictMode: true})(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		nextCalled = true
		if scope := tenantctx.TenantScopeFromContext(r.Context()); scope.Mode != tenantctx.TenantFilterCross || scope.OrganizationID != "target-org" {
			t.Fatalf("scope = %+v, want cross target-org", scope)
		}
	}))
	ctx := postgres.WithTenantScope(context.Background(), "target-org", "", tenantctx.TenantFilterCross)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/users", nil).WithContext(ctx))
	if !nextCalled || rec.Code == http.StatusUnauthorized {
		t.Fatalf("cross scope must pass through without session; called=%t status=%d", nextCalled, rec.Code)
	}
}
