package transactions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	apptransactions "github.com/guliangiam/guliangiam/internal/app/transactions"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/transactions"
	transactionsmock "github.com/guliangiam/guliangiam/internal/domain/transactions/mock"
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

// TestListTransactions_ForgedQueryOrganizationIDIgnored pins GET /transactions.
func TestListTransactions_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockAuditUoW(ctrl)
	repo := transactionsmock.NewMockRepository(ctrl)
	uow.EXPECT().Transactions().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, q domain.ListQuery) ([]*domain.Transaction, int64, error) {
			assert.Equal(t, "11111111-1111-1111-1111-111111111111", q.OrganizationID)
			return nil, 0, nil
		},
	)

	router := testRouter()
	RegisterChiRoutes(router, Handler{Service: apptransactions.Service{UOW: uow}})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, trustedTenantRequest(http.MethodGet,
		"/api/v1/transactions?organizationId=22222222-2222-2222-2222-222222222222"))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestListTransactionsWithoutTenantScopeRejected pins the empty-org fail-fast.
func TestListTransactionsWithoutTenantScopeRejected(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, Handler{})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/transactions", nil))
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}
