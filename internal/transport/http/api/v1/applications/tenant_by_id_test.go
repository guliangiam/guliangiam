package applications

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appapplications "github.com/guliangiam/guliangiam/internal/app/applications"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	consentdomain "github.com/guliangiam/guliangiam/internal/domain/consents"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	providerdomain "github.com/guliangiam/guliangiam/internal/domain/providers"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	httpmw "github.com/guliangiam/guliangiam/internal/transport/http/middleware"
)

const (
	byIDSessionOrg    = "11111111-1111-1111-1111-111111111111"
	byIDForeignOrg    = "22222222-2222-2222-2222-222222222222"
	byIDPlatformOrg   = "33333333-3333-3333-3333-333333333333"
	byIDApplicationID = "44444444-4444-4444-4444-444444444444"
	byIDUserID        = "55555555-5555-5555-5555-555555555555"
)

type byIDClock struct{}

func (byIDClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type byIDUUID struct{}

func (byIDUUID) NewString() (string, error) { return "generated-id", nil }

// byIDApplicationRepo is a tenant-aware in-memory repository. Reads only return
// rows owned by the requested organization, so a foreign id looks exactly like
// a missing one. Counters make cross-tenant writes observable.
type byIDApplicationRepo struct {
	apps        []*domain.Application
	getOrgs     []string
	getCalls    int
	updateCalls int
	deleteCalls int
}

func (r *byIDApplicationRepo) GetByID(_ context.Context, id string) (*domain.Application, error) {
	r.getCalls++
	for _, app := range r.apps {
		if app.ID == id {
			return app, nil
		}
	}
	return nil, apperrors.NewNotFound("application not found")
}

func (r *byIDApplicationRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Application, error) {
	r.getCalls++
	r.getOrgs = append(r.getOrgs, organizationID)
	for _, app := range r.apps {
		if app.ID == id && app.OrganizationID == organizationID {
			return app, nil
		}
	}
	return nil, apperrors.NewNotFound("application not found")
}

func (r *byIDApplicationRepo) GetByClientID(_ context.Context, clientID string) (*domain.Application, error) {
	for _, app := range r.apps {
		if app.ClientID == clientID {
			return app, nil
		}
	}
	return nil, apperrors.NewNotFound("application not found")
}

func (r *byIDApplicationRepo) GetByOrganizationAndSlug(_ context.Context, organizationID, slug string) (*domain.Application, error) {
	for _, app := range r.apps {
		if app.OrganizationID == organizationID && app.Slug == slug {
			return app, nil
		}
	}
	return nil, apperrors.NewNotFound("application not found")
}

func (r *byIDApplicationRepo) List(_ context.Context, query domain.ListQuery) ([]*domain.Application, int64, error) {
	items := make([]*domain.Application, 0, len(r.apps))
	for _, app := range r.apps {
		if query.OrganizationID == "" || app.OrganizationID == query.OrganizationID {
			items = append(items, app)
		}
	}
	return items, int64(len(items)), nil
}

func (r *byIDApplicationRepo) Create(_ context.Context, app *domain.Application) error {
	r.apps = append(r.apps, app)
	return nil
}

func (r *byIDApplicationRepo) Update(_ context.Context, app *domain.Application) error {
	r.updateCalls++
	for i, existing := range r.apps {
		if existing.ID == app.ID {
			r.apps[i] = app
			return nil
		}
	}
	return apperrors.NewNotFound("application not found")
}

func (r *byIDApplicationRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCalls++
	for i, app := range r.apps {
		if app.ID == id && app.OrganizationID == organizationID {
			r.apps = append(r.apps[:i], r.apps[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("application not found")
}

func (r *byIDApplicationRepo) ReplaceProviderBindings(_ context.Context, _ string, _ []domain.ApplicationProviderBinding) error {
	return nil
}

func (r *byIDApplicationRepo) HasDeletedWithOrgAndSlug(_ context.Context, _, _ string) (bool, error) {
	return false, nil
}

func (r *byIDApplicationRepo) FindHostConflicts(_ context.Context, _, _ string, _ []string) ([]domain.HostConflict, error) {
	return nil, nil
}

func (r *byIDApplicationRepo) SetRegistrationAccessToken(_ context.Context, _, _ string) error {
	return nil
}

func (r *byIDApplicationRepo) GetRegistrationAccessTokenHash(_ context.Context, _ string) (string, error) {
	return "", nil
}

type byIDTokens struct{ tokendomain.Repository }

func (byIDTokens) List(context.Context, tokendomain.ListQuery) ([]*tokendomain.Token, int64, error) {
	return nil, 0, nil
}

type byIDConsents struct{ consentdomain.Repository }

func (byIDConsents) CountActiveByApplication(context.Context, string) (int64, error) { return 0, nil }

type byIDRecords struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *byIDRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

type byIDUOW struct {
	appuow.UnitOfWork
	appRepo   *byIDApplicationRepo
	records   *byIDRecords
	providers *byIDProviderRepo
	orgs      *byIDOrgRepo
}

func (u *byIDUOW) Applications() domain.Repository    { return u.appRepo }
func (u *byIDUOW) Tokens() tokendomain.Repository     { return byIDTokens{} }
func (u *byIDUOW) Consents() consentdomain.Repository { return byIDConsents{} }
func (u *byIDUOW) Records() recordsdomain.Repository  { return u.records }
func (u *byIDUOW) Outbox() domainoutbox.Repository    { return nil }
func (u *byIDUOW) Providers() providerdomain.Repository {
	return u.providers
}
func (u *byIDUOW) Organizations() orgdomain.Repository { return u.orgs }
func (u *byIDUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

// byIDOrgRepo 只实现 resolve 需要的公开组织定位；其余方法不会被触达。
type byIDOrgRepo struct {
	orgdomain.Repository
	orgs []*orgdomain.Organization
}

func (r *byIDOrgRepo) GetBySlugPublic(_ context.Context, slug string) (*orgdomain.Organization, error) {
	for _, o := range r.orgs {
		if o.Slug == slug {
			return o, nil
		}
	}
	return nil, apperrors.NewNotFound("organization not found")
}

// byIDProviderRepo backs the provider-binding existence check: an id outside the
// requested organization is NotFound, exactly like the postgres repository.
type byIDProviderRepo struct {
	providers []*providerdomain.Provider
}

func (r *byIDProviderRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*providerdomain.Provider, error) {
	for _, p := range r.providers {
		if p.ID == id && p.OrganizationID == organizationID {
			return p, nil
		}
	}
	return nil, apperrors.NewNotFound("provider not found")
}

func (r *byIDProviderRepo) GetByID(_ context.Context, id string) (*providerdomain.Provider, error) {
	for _, p := range r.providers {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, apperrors.NewNotFound("provider not found")
}

func (r *byIDProviderRepo) List(_ context.Context, _ providerdomain.ListQuery) ([]*providerdomain.Provider, int64, error) {
	return nil, 0, nil
}
func (r *byIDProviderRepo) Create(_ context.Context, _ *providerdomain.Provider) error { return nil }
func (r *byIDProviderRepo) Update(_ context.Context, _ *providerdomain.Provider) error { return nil }
func (r *byIDProviderRepo) Delete(_ context.Context, _ string) error                   { return nil }
func (r *byIDProviderRepo) GetConfig(_ context.Context, _ string) (*providerdomain.ProviderConfig, error) {
	return nil, nil
}
func (r *byIDProviderRepo) UpdateConfig(_ context.Context, _ *providerdomain.ProviderConfig) error {
	return nil
}
func (r *byIDProviderRepo) ListSecretStatuses(_ context.Context, _ string) ([]providerdomain.ProviderSecretStatus, error) {
	return nil, nil
}

func newByIDRouter(repo *byIDApplicationRepo) (chi.Router, *byIDRecords) {
	records := &byIDRecords{}
	service := appapplications.NewService(byIDClock{}, byIDUUID{}, &byIDUOW{
		appRepo: repo,
		records: records,
		providers: &byIDProviderRepo{providers: []*providerdomain.Provider{
			{ID: "provider-1", OrganizationID: byIDSessionOrg},
			{ID: "provider-1", OrganizationID: byIDPlatformOrg},
		}},
		orgs: &byIDOrgRepo{orgs: []*orgdomain.Organization{
			{ID: byIDSessionOrg, Slug: "session-org"},
		}},
	})
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return router, records
}

func byIDRequest(method, target, body string) *http.Request {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), byIDSessionOrg, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: byIDUserID, OrganizationID: byIDSessionOrg,
	})
	return req.WithContext(ctx)
}

func byIDPlatformRequest(method, target, body string) *http.Request {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req.WithContext(tenantctx.WithTenantScope(req.Context(), "", "", tenantctx.TenantFilterSuperAdmin))
}

func serveByID(router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func tenantApplication(organizationID string) *domain.Application {
	return &domain.Application{
		ID:                      byIDApplicationID,
		OrganizationID:          organizationID,
		Slug:                    "tenant-app",
		DisplayName:             "Tenant Application",
		Status:                  "active",
		ClientID:                "tenant-app-client",
		TokenEndpointAuthMethod: domain.TokenEndpointAuthMethodClientSecretBasic,
		Revision:                1,
	}
}

var byIDManagementRoutes = []struct {
	name   string
	method string
	path   string
	body   string
}{
	{"get", http.MethodGet, "/api/v1/applications/" + byIDApplicationID, ""},
	{"delete-summary", http.MethodGet, "/api/v1/applications/" + byIDApplicationID + "/delete-summary", ""},
	{"patch", http.MethodPatch, "/api/v1/applications/" + byIDApplicationID, `{"displayName":"hijacked"}`},
	{"delete", http.MethodDelete, "/api/v1/applications/" + byIDApplicationID, ""},
	{"provider-bindings-get", http.MethodGet, "/api/v1/applications/" + byIDApplicationID + "/provider-bindings", ""},
	{"provider-bindings-put", http.MethodPut, "/api/v1/applications/" + byIDApplicationID + "/provider-bindings", `{"items":[{"providerId":"provider-1"}]}`},
	{"export", http.MethodGet, "/api/v1/applications/" + byIDApplicationID + "/export", ""},
	{"client-secret-rotate", http.MethodPost, "/api/v1/applications/" + byIDApplicationID + "/client-secret/rotate", ""},
}

// TestApplicationByID_ForeignTenantRejected pins that every management by-ID
// route resolves the authenticated tenant, and that a foreign application is
// indistinguishable from a missing one: 404, no write, no audit row.
func TestApplicationByID_ForeignTenantRejected(t *testing.T) {
	for _, tt := range byIDManagementRoutes {
		t.Run(tt.name, func(t *testing.T) {
			repo := &byIDApplicationRepo{apps: []*domain.Application{tenantApplication(byIDForeignOrg)}}
			router, records := newByIDRouter(repo)

			recorder := serveByID(router, byIDRequest(tt.method,
				tt.path+"?organizationId="+byIDForeignOrg, tt.body))

			assert.Equal(t, http.StatusNotFound, recorder.Code,
				"foreign application must look missing")
			assert.Zero(t, repo.updateCalls, "foreign tenant must not update the application")
			assert.Zero(t, repo.deleteCalls, "foreign tenant must not delete the application")
			assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
		})
	}
}

// TestApplicationByID_WithoutTenantScopeRejected pins the fail-fast: when
// neither the session nor a platform-admin target provides a tenant, every by-ID
// route is rejected before touching the repository.
func TestApplicationByID_WithoutTenantScopeRejected(t *testing.T) {
	for _, tt := range byIDManagementRoutes {
		t.Run(tt.name, func(t *testing.T) {
			repo := &byIDApplicationRepo{apps: []*domain.Application{tenantApplication(byIDSessionOrg)}}
			router, records := newByIDRouter(repo)

			var reader io.Reader
			if tt.body != "" {
				reader = strings.NewReader(tt.body)
			}
			req := httptest.NewRequest(tt.method, tt.path, reader)
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			recorder := serveByID(router, req)

			assert.Equal(t, http.StatusBadRequest, recorder.Code,
				"missing trusted tenant must be a client error")
			assert.Zero(t, repo.getCalls, "missing tenant must not reach the repository")
			assert.Zero(t, repo.updateCalls)
			assert.Zero(t, repo.deleteCalls)
			assert.Empty(t, records.created)
		})
	}
}

// TestApplicationByID_OwnTenantSucceeds pins that the same routes still work
// inside the authenticated tenant, that the forged query value is ignored, and
// that delete/patch audit the real organization.
func TestApplicationByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &byIDApplicationRepo{apps: []*domain.Application{tenantApplication(byIDSessionOrg)}}
		router, _ := newByIDRouter(repo)

		recorder := serveByID(router, byIDRequest(http.MethodGet,
			"/api/v1/applications/"+byIDApplicationID+"?organizationId="+byIDForeignOrg, ""))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "tenant-app")

		// 契约面隔离：GET 详情只允许 clientSecretSet/clientSecretMasked，
		// 不得出现明文键 clientSecret / clientSecretPlain。
		var body map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
		for _, forbidden := range []string{"clientSecret", "clientSecretPlain"} {
			_, exists := body[forbidden]
			assert.Falsef(t, exists, "GET /applications/{id} 响应体不得包含 %q", forbidden)
		}
		_, setExists := body["clientSecretSet"]
		_, maskedExists := body["clientSecretMasked"]
		assert.True(t, setExists, "读接口必须暴露脱敏占位 clientSecretSet")
		assert.True(t, maskedExists, "读接口必须暴露脱敏占位 clientSecretMasked")
	})

	t.Run("client-secret-rotate", func(t *testing.T) {
		repo := &byIDApplicationRepo{apps: []*domain.Application{tenantApplication(byIDSessionOrg)}}
		router, records := newByIDRouter(repo)

		recorder := serveByID(router, byIDRequest(http.MethodPost,
			"/api/v1/applications/"+byIDApplicationID+"/client-secret/rotate", ""))

		require.Equal(t, http.StatusOK, recorder.Code)
		require.Len(t, records.created, 1, "轮换必须产生审计")
		assert.Equal(t, byIDSessionOrg, records.created[0].OrganizationID)
	})

	t.Run("delete-summary", func(t *testing.T) {
		repo := &byIDApplicationRepo{apps: []*domain.Application{tenantApplication(byIDSessionOrg)}}
		router, _ := newByIDRouter(repo)

		recorder := serveByID(router, byIDRequest(http.MethodGet,
			"/api/v1/applications/"+byIDApplicationID+"/delete-summary", ""))

		require.Equal(t, http.StatusOK, recorder.Code)
	})

	t.Run("patch", func(t *testing.T) {
		repo := &byIDApplicationRepo{apps: []*domain.Application{tenantApplication(byIDSessionOrg)}}
		router, records := newByIDRouter(repo)

		recorder := serveByID(router, byIDRequest(http.MethodPatch,
			"/api/v1/applications/"+byIDApplicationID, `{"displayName":"renamed"}`))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCalls)
		require.Len(t, records.created, 1)
		assert.Equal(t, byIDSessionOrg, records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		repo := &byIDApplicationRepo{apps: []*domain.Application{tenantApplication(byIDSessionOrg)}}
		router, records := newByIDRouter(repo)

		recorder := serveByID(router, byIDRequest(http.MethodDelete,
			"/api/v1/applications/"+byIDApplicationID, ""))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCalls)
		require.Len(t, records.created, 1)
		assert.Equal(t, byIDSessionOrg, records.created[0].OrganizationID,
			"delete audit must carry the application organization")
	})

	t.Run("provider-bindings-get", func(t *testing.T) {
		repo := &byIDApplicationRepo{apps: []*domain.Application{tenantApplication(byIDSessionOrg)}}
		router, _ := newByIDRouter(repo)

		recorder := serveByID(router, byIDRequest(http.MethodGet,
			"/api/v1/applications/"+byIDApplicationID+"/provider-bindings", ""))

		require.Equal(t, http.StatusOK, recorder.Code)
	})

	t.Run("provider-bindings-put", func(t *testing.T) {
		repo := &byIDApplicationRepo{apps: []*domain.Application{tenantApplication(byIDSessionOrg)}}
		router, records := newByIDRouter(repo)

		recorder := serveByID(router, byIDRequest(http.MethodPut,
			"/api/v1/applications/"+byIDApplicationID+"/provider-bindings",
			`{"items":[{"providerId":"provider-1"}]}`))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCalls)
		require.Len(t, records.created, 2, "replace bindings embeds two audit events")
		assert.Equal(t, byIDSessionOrg, records.created[0].OrganizationID)
	})

	t.Run("provider-bindings-put-unknown-provider", func(t *testing.T) {
		repo := &byIDApplicationRepo{apps: []*domain.Application{tenantApplication(byIDSessionOrg)}}
		router, records := newByIDRouter(repo)

		recorder := serveByID(router, byIDRequest(http.MethodPut,
			"/api/v1/applications/"+byIDApplicationID+"/provider-bindings",
			`{"items":[{"providerId":"00000000-0000-0000-0000-000000000000"}]}`))

		require.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
		assert.Zero(t, repo.updateCalls, "rejected batch must not update the application")
		assert.Empty(t, records.created, "rejected batch must not produce an audit event")
	})

	t.Run("export", func(t *testing.T) {
		repo := &byIDApplicationRepo{apps: []*domain.Application{tenantApplication(byIDSessionOrg)}}
		router, _ := newByIDRouter(repo)

		recorder := serveByID(router, byIDRequest(http.MethodGet,
			"/api/v1/applications/"+byIDApplicationID+"/export", ""))

		require.Equal(t, http.StatusOK, recorder.Code)
	})
}

// TestApplicationByID_PlatformAdminCanTargetTenant pins that a platform scope
// keeps the explicit cross-tenant target; the requested organization is then the
// repository predicate.
func TestApplicationByID_PlatformAdminCanTargetTenant(t *testing.T) {
	for _, tt := range byIDManagementRoutes {
		t.Run(tt.name, func(t *testing.T) {
			repo := &byIDApplicationRepo{apps: []*domain.Application{tenantApplication(byIDPlatformOrg)}}
			router, _ := newByIDRouter(repo)

			recorder := serveByID(router, byIDPlatformRequest(tt.method,
				tt.path+"?organizationId="+byIDPlatformOrg, tt.body))

			require.Equal(t, http.StatusOK, recorder.Code)
			require.NotEmpty(t, repo.getOrgs, "the scoped lookup must be used")
			assert.Equal(t, byIDPlatformOrg, repo.getOrgs[0],
				"platform scope must honour the explicit target tenant")
		})
	}
}

// TestApplicationDetailRead_IsNoStore pins the cache policy split at the
// middleware boundary: the ID-level detail read must declare no-store (it
// carries masked credentials and a save-then-refresh must be fresh), while the
// collection list read keeps the ETag validator but revalidates on every reuse
// (no-cache) so a create/delete-then-refetch never replays a stale body.
func TestApplicationDetailRead_IsNoStore(t *testing.T) {
	repo := &byIDApplicationRepo{apps: []*domain.Application{tenantApplication(byIDSessionOrg)}}
	router, _ := newByIDRouter(repo)
	handler := httpmw.ETagCacheMiddleware(httpmw.DefaultETagConfig())(router)

	t.Run("detail", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, byIDRequest(http.MethodGet, "/api/v1/applications/"+byIDApplicationID, ""))
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
		assert.Empty(t, recorder.Header().Get("ETag"))
	})

	t.Run("list-keeps-validator-with-revalidation", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, byIDRequest(http.MethodGet, "/api/v1/applications", ""))
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "private, no-cache", recorder.Header().Get("Cache-Control"))
		assert.NotEmpty(t, recorder.Header().Get("ETag"))
	})
}
