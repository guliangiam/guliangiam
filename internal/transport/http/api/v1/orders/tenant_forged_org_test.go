package orders

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	apporders "github.com/guliangiam/guliangiam/internal/app/orders"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/orders"
	ordersmock "github.com/guliangiam/guliangiam/internal/domain/orders/mock"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	forgedOrgSessionID = "11111111-1111-1111-1111-111111111111"
	forgedOrgRequestID = "22222222-2222-2222-2222-222222222222"
)

func withTrustedTenant(req *http.Request) *http.Request {
	ctx := tenantctx.WithTenantScope(req.Context(), forgedOrgSessionID, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "user-1", OrganizationID: forgedOrgSessionID,
	})
	return req.WithContext(ctx)
}

func forgedOrgOrderRouter(t *testing.T, ctrl *gomock.Controller, trustedUser string) (*ordersmock.MockRepository, http.Handler) {
	t.Helper()
	uow := appuowmock.NewMockCommerceUoW(ctrl)
	repo := ordersmock.NewMockRepository(ctrl)
	uow.EXPECT().Orders().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, q domain.ListQuery) ([]*domain.Order, int64, error) {
			assert.Equal(t, forgedOrgSessionID, q.OrganizationID)
			assert.Equal(t, trustedUser, q.User)
			return nil, 0, nil
		},
	)

	router := testRouter()
	RegisterChiRoutes(router, Handler{Service: apporders.Service{UOW: uow}})
	return repo, router
}

// TestListOrders_ForgedQueryOrganizationIDIgnored pins GET /orders.
func TestListOrders_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	_, router := forgedOrgOrderRouter(t, ctrl, "")

	req := withTrustedTenant(httptest.NewRequest(http.MethodGet,
		"/api/v1/orders?organizationId="+forgedOrgRequestID+"&keyword=pro", nil))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestGetUserOrders_ForgedQueryOrganizationIDIgnored pins the user-scoped route:
// the forged org is dropped while the user filter is preserved.
func TestGetUserOrders_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	_, router := forgedOrgOrderRouter(t, ctrl, "user-1")

	req := withTrustedTenant(httptest.NewRequest(http.MethodGet,
		"/api/v1/orders/user-orders?organizationId="+forgedOrgRequestID+"&user=user-1", nil))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestListOrdersWithoutTenantScopeRejected pins the empty-org fail-fast.
func TestListOrdersWithoutTenantScopeRejected(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, Handler{})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil))
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}
