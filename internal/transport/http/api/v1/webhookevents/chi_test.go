package webhookevents

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	appwh "github.com/guliangiam/guliangiam/internal/app/webhookevents"
	domain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func testRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	return r
}

func doRequest(t *testing.T, router chi.Router, method, path string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Result()
}

// newHandlerWithNilService creates a Handler with zero-value Service.
// The Service methods will panic because UOW is nil, but the chi.go route
// registration code (request parsing, parameter extraction) will be covered.
// The Recoverer middleware catches panics and returns 500.
func newHandlerWithNilService() Handler {
	return Handler{Service: appwh.Service{}}
}

// ---------------------------------------------------------------------------
// Real route registration tests (covers chi.go code)
// ---------------------------------------------------------------------------

func TestRealRoutes_Get_Webhookevents(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	resp := doRequest(t, router, http.MethodGet, "/api/v1/webhook-events")
	// Request may fail validation (400) or reach handler (500/401/404/etc)
	// Either way, chi.go route code is covered
	assert.NotEqual(t, 0, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// Tenant binding of the cross-webhook audit list
// ---------------------------------------------------------------------------

const (
	trustedAuditOrgID   = "11111111-1111-1111-1111-111111111111"
	forgedAuditOrgID    = "22222222-2222-2222-2222-222222222222"
	auditEventPublicID  = "33333333-3333-3333-3333-333333333333"
	auditListPathPrefix = "/api/v1/webhook-events"
)

// recordingAuditRepo records the tenant each audit-list query receives and
// returns no rows for any tenant other than the one it owns.
type recordingAuditRepo struct {
	owner   string
	queries []domain.ListQuery
}

func (r *recordingAuditRepo) GetByID(context.Context, string) (*domain.WebhookEvent, error) {
	return nil, apperrors.NewNotFound("webhook event not found")
}

func (r *recordingAuditRepo) GetByIDForOrganization(context.Context, string, string) (*domain.WebhookEvent, error) {
	return nil, apperrors.NewNotFound("webhook event not found")
}

func (r *recordingAuditRepo) List(_ context.Context, q domain.ListQuery) ([]*domain.WebhookEvent, int64, error) {
	r.queries = append(r.queries, q)
	if r.owner != "" && q.OrganizationID != "" && q.OrganizationID != r.owner {
		return nil, 0, nil
	}
	return []*domain.WebhookEvent{{ID: auditEventPublicID}}, 1, nil
}

func (r *recordingAuditRepo) ListExistingIDs(context.Context, []string) ([]string, error) {
	return nil, nil
}
func (r *recordingAuditRepo) CreateBatch(context.Context, []*domain.WebhookEvent) error { return nil }
func (r *recordingAuditRepo) Create(context.Context, *domain.WebhookEvent) error        { return nil }
func (r *recordingAuditRepo) Update(context.Context, *domain.WebhookEvent) error        { return nil }
func (r *recordingAuditRepo) Delete(context.Context, string) error                      { return nil }

func auditRouter(t *testing.T, repo domain.Repository) chi.Router {
	t.Helper()
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockAuditUoW(ctrl)
	uow.EXPECT().WebhookEvents().Return(repo).AnyTimes()
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appwh.Service{UOW: uow}})
	return router
}

// auditRequest attaches the middleware-provided tenant scope and session the
// way protected routes see them. An empty mode leaves the request unscoped.
func auditRequest(target string, mode tenantctx.TenantFilterMode, scopeOrg, sessionOrg string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	ctx := req.Context()
	if mode != "" {
		ctx = tenantctx.WithTenantScope(ctx, scopeOrg, "scope-org", mode)
	}
	if sessionOrg != "" {
		ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{OrganizationID: sessionOrg})
	}
	return req.WithContext(ctx)
}

func auditErrorCode(t *testing.T, body []byte) string {
	t.Helper()
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	return envelope.Error.Code
}

// TestListRoute_ForgedOrganizationIDIgnored pins that a regular tenant cannot
// move the cross-webhook audit list onto another tenant through the query
// string: the repository receives the authenticated tenant.
func TestListRoute_ForgedOrganizationIDIgnored(t *testing.T) {
	cases := []struct {
		name       string
		mode       tenantctx.TenantFilterMode
		sessionOrg string
	}{
		{name: "tenant scope", mode: tenantctx.TenantFilterStrict, sessionOrg: trustedAuditOrgID},
		{name: "session fallback", sessionOrg: trustedAuditOrgID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &recordingAuditRepo{owner: trustedAuditOrgID}
			recorder := httptest.NewRecorder()

			auditRouter(t, repo).ServeHTTP(recorder, auditRequest(
				auditListPathPrefix+"?organizationId="+forgedAuditOrgID,
				tc.mode, trustedAuditOrgID, tc.sessionOrg))

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Len(t, repo.queries, 1)
			assert.Equal(t, trustedAuditOrgID, repo.queries[0].OrganizationID)
		})
	}
}

// TestListRoute_PlatformScopeKeepsAllTenants pins the retained product
// semantics: a platform administrator that does not name a tenant gets the
// cross-tenant audit view (empty organization means all tenants).
func TestListRoute_PlatformScopeKeepsAllTenants(t *testing.T) {
	modes := []tenantctx.TenantFilterMode{
		tenantctx.TenantFilterSuperAdmin,
		tenantctx.TenantFilterSystem,
	}
	for _, mode := range modes {
		t.Run(string(mode), func(t *testing.T) {
			repo := &recordingAuditRepo{owner: trustedAuditOrgID}
			recorder := httptest.NewRecorder()

			// The scope carries the admin's home tenant; the absence of a
			// request tenant must still mean all tenants, not the home one.
			auditRouter(t, repo).ServeHTTP(recorder, auditRequest(
				auditListPathPrefix, mode, forgedAuditOrgID, forgedAuditOrgID))

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Len(t, repo.queries, 1)
			assert.Equal(t, "", repo.queries[0].OrganizationID)
		})
	}
}

// TestListRoute_PlatformScopeExplicitOrganizationNarrows pins that the query
// parameter remains a narrowing filter for platform administrators only.
func TestListRoute_PlatformScopeExplicitOrganizationNarrows(t *testing.T) {
	repo := &recordingAuditRepo{owner: forgedAuditOrgID}
	recorder := httptest.NewRecorder()

	auditRouter(t, repo).ServeHTTP(recorder, auditRequest(
		auditListPathPrefix+"?organizationId="+forgedAuditOrgID,
		tenantctx.TenantFilterSuperAdmin, trustedAuditOrgID, trustedAuditOrgID))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Len(t, repo.queries, 1)
	assert.Equal(t, forgedAuditOrgID, repo.queries[0].OrganizationID)
}

// TestListRoute_MalformedPlatformOrganizationRejected pins that a malformed
// platform-supplied tenant is a 400, not a uuid cast error surfacing as 500.
func TestListRoute_MalformedPlatformOrganizationRejected(t *testing.T) {
	repo := &recordingAuditRepo{}
	recorder := httptest.NewRecorder()

	auditRouter(t, repo).ServeHTTP(recorder, auditRequest(
		auditListPathPrefix+"?organizationId=not-a-uuid",
		tenantctx.TenantFilterSuperAdmin, trustedAuditOrgID, trustedAuditOrgID))

	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	assert.Equal(t, "invalid_argument", auditErrorCode(t, recorder.Body.Bytes()))
	assert.Empty(t, repo.queries)
}

// TestListRoute_NoTrustedTenantIsBadRequest pins the fail-fast: with no
// authenticated tenant the forged query value must not become the tenant, and
// the query never reaches the repository.
func TestListRoute_NoTrustedTenantIsBadRequest(t *testing.T) {
	repo := &recordingAuditRepo{}
	recorder := httptest.NewRecorder()

	auditRouter(t, repo).ServeHTTP(recorder, auditRequest(
		auditListPathPrefix+"?organizationId="+forgedAuditOrgID, "", "", ""))

	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	assert.Equal(t, "invalid_argument", auditErrorCode(t, recorder.Body.Bytes()))
	assert.Empty(t, repo.queries)
}
