package pricings

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	apppricings "github.com/guliangiam/guliangiam/internal/app/pricings"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domain "github.com/guliangiam/guliangiam/internal/domain/pricings"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	idorTenantOrg  = "11111111-1111-1111-1111-111111111111"
	idorForeignOrg = "22222222-2222-2222-2222-222222222222"
	idorPricingID  = "33333333-3333-3333-3333-333333333333"
)

type idorClock struct{}

func (idorClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type idorRepo struct {
	pricings    []*domain.Pricing
	getOrgs     []string
	updateCalls int
	deleteCalls int
}

func (r *idorRepo) GetByID(_ context.Context, id, organizationID string) (*domain.Pricing, error) {
	r.getOrgs = append(r.getOrgs, organizationID)
	for _, p := range r.pricings {
		if p.ID == id && p.OrganizationID == organizationID {
			return p, nil
		}
	}
	return nil, apperrors.NewNotFound("pricing not found")
}

func (r *idorRepo) List(context.Context, domain.ListQuery) ([]*domain.Pricing, int64, error) {
	return r.pricings, int64(len(r.pricings)), nil
}

func (r *idorRepo) Create(_ context.Context, pricing *domain.Pricing) error {
	r.pricings = append(r.pricings, pricing)
	return nil
}

func (r *idorRepo) Update(_ context.Context, pricing *domain.Pricing) error {
	r.updateCalls++
	for i, p := range r.pricings {
		if p.ID == pricing.ID && p.OrganizationID == pricing.OrganizationID {
			r.pricings[i] = pricing
			return nil
		}
	}
	return apperrors.NewNotFound("pricing not found")
}

func (r *idorRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCalls++
	for i, p := range r.pricings {
		if p.ID == id && p.OrganizationID == organizationID {
			r.pricings = append(r.pricings[:i], r.pricings[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("pricing not found")
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

func (u *idorUOW) Pricings() domain.Repository       { return u.repo }
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

func newIDORHarness(items ...*domain.Pricing) *idorHarness {
	repo := &idorRepo{pricings: items}
	records := &idorRecords{}
	uow := &idorUOW{repo: repo, records: records}
	service := apppricings.Service{Clock: idorClock{}, UOW: uow}
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

func pricingFor(organizationID string) *domain.Pricing {
	return &domain.Pricing{ID: idorPricingID, OrganizationID: organizationID, Name: "tenant-pricing", Revision: 1}
}

func TestPricingByID_ForeignTenantRejected(t *testing.T) {
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
			h := newIDORHarness(pricingFor(idorForeignOrg))
			recorder := serveIDOR(t, h.router, idorRequest(tt.method,
				"/api/v1/pricings/"+idorPricingID+"?organizationId="+idorForeignOrg, idorBody(t, tt.body)))

			assert.Equal(t, http.StatusNotFound, recorder.Code)
			require.NotEmpty(t, h.repo.getOrgs)
			for _, org := range h.repo.getOrgs {
				assert.Equal(t, idorTenantOrg, org, "forged query organizationId must be ignored")
			}
			assert.Zero(t, h.repo.updateCalls)
			assert.Zero(t, h.repo.deleteCalls)
			assert.Empty(t, h.records.created)
			require.Len(t, h.repo.pricings, 1)
		})
	}
}

func TestPricingByID_WithoutTenantScopeRejected(t *testing.T) {
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
			h := newIDORHarness(pricingFor(idorTenantOrg))
			req := httptest.NewRequest(tt.method, "/api/v1/pricings/"+idorPricingID, idorBody(t, tt.body))
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

func TestPricingByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		h := newIDORHarness(pricingFor(idorTenantOrg))
		recorder := serveIDOR(t, h.router, idorRequest(http.MethodGet, "/api/v1/pricings/"+idorPricingID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "tenant-pricing")
		assert.Equal(t, []string{idorTenantOrg}, h.repo.getOrgs)
	})

	t.Run("patch", func(t *testing.T) {
		h := newIDORHarness(pricingFor(idorTenantOrg))
		recorder := serveIDOR(t, h.router, idorRequest(http.MethodPatch,
			"/api/v1/pricings/"+idorPricingID, strings.NewReader(`{"displayName":"renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.repo.updateCalls)
		require.Len(t, h.records.created, 1)
		assert.Equal(t, idorTenantOrg, h.records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		h := newIDORHarness(pricingFor(idorTenantOrg))
		recorder := serveIDOR(t, h.router, idorRequest(http.MethodDelete, "/api/v1/pricings/"+idorPricingID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.repo.deleteCalls)
		assert.Empty(t, h.repo.pricings)
		require.Len(t, h.records.created, 1)
		assert.Equal(t, idorTenantOrg, h.records.created[0].OrganizationID,
			"delete audit must carry the pricing organization")
	})
}

func TestPricingByID_PlatformAdminCanTargetTenant(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		h := newIDORHarness(pricingFor(idorForeignOrg))
		recorder := serveIDOR(t, h.router, idorPlatformRequest(http.MethodGet,
			"/api/v1/pricings/"+idorPricingID+"?organizationId="+idorForeignOrg, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, []string{idorForeignOrg}, h.repo.getOrgs)
	})

	t.Run("empty target rejected", func(t *testing.T) {
		h := newIDORHarness(pricingFor(idorForeignOrg))
		recorder := serveIDOR(t, h.router, idorPlatformRequest(http.MethodGet,
			"/api/v1/pricings/"+idorPricingID, nil))

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Empty(t, h.repo.getOrgs)
	})
}
