package payments

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pagingChannelConfigRepo struct {
	items []*domain.PayChannelConfig
}

func (r pagingChannelConfigRepo) GetByID(context.Context, string) (*domain.PayChannelConfig, error) {
	return nil, nil
}
func (r pagingChannelConfigRepo) GetByIDForOrganization(context.Context, string, string) (*domain.PayChannelConfig, error) {
	return nil, nil
}
func (r pagingChannelConfigRepo) GetActive(context.Context, string, string, string) (*domain.PayChannelConfig, error) {
	return nil, nil
}
func (r pagingChannelConfigRepo) List(context.Context, string) ([]*domain.PayChannelConfig, error) {
	return r.items, nil
}
func (r pagingChannelConfigRepo) Create(context.Context, *domain.PayChannelConfig) error { return nil }
func (r pagingChannelConfigRepo) Update(context.Context, *domain.PayChannelConfig) error { return nil }

type pagingCatalogProvider struct {
	items []domain.ChannelMetadata
}

func (p pagingCatalogProvider) ChannelMetadata() []domain.ChannelMetadata { return p.items }

func TestPaginateList_EdgeCases(t *testing.T) {
	items := []int{1, 2, 3}

	defaultPage := paginateList(items, 1, 0)
	require.Equal(t, platformpagination.DefaultPageSize, defaultPage.Page.PageSize)
	require.Equal(t, items, defaultPage.Items)

	lastPage := paginateList(items, 2, 2)
	require.Equal(t, []int{3}, lastPage.Items)
	require.Equal(t, int64(3), lastPage.Page.Total)

	hugePage := paginateList(items, 1<<62, 1<<62)
	require.NotNil(t, hugePage.Items)
	require.Empty(t, hugePage.Items)
	require.Equal(t, int64(3), hugePage.Page.Total)

	nilItems := paginateList[int](nil, 1, 20)
	require.NotNil(t, nilItems.Items)
	require.Empty(t, nilItems.Items)
}

// An omitted pageSize must serve (and echo) the default page, not the whole
// set: a list longer than the default has to require a second request, and an
// empty list must not report pageSize:0.
func TestPaginateList_OmittedPageSizeIsNotTheWholeSet(t *testing.T) {
	items := make([]int, platformpagination.DefaultPageSize+5)
	for i := range items {
		items[i] = i
	}

	first := paginateList(items, 1, 0)
	require.Equal(t, platformpagination.DefaultPageSize, first.Page.PageSize)
	require.Equal(t, int64(len(items)), first.Page.Total)
	require.Len(t, first.Items, platformpagination.DefaultPageSize)

	second := paginateList(items, 2, 0)
	require.Equal(t, platformpagination.DefaultPageSize, second.Page.PageSize)
	require.Len(t, second.Items, 5)
}

func TestCallbackReceiptsHandler_CheckedOffset(t *testing.T) {
	cases := []struct {
		name           string
		page, pageSize int
		wantLimit      int
		wantOffset     int
	}{
		{
			name: "hostile page is capped",
			page: math.MaxInt, pageSize: 20,
			wantLimit: 20, wantOffset: (platformpagination.MaxPage - 1) * 20,
		},
		{
			name: "hostile page size is capped",
			page: 2, pageSize: math.MaxInt,
			wantLimit: platformpagination.MaxPageSize, wantOffset: platformpagination.MaxPageSize,
		},
		{
			name: "normal page",
			page: 2, pageSize: 20,
			wantLimit: 20, wantOffset: 20,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &recordingAuditService{}
			_, err := NewCallbackReceiptsHandler(svc).List(context.Background(), "org-1", tc.page, tc.pageSize)
			require.NoError(t, err)
			assert.Equal(t, tc.wantLimit, svc.limit)
			assert.Equal(t, tc.wantOffset, svc.offset)
			assert.GreaterOrEqual(t, svc.offset, 0)
		})
	}
}

type recordingAuditService struct {
	limit, offset int
}

func (s *recordingAuditService) ListCallbackReceipts(_ context.Context, _ string, limit, offset int) ([]*domain.CallbackReceipt, int64, error) {
	s.limit, s.offset = limit, offset
	return nil, 0, nil
}

func TestPaymentSubresourceListsHonorPagination(t *testing.T) {
	const orgID = forgedOrgSessionID
	const paymentID = "payment-1"

	attempts := &tenantAttemptsRepo{items: []*domain.PayAttempt{
		{ID: "attempt-1", OrganizationID: orgID, PaymentID: paymentID, AttemptNo: 1},
		{ID: "attempt-2", OrganizationID: orgID, PaymentID: paymentID, AttemptNo: 2},
		{ID: "attempt-3", OrganizationID: orgID, PaymentID: paymentID, AttemptNo: 3},
		{ID: "attempt-4", OrganizationID: orgID, PaymentID: paymentID, AttemptNo: 4},
	}}
	refunds := &tenantRefundsRepo{items: []*domain.Refund{
		{ID: "refund-1", OrganizationID: orgID, PaymentID: paymentID},
		{ID: "refund-2", OrganizationID: orgID, PaymentID: paymentID},
		{ID: "refund-3", OrganizationID: orgID, PaymentID: paymentID},
		{ID: "refund-4", OrganizationID: orgID, PaymentID: paymentID},
	}}
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	configs := []*domain.PayChannelConfig{
		{ID: "cfg-1", OrganizationID: orgID, Channel: "alipay", Environment: "production", CreatedAt: now, UpdatedAt: now},
		{ID: "cfg-2", OrganizationID: orgID, Channel: "wechat", Environment: "production", CreatedAt: now, UpdatedAt: now},
		{ID: "cfg-3", OrganizationID: orgID, Channel: "stripe", Environment: "production", CreatedAt: now, UpdatedAt: now},
		{ID: "cfg-4", OrganizationID: orgID, Channel: "paypal", Environment: "production", CreatedAt: now, UpdatedAt: now},
	}
	catalog := []domain.ChannelMetadata{
		{Channel: "alipay"}, {Channel: "wechat"}, {Channel: "stripe"}, {Channel: "paypal"},
	}

	t.Run("attempts", func(t *testing.T) {
		router := testRouter()
		RegisterAttemptsChiRoutes(router, NewAttemptsHandler(apppayments.NewHistoryService(attempts, refunds)))
		recorder := serveTenantPaymentRequest(t, router, withTrustedTenant(httptest.NewRequest(
			http.MethodGet, "/api/v1/payments/"+paymentID+"/attempts?page=2&pageSize=2", nil)))
		require.Equal(t, http.StatusOK, recorder.Code)

		var body presenter.ListResponse[AttemptListItem]
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
		require.Equal(t, 2, body.Page.Page)
		require.Equal(t, 2, body.Page.PageSize)
		require.Equal(t, int64(4), body.Page.Total)
		require.Len(t, body.Items, 2)
		require.Equal(t, "attempt-3", body.Items[0].ID)
		require.Equal(t, "attempt-4", body.Items[1].ID)
	})

	t.Run("refunds", func(t *testing.T) {
		router := testRouter()
		RegisterRefundsChiRoutes(router, NewRefundsHandler(apppayments.NewHistoryService(attempts, refunds)))
		recorder := serveTenantPaymentRequest(t, router, withTrustedTenant(httptest.NewRequest(
			http.MethodGet, "/api/v1/payments/"+paymentID+"/refunds?page=2&pageSize=2", nil)))
		require.Equal(t, http.StatusOK, recorder.Code)

		var body presenter.ListResponse[RefundListItem]
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
		require.Equal(t, 2, body.Page.Page)
		require.Equal(t, 2, body.Page.PageSize)
		require.Equal(t, int64(4), body.Page.Total)
		require.Len(t, body.Items, 2)
		require.Equal(t, "refund-3", body.Items[0].ID)
		require.Equal(t, "refund-4", body.Items[1].ID)
	})

	t.Run("channel configs", func(t *testing.T) {
		service := apppayments.NewChannelConfigService(pagingChannelConfigRepo{items: configs}, tenantSystemClock{}, nil, platformuuid.V7Generator{})
		router := testRouter()
		RegisterChannelConfigChiRoutes(router, NewChannelConfigHandler(service))
		recorder := serveTenantPaymentRequest(t, router, withTrustedTenant(httptest.NewRequest(
			http.MethodGet, "/api/v1/pay-channel-configs?page=2&pageSize=2", nil)))
		require.Equal(t, http.StatusOK, recorder.Code)

		var body presenter.ListResponse[ChannelConfigView]
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
		require.Equal(t, 2, body.Page.Page)
		require.Equal(t, 2, body.Page.PageSize)
		require.Equal(t, int64(4), body.Page.Total)
		require.Len(t, body.Items, 2)
		require.Equal(t, "cfg-3", body.Items[0].ID)
		require.Equal(t, "cfg-4", body.Items[1].ID)
	})

	t.Run("channel catalog", func(t *testing.T) {
		router := testRouter()
		RegisterChannelCatalogChiRoutes(router, NewChannelCatalogHandler(apppayments.NewChannelCatalogService(pagingCatalogProvider{items: catalog})))
		recorder := serveTenantPaymentRequest(t, router, httptest.NewRequest(
			http.MethodGet, "/api/v1/pay-channels?page=2&pageSize=2", nil))
		require.Equal(t, http.StatusOK, recorder.Code)

		var body presenter.ListResponse[apppayments.ChannelMetadata]
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
		require.Equal(t, 2, body.Page.Page)
		require.Equal(t, 2, body.Page.PageSize)
		require.Equal(t, int64(4), body.Page.Total)
		require.Len(t, body.Items, 2)
		require.Equal(t, "stripe", body.Items[0].Channel)
		require.Equal(t, "paypal", body.Items[1].Channel)
	})

	t.Run("empty lists stay arrays", func(t *testing.T) {
		emptyAttempts := &tenantAttemptsRepo{}
		emptyRefunds := &tenantRefundsRepo{}
		router := testRouter()
		RegisterAttemptsChiRoutes(router, NewAttemptsHandler(apppayments.NewHistoryService(emptyAttempts, emptyRefunds)))
		recorder := serveTenantPaymentRequest(t, router, withTrustedTenant(httptest.NewRequest(
			http.MethodGet, "/api/v1/payments/"+paymentID+"/attempts", nil)))
		require.Equal(t, http.StatusOK, recorder.Code)

		var body presenter.ListResponse[AttemptListItem]
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
		require.NotNil(t, body.Items, "empty list must serialize as [] rather than null")
		require.Empty(t, body.Items)
		require.Equal(t, platformpagination.DefaultPageSize, body.Page.PageSize)
	})
}
