package coupons

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appcoupons "github.com/guliangiam/guliangiam/internal/app/coupons"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/coupons"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	tenantOrgID     = "11111111-1111-1111-1111-111111111111"
	foreignOrgID    = "22222222-2222-2222-2222-222222222222"
	ownCouponID     = "own-coupon"
	foreignCouponID = "foreign-coupon"
)

type idorClock struct{}

func (idorClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type idorUUID struct{}

func (idorUUID) NewString() (string, error) { return "generated-coupon-id", nil }

func tenantRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), tenantOrgID, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333", OrganizationID: tenantOrgID,
	})
	return req.WithContext(ctx)
}

// platformRequest simulates a platform administrator scope: only this mode may
// narrow the tenant with an explicit organizationId.
func platformRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), tenantOrgID, "session-org", tenantctx.TenantFilterSuperAdmin)
	return req.WithContext(ctx)
}

// bareRequest carries neither a session nor a tenant scope.
func bareRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// idorCouponRepo is a tenant-aware in-memory repository. Call counts stay
// observable so a rejected cross-tenant request proves it never reached a write.
type idorCouponRepo struct {
	coupons   []*domain.Coupon
	getOrgIDs []string
	updateCnt int
	deleteCnt int
}

func (r *idorCouponRepo) GetByID(_ context.Context, id string) (*domain.Coupon, error) {
	for _, c := range r.coupons {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, apperrors.NewNotFound("coupon not found")
}

func (r *idorCouponRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Coupon, error) {
	r.getOrgIDs = append(r.getOrgIDs, organizationID)
	for _, c := range r.coupons {
		if c.ID == id && c.OrganizationID == organizationID {
			return c, nil
		}
	}
	return nil, apperrors.NewNotFound("coupon not found")
}

func (r *idorCouponRepo) List(_ context.Context, _ domain.ListQuery) ([]*domain.Coupon, int64, error) {
	return r.coupons, int64(len(r.coupons)), nil
}

func (r *idorCouponRepo) Create(_ context.Context, coupon *domain.Coupon) error {
	r.coupons = append(r.coupons, coupon)
	return nil
}

func (r *idorCouponRepo) Update(_ context.Context, coupon *domain.Coupon) error {
	r.updateCnt++
	for i, c := range r.coupons {
		if c.ID == coupon.ID && c.OrganizationID == coupon.OrganizationID {
			r.coupons[i] = coupon
			return nil
		}
	}
	return apperrors.NewNotFound("coupon not found")
}

func (r *idorCouponRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCnt++
	for i, c := range r.coupons {
		if c.ID == id && c.OrganizationID == organizationID {
			r.coupons = append(r.coupons[:i], r.coupons[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("coupon not found")
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

func (u *idorUOW) Coupons() domain.Repository { return u.repo }
func (u *idorUOW) Records() recordsdomain.Repository {
	return u.records
}
func (u *idorUOW) Outbox() domainoutbox.Repository { return nil }
func (u *idorUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func newTenantService(repo domain.Repository) (appcoupons.Service, *idorRecords) {
	records := &idorRecords{}
	return appcoupons.Service{UOW: &idorUOW{repo: repo, records: records}, Clock: idorClock{}, UUID: idorUUID{}}, records
}

func couponFixture(id, orgID string) *domain.Coupon {
	return &domain.Coupon{ID: id, OrganizationID: orgID, Name: "tenant-coupon", Revision: 1}
}

func tenantRouter(service appcoupons.Service) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return router
}

func serveTenant(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// TestCouponByID_ForeignTenantRejected pins that every by-ID operation resolves
// the authenticated tenant, never the caller-supplied organizationId, and that
// a foreign coupon looks exactly like a missing one: 404, no write, no audit.
func TestCouponByID_ForeignTenantRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/coupons/" + foreignCouponID + "?organizationId=" + foreignOrgID, ""},
		{"patch", http.MethodPatch, "/api/v1/coupons/" + foreignCouponID + "?organizationId=" + foreignOrgID, `{"displayName":"hijacked","revision":1}`},
		{"delete", http.MethodDelete, "/api/v1/coupons/" + foreignCouponID + "?organizationId=" + foreignOrgID, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &idorCouponRepo{coupons: []*domain.Coupon{couponFixture(foreignCouponID, foreignOrgID)}}
			service, records := newTenantService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveTenant(t, tenantRouter(service), tenantRequest(tt.method, tt.target, body))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign coupon must be indistinguishable from missing")
			assert.Zero(t, repo.updateCnt, "foreign tenant must not update the coupon")
			assert.Zero(t, repo.deleteCnt, "foreign tenant must not delete the coupon")
			assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
			assert.Equal(t, []string{tenantOrgID}, repo.getOrgIDs, "repository must receive the trusted session tenant")
			require.Len(t, repo.coupons, 1, "foreign coupon row must remain untouched")
		})
	}
}

// TestCouponByID_WithoutTenantScopeRejected pins the fail-fast: when neither the
// session nor a platform-admin target provides a tenant, every by-ID route is
// rejected before touching the repository (MySQL has no RLS).
func TestCouponByID_WithoutTenantScopeRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		body   string
	}{
		{"get", http.MethodGet, ""},
		{"patch", http.MethodPatch, `{}`},
		{"delete", http.MethodDelete, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &idorCouponRepo{coupons: []*domain.Coupon{couponFixture(ownCouponID, tenantOrgID)}}
			service, records := newTenantService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveTenant(t, tenantRouter(service), bareRequest(tt.method, "/api/v1/coupons/"+ownCouponID, body))

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, repo.updateCnt)
			assert.Zero(t, repo.deleteCnt)
			assert.Empty(t, records.created)
			assert.Empty(t, repo.getOrgIDs, "an empty tenant must not reach the repository")
		})
	}
}

// TestCouponByID_OwnTenantSucceeds pins that the same operations still work for
// the authenticated tenant and that a delete audits the real organization.
func TestCouponByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &idorCouponRepo{coupons: []*domain.Coupon{couponFixture(ownCouponID, tenantOrgID)}}
		service, _ := newTenantService(repo)
		recorder := serveTenant(t, tenantRouter(service), tenantRequest(http.MethodGet, "/api/v1/coupons/"+ownCouponID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"organizationId":"`+tenantOrgID+`"`)
	})

	t.Run("patch", func(t *testing.T) {
		repo := &idorCouponRepo{coupons: []*domain.Coupon{couponFixture(ownCouponID, tenantOrgID)}}
		service, records := newTenantService(repo)
		recorder := serveTenant(t, tenantRouter(service),
			tenantRequest(http.MethodPatch, "/api/v1/coupons/"+ownCouponID, strings.NewReader(`{"displayName":"renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, tenantOrgID, records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		repo := &idorCouponRepo{coupons: []*domain.Coupon{couponFixture(ownCouponID, tenantOrgID)}}
		service, records := newTenantService(repo)
		recorder := serveTenant(t, tenantRouter(service), tenantRequest(http.MethodDelete, "/api/v1/coupons/"+ownCouponID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Empty(t, repo.coupons)
		require.Len(t, records.created, 1)
		assert.Equal(t, tenantOrgID, records.created[0].OrganizationID, "delete audit must carry the coupon organization")
	})
}

// TestCouponByID_PlatformAdminCanCrossTenant pins the single legitimate override:
// super_admin/system scopes may name another tenant explicitly.
func TestCouponByID_PlatformAdminCanCrossTenant(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &idorCouponRepo{coupons: []*domain.Coupon{couponFixture(foreignCouponID, foreignOrgID)}}
		service, _ := newTenantService(repo)
		recorder := serveTenant(t, tenantRouter(service),
			platformRequest(http.MethodGet, "/api/v1/coupons/"+foreignCouponID+"?organizationId="+foreignOrgID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), foreignOrgID)
	})

	t.Run("delete audits the coupon tenant", func(t *testing.T) {
		repo := &idorCouponRepo{coupons: []*domain.Coupon{couponFixture(foreignCouponID, foreignOrgID)}}
		service, records := newTenantService(repo)
		recorder := serveTenant(t, tenantRouter(service),
			platformRequest(http.MethodDelete, "/api/v1/coupons/"+foreignCouponID+"?organizationId="+foreignOrgID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, foreignOrgID, records.created[0].OrganizationID)
	})
}
