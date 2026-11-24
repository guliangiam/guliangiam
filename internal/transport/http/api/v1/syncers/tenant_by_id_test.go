package syncers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appsyncers "github.com/guliangiam/guliangiam/internal/app/syncers"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	domain "github.com/guliangiam/guliangiam/internal/domain/syncers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	idorSyncerTenantOrgID  = "11111111-1111-1111-1111-111111111111"
	idorSyncerForeignOrgID = "22222222-2222-2222-2222-222222222222"
	idorSyncerOwnID        = "own-syncer"
	idorSyncerForeignID    = "foreign-syncer"
	idorSyncerPassword     = "super-secret-syncer-password"
)

type idorSyncerClock struct{}

func (idorSyncerClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type idorSyncerUUID struct{}

func (idorSyncerUUID) NewString() (string, error) { return "generated-syncer-id", nil }

func idorSyncerRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), idorSyncerTenantOrgID, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333", OrganizationID: idorSyncerTenantOrgID,
	})
	return req.WithContext(ctx)
}

func idorSyncerPlatformRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), idorSyncerTenantOrgID, "session-org", tenantctx.TenantFilterSuperAdmin)
	return req.WithContext(ctx)
}

func idorSyncerBareRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// idorSyncerRepo is a tenant-aware in-memory repository. Call counts stay
// observable so a rejected cross-tenant request proves it never reached a write.
type idorSyncerRepo struct {
	syncers         []*domain.Syncer
	updateCnt       int
	deleteCnt       int
	lastScopedOrgID string
	lastScopedID    string
}

func (r *idorSyncerRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Syncer, error) {
	r.lastScopedID = id
	r.lastScopedOrgID = organizationID
	for _, s := range r.syncers {
		if s.ID == id && s.OrganizationID == organizationID {
			return s, nil
		}
	}
	return nil, apperrors.NewNotFound("syncer not found")
}

func (r *idorSyncerRepo) List(_ context.Context, query domain.ListQuery) ([]*domain.Syncer, int64, error) {
	return r.syncers, int64(len(r.syncers)), nil
}

func (r *idorSyncerRepo) Create(_ context.Context, syncer *domain.Syncer) error {
	r.syncers = append(r.syncers, syncer)
	return nil
}

func (r *idorSyncerRepo) Update(_ context.Context, syncer *domain.Syncer) error {
	r.updateCnt++
	for i, s := range r.syncers {
		if s.ID == syncer.ID && s.OrganizationID == syncer.OrganizationID {
			r.syncers[i] = syncer
			return nil
		}
	}
	return apperrors.NewNotFound("syncer not found")
}

func (r *idorSyncerRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCnt++
	for i, s := range r.syncers {
		if s.ID == id && s.OrganizationID == organizationID {
			r.syncers = append(r.syncers[:i], r.syncers[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("syncer not found")
}

type idorSyncerRecords struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *idorSyncerRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

type idorSyncerUOW struct {
	appuow.UnitOfWork
	repo    domain.Repository
	records *idorSyncerRecords
}

func (u *idorSyncerUOW) Syncers() domain.Repository        { return u.repo }
func (u *idorSyncerUOW) Records() recordsdomain.Repository { return u.records }
func (u *idorSyncerUOW) Outbox() domainoutbox.Repository   { return nil }
func (u *idorSyncerUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func newSyncerIDORService(repo domain.Repository) (appsyncers.Service, *idorSyncerRecords) {
	records := &idorSyncerRecords{}
	service := appsyncers.NewService(idorSyncerClock{}, idorSyncerUUID{}, &idorSyncerUOW{repo: repo, records: records})
	return service, records
}

func idorSyncer(id, orgID string) *domain.Syncer {
	return &domain.Syncer{
		ID: id, OrganizationID: orgID, Name: "tenant-syncer", DisplayName: "Tenant Syncer",
		Type: domain.SyncerTypeDatabase, Host: "db.example.com", Port: 5432,
		Username: "admin", Password: idorSyncerPassword, IsEnabled: true, Revision: 1,
	}
}

func idorSyncerRouter(service appsyncers.Service) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return router
}

func serveSyncerIDOR(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// TestSyncerByID_ForeignTenantRejected pins that every by-ID operation resolves
// the authenticated tenant, never the caller-supplied organizationId, and that
// a foreign syncer looks exactly like a missing one: 404, no write, no audit
// record, and the row untouched.
func TestSyncerByID_ForeignTenantRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/syncers/" + idorSyncerForeignID + "?organizationId=" + idorSyncerForeignOrgID, ""},
		{"patch", http.MethodPatch, "/api/v1/syncers/" + idorSyncerForeignID + "?organizationId=" + idorSyncerForeignOrgID, `{"displayName":"hijacked","revision":1}`},
		{"delete", http.MethodDelete, "/api/v1/syncers/" + idorSyncerForeignID + "?organizationId=" + idorSyncerForeignOrgID, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &idorSyncerRepo{syncers: []*domain.Syncer{idorSyncer(idorSyncerForeignID, idorSyncerForeignOrgID)}}
			service, records := newSyncerIDORService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveSyncerIDOR(t, idorSyncerRouter(service), idorSyncerRequest(tt.method, tt.target, body))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign syncer must be indistinguishable from missing")
			assert.Zero(t, repo.updateCnt, "foreign tenant must not update the syncer")
			assert.Zero(t, repo.deleteCnt, "foreign tenant must not delete the syncer")
			assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
			assert.Equal(t, idorSyncerTenantOrgID, repo.lastScopedOrgID, "repository must receive the trusted session tenant")
			require.Len(t, repo.syncers, 1, "foreign syncer row must remain untouched")
			assert.NotContains(t, recorder.Body.String(), idorSyncerPassword)
		})
	}
}

// TestSyncerByID_WithoutTenantScopeRejected pins the fail-fast: when neither
// the session nor a platform-admin target provides a tenant, every by-ID route
// is rejected before touching the repository (MySQL has no RLS).
func TestSyncerByID_WithoutTenantScopeRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/syncers/" + idorSyncerOwnID, ""},
		{"patch", http.MethodPatch, "/api/v1/syncers/" + idorSyncerOwnID, `{}`},
		{"delete", http.MethodDelete, "/api/v1/syncers/" + idorSyncerOwnID, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &idorSyncerRepo{syncers: []*domain.Syncer{idorSyncer(idorSyncerOwnID, idorSyncerTenantOrgID)}}
			service, records := newSyncerIDORService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveSyncerIDOR(t, idorSyncerRouter(service), idorSyncerBareRequest(tt.method, tt.target, body))

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, repo.updateCnt)
			assert.Zero(t, repo.deleteCnt)
			assert.Empty(t, records.created)
			assert.Empty(t, repo.lastScopedID, "an empty tenant must not reach the repository")
		})
	}
}

// TestSyncerByID_OwnTenantSucceeds pins that the same operations still work for
// the authenticated tenant and that a delete audits the real organization.
func TestSyncerByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &idorSyncerRepo{syncers: []*domain.Syncer{idorSyncer(idorSyncerOwnID, idorSyncerTenantOrgID)}}
		service, _ := newSyncerIDORService(repo)
		recorder := serveSyncerIDOR(t, idorSyncerRouter(service),
			idorSyncerRequest(http.MethodGet, "/api/v1/syncers/"+idorSyncerOwnID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"organizationId":"`+idorSyncerTenantOrgID+`"`)
		assert.NotContains(t, recorder.Body.String(), idorSyncerPassword)
	})

	t.Run("patch", func(t *testing.T) {
		repo := &idorSyncerRepo{syncers: []*domain.Syncer{idorSyncer(idorSyncerOwnID, idorSyncerTenantOrgID)}}
		service, records := newSyncerIDORService(repo)
		recorder := serveSyncerIDOR(t, idorSyncerRouter(service),
			idorSyncerRequest(http.MethodPatch, "/api/v1/syncers/"+idorSyncerOwnID,
				strings.NewReader(`{"displayName":"renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, idorSyncerTenantOrgID, records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		repo := &idorSyncerRepo{syncers: []*domain.Syncer{idorSyncer(idorSyncerOwnID, idorSyncerTenantOrgID)}}
		service, records := newSyncerIDORService(repo)
		recorder := serveSyncerIDOR(t, idorSyncerRouter(service),
			idorSyncerRequest(http.MethodDelete, "/api/v1/syncers/"+idorSyncerOwnID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Empty(t, repo.syncers)
		require.Len(t, records.created, 1)
		assert.Equal(t, idorSyncerTenantOrgID, records.created[0].OrganizationID, "delete audit must carry the syncer organization")
	})
}

// TestSyncerByID_PlatformAdminCanCrossTenant pins the single legitimate
// override: super_admin/system scopes may name another tenant explicitly.
func TestSyncerByID_PlatformAdminCanCrossTenant(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &idorSyncerRepo{syncers: []*domain.Syncer{idorSyncer(idorSyncerForeignID, idorSyncerForeignOrgID)}}
		service, _ := newSyncerIDORService(repo)
		recorder := serveSyncerIDOR(t, idorSyncerRouter(service),
			idorSyncerPlatformRequest(http.MethodGet, "/api/v1/syncers/"+idorSyncerForeignID+"?organizationId="+idorSyncerForeignOrgID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), idorSyncerForeignOrgID)
	})

	t.Run("patch keeps the syncer tenant", func(t *testing.T) {
		repo := &idorSyncerRepo{syncers: []*domain.Syncer{idorSyncer(idorSyncerForeignID, idorSyncerForeignOrgID)}}
		service, records := newSyncerIDORService(repo)
		recorder := serveSyncerIDOR(t, idorSyncerRouter(service),
			idorSyncerPlatformRequest(http.MethodPatch, "/api/v1/syncers/"+idorSyncerForeignID+"?organizationId="+idorSyncerForeignOrgID,
				strings.NewReader(`{"displayName":"admin-renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCnt)
		require.Len(t, repo.syncers, 1)
		assert.Equal(t, idorSyncerForeignOrgID, repo.syncers[0].OrganizationID)
		require.Len(t, records.created, 1)
		assert.Equal(t, idorSyncerForeignOrgID, records.created[0].OrganizationID)
	})

	t.Run("delete audits the syncer tenant", func(t *testing.T) {
		repo := &idorSyncerRepo{syncers: []*domain.Syncer{idorSyncer(idorSyncerForeignID, idorSyncerForeignOrgID)}}
		service, records := newSyncerIDORService(repo)
		recorder := serveSyncerIDOR(t, idorSyncerRouter(service),
			idorSyncerPlatformRequest(http.MethodDelete, "/api/v1/syncers/"+idorSyncerForeignID+"?organizationId="+idorSyncerForeignOrgID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, idorSyncerForeignOrgID, records.created[0].OrganizationID)
	})
}
