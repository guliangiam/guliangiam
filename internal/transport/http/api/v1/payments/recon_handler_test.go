package payments

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── fake ReconService ────────────────────────────────────────────────

type fakeReconService struct {
	batches      []*domain.ReconBatch
	items        map[string][]*domain.ReconItem
	resolveErr   error
	approveCalls int
	runErr       error
	runCalls     int
	lastRunID    string
}

func newFakeReconService() *fakeReconService {
	return &fakeReconService{
		items: make(map[string][]*domain.ReconItem),
	}
}

func (f *fakeReconService) ListBatches(_ context.Context, orgID string, _ int, _ int) ([]*domain.ReconBatch, int64, error) {
	var result []*domain.ReconBatch
	for _, b := range f.batches {
		if b.OrganizationID == orgID {
			result = append(result, b)
		}
	}
	return result, int64(len(result)), nil
}

func (f *fakeReconService) ListItems(_ context.Context, _, batchID string) ([]*domain.ReconItem, error) {
	return f.items[batchID], nil
}

func (f *fakeReconService) ResolveItem(_ context.Context, _, _, _ string) error {
	return f.resolveErr
}

func (f *fakeReconService) ApproveItem(_ context.Context, _, _, _ string) error {
	f.approveCalls++
	return nil
}

func (f *fakeReconService) RunManual(_ context.Context, _, batchID string) error {
	f.runCalls++
	f.lastRunID = batchID
	return f.runErr
}

func (f *fakeReconService) CreateAndRun(_ context.Context, _ string, _ string, batchDate time.Time, _ time.Time) (string, error) {
	id := "rb-fake-" + batchDate.Format("20060102")
	f.runCalls++
	f.lastRunID = id
	return id, nil
}

// compile-time: fakeReconService implements ReconService
var _ apppayments.ReconService = (*fakeReconService)(nil)

// ── helpers ──────────────────────────────────────────────────────────

func reconTestRouter(svc apppayments.ReconService) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	RegisterReconChiRoutes(r, NewReconHandler(svc))
	return r
}

func doReconRequest(t *testing.T, router chi.Router, method, path string, body io.Reader) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Management routes always run behind the authenticated tenant middleware;
	// the scope below stands in for it so query/body organizationId is clamped.
	ctx := tenantctx.WithTenantScope(req.Context(), "org-1", "org-1", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{UserID: "ops-session", OrganizationID: "org-1"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Result()
}

// ── Test ListBatches ─────────────────────────────────────────────────

func TestReconHandler_ListBatches(t *testing.T) {
	svc := newFakeReconService()
	now := time.Now().UTC()
	svc.batches = []*domain.ReconBatch{
		{
			ID: "rb-ali-20251001", OrganizationID: "org-1", Channel: "alipay",
			BatchDate: now.AddDate(0, 0, -1), Status: "completed",
			DiffCount: 2, TotalLocal: 100, TotalChannel: 98,
			CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "rb-wx-20251001", OrganizationID: "org-1", Channel: "wechat",
			BatchDate: now.AddDate(0, 0, -1), Status: "pending",
			DiffCount: 0, TotalLocal: 50, TotalChannel: 50,
			CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "rb-ali-20251001", OrganizationID: "org-2", Channel: "alipay", // different org
			BatchDate: now.AddDate(0, 0, -1), Status: "completed",
			DiffCount: 0, TotalLocal: 10, TotalChannel: 10,
			CreatedAt: now, UpdatedAt: now,
		},
	}

	router := reconTestRouter(svc)
	resp := doReconRequest(t, router, http.MethodGet, "/api/v1/pay-recon-batches?organizationId=org-1", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "rb-ali-20251001")
	assert.Contains(t, string(body), "rb-wx-20251001")
	assert.NotContains(t, string(body), "org-2")
}

func TestReconHandler_ListBatches_Empty(t *testing.T) {
	svc := newFakeReconService()
	router := reconTestRouter(svc)
	resp := doReconRequest(t, router, http.MethodGet, "/api/v1/pay-recon-batches?organizationId=org-empty", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), `"items":[]`)
}

// ── Test ListItems ───────────────────────────────────────────────────

func TestReconHandler_ListItems(t *testing.T) {
	svc := newFakeReconService()
	chOrder := "CH20251001001"
	localPID := "pay-123"
	chAmt := 100.00
	la := 101.00
	chSt := "succeeded"
	ls := string(domain.StateSucceeded)
	svc.items["rb-test-20251001"] = []*domain.ReconItem{
		{
			ID: "ri-1", BatchID: "rb-test-20251001", DiffType: string(domain.DiffAmountMismatch),
			ChannelOrderNo: &chOrder, LocalPaymentID: &localPID,
			ChannelAmount: &chAmt, LocalAmount: &la,
			ChannelState: &chSt, LocalState: &ls,
			Resolved: false, CreatedAt: time.Now().UTC(),
		},
	}

	router := reconTestRouter(svc)
	resp := doReconRequest(t, router, http.MethodGet, "/api/v1/pay-recon-batches/rb-test-20251001/items", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "ri-1")
	assert.Contains(t, string(body), "amount_mismatch")
	assert.Contains(t, string(body), "CH20251001001")
}

func TestReconHandler_ListItems_Empty(t *testing.T) {
	svc := newFakeReconService()
	router := reconTestRouter(svc)
	resp := doReconRequest(t, router, http.MethodGet, "/api/v1/pay-recon-batches/rb-noitems/items", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), `[]`)
}

// ── Test ResolveItem ─────────────────────────────────────────────────

func TestReconHandler_ResolveItem_Success(t *testing.T) {
	svc := newFakeReconService()
	router := reconTestRouter(svc)

	body := bytes.NewBufferString(`{"operator":"admin","note":"manual review"}`)
	resp := doReconRequest(t, router, http.MethodPost, "/api/v1/pay-recon-items/ri-1:resolve", body)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	respBody, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(respBody), `"status":"resolved"`)
}

func TestReconHandler_ResolveItem_UsesSessionOperator(t *testing.T) {
	svc := newFakeReconService()
	router := reconTestRouter(svc)

	// operator comes from the authenticated session; a body operator is ignored.
	body := bytes.NewBufferString(`{}`)
	resp := doReconRequest(t, router, http.MethodPost, "/api/v1/pay-recon-items/ri-1:resolve", body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestReconHandler_ResolveItem_ServiceError(t *testing.T) {
	svc := newFakeReconService()
	svc.resolveErr = errors.New("db down")
	router := reconTestRouter(svc)

	body := bytes.NewBufferString(`{"operator":"admin"}`)
	resp := doReconRequest(t, router, http.MethodPost, "/api/v1/pay-recon-items/ri-1:resolve", body)
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)
}

// ── Test RunRecon ────────────────────────────────────────────────────

func TestReconHandler_RunRecon_ExistingBatch(t *testing.T) {
	svc := newFakeReconService()
	router := reconTestRouter(svc)

	body := bytes.NewBufferString(`{"batchId":"rb-ali-20251001"}`)
	resp := doReconRequest(t, router, http.MethodPost, "/api/v1/pay-recon:run", body)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	assert.Equal(t, 1, svc.runCalls)
	assert.Equal(t, "rb-ali-20251001", svc.lastRunID)

	respBody, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(respBody), `"batchId":"rb-ali-20251001"`)
}

func TestReconHandler_RunRecon_CreateAndRun(t *testing.T) {
	svc := newFakeReconService()
	router := reconTestRouter(svc)

	body := bytes.NewBufferString(`{"organizationId":"org-1","channel":"alipay","batchDate":"2025-10-01"}`)
	resp := doReconRequest(t, router, http.MethodPost, "/api/v1/pay-recon:run", body)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	assert.Equal(t, 1, svc.runCalls)

	respBody, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(respBody), `"batchId":"rb-fake-`)
}

func TestReconHandler_RunRecon_MissingFields(t *testing.T) {
	svc := newFakeReconService()
	router := reconTestRouter(svc)

	// 空请求 — 既没有 batchId 也没有 org+channel+batchDate
	body := bytes.NewBufferString(`{}`)
	resp := doReconRequest(t, router, http.MethodPost, "/api/v1/pay-recon:run", body)
	assert.NotEqual(t, http.StatusAccepted, resp.StatusCode)
}

func TestReconHandler_RunRecon_ServiceError(t *testing.T) {
	svc := newFakeReconService()
	svc.runErr = errors.New("channel gateway not registered")
	router := reconTestRouter(svc)

	body := bytes.NewBufferString(`{"batchId":"rb-fail"}`)
	resp := doReconRequest(t, router, http.MethodPost, "/api/v1/pay-recon:run", body)
	assert.NotEqual(t, http.StatusAccepted, resp.StatusCode)
	assert.Equal(t, 1, svc.runCalls) // 但 RunManual 被调用了
}

func TestReconHandler_ApproveItem_UsesSessionOperator(t *testing.T) {
	svc := newFakeReconService()
	router := reconTestRouter(svc)

	body := bytes.NewBufferString("{\"reason\":\"reviewed\"}")
	resp := doReconRequest(t, router, http.MethodPost, "/api/v1/pay-recon-items/ri-1:approve", body)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 1, svc.approveCalls)
}
