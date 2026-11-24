package subscriptions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	appsubscriptions "github.com/guliangiam/guliangiam/internal/app/subscriptions"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/subscriptions"
	subscriptionsmock "github.com/guliangiam/guliangiam/internal/domain/subscriptions/mock"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func trustedTenantRequest(method, target string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	ctx := tenantctx.WithTenantScope(req.Context(), "11111111-1111-1111-1111-111111111111", "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333", OrganizationID: "11111111-1111-1111-1111-111111111111",
	})
	return req.WithContext(ctx)
}

// TestListSubscriptions_ForgedQueryOrganizationIDIgnored pins GET /subscriptions.
func TestListSubscriptions_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockCommerceUoW(ctrl)
	repo := subscriptionsmock.NewMockRepository(ctrl)
	uow.EXPECT().Subscriptions().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, q domain.ListQuery) ([]*domain.Subscription, int64, error) {
			assert.Equal(t, "11111111-1111-1111-1111-111111111111", q.OrganizationID)
			return nil, 0, nil
		},
	)

	router := testRouter()
	RegisterChiRoutes(router, Handler{Service: appsubscriptions.Service{UOW: uow}})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, trustedTenantRequest(http.MethodGet,
		"/api/v1/subscriptions?organizationId=22222222-2222-2222-2222-222222222222"))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestListSubscriptionsWithoutTenantScopeRejected pins the empty-org fail-fast.
func TestListSubscriptionsWithoutTenantScopeRejected(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, Handler{})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions", nil))
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}
