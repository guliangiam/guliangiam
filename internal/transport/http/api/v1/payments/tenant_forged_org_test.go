package payments

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	paymentsmock "github.com/guliangiam/guliangiam/internal/domain/payments/mock"
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

// withTrustedTenant attaches the authenticated tenant scope the finance
// handlers must trust instead of the caller-supplied organizationId.
func withTrustedTenant(req *http.Request) *http.Request {
	ctx := tenantctx.WithTenantScope(req.Context(), forgedOrgSessionID, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333", OrganizationID: forgedOrgSessionID,
	})
	return req.WithContext(ctx)
}

type forgedOrgTokenVerifier struct{}

func (forgedOrgTokenVerifier) ValidateToken(string) (map[string]interface{}, error) {
	return map[string]interface{}{"username": "alice"}, nil
}

type forgedOrgReconService struct {
	listOrg string
	runOrg  string
}

func (f *forgedOrgReconService) ListBatches(_ context.Context, orgID string, _, _ int) ([]*domain.ReconBatch, int64, error) {
	f.listOrg = orgID
	return nil, 0, nil
}

func (f *forgedOrgReconService) ListItems(context.Context, string, string) ([]*domain.ReconItem, error) {
	return nil, nil
}

func (f *forgedOrgReconService) ResolveItem(context.Context, string, string, string) error {
	return nil
}

func (f *forgedOrgReconService) RunManual(context.Context, string, string) error { return nil }

func (f *forgedOrgReconService) ApproveItem(context.Context, string, string, string) error {
	return nil
}

func (f *forgedOrgReconService) CreateAndRun(_ context.Context, orgID, _ string, _ time.Time, _ time.Time) (string, error) {
	f.runOrg = orgID
	return "rb-trusted", nil
}

var _ apppayments.ReconService = (*forgedOrgReconService)(nil)

type forgedOrgReceiptsRepo struct{ listOrg string }

func (r *forgedOrgReceiptsRepo) GetByPayloadHash(context.Context, string, string) (*domain.CallbackReceipt, error) {
	return nil, nil
}

func (r *forgedOrgReceiptsRepo) GetByEventKey(context.Context, string, string) (*domain.CallbackReceipt, error) {
	return nil, nil
}

func (r *forgedOrgReceiptsRepo) Create(context.Context, *domain.CallbackReceipt) error { return nil }

func (r *forgedOrgReceiptsRepo) Update(context.Context, *domain.CallbackReceipt) error { return nil }

func (r *forgedOrgReceiptsRepo) List(_ context.Context, organizationID string, _, _ int) ([]*domain.CallbackReceipt, int64, error) {
	r.listOrg = organizationID
	return nil, 0, nil
}

// TestListPayments_ForgedQueryOrganizationIDIgnored pins GET /payments.
func TestListPayments_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockCommerceUoW(ctrl)
	repo := paymentsmock.NewMockRepository(ctrl)
	uow.EXPECT().Payments().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, q domain.ListQuery) ([]*domain.Payment, int64, error) {
			assert.Equal(t, forgedOrgSessionID, q.OrganizationID)
			return nil, 0, nil
		},
	)

	router := testRouter()
	RegisterChiRoutes(router, Handler{Service: apppayments.Service{UOW: uow}})

	req := withTrustedTenant(httptest.NewRequest(http.MethodGet,
		"/api/v1/payments?organizationId="+forgedOrgRequestID+"&keyword=pro", nil))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestListUserPayments_ForgedQueryOrganizationIDIgnored pins the JWT-scoped
// user-payments view; the username still comes from the token.
func TestListUserPayments_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockCommerceUoW(ctrl)
	repo := paymentsmock.NewMockRepository(ctrl)
	uow.EXPECT().Payments().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, q domain.ListQuery) ([]*domain.Payment, int64, error) {
			assert.Equal(t, forgedOrgSessionID, q.OrganizationID)
			assert.Equal(t, "alice", q.User)
			return nil, 0, nil
		},
	)

	router := testRouter()
	RegisterChiRoutes(router, Handler{Service: apppayments.Service{UOW: uow}, JWTIssuer: forgedOrgTokenVerifier{}})

	req := withTrustedTenant(httptest.NewRequest(http.MethodGet,
		"/api/v1/payments/user-payments?organizationId="+forgedOrgRequestID, nil))
	req.Header.Set("Authorization", "Bearer test-token")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestListReconBatches_ForgedQueryOrganizationIDIgnored pins the reconciliation
// batch list.
func TestListReconBatches_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	svc := &forgedOrgReconService{}
	router := testRouter()
	RegisterReconChiRoutes(router, NewReconHandler(svc))

	req := withTrustedTenant(httptest.NewRequest(http.MethodGet,
		"/api/v1/pay-recon-batches?organizationId="+forgedOrgRequestID, nil))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, forgedOrgSessionID, svc.listOrg)
}

// TestRunRecon_ForgedBodyOrganizationIDIgnored pins the write side of the recon
// surface: mode 2 names a tenant in the body, which must be clamped.
func TestRunRecon_ForgedBodyOrganizationIDIgnored(t *testing.T) {
	svc := &forgedOrgReconService{}
	router := testRouter()
	RegisterReconChiRoutes(router, NewReconHandler(svc))

	body := `{"organizationId":"` + forgedOrgRequestID + `","channel":"alipay","batchDate":"2026-11-01"}`
	req := withTrustedTenant(httptest.NewRequest(http.MethodPost, "/api/v1/pay-recon:run", strings.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusAccepted, recorder.Code)
	assert.Equal(t, forgedOrgSessionID, svc.runOrg)
}

// TestListCallbackReceipts_ForgedQueryOrganizationIDIgnored pins the callback
// receipt audit list.
func TestListCallbackReceipts_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	repo := &forgedOrgReceiptsRepo{}
	router := testRouter()
	RegisterCallbackReceiptsChiRoutes(router, NewCallbackReceiptsHandler(apppayments.NewPaymentAuditService(repo)))

	req := withTrustedTenant(httptest.NewRequest(http.MethodGet,
		"/api/v1/pay-callback-receipts?organizationId="+forgedOrgRequestID, nil))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, forgedOrgSessionID, repo.listOrg)
}

// TestListPaymentsWithoutTenantScopeRejected pins the empty-org fail-fast: an
// unauthenticated empty tenant must not degrade to "all organizations".
func TestListPaymentsWithoutTenantScopeRejected(t *testing.T) {
	router := testRouter()
	RegisterChiRoutes(router, Handler{})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/payments", nil))
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}
