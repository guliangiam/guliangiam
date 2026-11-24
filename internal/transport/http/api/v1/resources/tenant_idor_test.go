package resources

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appresources "github.com/guliangiam/guliangiam/internal/app/resources"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	domain "github.com/guliangiam/guliangiam/internal/domain/resources"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	idorTenantOrg  = "11111111-1111-1111-1111-111111111111"
	idorForeignOrg = "22222222-2222-2222-2222-222222222222"
	idorResourceID = "33333333-3333-3333-3333-333333333333"
)

type idorClock struct{}

func (idorClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type idorRepo struct {
	items       []*domain.Resource
	getOrgs     []string
	updateCalls int
	deleteCalls int
}

func (r *idorRepo) GetByID(_ context.Context, id, organizationID string) (*domain.Resource, error) {
	r.getOrgs = append(r.getOrgs, organizationID)
	for _, item := range r.items {
		if item.ID == id && item.OrganizationID == organizationID {
			return item, nil
		}
	}
	return nil, apperrors.NewNotFound("resource not found")
}

func (r *idorRepo) List(context.Context, domain.ListQuery) ([]*domain.Resource, int64, error) {
	return r.items, int64(len(r.items)), nil
}

func (r *idorRepo) Create(_ context.Context, resource *domain.Resource) error {
	r.items = append(r.items, resource)
	return nil
}

func (r *idorRepo) Update(_ context.Context, resource *domain.Resource) error {
	r.updateCalls++
	for i, item := range r.items {
		if item.ID == resource.ID && item.OrganizationID == resource.OrganizationID {
			r.items[i] = resource
			return nil
		}
	}
	return apperrors.NewNotFound("resource not found")
}

func (r *idorRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCalls++
	for i, item := range r.items {
		if item.ID == id && item.OrganizationID == organizationID {
			r.items = append(r.items[:i], r.items[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("resource not found")
}

type idorRecords struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *idorRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

type idorUOW struct {
	appuow.UnitOfWork
	repo    domain.Repository
	records *idorRecords
}

func (u *idorUOW) Resources() domain.Repository      { return u.repo }
func (u *idorUOW) Records() recordsdomain.Repository { return u.records }
func (u *idorUOW) Outbox() domainoutbox.Repository   { return nil }
func (u *idorUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

type idorHarness struct {
	repo    *idorRepo
	records *idorRecords
	router  chi.Router
}

func newIDORHarness(items ...*domain.Resource) *idorHarness {
	repo := &idorRepo{items: items}
	records := &idorRecords{}
	uow := &idorUOW{repo: repo, records: records}
	service := appresources.Service{Clock: idorClock{}, UOW: uow}
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return &idorHarness{repo: repo, records: records, router: router}
}

func idorRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), idorTenantOrg, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "44444444-4444-4444-4444-444444444444", OrganizationID: idorTenantOrg,
	})
	return req.WithContext(ctx)
}

func idorPlatformRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req.WithContext(tenantctx.WithTenantScope(req.Context(), "", "", tenantctx.TenantFilterSuperAdmin))
}

func idorBody(t *testing.T, body string) io.Reader {
	t.Helper()
	if body == "" {
		return nil
	}
	return strings.NewReader(body)
}

func serveIDOR(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func resourceFor(organizationID string) *domain.Resource {
	return &domain.Resource{ID: idorResourceID, OrganizationID: organizationID, Name: "tenant-resource", Revision: 1}
}

func TestResourceByID_ForeignTenantRejected(t *testing.T) {
	for _, tt := range []struct {
		name   string
		method string
		body   string
	}{
		{"get", http.MethodGet, ""},
		{"patch", http.MethodPatch, `{"displayName":"hijacked","revision":1}`},
		{"delete", http.MethodDelete, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newIDORHarness(resourceFor(idorForeignOrg))
			recorder := serveIDOR(t, h.router, idorRequest(tt.method,
				"/api/v1/resources/"+idorResourceID+"?organizationId="+idorForeignOrg, idorBody(t, tt.body)))

			assert.Equal(t, http.StatusNotFound, recorder.Code)
			require.NotEmpty(t, h.repo.getOrgs)
			for _, org := range h.repo.getOrgs {
				assert.Equal(t, idorTenantOrg, org, "forged query organizationId must be ignored")
			}
			assert.Zero(t, h.repo.updateCalls)
			assert.Zero(t, h.repo.deleteCalls)
			assert.Empty(t, h.records.created)
			require.Len(t, h.repo.items, 1)
		})
	}
}

func TestResourceByID_WithoutTenantScopeRejected(t *testing.T) {
	for _, tt := range []struct {
		name   string
		method string
		body   string
	}{
		{"get", http.MethodGet, ""},
		{"patch", http.MethodPatch, `{}`},
		{"delete", http.MethodDelete, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newIDORHarness(resourceFor(idorTenantOrg))
			req := httptest.NewRequest(tt.method, "/api/v1/resources/"+idorResourceID, idorBody(t, tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := serveIDOR(t, h.router, req)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Empty(t, h.repo.getOrgs, "an empty tenant must not reach the repository")
			assert.Zero(t, h.repo.updateCalls)
			assert.Zero(t, h.repo.deleteCalls)
			assert.Empty(t, h.records.created)
		})
	}
}

func TestResourceByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		h := newIDORHarness(resourceFor(idorTenantOrg))
		recorder := serveIDOR(t, h.router, idorRequest(http.MethodGet, "/api/v1/resources/"+idorResourceID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "tenant-resource")
		assert.Equal(t, []string{idorTenantOrg}, h.repo.getOrgs)
	})

	t.Run("patch", func(t *testing.T) {
		h := newIDORHarness(resourceFor(idorTenantOrg))
		recorder := serveIDOR(t, h.router, idorRequest(http.MethodPatch,
			"/api/v1/resources/"+idorResourceID, strings.NewReader(`{"displayName":"renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.repo.updateCalls)
		require.Len(t, h.records.created, 1)
		assert.Equal(t, idorTenantOrg, h.records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		h := newIDORHarness(resourceFor(idorTenantOrg))
		recorder := serveIDOR(t, h.router, idorRequest(http.MethodDelete, "/api/v1/resources/"+idorResourceID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.repo.deleteCalls)
		assert.Empty(t, h.repo.items)
		require.Len(t, h.records.created, 1)
		assert.Equal(t, idorTenantOrg, h.records.created[0].OrganizationID,
			"delete audit must carry the resource organization")
	})
}

func TestResourceByID_PlatformAdminCanTargetTenant(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		h := newIDORHarness(resourceFor(idorForeignOrg))
		recorder := serveIDOR(t, h.router, idorPlatformRequest(http.MethodGet,
			"/api/v1/resources/"+idorResourceID+"?organizationId="+idorForeignOrg, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, []string{idorForeignOrg}, h.repo.getOrgs)
	})

	t.Run("empty target rejected", func(t *testing.T) {
		h := newIDORHarness(resourceFor(idorForeignOrg))
		recorder := serveIDOR(t, h.router, idorPlatformRequest(http.MethodGet,
			"/api/v1/resources/"+idorResourceID, nil))

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Empty(t, h.repo.getOrgs)
	})
}
