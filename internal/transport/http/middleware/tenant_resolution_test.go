package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	apppkg "github.com/guliangiam/guliangiam/internal/app/organizations"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	"github.com/guliangiam/guliangiam/internal/infra/postgres"
	"github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
	tenantctx "github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	orghandler "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/organizations"
)

func TestParseTenantFromBodyPreservesLargeBodyWhenScanTruncated(t *testing.T) {
	tenantID := uuid.NewString()
	payload := `{"tenantId":"` + tenantID + `","padding":"` + strings.Repeat("x", 1<<17) + `"}`
	r := &http.Request{Body: io.NopCloser(strings.NewReader(payload))}

	// 64KB 扫描前缀不是完整 JSON，保持既有“无法从本请求解析租户”的结果；
	// 关键是后续 handler 仍能读到完整原始 Body，而不是被截断的前缀。
	if tenantStringFromBody(r) != "" {
		t.Fatal("truncated JSON prefix must not be treated as a complete document")
	}

	restored, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read restored body: %v", err)
	}
	if string(restored) != payload {
		t.Fatalf("restored body changed: got %d bytes, want %d", len(restored), len(payload))
	}
}

func TestParseTenantFromBodyPreservesSmallBody(t *testing.T) {
	tenantID := uuid.NewString()
	payload := `{"organizationId":"` + tenantID + `"}`
	r := &http.Request{Body: io.NopCloser(strings.NewReader(payload))}

	if tenantStringFromBody(r) != tenantID {
		t.Fatal("expected tenant to be resolved")
	}
	restored, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != payload {
		t.Fatalf("restored = %q, want %q", restored, payload)
	}
}

func sessionRequest(method, target, body, principal string) *http.Request {
	return sessionRequestWithOrg(method, target, body, principal, uuid.NewString(), "acme")
}

func sessionRequestWithOrg(method, target, body, principal, orgID, orgSlug string) *http.Request {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, target, reader)
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	ctx := auth.WithSessionContext(r.Context(), auth.SessionContext{
		UserID:           uuid.NewString(),
		OrganizationID:   orgID,
		OrganizationSlug: orgSlug,
		Username:         "alice",
		PrincipalType:    principal,
	})
	return r.WithContext(ctx)
}

type fakeSlugResolver struct {
	ids   map[string]uuid.UUID
	calls []string
	err   error
}

func (r *fakeSlugResolver) Resolve(_ context.Context, slug string) (uuid.UUID, error) {
	r.calls = append(r.calls, slug)
	if r.err != nil {
		return uuid.Nil, r.err
	}
	if id, ok := r.ids[slug]; ok {
		return id, nil
	}
	return uuid.Nil, errors.New("organization not found")
}

type recordingScopeAuthorizer struct {
	allowed map[uuid.UUID]bool
	targets []uuid.UUID
}

func (a *recordingScopeAuthorizer) Authorize(_ context.Context, req scopeauthorizer.AuthorizationRequest) (scopeauthorizer.AuthorizationResult, error) {
	a.targets = append(a.targets, req.TargetTenant)
	if a.allowed[req.TargetTenant] {
		return scopeauthorizer.AuthorizationResult{Allowed: true, Reason: "scope covers target"}, nil
	}
	return scopeauthorizer.AuthorizationResult{Allowed: false, Reason: "scope does not cover target"}, nil
}

func scopeChain(resolver OrganizationSlugResolver, authorizer scopeauthorizer.ScopeAuthorizer, next http.HandlerFunc) http.Handler {
	return TenantResolutionMiddlewareWithResolver(resolver)(ScopeAuthorizerMiddleware(authorizer)(next))
}

func TestTenantResolutionMiddleware_RejectsForeignOrganizationSlug(t *testing.T) {
	handler := TenantResolutionMiddleware()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, sessionRequest(http.MethodGet, "/api/v1/users?organizationId=globex", "", "user"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestTenantResolutionMiddleware_AllowsOwnOrganizationSlug(t *testing.T) {
	called := false
	handler := TenantResolutionMiddleware()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		called = true
		if _, ok := TargetTenantFromContext(r.Context()); ok {
			t.Fatal("own organization slug must not be treated as a cross-tenant target")
		}
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, sessionRequest(http.MethodGet, "/api/v1/users?organizationId=acme", "", "user"))
	if !called {
		t.Fatalf("next handler not called; status=%d", rec.Code)
	}
}

func TestTenantResolutionMiddleware_PlatformAdminForeignSlugWithoutResolverFailsClosed(t *testing.T) {
	called := false
	handler := TenantResolutionMiddleware()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, sessionRequest(http.MethodGet, "/api/v1/users?organizationId=globex", "", "platform_admin"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if called {
		t.Fatal("business handler must not be entered for an unresolved foreign slug")
	}
}

func TestTenantResolutionMiddleware_PlatformAdminForeignUUIDDeniedByScope(t *testing.T) {
	ownOrg := uuid.NewString()
	foreign := uuid.NewString()
	authorizer := &recordingScopeAuthorizer{}
	called := false
	handler := scopeChain(nil, authorizer, func(http.ResponseWriter, *http.Request) { called = true })
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, sessionRequestWithOrg(
		http.MethodGet, "/api/v1/users?organizationId="+foreign, "", "platform_admin", ownOrg, "acme"))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if called {
		t.Fatal("business handler must not be entered when the scope authorizer denies")
	}
	if len(authorizer.targets) != 1 || authorizer.targets[0].String() != foreign {
		t.Fatalf("scope targets = %v, want [%s]", authorizer.targets, foreign)
	}
}

func TestTenantResolutionMiddleware_RegularUserForeignUUIDDeniedByScope(t *testing.T) {
	foreign := uuid.NewString()
	authorizer := &recordingScopeAuthorizer{}
	called := false
	handler := scopeChain(nil, authorizer, func(http.ResponseWriter, *http.Request) { called = true })
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, sessionRequest(http.MethodGet, "/api/v1/users?organizationId="+foreign, "", "user"))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if called {
		t.Fatal("regular tenant must not enter the handler for a foreign UUID")
	}
	if len(authorizer.targets) != 1 || authorizer.targets[0].String() != foreign {
		t.Fatalf("scope targets = %v, want [%s]", authorizer.targets, foreign)
	}
}

// P2 硬化回归：ScopeAuthorizer 未装配时必须 fail-closed。
// 显式跨租户 target 依赖缺失时放行等于绕过 TenantFilter/RLS 裁决，
// 因此应与 PEP 引擎未初始化同口径返回 503，而不是调用业务 handler。
func TestScopeAuthorizerMiddleware_NilAuthorizerFailsClosed(t *testing.T) {
	foreign := uuid.NewString()
	called := false
	handler := scopeChain(nil, nil, func(http.ResponseWriter, *http.Request) { called = true })
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, sessionRequestWithOrg(
		http.MethodGet, "/api/v1/users?organizationId="+foreign, "", "platform_admin", uuid.NewString(), "acme"))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if called {
		t.Fatal("business handler must not be entered when the scope authorizer is not configured")
	}
}

func TestTenantResolutionMiddleware_PlatformAdminForeignSlugResolvedAndDenied(t *testing.T) {
	for _, tt := range []struct {
		name   string
		method string
		target string
		body   string
		header string
	}{
		{name: "query", method: http.MethodGet, target: "/api/v1/users?organizationId=globex"},
		{name: "body", method: http.MethodPost, target: "/api/v1/users", body: `{"organizationId":"globex"}`},
		{name: "header", method: http.MethodGet, target: "/api/v1/users", header: "globex"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ownOrg := uuid.NewString()
			foreignOrg := uuid.New()
			resolver := &fakeSlugResolver{ids: map[string]uuid.UUID{"globex": foreignOrg}}
			authorizer := &recordingScopeAuthorizer{}
			called := false
			handler := scopeChain(resolver.Resolve, authorizer, func(http.ResponseWriter, *http.Request) { called = true })

			req := sessionRequestWithOrg(tt.method, tt.target, tt.body, "platform_admin", ownOrg, "acme")
			if tt.header != "" {
				req.Header.Set("X-Target-Organization-Id", tt.header)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
			}
			if called {
				t.Fatal("business handler must not be entered for a denied foreign slug")
			}
			if len(resolver.calls) != 1 || resolver.calls[0] != "globex" {
				t.Fatalf("resolver calls = %v, want [globex]", resolver.calls)
			}
			if len(authorizer.targets) != 1 || authorizer.targets[0] != foreignOrg {
				t.Fatalf("scope targets = %v, want [%s]", authorizer.targets, foreignOrg)
			}
		})
	}
}

func TestTenantResolutionMiddleware_PlatformAdminForeignSlugAllowedByScope(t *testing.T) {
	ownOrg := uuid.NewString()
	foreignOrg := uuid.New()
	resolver := &fakeSlugResolver{ids: map[string]uuid.UUID{"globex": foreignOrg}}
	authorizer := &recordingScopeAuthorizer{allowed: map[uuid.UUID]bool{foreignOrg: true}}

	var got uuid.UUID
	handler := scopeChain(resolver.Resolve, authorizer, func(_ http.ResponseWriter, r *http.Request) {
		got, _ = TargetTenantFromContext(r.Context())
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, sessionRequestWithOrg(
		http.MethodGet, "/api/v1/users?organizationId=globex", "", "platform_admin", ownOrg, "acme"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got.String() != foreignOrg.String() {
		t.Fatalf("target tenant = %q, want %q", got, foreignOrg)
	}
	if len(resolver.calls) != 1 || resolver.calls[0] != "globex" {
		t.Fatalf("resolver calls = %v, want [globex]", resolver.calls)
	}
	if len(authorizer.targets) != 1 || authorizer.targets[0] != foreignOrg {
		t.Fatalf("scope targets = %v, want [%s]", authorizer.targets, foreignOrg)
	}
}

func TestTenantResolutionMiddleware_PlatformAdminOwnSlugDoesNotResolve(t *testing.T) {
	ownOrg := uuid.New()
	resolver := &fakeSlugResolver{ids: map[string]uuid.UUID{"acme": ownOrg}}
	authorizer := &recordingScopeAuthorizer{}
	called := false
	handler := scopeChain(resolver.Resolve, authorizer, func(_ http.ResponseWriter, r *http.Request) {
		called = true
		if _, ok := TargetTenantFromContext(r.Context()); ok {
			t.Fatal("own organization slug must not be treated as a cross-tenant target")
		}
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, sessionRequestWithOrg(
		http.MethodGet, "/api/v1/users?organizationId=acme", "", "platform_admin", ownOrg.String(), "acme"))

	if !called {
		t.Fatalf("next handler not called; status=%d", rec.Code)
	}
	if len(resolver.calls) != 0 {
		t.Fatalf("own slug must not hit the resolver, calls = %v", resolver.calls)
	}
	if len(authorizer.targets) != 0 {
		t.Fatalf("own slug must not reach the scope authorizer, targets = %v", authorizer.targets)
	}
}

func TestTenantResolutionMiddleware_PlatformAdminOwnUUIDIsNotTarget(t *testing.T) {
	ownOrg := uuid.NewString()
	authorizer := &recordingScopeAuthorizer{}
	called := false
	handler := scopeChain(nil, authorizer, func(_ http.ResponseWriter, r *http.Request) {
		called = true
		if _, ok := TargetTenantFromContext(r.Context()); ok {
			t.Fatal("own organization UUID must not be treated as a cross-tenant target")
		}
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, sessionRequestWithOrg(
		http.MethodGet, "/api/v1/users?organizationId="+ownOrg, "", "platform_admin", ownOrg, "acme"))

	if !called {
		t.Fatalf("next handler not called; status=%d", rec.Code)
	}
	if len(authorizer.targets) != 0 {
		t.Fatalf("own UUID must not reach the scope authorizer, targets = %v", authorizer.targets)
	}
}

func TestTenantResolutionMiddleware_PlatformAdminUnknownSlugFailsClosed(t *testing.T) {
	resolver := &fakeSlugResolver{ids: map[string]uuid.UUID{}}
	authorizer := &recordingScopeAuthorizer{}
	called := false
	handler := scopeChain(resolver.Resolve, authorizer, func(http.ResponseWriter, *http.Request) { called = true })
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, sessionRequest(http.MethodGet, "/api/v1/users?organizationId=globex", "", "platform_admin"))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if called {
		t.Fatal("business handler must not be entered when slug resolution fails")
	}
	if len(authorizer.targets) != 0 {
		t.Fatalf("scope authorizer must not run for an unresolved slug; targets = %v", authorizer.targets)
	}
}

func TestTenantResolutionMiddleware_PlatformAdminPathSlugResolvedAndDenied(t *testing.T) {
	ownOrg := uuid.NewString()
	foreignOrg := uuid.New()
	resolver := &fakeSlugResolver{ids: map[string]uuid.UUID{"globex": foreignOrg}}
	authorizer := &recordingScopeAuthorizer{}
	called := false

	router := chi.NewRouter()
	protected := router.Group(nil)
	protected.Use(TenantResolutionMiddlewareWithResolver(resolver.Resolve))
	protected.Use(ScopeAuthorizerMiddleware(authorizer))
	protected.Get("/api/v1/organizations/{orgId}/domains", func(http.ResponseWriter, *http.Request) { called = true })

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, sessionRequestWithOrg(
		http.MethodGet, "/api/v1/organizations/globex/domains", "", "platform_admin", ownOrg, "acme"))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if called {
		t.Fatal("business handler must not be entered for a denied path slug")
	}
	if len(resolver.calls) != 1 || resolver.calls[0] != "globex" {
		t.Fatalf("resolver calls = %v, want [globex]", resolver.calls)
	}
	if len(authorizer.targets) != 1 || authorizer.targets[0] != foreignOrg {
		t.Fatalf("scope targets = %v, want [%s]", authorizer.targets, foreignOrg)
	}
}

func TestTenantResolutionMiddleware_QueryOrganizationUUIDBecomesTarget(t *testing.T) {
	foreign := uuid.NewString()
	var got uuid.UUID
	handler := TenantResolutionMiddleware()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got, _ = TargetTenantFromContext(r.Context())
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, sessionRequest(http.MethodGet, "/api/v1/users?organizationId="+foreign, "", "user"))
	if got.String() != foreign {
		t.Fatalf("target tenant = %q, want %q", got, foreign)
	}
}

func TestTenantResolutionMiddleware_OwnOrganizationUUIDIsNotTarget(t *testing.T) {
	r := sessionRequest(http.MethodGet, "/api/v1/users", "", "user")
	session, _ := auth.SessionContextFromContext(r.Context())
	r.URL.RawQuery = "organizationId=" + session.OrganizationID
	calledWithTarget := false
	handler := TenantResolutionMiddleware()(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		_, calledWithTarget = TargetTenantFromContext(req.Context())
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)
	if calledWithTarget {
		t.Fatal("own organization UUID must not be treated as a cross-tenant target")
	}
}

func TestTenantResolutionMiddleware_RejectsForeignSlugInBody(t *testing.T) {
	handler := TenantResolutionMiddleware()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, sessionRequest(http.MethodPost, "/api/v1/entries", `{"organizationId":"globex"}`, "user"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// classifiedScopeAuthorizer 在 recordingScopeAuthorizer 之上提供 AdminScopeResolver，
// 用于验证 Gate 1（无 target）会把作用域分类写入 context。
type classifiedScopeAuthorizer struct {
	recordingScopeAuthorizer
	scope        scopeauthorizer.AdminScope
	resolveErr   error
	bypassCalls  int
	bypassReason string
}

func (a *classifiedScopeAuthorizer) AuditTenantBypass(_ context.Context, _ uuid.UUID, _ scopeauthorizer.PrincipalType, _ uuid.UUID, reason string) {
	a.bypassCalls++
	a.bypassReason = reason
}

func (a *classifiedScopeAuthorizer) ResolveAdminScope(context.Context, string) (scopeauthorizer.AdminScope, error) {
	return a.scope, a.resolveErr
}

func TestScopeAuthorizer_NoTargetAnnotatesTargetedScope(t *testing.T) {
	target := uuid.New()
	authorizer := &classifiedScopeAuthorizer{scope: scopeauthorizer.AdminScope{
		Class:         scopeauthorizer.AdminScopeClassTargeted,
		TargetTenants: []uuid.UUID{target},
	}}

	var got scopeauthorizer.AdminScope
	var classified bool
	var hasAuthorizedTarget bool
	handler := scopeChain(nil, authorizer, func(_ http.ResponseWriter, r *http.Request) {
		got, classified = scopeauthorizer.AdminScopeFromContext(r.Context())
		_, hasAuthorizedTarget = scopeauthorizer.AuthorizedTargetTenantFromContext(r.Context())
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, sessionRequest(http.MethodGet, "/api/v1/users", "", "platform_admin"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !classified || got.Class != scopeauthorizer.AdminScopeClassTargeted {
		t.Fatalf("admin scope = %+v classified=%t, want targeted", got, classified)
	}
	if hasAuthorizedTarget {
		t.Fatal("no-target request must not carry an authorized target")
	}
}

func TestScopeAuthorizer_TargetedTargetRecordsAuthorizedTarget(t *testing.T) {
	ownOrg := uuid.NewString()
	foreign := uuid.New()
	authorizer := &classifiedScopeAuthorizer{
		recordingScopeAuthorizer: recordingScopeAuthorizer{allowed: map[uuid.UUID]bool{foreign: true}},
		scope: scopeauthorizer.AdminScope{
			Class:         scopeauthorizer.AdminScopeClassTargeted,
			TargetTenants: []uuid.UUID{foreign},
		},
	}

	var got scopeauthorizer.AdminScope
	var authorized uuid.UUID
	var hasAuthorized bool
	handler := scopeChain(nil, authorizer, func(_ http.ResponseWriter, r *http.Request) {
		got, _ = scopeauthorizer.AdminScopeFromContext(r.Context())
		authorized, hasAuthorized = scopeauthorizer.AuthorizedTargetTenantFromContext(r.Context())
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, sessionRequestWithOrg(
		http.MethodGet, "/api/v1/organizations?organizationId="+foreign.String(), "", "platform_admin", ownOrg, "acme"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got.Class != scopeauthorizer.AdminScopeClassTargeted {
		t.Fatalf("admin scope class = %q, want targeted", got.Class)
	}
	if !hasAuthorized || authorized != foreign {
		t.Fatalf("authorized target = %s (present=%t), want %s", authorized, hasAuthorized, foreign)
	}
}

func TestScopeAuthorizer_CrossScopeTargetsSingleTenant(t *testing.T) {
	ownOrg := uuid.NewString()
	foreign := uuid.New()
	authorizer := &classifiedScopeAuthorizer{
		recordingScopeAuthorizer: recordingScopeAuthorizer{allowed: map[uuid.UUID]bool{foreign: true}},
		scope: scopeauthorizer.AdminScope{
			Class:         scopeauthorizer.AdminScopeClassTargeted,
			TargetTenants: []uuid.UUID{foreign},
		},
	}

	var scope tenantctx.TenantScope
	handler := scopeChain(nil, authorizer, func(_ http.ResponseWriter, r *http.Request) {
		scope = tenantctx.TenantScopeFromContext(r.Context())
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, sessionRequestWithOrg(
		http.MethodGet, "/api/v1/users?organizationId="+foreign.String(), "", "platform_admin", ownOrg, "acme"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if scope.Mode != tenantctx.TenantFilterCross {
		t.Fatalf("mode = %q, want cross", scope.Mode)
	}
	if scope.OrganizationID != foreign.String() {
		t.Fatalf("org = %q, want target %s", scope.OrganizationID, foreign)
	}
}

// OPEN-API-42 C1 把显式跨租户注入从 System 收紧为 Cross 后，payments/orders 等
// 业务层 requireUserScopeAccess 不再能靠 Mode==System 放行，只能依赖
// AdminScopeFromContext。放行路径必须对所有端点注入作用域，否则合法的平台
// 管理员跨租户访问会被误判 403（回归用例锁定非 organizations 路径）。
func TestScopeAuthorizer_AnnotatesAdminScopeForNonOrganizationsPath(t *testing.T) {
	ownOrg := uuid.NewString()
	foreign := uuid.New()
	authorizer := &classifiedScopeAuthorizer{
		recordingScopeAuthorizer: recordingScopeAuthorizer{allowed: map[uuid.UUID]bool{foreign: true}},
		scope:                    scopeauthorizer.AdminScope{Class: scopeauthorizer.AdminScopeClassAll},
	}

	var got scopeauthorizer.AdminScope
	var classified bool
	handler := scopeChain(nil, authorizer, func(_ http.ResponseWriter, r *http.Request) {
		got, classified = scopeauthorizer.AdminScopeFromContext(r.Context())
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, sessionRequestWithOrg(
		http.MethodGet, "/api/v1/users/u-1/payments?organizationId="+foreign.String(), "", "platform_admin", ownOrg, "acme"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !classified || got.Class != scopeauthorizer.AdminScopeClassAll {
		t.Fatalf("admin scope = %+v classified=%t, want all", got, classified)
	}
	if !got.Covers(foreign) {
		t.Fatal("annotated scope must cover the authorized target tenant")
	}
}

// 无显式 target 的 all-scope 平台管理员会走 RLS 全库旁路：必须写旁路审计
// （主体/范围/决策/原因），否则无法在审计链上追溯。
func TestScopeAuthorizer_AllScopeNoTargetAuditsBypass(t *testing.T) {
	authorizer := &classifiedScopeAuthorizer{scope: scopeauthorizer.AdminScope{
		Class: scopeauthorizer.AdminScopeClassAll,
	}}

	handler := scopeChain(nil, authorizer, func(http.ResponseWriter, *http.Request) {})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, sessionRequest(http.MethodGet, "/api/v1/users", "", "platform_admin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if authorizer.bypassCalls != 1 {
		t.Fatalf("bypass audit calls = %d, want 1", authorizer.bypassCalls)
	}
	if authorizer.bypassReason != "platform_admin_all_tenants_no_explicit_target" {
		t.Fatalf("bypass reason = %q", authorizer.bypassReason)
	}
}

func TestScopeAuthorizer_AllScopeDoesNotRecordAuthorizedTarget(t *testing.T) {
	ownOrg := uuid.NewString()
	foreign := uuid.New()
	authorizer := &classifiedScopeAuthorizer{
		recordingScopeAuthorizer: recordingScopeAuthorizer{allowed: map[uuid.UUID]bool{foreign: true}},
		scope:                    scopeauthorizer.AdminScope{Class: scopeauthorizer.AdminScopeClassAll},
	}

	var hasAuthorized bool
	handler := scopeChain(nil, authorizer, func(_ http.ResponseWriter, r *http.Request) {
		_, hasAuthorized = scopeauthorizer.AuthorizedTargetTenantFromContext(r.Context())
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, sessionRequestWithOrg(
		http.MethodGet, "/api/v1/organizations?organizationId="+foreign.String(), "", "platform_admin", ownOrg, "acme"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if hasAuthorized {
		t.Fatal("all-scope request must keep the current full-visibility behavior (no target narrowing)")
	}
}

// chainAdminRepo/chainScopeRepo 只实现 ResolveAdminScope 所需的两条查询。
type chainAdminRepo struct {
	platformadmins.PlatformAdminRepository
	admin *platformadmins.PlatformAdmin
}

func (r chainAdminRepo) GetByUserID(context.Context, string) (*platformadmins.PlatformAdmin, error) {
	return r.admin, nil
}

type chainScopeRepo struct {
	platformadmins.PlatformAdminScopeRepository
	scopes []*platformadmins.PlatformAdminScope
}

func (r chainScopeRepo) ListByPlatformAdmin(context.Context, string, platformadmins.ScopeListOptions) ([]*platformadmins.PlatformAdminScope, error) {
	return r.scopes, nil
}

type chainOrgRepo struct {
	orgdomain.Repository
	organizations []*orgdomain.Organization
}

func (r chainOrgRepo) List(context.Context, orgdomain.ListQuery) ([]*orgdomain.Organization, int64, error) {
	return r.organizations, int64(len(r.organizations)), nil
}

type chainIdentityUoW struct {
	appuow.IdentityUoW
	repository orgdomain.Repository
}

func (u chainIdentityUoW) Organizations() orgdomain.Repository { return u.repository }

// 端到端（中间件层）防线：种子形态的 all scope 必须一路走到
// TenantFilterSuperAdmin，并由真实 organizations 路由返回非空列表。
func TestScopeAuthorizer_SeedAllScopeListsOrganizations(t *testing.T) {
	const ownOrg = "01a0ea76-0c00-7000-8000-000000000001"

	router := chi.NewRouter()
	orghandler.RegisterChiRoutes(router, orghandler.Handler{Service: apppkg.Service{UOW: chainIdentityUoW{
		repository: chainOrgRepo{organizations: []*orgdomain.Organization{
			{ID: "01a0ea76-0c00-7000-8000-000000000001", Slug: "built-in", DisplayName: "Built-in Organization", Status: "active"},
			{ID: "01a0ea76-0c00-7000-8000-000000000002", Slug: "second-org", DisplayName: "Second Org", Status: "active"},
		}},
	}}})

	var downstreamMode postgres.TenantScope
	capture := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamMode = postgres.TenantScopeFromContext(r.Context())
		router.ServeHTTP(w, r)
	})

	authorizer := scopeauthorizer.NewAllowIfScopedScopeAuthorizer(
		chainAdminRepo{admin: &platformadmins.PlatformAdmin{ID: "pa-1", IsActive: true}},
		chainScopeRepo{scopes: []*platformadmins.PlatformAdminScope{{
			ScopeType: platformadmins.ScopeTypeAll,
			ExpiresAt: time.Now().Add(time.Hour),
			// approved_by/approved_at 为 nil：与 db/seeds/V1 种子行一致。
		}}},
		nil, nil, nil,
	)

	handler := TenantResolutionMiddlewareWithResolver(nil)(
		ScopeAuthorizerMiddleware(authorizer)(
			TenantFilterMiddleware(TenantFilterConfig{StrictMode: false})(capture)))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, sessionRequestWithOrg(
		http.MethodGet, "/api/v1/organizations?page=1&pageSize=10", "", "platform_admin", ownOrg, "built-in"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Items []struct {
			Slug string `json:"slug"`
		} `json:"items"`
		Page struct {
			Total int64 `json:"total"`
		} `json:"page"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.EqualValues(t, 2, body.Page.Total)
	require.Len(t, body.Items, 2)
	require.Equal(t, "built-in", body.Items[0].Slug)

	if downstreamMode.Mode != postgres.TenantFilterSuperAdmin {
		t.Fatalf("downstream Mode = %v, want TenantFilterSuperAdmin", downstreamMode.Mode)
	}
}
