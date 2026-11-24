package providers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appproviders "github.com/guliangiam/guliangiam/internal/app/providers"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	domainmock "github.com/guliangiam/guliangiam/internal/domain/providers/mock"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	httpmw "github.com/guliangiam/guliangiam/internal/transport/http/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	forgedOrgSessionID = "11111111-1111-1111-1111-111111111111"
	forgedOrgRequestID = "22222222-2222-2222-2222-222222222222"
)

type forgedOrgClock struct{}

func (forgedOrgClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type forgedOrgUUID struct{}

func (forgedOrgUUID) NewString() (string, error) { return "generated-provider-id", nil }

func forgedOrgRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), forgedOrgSessionID, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333", OrganizationID: forgedOrgSessionID,
	})
	return req.WithContext(ctx)
}

// TestListProviders_ForgedQueryOrganizationIDIgnored pins that the query value
// only requests a tenant; the authenticated scope decides the repository filter.
func TestListProviders_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := domainmock.NewMockRepository(ctrl)
	uow.EXPECT().Providers().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, q domain.ListQuery) ([]*domain.Provider, int64, error) {
			assert.Equal(t, forgedOrgSessionID, q.OrganizationID)
			return nil, 0, nil
		},
	)

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appproviders.NewService(forgedOrgClock{}, forgedOrgUUID{}, uow)})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet,
		"/api/v1/providers?organizationId="+forgedOrgRequestID, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestCreateProvider_WithoutTenantScopeRejected pins the write fail-fast.
func TestCreateProvider_WithoutTenantScopeRejected(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/providers",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","name":"provider","category":"oauth"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{})
	router.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

// ---------------------------------------------------------------------------
// By-ID tenant binding (IDOR P1)
// ---------------------------------------------------------------------------

const (
	tenantScopeSecret  = "super-secret-client-secret"
	foreignProviderID  = "foreign-provider"
	ownProviderID      = "own-provider"
	tenantProviderName = "tenant-provider"
)

// tenantScopeRepo is an in-memory provider repository that honours the
// by-ID organization predicate like the SQL adapters do. Ownership is enforced
// at the repository boundary; the app-layer assertion and the call counters
// prove a rejected request never reached a write.
type tenantScopeRepo struct {
	providers         []*domain.Provider
	configs           []*domain.ProviderConfig
	secretStatuses    []domain.ProviderSecretStatus
	getCalls          int
	updateCalls       int
	deleteCalls       int
	updateConfigCalls int
	configCalls       int
	secretStatusCalls int
}

func (r *tenantScopeRepo) GetByID(_ context.Context, id string) (*domain.Provider, error) {
	r.getCalls++
	for _, p := range r.providers {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, apperrors.NewNotFound("provider not found")
}

func (r *tenantScopeRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Provider, error) {
	r.getCalls++
	for _, p := range r.providers {
		if p.ID == id && p.OrganizationID == organizationID {
			return p, nil
		}
	}
	return nil, apperrors.NewNotFound("provider not found")
}

func (r *tenantScopeRepo) List(context.Context, domain.ListQuery) ([]*domain.Provider, int64, error) {
	return r.providers, int64(len(r.providers)), nil
}

func (r *tenantScopeRepo) Create(_ context.Context, provider *domain.Provider) error {
	r.providers = append(r.providers, provider)
	return nil
}

func (r *tenantScopeRepo) Update(_ context.Context, provider *domain.Provider) error {
	r.updateCalls++
	for i, p := range r.providers {
		if p.ID == provider.ID {
			r.providers[i] = provider
			return nil
		}
	}
	return apperrors.NewNotFound("provider not found")
}

func (r *tenantScopeRepo) Delete(_ context.Context, id string) error {
	r.deleteCalls++
	for i, p := range r.providers {
		if p.ID == id {
			r.providers = append(r.providers[:i], r.providers[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("provider not found")
}

func (r *tenantScopeRepo) GetConfig(_ context.Context, providerID string) (*domain.ProviderConfig, error) {
	r.configCalls++
	for _, c := range r.configs {
		if c.ProviderID == providerID {
			return c, nil
		}
	}
	return nil, apperrors.NewNotFound("config not found")
}

func (r *tenantScopeRepo) UpdateConfig(_ context.Context, config *domain.ProviderConfig) error {
	r.updateConfigCalls++
	for i, c := range r.configs {
		if c.ProviderID == config.ProviderID && c.ConfigType == config.ConfigType {
			r.configs[i] = config
			return nil
		}
	}
	r.configs = append(r.configs, config)
	return nil
}

func (r *tenantScopeRepo) ListSecretStatuses(_ context.Context, providerID string) ([]domain.ProviderSecretStatus, error) {
	r.secretStatusCalls++
	items := make([]domain.ProviderSecretStatus, 0, len(r.secretStatuses))
	for i := range r.secretStatuses {
		if r.secretStatuses[i].ProviderID == providerID {
			items = append(items, r.secretStatuses[i])
		}
	}
	return items, nil
}

type tenantScopeRecords struct {
	created []*recordsdomain.Record
}

func (r *tenantScopeRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

func (r *tenantScopeRecords) List(context.Context, recordsdomain.ListQuery) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *tenantScopeRecords) Search(context.Context, recordsdomain.SearchCriteria) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *tenantScopeRecords) LoadLastChainEntry(context.Context, string) (int64, []byte, error) {
	return 0, nil, nil
}

func (r *tenantScopeRecords) LoadAnchorSeq(context.Context, string, string) (int64, error) {
	return 0, nil
}

func (r *tenantScopeRecords) InsertAnchor(context.Context, *recordsdomain.ChainAnchor) error {
	return nil
}

func (r *tenantScopeRecords) ListChainEntries(context.Context, string) ([]recordsdomain.ChainEntry, error) {
	return nil, nil
}

type tenantScopeOutbox struct {
	enqueued []*domainoutbox.OutboxEvent
}

func (r *tenantScopeOutbox) Enqueue(_ context.Context, event *domainoutbox.OutboxEvent) error {
	r.enqueued = append(r.enqueued, event)
	return nil
}

func (r *tenantScopeOutbox) EnqueueBatch(_ context.Context, events []*domainoutbox.OutboxEvent) error {
	r.enqueued = append(r.enqueued, events...)
	return nil
}

// tenantScopeUOW exposes only the repositories the provider service uses; the
// embedded UnitOfWork carries the rest of the interface without implementation.
type tenantScopeUOW struct {
	appuow.UnitOfWork
	repo    *tenantScopeRepo
	records *tenantScopeRecords
	outbox  *tenantScopeOutbox
}

func (u *tenantScopeUOW) Providers() domain.Repository      { return u.repo }
func (u *tenantScopeUOW) Records() recordsdomain.Repository { return u.records }
func (u *tenantScopeUOW) Outbox() domainoutbox.Repository   { return u.outbox }
func (u *tenantScopeUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func tenantScopeProvider(id, organizationID string) *domain.Provider {
	return &domain.Provider{
		ID: id, OrganizationID: organizationID, Name: tenantProviderName, DisplayName: "Tenant Provider",
		Category: "OAuth", ProviderType: "oidc", ClientSecret: tenantScopeSecret,
		Revision: 1, CreatedAt: time.Unix(0, 0).UTC(), UpdatedAt: time.Unix(0, 0).UTC(),
	}
}

func tenantScopeService(repo *tenantScopeRepo) (appproviders.Service, *tenantScopeRecords, *tenantScopeOutbox) {
	records := &tenantScopeRecords{}
	outbox := &tenantScopeOutbox{}
	uow := &tenantScopeUOW{repo: repo, records: records, outbox: outbox}
	return appproviders.NewService(forgedOrgClock{}, forgedOrgUUID{}, uow), records, outbox
}

func tenantScopeRouter(service appproviders.Service) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return router
}

func platformOrgRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req.WithContext(tenantctx.WithTenantScope(req.Context(), "", "", tenantctx.TenantFilterSuperAdmin))
}

func serveProviderRequest(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func newTenantScopeRepo(providerID, providerOrgID string) *tenantScopeRepo {
	return &tenantScopeRepo{
		providers: []*domain.Provider{tenantScopeProvider(providerID, providerOrgID)},
		configs: []*domain.ProviderConfig{{
			ProviderID: providerID, ConfigType: "oauth2",
			Config: map[string]any{"clientId": "config-client-id"}, UpdatedAt: time.Unix(0, 0).UTC(),
		}},
		secretStatuses: []domain.ProviderSecretStatus{{
			ID: "secret-status-1", ProviderID: providerID, SecretType: "client_secret",
			Status: "active", LastRotatedAt: time.Unix(0, 0).UTC(),
		}},
	}
}

// TestProviderByID_ForeignTenantRejected pins that every by-ID read and write
// resolves the authenticated tenant, not the row id: a foreign provider must be
// indistinguishable from a missing one, no write and no audit may happen, and
// no provider field or secret may appear in the 404 body.
func TestProviderByID_ForeignTenantRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/providers/" + foreignProviderID + "?organizationId=" + forgedOrgRequestID, ""},
		{"patch", http.MethodPatch, "/api/v1/providers/" + foreignProviderID + "?organizationId=" + forgedOrgRequestID, `{"displayName":"hijacked","revision":1}`},
		{"delete", http.MethodDelete, "/api/v1/providers/" + foreignProviderID + "?organizationId=" + forgedOrgRequestID, ""},
		{"get-config", http.MethodGet, "/api/v1/providers/" + foreignProviderID + "/config?organizationId=" + forgedOrgRequestID, ""},
		{"update-config", http.MethodPut, "/api/v1/providers/" + foreignProviderID + "/config?organizationId=" + forgedOrgRequestID, `{"configType":"oauth2","config":{"clientId":"hijacked"}}`},
		{"secret-status", http.MethodGet, "/api/v1/providers/" + foreignProviderID + "/secret-status?organizationId=" + forgedOrgRequestID, ""},
		{"test-connection", http.MethodPost, "/api/v1/providers/" + foreignProviderID + "/test-connection?organizationId=" + forgedOrgRequestID, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newTenantScopeRepo(foreignProviderID, forgedOrgRequestID)
			service, records, outbox := tenantScopeService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveProviderRequest(t, tenantScopeRouter(service), forgedOrgRequest(tt.method, tt.target, body))
			responseBody := recorder.Body.String()

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign provider must look like a missing one")
			assert.Zero(t, repo.updateCalls, "foreign tenant must not update the provider")
			assert.Zero(t, repo.deleteCalls, "foreign tenant must not delete the provider")
			assert.Zero(t, repo.updateConfigCalls, "foreign tenant must not write the provider config")
			assert.Empty(t, records.created, "foreign tenant must not produce an audit record")
			assert.Empty(t, outbox.enqueued, "foreign tenant must not produce an outbox event")
			assert.NotContains(t, responseBody, tenantScopeSecret)
			assert.NotContains(t, responseBody, tenantProviderName)
			assert.NotContains(t, responseBody, "config-client-id")
			if tt.name == "get-config" {
				assert.Zero(t, repo.configCalls, "ownership must be decided before reading the config")
			}
			if tt.name == "secret-status" {
				assert.Zero(t, repo.secretStatusCalls, "ownership must be decided before reading secret status")
			}
		})
	}
}

// TestProviderByID_OwnTenantSucceeds pins that the same paths still work for
// the authenticated tenant and that a forged ?organizationId= cannot re-target
// a provider of the caller's own tenant.
func TestProviderByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("get ignores forged query organization", func(t *testing.T) {
		repo := newTenantScopeRepo(ownProviderID, forgedOrgSessionID)
		service, _, _ := tenantScopeService(repo)
		recorder := serveProviderRequest(t, tenantScopeRouter(service), forgedOrgRequest(http.MethodGet,
			"/api/v1/providers/"+ownProviderID+"?organizationId="+forgedOrgRequestID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "clientSecretMasked")
		assert.NotContains(t, recorder.Body.String(), tenantScopeSecret)
	})

	t.Run("get-config", func(t *testing.T) {
		repo := newTenantScopeRepo(ownProviderID, forgedOrgSessionID)
		service, _, _ := tenantScopeService(repo)
		recorder := serveProviderRequest(t, tenantScopeRouter(service), forgedOrgRequest(http.MethodGet,
			"/api/v1/providers/"+ownProviderID+"/config", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "configType")
	})

	t.Run("update-config", func(t *testing.T) {
		repo := newTenantScopeRepo(ownProviderID, forgedOrgSessionID)
		service, records, _ := tenantScopeService(repo)
		recorder := serveProviderRequest(t, tenantScopeRouter(service), forgedOrgRequest(http.MethodPut,
			"/api/v1/providers/"+ownProviderID+"/config", strings.NewReader(`{"configType":"oauth2","config":{"clientId":"rotated"}}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateConfigCalls)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID)
	})

	t.Run("secret-status", func(t *testing.T) {
		repo := newTenantScopeRepo(ownProviderID, forgedOrgSessionID)
		service, _, _ := tenantScopeService(repo)
		recorder := serveProviderRequest(t, tenantScopeRouter(service), forgedOrgRequest(http.MethodGet,
			"/api/v1/providers/"+ownProviderID+"/secret-status", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.secretStatusCalls)
	})

	t.Run("test-connection", func(t *testing.T) {
		repo := newTenantScopeRepo(ownProviderID, forgedOrgSessionID)
		service, records, _ := tenantScopeService(repo)
		recorder := serveProviderRequest(t, tenantScopeRouter(service), forgedOrgRequest(http.MethodPost,
			"/api/v1/providers/"+ownProviderID+"/test-connection", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		// 探测是只读操作：不推进状态，也不产生 audit/outbox。
		assert.Empty(t, records.created)
	})

	t.Run("patch ignores forged query organization", func(t *testing.T) {
		repo := newTenantScopeRepo(ownProviderID, forgedOrgSessionID)
		service, records, _ := tenantScopeService(repo)
		recorder := serveProviderRequest(t, tenantScopeRouter(service), forgedOrgRequest(http.MethodPatch,
			"/api/v1/providers/"+ownProviderID+"?organizationId="+forgedOrgRequestID,
			strings.NewReader(`{"displayName":"renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCalls)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		repo := newTenantScopeRepo(ownProviderID, forgedOrgSessionID)
		service, records, _ := tenantScopeService(repo)
		recorder := serveProviderRequest(t, tenantScopeRouter(service), forgedOrgRequest(http.MethodDelete,
			"/api/v1/providers/"+ownProviderID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCalls)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID)
	})
}

// TestProviderByID_PlatformScopeCrossesTenants pins the explicit platform
// administrator override: the requested organizationId is trusted only when
// the tenant scope is super_admin/system.
func TestProviderByID_PlatformScopeCrossesTenants(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := newTenantScopeRepo(foreignProviderID, forgedOrgRequestID)
		service, _, _ := tenantScopeService(repo)
		recorder := serveProviderRequest(t, tenantScopeRouter(service), platformOrgRequest(http.MethodGet,
			"/api/v1/providers/"+foreignProviderID+"?organizationId="+forgedOrgRequestID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
	})

	t.Run("delete", func(t *testing.T) {
		repo := newTenantScopeRepo(foreignProviderID, forgedOrgRequestID)
		service, records, _ := tenantScopeService(repo)
		recorder := serveProviderRequest(t, tenantScopeRouter(service), platformOrgRequest(http.MethodDelete,
			"/api/v1/providers/"+foreignProviderID+"?organizationId="+forgedOrgRequestID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCalls)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgRequestID, records.created[0].OrganizationID)
	})
}

// TestProviderByID_WithoutTenantScopeRejected pins that every by-ID route
// fails fast when neither the session nor a platform-admin target provides a
// tenant: the repository must never be reached.
func TestProviderByID_WithoutTenantScopeRejected(t *testing.T) {
	repo := newTenantScopeRepo(ownProviderID, forgedOrgSessionID)
	service, _, _ := tenantScopeService(repo)
	router := tenantScopeRouter(service)

	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/providers/" + ownProviderID, ""},
		{"patch", http.MethodPatch, "/api/v1/providers/" + ownProviderID, `{"displayName":"renamed"}`},
		{"delete", http.MethodDelete, "/api/v1/providers/" + ownProviderID, ""},
		{"get-config", http.MethodGet, "/api/v1/providers/" + ownProviderID + "/config", ""},
		{"update-config", http.MethodPut, "/api/v1/providers/" + ownProviderID + "/config", `{"configType":"oauth2"}`},
		{"secret-status", http.MethodGet, "/api/v1/providers/" + ownProviderID + "/secret-status", ""},
		{"test-connection", http.MethodPost, "/api/v1/providers/" + ownProviderID + "/test-connection", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			req := httptest.NewRequest(tt.method, tt.target, body)
			if body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := serveProviderRequest(t, router, req)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, repo.getCalls, "missing tenant must not reach the repository")
			assert.Zero(t, repo.updateCalls)
			assert.Zero(t, repo.deleteCalls)
			assert.Zero(t, repo.updateConfigCalls)
		})
	}
}

// TestProviderDetailRead_IsNoStore pins that GET /providers/{id} never falls
// through to the ETag middleware's `private, no-cache` branch: the payload
// exposes masked credential placeholders and must reflect a save immediately.
func TestProviderDetailRead_IsNoStore(t *testing.T) {
	repo := newTenantScopeRepo(ownProviderID, forgedOrgSessionID)
	service, _, _ := tenantScopeService(repo)
	handler := httpmw.ETagCacheMiddleware(httpmw.DefaultETagConfig())(tenantScopeRouter(service))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet, "/api/v1/providers/"+ownProviderID, nil))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	assert.Empty(t, recorder.Header().Get("ETag"))
}
