package radiusaccountings

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appradiusaccountings "github.com/guliangiam/guliangiam/internal/app/radiusaccountings"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domain "github.com/guliangiam/guliangiam/internal/domain/radiusaccountings"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	idorRATenantOrg  = "11111111-1111-1111-1111-111111111111"
	idorRAForeignOrg = "22222222-2222-2222-2222-222222222222"
	idorRAForeignID  = "foreign-radius-accounting"
	idorRAOwnID      = "own-radius-accounting"
)

type idorRAClock struct{}

func (idorRAClock) Now() time.Time { return time.Unix(0, 0).UTC() }

// idorRARepo is a tenant-aware in-memory repository. Write counts stay
// observable so a rejected cross-tenant request proves it never wrote.
type idorRARepo struct {
	items     []*domain.RadiusAccounting
	updateCnt int
	deleteCnt int
}

func (r *idorRARepo) GetByID(_ context.Context, id, organizationID string) (*domain.RadiusAccounting, error) {
	for _, item := range r.items {
		if item.ID == id && item.OrganizationID == organizationID {
			return item, nil
		}
	}
	return nil, apperrors.NewNotFound("radius accounting not found")
}

func (r *idorRARepo) List(_ context.Context, _ domain.ListQuery) ([]*domain.RadiusAccounting, int64, error) {
	return r.items, int64(len(r.items)), nil
}

func (r *idorRARepo) Create(_ context.Context, item *domain.RadiusAccounting) error {
	r.items = append(r.items, item)
	return nil
}

func (r *idorRARepo) Update(_ context.Context, item *domain.RadiusAccounting) error {
	r.updateCnt++
	for i, existing := range r.items {
		if existing.ID == item.ID && existing.OrganizationID == item.OrganizationID {
			r.items[i] = item
			return nil
		}
	}
	return apperrors.NewNotFound("radius accounting not found")
}

func (r *idorRARepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCnt++
	for i, existing := range r.items {
		if existing.ID == id && existing.OrganizationID == organizationID {
			r.items = append(r.items[:i], r.items[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("radius accounting not found")
}

type idorRARecords struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *idorRARecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

type idorRAUOW struct {
	appuow.UnitOfWork
	repo    domain.Repository
	records *idorRARecords
}

func (u *idorRAUOW) RadiusAccountings() domain.Repository { return u.repo }
func (u *idorRAUOW) Records() recordsdomain.Repository    { return u.records }
func (u *idorRAUOW) Outbox() domainoutbox.Repository      { return nil }
func (u *idorRAUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func newIdorRAService(repo domain.Repository) (appradiusaccountings.Service, *idorRARecords) {
	records := &idorRARecords{}
	service := appradiusaccountings.Service{
		Clock: idorRAClock{},
		UOW:   &idorRAUOW{repo: repo, records: records},
	}
	return service, records
}

func idorRARequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), idorRATenantOrg, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333", OrganizationID: idorRATenantOrg,
	})
	return req.WithContext(ctx)
}

func idorRAPlatformRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), idorRATenantOrg, "session-org", tenantctx.TenantFilterSuperAdmin)
	return req.WithContext(ctx)
}

func idorRABareRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func idorRARouter(service appradiusaccountings.Service) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return router
}

func idorRA(id, org string) *domain.RadiusAccounting {
	return &domain.RadiusAccounting{ID: id, OrganizationID: org, Name: "tenant-accounting", Revision: 1}
}

func serveIdorRA(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// TestRadiusAccountingByID_ForeignTenantRejected pins that GET/PATCH/DELETE
// resolve the authenticated tenant, never the caller-supplied organizationId,
// and that a foreign accounting looks exactly like a missing one: 404, no
// write, no audit record, and the row untouched.
func TestRadiusAccountingByID_ForeignTenantRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/radius-accountings/" + idorRAForeignID + "?organizationId=" + idorRAForeignOrg, ""},
		{"patch", http.MethodPatch, "/api/v1/radius-accountings/" + idorRAForeignID + "?organizationId=" + idorRAForeignOrg, `{"acctSessionTime":60}`},
		{"delete", http.MethodDelete, "/api/v1/radius-accountings/" + idorRAForeignID + "?organizationId=" + idorRAForeignOrg, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &idorRARepo{items: []*domain.RadiusAccounting{idorRA(idorRAForeignID, idorRAForeignOrg)}}
			service, records := newIdorRAService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveIdorRA(t, idorRARouter(service), idorRARequest(tt.method, tt.target, body))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign accounting must be indistinguishable from missing")
			assert.Zero(t, repo.updateCnt)
			assert.Zero(t, repo.deleteCnt)
			assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
			require.Len(t, repo.items, 1, "foreign accounting row must remain untouched")
		})
	}
}

// TestRadiusAccountingByID_WithoutTenantScopeRejected pins the fail-fast.
func TestRadiusAccountingByID_WithoutTenantScopeRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/radius-accountings/" + idorRAOwnID, ""},
		{"patch", http.MethodPatch, "/api/v1/radius-accountings/" + idorRAOwnID, `{"acctSessionTime":60}`},
		{"delete", http.MethodDelete, "/api/v1/radius-accountings/" + idorRAOwnID, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &idorRARepo{items: []*domain.RadiusAccounting{idorRA(idorRAOwnID, idorRATenantOrg)}}
			service, records := newIdorRAService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveIdorRA(t, idorRARouter(service), idorRABareRequest(tt.method, tt.target, body))

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, repo.updateCnt)
			assert.Zero(t, repo.deleteCnt)
			assert.Empty(t, records.created)
		})
	}
}

// TestRadiusAccountingByID_OwnTenantSucceeds pins same-tenant operations and
// that a delete audits the accounting's real organization.
func TestRadiusAccountingByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &idorRARepo{items: []*domain.RadiusAccounting{idorRA(idorRAOwnID, idorRATenantOrg)}}
		service, _ := newIdorRAService(repo)
		recorder := serveIdorRA(t, idorRARouter(service),
			idorRARequest(http.MethodGet, "/api/v1/radius-accountings/"+idorRAOwnID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), idorRATenantOrg)
	})

	t.Run("patch", func(t *testing.T) {
		repo := &idorRARepo{items: []*domain.RadiusAccounting{idorRA(idorRAOwnID, idorRATenantOrg)}}
		service, records := newIdorRAService(repo)
		recorder := serveIdorRA(t, idorRARouter(service),
			idorRARequest(http.MethodPatch, "/api/v1/radius-accountings/"+idorRAOwnID,
				strings.NewReader(`{"acctSessionTime":60}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, idorRATenantOrg, records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		repo := &idorRARepo{items: []*domain.RadiusAccounting{idorRA(idorRAOwnID, idorRATenantOrg)}}
		service, records := newIdorRAService(repo)
		recorder := serveIdorRA(t, idorRARouter(service),
			idorRARequest(http.MethodDelete, "/api/v1/radius-accountings/"+idorRAOwnID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Empty(t, repo.items)
		require.Len(t, records.created, 1)
		assert.Equal(t, idorRATenantOrg, records.created[0].OrganizationID, "delete audit must carry the accounting organization")
	})
}

// TestRadiusAccountingByID_PlatformAdminCanCrossTenant pins the single
// legitimate override and the audit organization on a cross-tenant delete.
func TestRadiusAccountingByID_PlatformAdminCanCrossTenant(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &idorRARepo{items: []*domain.RadiusAccounting{idorRA(idorRAForeignID, idorRAForeignOrg)}}
		service, _ := newIdorRAService(repo)
		recorder := serveIdorRA(t, idorRARouter(service),
			idorRAPlatformRequest(http.MethodGet, "/api/v1/radius-accountings/"+idorRAForeignID+"?organizationId="+idorRAForeignOrg, nil))
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), idorRAForeignOrg)
	})

	t.Run("delete audits the accounting tenant", func(t *testing.T) {
		repo := &idorRARepo{items: []*domain.RadiusAccounting{idorRA(idorRAForeignID, idorRAForeignOrg)}}
		service, records := newIdorRAService(repo)
		recorder := serveIdorRA(t, idorRARouter(service),
			idorRAPlatformRequest(http.MethodDelete, "/api/v1/radius-accountings/"+idorRAForeignID+"?organizationId="+idorRAForeignOrg, nil))
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, idorRAForeignOrg, records.created[0].OrganizationID)
	})
}
