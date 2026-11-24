package coupons

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	apppkg "github.com/guliangiam/guliangiam/internal/app/coupons"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/coupons"
	domainmock "github.com/guliangiam/guliangiam/internal/domain/coupons/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	trustedOrg = "11111111-1111-1111-1111-111111111111"
	forgedOrg  = "22222222-2222-2222-2222-222222222222"
)

func forgedOrgRequest(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req.WithContext(tenantctx.WithTenantScope(req.Context(), trustedOrg, "session-org", tenantctx.TenantFilterStrict))
}

func forgedOrgCouponRouter(t *testing.T, ctrl *gomock.Controller, keyword string) (*domainmock.MockRepository, chi.Router) {
	t.Helper()
	uow := appuowmock.NewMockCommerceUoW(ctrl)
	repo := domainmock.NewMockRepository(ctrl)
	uow.EXPECT().Coupons().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, q domain.ListQuery) ([]*domain.Coupon, int64, error) {
			assert.Equal(t, trustedOrg, q.OrganizationID)
			assert.Equal(t, keyword, q.Keyword)
			return nil, 0, nil
		})

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: apppkg.Service{UOW: uow}})
	return repo, router
}

// TestListCoupons_ForgedQueryOrganizationIDIgnored pins GET /coupons.
func TestListCoupons_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	_, router := forgedOrgCouponRouter(t, ctrl, "")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet,
		"/api/v1/coupons?organizationId="+forgedOrg, ""))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestValidateCoupon_ForgedBodyOrganizationIDIgnored pins the body-org write/read
// surface: the coupon lookup must use the trusted tenant.
// BE-C-07 后校验走 CodeLookup.GetByCode 精确匹配（不再 List 关键词搜索）。
func TestValidateCoupon_ForgedBodyOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := domainmock.NewMockRepository(ctrl)
	lookup := &exactCodeLookupStub{MockRepository: repo}
	uow := appuowmock.NewMockCommerceUoW(ctrl)
	uow.EXPECT().Coupons().Return(lookup).AnyTimes()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: apppkg.Service{UOW: uow}})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost,
		"/api/v1/coupons/validate", `{"organizationId":"`+forgedOrg+`","code":"SAVE10"}`))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, trustedOrg, lookup.gotOrg, "validate must use the trusted tenant, not the body organizationId")
	assert.Equal(t, "SAVE10", lookup.gotCode)
}

type exactCodeLookupStub struct {
	*domainmock.MockRepository
	gotOrg  string
	gotCode string
}

func (s *exactCodeLookupStub) GetByCode(_ context.Context, organizationID, code string) (*domain.Coupon, error) {
	s.gotOrg, s.gotCode = organizationID, code
	return nil, apperrors.NewNotFound("coupon not found")
}
