package pricings

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	apppkg "github.com/guliangiam/guliangiam/internal/app/pricings"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/pricings"
	domainmock "github.com/guliangiam/guliangiam/internal/domain/pricings/mock"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	trustedOrg = "11111111-1111-1111-1111-111111111111"
	forgedOrg  = "22222222-2222-2222-2222-222222222222"
)

func forgedOrgRequest(target string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	return req.WithContext(tenantctx.WithTenantScope(req.Context(), trustedOrg, "session-org", tenantctx.TenantFilterStrict))
}

// TestListPricings_ForgedQueryOrganizationIDIgnored pins GET /pricings.
func TestListPricings_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockCommerceUoW(ctrl)
	repo := domainmock.NewMockRepository(ctrl)
	uow.EXPECT().Pricings().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, q domain.ListQuery) ([]*domain.Pricing, int64, error) {
			assert.Equal(t, trustedOrg, q.OrganizationID)
			return nil, 0, nil
		})

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: apppkg.Service{UOW: uow}})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest("/api/v1/pricings?organizationId="+forgedOrg))
	require.Equal(t, http.StatusOK, recorder.Code)
}
