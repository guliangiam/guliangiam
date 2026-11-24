package radiusconfigs

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	appradiusconfigs "github.com/guliangiam/guliangiam/internal/app/radiusconfigs"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domain "github.com/guliangiam/guliangiam/internal/domain/radiusconfigs"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	idorRadiusOwnID     = "own-radius-config"
	idorRadiusForeignID = "foreign-radius-config"
	idorRadiusSecret    = "super-secret-radius-shared-secret"
)

func radiusIDORRequest(method, target string, body io.Reader) *http.Request {
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

func radiusIDORPlatformRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), forgedOrgSessionID, "session-org", tenantctx.TenantFilterSuperAdmin)
	return req.WithContext(ctx)
}

func radiusIDORBareRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// idorRadiusRepo is a tenant-aware in-memory repository. Call counts stay
// observable so a rejected cross-tenant request proves it never reached a write.
type idorRadiusRepo struct {
	configs    []*domain.RadiusConfig
	updateCnt  int
	deleteCnt  int
	lastGetOrg string
}

func (r *idorRadiusRepo) GetByID(_ context.Context, id, organizationID string) (*domain.RadiusConfig, error) {
	r.lastGetOrg = organizationID
	for _, c := range r.configs {
		if c.ID == id && c.OrganizationID == organizationID {
			return c, nil
		}
	}
	return nil, apperrors.NewNotFound("radius config not found")
}

func (r *idorRadiusRepo) List(_ context.Context, query domain.ListQuery) ([]*domain.RadiusConfig, int64, error) {
	return r.configs, int64(len(r.configs)), nil
}

func (r *idorRadiusRepo) Create(_ context.Context, config *domain.RadiusConfig) error {
	r.configs = append(r.configs, config)
	return nil
}

func (r *idorRadiusRepo) Update(_ context.Context, config *domain.RadiusConfig) error {
	r.updateCnt++
	for i, c := range r.configs {
		if c.ID == config.ID && c.OrganizationID == config.OrganizationID {
			r.configs[i] = config
			return nil
		}
	}
	return apperrors.NewNotFound("radius config not found")
}

func (r *idorRadiusRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCnt++
	for i, c := range r.configs {
		if c.ID == id && c.OrganizationID == organizationID {
			r.configs = append(r.configs[:i], r.configs[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("radius config not found")
}

type idorRadiusRecords struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *idorRadiusRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

type idorRadiusUOW struct {
	appuow.UnitOfWork
	repo    domain.Repository
	records *idorRadiusRecords
}

func (u *idorRadiusUOW) RadiusConfigs() domain.Repository  { return u.repo }
func (u *idorRadiusUOW) Records() recordsdomain.Repository { return u.records }
func (u *idorRadiusUOW) Outbox() domainoutbox.Repository   { return nil }
func (u *idorRadiusUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func newRadiusIDORService(repo domain.Repository) (appradiusconfigs.Service, *idorRadiusRecords) {
	records := &idorRadiusRecords{}
	service := appradiusconfigs.NewService(forgedOrgClock{}, forgedOrgUUID{}, &idorRadiusUOW{repo: repo, records: records})
	return service, records
}

func idorRadiusConfig(id, orgID string) *domain.RadiusConfig {
	return &domain.RadiusConfig{
		ID: id, OrganizationID: orgID, Name: "tenant-radius", Host: "10.0.0.1", Port: 1812,
		SharedSecret: idorRadiusSecret, AuthProtocol: "PAP", IsEnabled: true, Revision: 1,
	}
}

func idorRadiusRouter(service appradiusconfigs.Service) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return router
}

func serveRadiusIDOR(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// TestRadiusConfigByID_ForeignTenantRejected pins that every by-ID operation
// resolves the authenticated tenant, never the caller-supplied organizationId,
// and that a foreign config looks exactly like a missing one: 404, no write,
// no audit record, and the row untouched.
func TestRadiusConfigByID_ForeignTenantRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/radius-configs/" + idorRadiusForeignID + "?organizationId=" + forgedOrgRequestID, ""},
		{"patch", http.MethodPatch, "/api/v1/radius-configs/" + idorRadiusForeignID + "?organizationId=" + forgedOrgRequestID, `{"name":"hijacked","revision":1}`},
		{"delete", http.MethodDelete, "/api/v1/radius-configs/" + idorRadiusForeignID + "?organizationId=" + forgedOrgRequestID, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &idorRadiusRepo{configs: []*domain.RadiusConfig{idorRadiusConfig(idorRadiusForeignID, forgedOrgRequestID)}}
			service, records := newRadiusIDORService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveRadiusIDOR(t, idorRadiusRouter(service), radiusIDORRequest(tt.method, tt.target, body))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign config must be indistinguishable from missing")
			assert.Zero(t, repo.updateCnt, "foreign tenant must not update the config")
			assert.Zero(t, repo.deleteCnt, "foreign tenant must not delete the config")
			assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
			assert.Equal(t, forgedOrgSessionID, repo.lastGetOrg, "repository must receive the trusted session tenant")
			require.Len(t, repo.configs, 1, "foreign config row must remain untouched")
			assert.NotContains(t, recorder.Body.String(), idorRadiusSecret)
		})
	}
}

// TestRadiusConfigByID_WithoutTenantScopeRejected pins the fail-fast: when
// neither the session nor a platform-admin target provides a tenant, every
// by-ID route is rejected before touching the repository (MySQL has no RLS).
func TestRadiusConfigByID_WithoutTenantScopeRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/radius-configs/" + idorRadiusOwnID, ""},
		{"patch", http.MethodPatch, "/api/v1/radius-configs/" + idorRadiusOwnID, `{}`},
		{"delete", http.MethodDelete, "/api/v1/radius-configs/" + idorRadiusOwnID, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &idorRadiusRepo{configs: []*domain.RadiusConfig{idorRadiusConfig(idorRadiusOwnID, forgedOrgSessionID)}}
			service, records := newRadiusIDORService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveRadiusIDOR(t, idorRadiusRouter(service), radiusIDORBareRequest(tt.method, tt.target, body))

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, repo.updateCnt)
			assert.Zero(t, repo.deleteCnt)
			assert.Empty(t, records.created)
			assert.Empty(t, repo.lastGetOrg, "an empty tenant must not reach the repository")
		})
	}
}

// TestRadiusConfigByID_OwnTenantSucceeds pins that the same operations still
// work for the authenticated tenant and that a delete audits the real organization.
func TestRadiusConfigByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &idorRadiusRepo{configs: []*domain.RadiusConfig{idorRadiusConfig(idorRadiusOwnID, forgedOrgSessionID)}}
		service, _ := newRadiusIDORService(repo)
		recorder := serveRadiusIDOR(t, idorRadiusRouter(service),
			radiusIDORRequest(http.MethodGet, "/api/v1/radius-configs/"+idorRadiusOwnID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"organizationId":"`+forgedOrgSessionID+`"`)
		assert.NotContains(t, recorder.Body.String(), idorRadiusSecret)
	})

	t.Run("patch", func(t *testing.T) {
		repo := &idorRadiusRepo{configs: []*domain.RadiusConfig{idorRadiusConfig(idorRadiusOwnID, forgedOrgSessionID)}}
		service, records := newRadiusIDORService(repo)
		recorder := serveRadiusIDOR(t, idorRadiusRouter(service),
			radiusIDORRequest(http.MethodPatch, "/api/v1/radius-configs/"+idorRadiusOwnID,
				strings.NewReader(`{"name":"renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		repo := &idorRadiusRepo{configs: []*domain.RadiusConfig{idorRadiusConfig(idorRadiusOwnID, forgedOrgSessionID)}}
		service, records := newRadiusIDORService(repo)
		recorder := serveRadiusIDOR(t, idorRadiusRouter(service),
			radiusIDORRequest(http.MethodDelete, "/api/v1/radius-configs/"+idorRadiusOwnID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Empty(t, repo.configs)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID, "delete audit must carry the config organization")
	})
}

// TestRadiusConfigByID_PlatformAdminCanCrossTenant pins the single legitimate
// override: super_admin/system scopes may name another tenant explicitly.
func TestRadiusConfigByID_PlatformAdminCanCrossTenant(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &idorRadiusRepo{configs: []*domain.RadiusConfig{idorRadiusConfig(idorRadiusForeignID, forgedOrgRequestID)}}
		service, _ := newRadiusIDORService(repo)
		recorder := serveRadiusIDOR(t, idorRadiusRouter(service),
			radiusIDORPlatformRequest(http.MethodGet, "/api/v1/radius-configs/"+idorRadiusForeignID+"?organizationId="+forgedOrgRequestID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), forgedOrgRequestID)
	})

	t.Run("patch keeps the config tenant", func(t *testing.T) {
		repo := &idorRadiusRepo{configs: []*domain.RadiusConfig{idorRadiusConfig(idorRadiusForeignID, forgedOrgRequestID)}}
		service, records := newRadiusIDORService(repo)
		recorder := serveRadiusIDOR(t, idorRadiusRouter(service),
			radiusIDORPlatformRequest(http.MethodPatch, "/api/v1/radius-configs/"+idorRadiusForeignID+"?organizationId="+forgedOrgRequestID,
				strings.NewReader(`{"name":"admin-renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCnt)
		require.Len(t, repo.configs, 1)
		assert.Equal(t, forgedOrgRequestID, repo.configs[0].OrganizationID)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgRequestID, records.created[0].OrganizationID)
	})

	t.Run("delete audits the config tenant", func(t *testing.T) {
		repo := &idorRadiusRepo{configs: []*domain.RadiusConfig{idorRadiusConfig(idorRadiusForeignID, forgedOrgRequestID)}}
		service, records := newRadiusIDORService(repo)
		recorder := serveRadiusIDOR(t, idorRadiusRouter(service),
			radiusIDORPlatformRequest(http.MethodDelete, "/api/v1/radius-configs/"+idorRadiusForeignID+"?organizationId="+forgedOrgRequestID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgRequestID, records.created[0].OrganizationID)
	})
}
