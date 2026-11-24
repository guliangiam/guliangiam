package notificationproviders

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	apppkg "github.com/guliangiam/guliangiam/internal/app/notificationproviders"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	npIDORTenantOrgID     = "11111111-1111-1111-1111-111111111111"
	npIDORForeignOrgID    = "22222222-2222-2222-2222-222222222222"
	npIDORForeignProvider = "foreign-notification-provider"
	npIDOROwnProvider     = "own-notification-provider"
	npIDORAccessToken     = "super-secret-access-token"
)

type npIDORClock struct{}

func (npIDORClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type npIDORUUID struct{}

func (npIDORUUID) NewString() (string, error) { return "generated-notification-provider-id", nil }

// npIDORRepo is a tenant-aware in-memory repository. Call counts stay
// observable so a rejected cross-tenant request proves it never reached a write.
type npIDORRepo struct {
	providers   []*domain.NotificationProvider
	getOrgIDs   []string
	updateCalls int
	deleteCalls int
}

func (r *npIDORRepo) GetByID(context.Context, string) (*domain.NotificationProvider, error) {
	// Unscoped lookup is reserved for background delivery; the request path must
	// never reach it.
	return nil, apperrors.NewNotFound("notification provider not found")
}

func (r *npIDORRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.NotificationProvider, error) {
	r.getOrgIDs = append(r.getOrgIDs, organizationID)
	for _, provider := range r.providers {
		if provider.ID == id && provider.OrganizationID == organizationID {
			return provider, nil
		}
	}
	return nil, apperrors.NewNotFound("notification provider not found")
}

func (r *npIDORRepo) List(context.Context, domain.ListQuery) ([]*domain.NotificationProvider, int64, error) {
	return r.providers, int64(len(r.providers)), nil
}

func (r *npIDORRepo) Create(_ context.Context, provider *domain.NotificationProvider) error {
	r.providers = append(r.providers, provider)
	return nil
}

func (r *npIDORRepo) Update(_ context.Context, provider *domain.NotificationProvider) error {
	r.updateCalls++
	for i, existing := range r.providers {
		if existing.ID == provider.ID && existing.OrganizationID == provider.OrganizationID {
			r.providers[i] = provider
			return nil
		}
	}
	return apperrors.NewNotFound("notification provider not found")
}

func (r *npIDORRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCalls++
	for i, existing := range r.providers {
		if existing.ID == id && existing.OrganizationID == organizationID {
			r.providers = append(r.providers[:i], r.providers[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("notification provider not found")
}

type npIDORRecords struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *npIDORRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

// npIDORUOW exposes only the repositories used by the notification provider
// service; the embedded UnitOfWork carries the remaining interface surface.
type npIDORUOW struct {
	appuow.UnitOfWork
	repo    domain.Repository
	records *npIDORRecords
}

func (u *npIDORUOW) NotificationProviders() domain.Repository { return u.repo }
func (u *npIDORUOW) Records() recordsdomain.Repository        { return u.records }
func (u *npIDORUOW) Outbox() domainoutbox.Repository          { return nil }
func (u *npIDORUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func newNPIDORService(repo domain.Repository) (apppkg.Service, *npIDORRecords) {
	records := &npIDORRecords{}
	return apppkg.NewService(npIDORClock{}, npIDORUUID{}, &npIDORUOW{repo: repo, records: records}), records
}

func npIDORProvider(id, organizationID string) *domain.NotificationProvider {
	return &domain.NotificationProvider{
		ID:             id,
		OrganizationID: organizationID,
		Name:           "tenant-notification",
		ProviderType:   "webhook",
		WebhookURL:     "https://hooks.example.com/tenant",
		AccessToken:    npIDORAccessToken,
		Config:         map[string]any{},
		IsEnabled:      true,
		Revision:       1,
	}
}

// npIDORRequest carries the authenticated tenant scope; the target's query
// organizationId is the forged value the route must ignore.
func npIDORRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), npIDORTenantOrgID, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333", OrganizationID: npIDORTenantOrgID,
	})
	return req.WithContext(ctx)
}

// npIDORPlatformRequest simulates a platform administrator scope: only this
// mode may narrow the tenant with an explicit organizationId.
func npIDORPlatformRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), npIDORTenantOrgID, "session-org", tenantctx.TenantFilterSuperAdmin)
	return req.WithContext(ctx)
}

// npIDORBareRequest carries neither a session nor a tenant scope.
func npIDORBareRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func npIDORRouter(service apppkg.Service) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return router
}

func serveNPIDOR(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func npIDORBody(raw string) io.Reader {
	if raw == "" {
		return nil
	}
	return strings.NewReader(raw)
}

// TestNotificationProviderByID_ForeignTenantRejected pins that a by-ID
// read/write against another tenant's provider is indistinguishable from a
// missing one, and that no write or audit event is produced.
func TestNotificationProviderByID_ForeignTenantRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/notification-providers/" + npIDORForeignProvider + "?organizationId=" + npIDORForeignOrgID, ""},
		{"patch", http.MethodPatch, "/api/v1/notification-providers/" + npIDORForeignProvider + "?organizationId=" + npIDORForeignOrgID, `{"name":"hijacked","revision":1}`},
		{"delete", http.MethodDelete, "/api/v1/notification-providers/" + npIDORForeignProvider + "?organizationId=" + npIDORForeignOrgID, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &npIDORRepo{providers: []*domain.NotificationProvider{npIDORProvider(npIDORForeignProvider, npIDORForeignOrgID)}}
			service, records := newNPIDORService(repo)

			recorder := serveNPIDOR(t, npIDORRouter(service), npIDORRequest(tt.method, tt.target, npIDORBody(tt.body)))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign provider must be indistinguishable from missing")
			assert.Zero(t, repo.updateCalls, "foreign tenant must not update the provider")
			assert.Zero(t, repo.deleteCalls, "foreign tenant must not delete the provider")
			assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
			assert.NotContains(t, recorder.Body.String(), npIDORAccessToken)
			require.Len(t, repo.providers, 1, "foreign provider row must remain untouched")
			assert.Equal(t, []string{npIDORTenantOrgID}, repo.getOrgIDs,
				"repository must receive the trusted session tenant")
		})
	}
}

// TestNotificationProviderByID_WithoutTenantScopeRejected pins the fail-fast:
// when neither the session nor a platform-admin target provides a tenant, every
// by-ID route is rejected before touching the repository (MySQL has no RLS).
func TestNotificationProviderByID_WithoutTenantScopeRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/notification-providers/" + npIDOROwnProvider, ""},
		{"patch", http.MethodPatch, "/api/v1/notification-providers/" + npIDOROwnProvider, `{}`},
		{"delete", http.MethodDelete, "/api/v1/notification-providers/" + npIDOROwnProvider, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &npIDORRepo{providers: []*domain.NotificationProvider{npIDORProvider(npIDOROwnProvider, npIDORTenantOrgID)}}
			service, records := newNPIDORService(repo)

			recorder := serveNPIDOR(t, npIDORRouter(service), npIDORBareRequest(tt.method, tt.target, npIDORBody(tt.body)))

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, repo.updateCalls)
			assert.Zero(t, repo.deleteCalls)
			assert.Empty(t, records.created)
			assert.Empty(t, repo.getOrgIDs, "an empty tenant must not reach the repository")
		})
	}
}

// TestNotificationProviderByID_OwnTenantSucceeds pins that the same three
// operations still work for the authenticated tenant and that writes audit the
// real organization.
func TestNotificationProviderByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &npIDORRepo{providers: []*domain.NotificationProvider{npIDORProvider(npIDOROwnProvider, npIDORTenantOrgID)}}
		service, _ := newNPIDORService(repo)

		recorder := serveNPIDOR(t, npIDORRouter(service),
			npIDORRequest(http.MethodGet, "/api/v1/notification-providers/"+npIDOROwnProvider, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "accessTokenMasked")
		assert.NotContains(t, recorder.Body.String(), npIDORAccessToken)
	})

	t.Run("patch", func(t *testing.T) {
		repo := &npIDORRepo{providers: []*domain.NotificationProvider{npIDORProvider(npIDOROwnProvider, npIDORTenantOrgID)}}
		service, records := newNPIDORService(repo)

		recorder := serveNPIDOR(t, npIDORRouter(service), npIDORRequest(http.MethodPatch,
			"/api/v1/notification-providers/"+npIDOROwnProvider, strings.NewReader(`{"name":"renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCalls)
		require.Len(t, records.created, 1)
		assert.Equal(t, npIDORTenantOrgID, records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		repo := &npIDORRepo{providers: []*domain.NotificationProvider{npIDORProvider(npIDOROwnProvider, npIDORTenantOrgID)}}
		service, records := newNPIDORService(repo)

		recorder := serveNPIDOR(t, npIDORRouter(service), npIDORRequest(http.MethodDelete,
			"/api/v1/notification-providers/"+npIDOROwnProvider, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCalls)
		require.Empty(t, repo.providers)
		require.Len(t, records.created, 1)
		assert.Equal(t, npIDORTenantOrgID, records.created[0].OrganizationID,
			"delete audit must carry the provider organization")
	})
}

// TestNotificationProviderByID_PlatformAdminCanCrossTenant pins the single
// legitimate override: super_admin/system scopes may name another tenant.
func TestNotificationProviderByID_PlatformAdminCanCrossTenant(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &npIDORRepo{providers: []*domain.NotificationProvider{npIDORProvider(npIDORForeignProvider, npIDORForeignOrgID)}}
		service, _ := newNPIDORService(repo)

		recorder := serveNPIDOR(t, npIDORRouter(service), npIDORPlatformRequest(http.MethodGet,
			"/api/v1/notification-providers/"+npIDORForeignProvider+"?organizationId="+npIDORForeignOrgID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), npIDORForeignOrgID)
	})

	t.Run("patch keeps the provider tenant", func(t *testing.T) {
		repo := &npIDORRepo{providers: []*domain.NotificationProvider{npIDORProvider(npIDORForeignProvider, npIDORForeignOrgID)}}
		service, records := newNPIDORService(repo)

		recorder := serveNPIDOR(t, npIDORRouter(service), npIDORPlatformRequest(http.MethodPatch,
			"/api/v1/notification-providers/"+npIDORForeignProvider+"?organizationId="+npIDORForeignOrgID,
			strings.NewReader(`{"name":"admin-renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCalls)
		require.Len(t, repo.providers, 1)
		assert.Equal(t, npIDORForeignOrgID, repo.providers[0].OrganizationID)
		require.Len(t, records.created, 1)
		assert.Equal(t, npIDORForeignOrgID, records.created[0].OrganizationID)
	})

	t.Run("delete audits the provider tenant", func(t *testing.T) {
		repo := &npIDORRepo{providers: []*domain.NotificationProvider{npIDORProvider(npIDORForeignProvider, npIDORForeignOrgID)}}
		service, records := newNPIDORService(repo)

		recorder := serveNPIDOR(t, npIDORRouter(service), npIDORPlatformRequest(http.MethodDelete,
			"/api/v1/notification-providers/"+npIDORForeignProvider+"?organizationId="+npIDORForeignOrgID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCalls)
		require.Len(t, records.created, 1)
		assert.Equal(t, npIDORForeignOrgID, records.created[0].OrganizationID)
	})
}
