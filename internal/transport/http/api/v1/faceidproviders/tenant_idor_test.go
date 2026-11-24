package faceidproviders

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appfaceidproviders "github.com/guliangiam/guliangiam/internal/app/faceidproviders"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/faceidproviders"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	idorTenantOrgID     = "11111111-1111-1111-1111-111111111111"
	idorForeignOrgID    = "22222222-2222-2222-2222-222222222222"
	idorForeignProvider = "foreign-face-id-provider"
	idorOwnProvider     = "own-face-id-provider"
	idorFaceIDSecret    = "super-secret-face-id-key"
)

type idorClock struct{}

func (idorClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type idorUUID struct{}

func (idorUUID) NewString() (string, error) { return "generated-face-id-provider-id", nil }

func idorRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), idorTenantOrgID, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333", OrganizationID: idorTenantOrgID,
	})
	return req.WithContext(ctx)
}

// idorPlatformRequest simulates a platform administrator scope: only this mode
// may narrow the tenant with an explicit organizationId.
func idorPlatformRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), idorTenantOrgID, "session-org", tenantctx.TenantFilterSuperAdmin)
	return req.WithContext(ctx)
}

// idorBareRequest carries neither a session nor a tenant scope.
func idorBareRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// idorFaceIDRepo is a tenant-aware in-memory repository. Call counts stay
// observable so a rejected cross-tenant request proves it never reached a write.
type idorFaceIDRepo struct {
	providers []*domain.FaceIdProvider
	getOrgIDs []string
	updateCnt int
	deleteCnt int
	lastList  domain.ListQuery
}

func (r *idorFaceIDRepo) GetByID(_ context.Context, id, organizationID string) (*domain.FaceIdProvider, error) {
	r.getOrgIDs = append(r.getOrgIDs, organizationID)
	for _, p := range r.providers {
		if p.ID == id && p.OrganizationID == organizationID {
			return p, nil
		}
	}
	return nil, apperrors.NewNotFound("face id provider not found")
}

func (r *idorFaceIDRepo) List(_ context.Context, query domain.ListQuery) ([]*domain.FaceIdProvider, int64, error) {
	r.lastList = query
	return r.providers, int64(len(r.providers)), nil
}

func (r *idorFaceIDRepo) Create(_ context.Context, provider *domain.FaceIdProvider) error {
	r.providers = append(r.providers, provider)
	return nil
}

func (r *idorFaceIDRepo) Update(_ context.Context, provider *domain.FaceIdProvider) error {
	r.updateCnt++
	for i, p := range r.providers {
		if p.ID == provider.ID && p.OrganizationID == provider.OrganizationID {
			r.providers[i] = provider
			return nil
		}
	}
	return apperrors.NewNotFound("face id provider not found")
}

func (r *idorFaceIDRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCnt++
	for i, p := range r.providers {
		if p.ID == id && p.OrganizationID == organizationID {
			r.providers = append(r.providers[:i], r.providers[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("face id provider not found")
}

type idorRecords struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *idorRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

// idorUOW exposes only the repositories used by the face-id service.
type idorUOW struct {
	appuow.UnitOfWork
	repo    domain.Repository
	records *idorRecords
}

func (u *idorUOW) FaceIdProviders() domain.Repository { return u.repo }
func (u *idorUOW) Records() recordsdomain.Repository  { return u.records }
func (u *idorUOW) Outbox() domainoutbox.Repository    { return nil }
func (u *idorUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func newIdorService(repo domain.Repository) (appfaceidproviders.Service, *idorRecords) {
	records := &idorRecords{}
	return appfaceidproviders.Service{UOW: &idorUOW{repo: repo, records: records}, Clock: idorClock{}, UUID: idorUUID{}}, records
}

func idorFaceIDProvider(id, orgID string) *domain.FaceIdProvider {
	return &domain.FaceIdProvider{
		ID: id, OrganizationID: orgID, Name: "tenant-face-id", ProviderType: "Aliyun",
		AccessKeyID: "AKID", AccessKeySecret: idorFaceIDSecret, Endpoint: "https://faceid.example.com",
		Config: map[string]any{"region": "cn-shanghai"}, IsEnabled: true, Revision: 1,
	}
}

func idorFaceIDRouter(service appfaceidproviders.Service) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return router
}

func serveIDOR(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// TestFaceIdProviderByID_ForeignTenantRejected pins that every by-ID operation
// resolves the authenticated tenant, never the caller-supplied organizationId,
// and that a foreign provider looks exactly like a missing one: 404, no write,
// no audit record, and the row untouched.
func TestFaceIdProviderByID_ForeignTenantRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/face-id-providers/" + idorForeignProvider + "?organizationId=" + idorForeignOrgID, ""},
		{"patch", http.MethodPatch, "/api/v1/face-id-providers/" + idorForeignProvider + "?organizationId=" + idorForeignOrgID, `{"name":"hijacked","revision":1}`},
		{"delete", http.MethodDelete, "/api/v1/face-id-providers/" + idorForeignProvider + "?organizationId=" + idorForeignOrgID, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &idorFaceIDRepo{providers: []*domain.FaceIdProvider{idorFaceIDProvider(idorForeignProvider, idorForeignOrgID)}}
			service, records := newIdorService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveIDOR(t, idorFaceIDRouter(service), idorRequest(tt.method, tt.target, body))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign provider must be indistinguishable from missing")
			assert.Zero(t, repo.updateCnt, "foreign tenant must not update the provider")
			assert.Zero(t, repo.deleteCnt, "foreign tenant must not delete the provider")
			assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
			assert.Equal(t, []string{idorTenantOrgID}, repo.getOrgIDs, "repository must receive the trusted session tenant")
			require.Len(t, repo.providers, 1, "foreign provider row must remain untouched")
			assert.NotContains(t, recorder.Body.String(), idorFaceIDSecret)
		})
	}
}

// TestFaceIdProviderByID_WithoutTenantScopeRejected pins the fail-fast: when
// neither the session nor a platform-admin target provides a tenant, every
// by-ID route is rejected before touching the repository (MySQL has no RLS).
func TestFaceIdProviderByID_WithoutTenantScopeRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/face-id-providers/" + idorOwnProvider, ""},
		{"patch", http.MethodPatch, "/api/v1/face-id-providers/" + idorOwnProvider, `{}`},
		{"delete", http.MethodDelete, "/api/v1/face-id-providers/" + idorOwnProvider, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &idorFaceIDRepo{providers: []*domain.FaceIdProvider{idorFaceIDProvider(idorOwnProvider, idorTenantOrgID)}}
			service, records := newIdorService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveIDOR(t, idorFaceIDRouter(service), idorBareRequest(tt.method, tt.target, body))

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, repo.updateCnt)
			assert.Zero(t, repo.deleteCnt)
			assert.Empty(t, records.created)
			assert.Empty(t, repo.getOrgIDs, "an empty tenant must not reach the repository")
		})
	}
}

// TestFaceIdProviderByID_OwnTenantSucceeds pins that the same operations still
// work for the authenticated tenant and that a delete audits the real organization.
func TestFaceIdProviderByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &idorFaceIDRepo{providers: []*domain.FaceIdProvider{idorFaceIDProvider(idorOwnProvider, idorTenantOrgID)}}
		service, _ := newIdorService(repo)
		recorder := serveIDOR(t, idorFaceIDRouter(service),
			idorRequest(http.MethodGet, "/api/v1/face-id-providers/"+idorOwnProvider, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"organizationId":"`+idorTenantOrgID+`"`)
		assert.NotContains(t, recorder.Body.String(), idorFaceIDSecret)
	})

	t.Run("patch", func(t *testing.T) {
		repo := &idorFaceIDRepo{providers: []*domain.FaceIdProvider{idorFaceIDProvider(idorOwnProvider, idorTenantOrgID)}}
		service, records := newIdorService(repo)
		recorder := serveIDOR(t, idorFaceIDRouter(service),
			idorRequest(http.MethodPatch, "/api/v1/face-id-providers/"+idorOwnProvider,
				strings.NewReader(`{"name":"renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, idorTenantOrgID, records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		repo := &idorFaceIDRepo{providers: []*domain.FaceIdProvider{idorFaceIDProvider(idorOwnProvider, idorTenantOrgID)}}
		service, records := newIdorService(repo)
		recorder := serveIDOR(t, idorFaceIDRouter(service),
			idorRequest(http.MethodDelete, "/api/v1/face-id-providers/"+idorOwnProvider, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Empty(t, repo.providers)
		require.Len(t, records.created, 1)
		assert.Equal(t, idorTenantOrgID, records.created[0].OrganizationID, "delete audit must carry the provider organization")
	})
}

// TestFaceIdProviderByID_PlatformAdminCanCrossTenant pins the single legitimate
// override: super_admin/system scopes may name another tenant explicitly.
func TestFaceIdProviderByID_PlatformAdminCanCrossTenant(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &idorFaceIDRepo{providers: []*domain.FaceIdProvider{idorFaceIDProvider(idorForeignProvider, idorForeignOrgID)}}
		service, _ := newIdorService(repo)
		recorder := serveIDOR(t, idorFaceIDRouter(service),
			idorPlatformRequest(http.MethodGet, "/api/v1/face-id-providers/"+idorForeignProvider+"?organizationId="+idorForeignOrgID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), idorForeignOrgID)
	})

	t.Run("patch keeps the provider tenant", func(t *testing.T) {
		repo := &idorFaceIDRepo{providers: []*domain.FaceIdProvider{idorFaceIDProvider(idorForeignProvider, idorForeignOrgID)}}
		service, records := newIdorService(repo)
		recorder := serveIDOR(t, idorFaceIDRouter(service),
			idorPlatformRequest(http.MethodPatch, "/api/v1/face-id-providers/"+idorForeignProvider+"?organizationId="+idorForeignOrgID,
				strings.NewReader(`{"name":"admin-renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCnt)
		require.Len(t, repo.providers, 1)
		assert.Equal(t, idorForeignOrgID, repo.providers[0].OrganizationID)
		require.Len(t, records.created, 1)
		assert.Equal(t, idorForeignOrgID, records.created[0].OrganizationID)
	})

	t.Run("delete audits the provider tenant", func(t *testing.T) {
		repo := &idorFaceIDRepo{providers: []*domain.FaceIdProvider{idorFaceIDProvider(idorForeignProvider, idorForeignOrgID)}}
		service, records := newIdorService(repo)
		recorder := serveIDOR(t, idorFaceIDRouter(service),
			idorPlatformRequest(http.MethodDelete, "/api/v1/face-id-providers/"+idorForeignProvider+"?organizationId="+idorForeignOrgID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, idorForeignOrgID, records.created[0].OrganizationID)
	})
}
