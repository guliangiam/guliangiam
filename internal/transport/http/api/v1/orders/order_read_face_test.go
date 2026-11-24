package orders

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	apppkg "github.com/guliangiam/guliangiam/internal/app/orders"
	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	orderdomain "github.com/guliangiam/guliangiam/internal/domain/orders"
	ordersmock "github.com/guliangiam/guliangiam/internal/domain/orders/mock"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	httpmw "github.com/guliangiam/guliangiam/internal/transport/http/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const orderReadFaceOrgID = "org-1"

// newReadFaceRouter builds the real orders routes behind the production ETag
// middleware so the conditional-request contract of design report §42.3.11 can
// be asserted end to end at the transport layer.
func newReadFaceRouter(t *testing.T, order *orderdomain.Order) chi.Router {
	t.Helper()

	ctrl := gomock.NewController(t)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := ordersmock.NewMockRepository(ctrl)

	uow.EXPECT().Orders().Return(repo).AnyTimes()
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), gomock.Any(), gomock.Any()).Return(order, nil).AnyTimes()

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(httpmw.ETagCacheMiddleware(httpmw.DefaultETagConfig()))
	RegisterChiRoutes(router, Handler{Service: apppkg.Service{UOW: uow}})
	return router
}

func getOrderResource(t *testing.T, router chi.Router, path, ifNoneMatch string) (*http.Response, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	ctx := tenantctx.WithTenantScope(req.Context(), orderReadFaceOrgID, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "44444444-4444-4444-4444-444444444444", OrganizationID: orderReadFaceOrgID,
	})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	resp := rec.Result()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return resp, body
}

func decodeOrderResource(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	return payload
}

// TestOrderReadFace_ServesPollingGuidance verifies the single read face carries
// the server-driven polling contract: awaiting payment orders poll fast, settled
// orders poll slow, and terminality is decided by the server state machine.
func TestOrderReadFace_ServesPollingGuidance(t *testing.T) {
	paidAt := time.Date(2024, 5, 1, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name              string
		state             string
		paidAt            *time.Time
		wantTerminal      bool
		wantAwaiting      bool
		wantPollAfterSecs float64
		wantPaidAt        bool
	}{
		{name: "payment_pending", state: orderdomain.StatePaymentPending, wantAwaiting: true, wantPollAfterSecs: 3},
		{name: "created", state: orderdomain.StateCreated, wantAwaiting: true, wantPollAfterSecs: 3},
		{name: "paid", state: orderdomain.StatePaid, paidAt: &paidAt, wantPollAfterSecs: 15, wantPaidAt: true},
		{name: "closed_terminal", state: orderdomain.StateClosed, paidAt: &paidAt, wantTerminal: true, wantPollAfterSecs: 15, wantPaidAt: true},
		{name: "cancelled_terminal", state: orderdomain.StateCancelled, wantTerminal: true, wantPollAfterSecs: 15},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			order := &orderdomain.Order{
				ID:             "order-1",
				OrganizationID: "org-1",
				Name:           "pro-plan",
				User:           "user-1",
				Price:          12.5,
				Currency:       "CNY",
				State:          tc.state,
				PaidAt:         tc.paidAt,
				CreatedAt:      time.Date(2024, 5, 1, 9, 0, 0, 0, time.UTC),
				UpdatedAt:      time.Date(2024, 5, 1, 9, 30, 0, 0, time.UTC),
				Revision:       3,
			}

			resp, body := getOrderResource(t, newReadFaceRouter(t, order), "/api/v1/orders/order-1", "")
			require.Equal(t, http.StatusOK, resp.StatusCode)

			payload := decodeOrderResource(t, body)
			assert.Equal(t, tc.state, payload["state"])
			assert.Equal(t, tc.wantTerminal, payload["terminal"])
			assert.Equal(t, tc.wantAwaiting, payload["awaitingPayment"])
			assert.Equal(t, tc.wantPollAfterSecs, payload["pollAfterSeconds"])
			assert.Equal(t, float64(3), payload["revision"])
			if tc.wantPaidAt {
				assert.Equal(t, paidAt.Format(time.RFC3339), payload["paidAt"])
			} else {
				assert.NotContains(t, payload, "paidAt")
			}
		})
	}
}

// TestOrderReadFace_ConditionalRequestReturns304 verifies §42.3.11's requirement
// that repeated polling of an unchanged order costs a zero-payload 304 instead
// of a full resource body.
func TestOrderReadFace_ConditionalRequestReturns304(t *testing.T) {
	order := &orderdomain.Order{
		ID:             "order-1",
		OrganizationID: "org-1",
		Name:           "pro-plan",
		User:           "user-1",
		Price:          12.5,
		Currency:       "CNY",
		State:          orderdomain.StatePaymentPending,
		CreatedAt:      time.Date(2024, 5, 1, 9, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2024, 5, 1, 9, 30, 0, 0, time.UTC),
		Revision:       1,
	}
	router := newReadFaceRouter(t, order)

	first, firstBody := getOrderResource(t, router, "/api/v1/orders/order-1", "")
	require.Equal(t, http.StatusOK, first.StatusCode)

	etag := first.Header.Get("ETag")
	require.NotEmpty(t, etag, "order read face must expose an ETag for conditional polling")
	require.Contains(t, first.Header.Get("Cache-Control"), "private")
	require.NotZero(t, len(firstBody))

	second, secondBody := getOrderResource(t, router, "/api/v1/orders/order-1", etag)
	assert.Equal(t, http.StatusNotModified, second.StatusCode)
	assert.Empty(t, secondBody, "304 responses must not carry a resource body")
	assert.Equal(t, etag, second.Header.Get("ETag"))
}

// TestOrderReadFace_PaymentStatusFaceRemoved anchors the de-duplication: polling
// must read the order resource, so the drifted duplicate read face must not come
// back into the permission surface or the router.
func TestOrderReadFace_PaymentStatusFaceRemoved(t *testing.T) {
	for _, def := range Routes() {
		assert.NotContains(t, def.Pattern, "payment-status",
			"the order read face is the single source for payment state (42.3.11)")
	}
}
