package aigateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appgateway "github.com/guliangiam/guliangiam/internal/app/aigateway"
	aigatewaydomain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	trustedQuotaID = "q-trusted"
	trustedPriceID = "pv-trusted"
	foreignQuotaID = "q-foreign"
	foreignPriceID = "pv-foreign"
)

// ── Scoped methods for the shared test fakes ────────────────────────────
//
// Declared here (same package) so the E2a/E2b fixtures keep using the same
// in-memory stores while the management service can resolve the scoped port.

func (f *fakeQuotasRepo) GetByIDForOrganization(_ context.Context, org, id string) (*aigatewaydomain.LlmQuotaPolicy, error) {
	q, ok := f.items[id]
	if !ok || q.OrganizationID != org {
		return nil, apperrors.NewNotFound("quota not found")
	}
	cp := *q
	return &cp, nil
}
func (f *fakeQuotasRepo) UpdateForOrganization(_ context.Context, q *aigatewaydomain.LlmQuotaPolicy, org string) error {
	cur, ok := f.items[q.ID]
	if !ok || cur.OrganizationID != org {
		return apperrors.NewNotFound("quota not found")
	}
	cp := *q
	f.items[q.ID] = &cp
	return nil
}
func (f *fakeQuotasRepo) DeleteForOrganization(_ context.Context, org, id string) error {
	q, ok := f.items[id]
	if !ok || q.OrganizationID != org {
		return apperrors.NewNotFound("quota not found")
	}
	delete(f.items, id)
	return nil
}

func (f *fakePricesRepo) GetByIDForOrganization(_ context.Context, org, id string) (*aigatewaydomain.LlmPriceVersion, error) {
	v, ok := f.items[id]
	if !ok || v.OrganizationID != org {
		return nil, apperrors.NewNotFound("price version not found")
	}
	cp := *v
	return &cp, nil
}
func (f *fakePricesRepo) UpdateForOrganization(_ context.Context, v *aigatewaydomain.LlmPriceVersion, org string) error {
	cur, ok := f.items[v.ID]
	if !ok || cur.OrganizationID != org {
		return apperrors.NewNotFound("price version not found")
	}
	cp := *v
	f.items[v.ID] = &cp
	return nil
}
func (f *fakePricesRepo) DeleteForOrganization(_ context.Context, org, id string) error {
	v, ok := f.items[id]
	if !ok || v.OrganizationID != org {
		return apperrors.NewNotFound("price version not found")
	}
	delete(f.items, id)
	return nil
}

var (
	_ aigatewaydomain.TenantScopedQuotaPoliciesRepo = (*fakeQuotasRepo)(nil)
	_ aigatewaydomain.TenantScopedPriceVersionsRepo = (*fakePricesRepo)(nil)
)

// ── Counting wrappers (prove forged writes never reach the store) ───────

type countingQuotasRepo struct {
	*fakeQuotasRepo
	updateCalls int
	deleteCalls int
}

func (r *countingQuotasRepo) UpdateForOrganization(ctx context.Context, q *aigatewaydomain.LlmQuotaPolicy, org string) error {
	if err := r.fakeQuotasRepo.UpdateForOrganization(ctx, q, org); err != nil {
		return err
	}
	r.updateCalls++
	return nil
}
func (r *countingQuotasRepo) DeleteForOrganization(ctx context.Context, org, id string) error {
	if err := r.fakeQuotasRepo.DeleteForOrganization(ctx, org, id); err != nil {
		return err
	}
	r.deleteCalls++
	return nil
}

type countingPricesRepo struct {
	*fakePricesRepo
	updateCalls int
	deleteCalls int
}

func (r *countingPricesRepo) UpdateForOrganization(ctx context.Context, v *aigatewaydomain.LlmPriceVersion, org string) error {
	if err := r.fakePricesRepo.UpdateForOrganization(ctx, v, org); err != nil {
		return err
	}
	r.updateCalls++
	return nil
}
func (r *countingPricesRepo) DeleteForOrganization(ctx context.Context, org, id string) error {
	if err := r.fakePricesRepo.DeleteForOrganization(ctx, org, id); err != nil {
		return err
	}
	r.deleteCalls++
	return nil
}

// ── Fixture ─────────────────────────────────────────────────────────────

type quotaPriceAdminFixture struct {
	router chi.Router
	quotas *countingQuotasRepo
	prices *countingPricesRepo
}

func newQuotaPriceAdminFixture(t *testing.T, scopeCtx func(context.Context) context.Context) *quotaPriceAdminFixture {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY_CURRENT", "BX1Ab1Jz6cRDESuWCleZWGRv7RHQRAf0sqikOVcVzkg=")

	f := &quotaPriceAdminFixture{
		quotas: &countingQuotasRepo{fakeQuotasRepo: newFakeQuotasRepo()},
		prices: &countingPricesRepo{fakePricesRepo: newFakePricesRepo()},
	}
	svc := appgateway.NewAdminService(
		&sequentialUUIDGen{}, fixedClock{}, nil,
		newFakeProvidersRepo(), newFakeModelsRepo(), newFakeAliasesRepo(), newFakeRoutesRepo(),
		newFakeApiKeysRepo(), f.quotas, stubUsageLogsRepo{}, f.prices, newFakeHealthRepo(),
	)
	handler := NewAdminHandler(svc)
	r := chi.NewRouter()
	if scopeCtx != nil {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				req = req.WithContext(scopeCtx(req.Context()))
				next.ServeHTTP(w, req)
			})
		})
	}
	RegisterAdminChiRoutes(r, handler)
	f.router = r
	return f
}

func (f *quotaPriceAdminFixture) seedForeignTenantRows(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, f.quotas.Create(ctx, &aigatewaydomain.LlmQuotaPolicy{
		ID: foreignQuotaID, OrganizationID: foreignTenantOrg, Scope: "organization",
		LimitType: "rpm", LimitValue: 60, ResetCycle: "daily", Status: "active",
		CreatedAt: fixedNow, UpdatedAt: fixedNow, Revision: 1,
	}))
	require.NoError(t, f.prices.Create(ctx, &aigatewaydomain.LlmPriceVersion{
		ID: foreignPriceID, OrganizationID: foreignTenantOrg, ModelID: "m-foreign", VersionNo: 1,
		InputPricePer1M: 1, OutputPricePer1M: 2, Currency: "USD",
		EffectiveFrom: fixedNow.Add(-24 * time.Hour), CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}))
}

func (f *quotaPriceAdminFixture) get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// TestAdminHandler_QuotaPriceByIDEmptyTenantIsRejected pins the first state:
// with no trusted tenant all six by-ID entries answer 400 before a path id or
// request body is decoded.
func TestAdminHandler_QuotaPriceByIDEmptyTenantIsRejected(t *testing.T) {
	f := newQuotaPriceAdminFixture(t, nil)
	f.seedForeignTenantRows(t)

	tests := []struct {
		method string
		path   string
		body   any
	}{
		{method: http.MethodGet, path: "/api/v1/ai/quotas/" + foreignQuotaID},
		{method: http.MethodPatch, path: "/api/v1/ai/quotas/" + foreignQuotaID, body: map[string]any{"limitValue": 120}},
		{method: http.MethodDelete, path: "/api/v1/ai/quotas/" + foreignQuotaID},
		{method: http.MethodGet, path: "/api/v1/ai/prices/" + foreignPriceID},
		{method: http.MethodPatch, path: "/api/v1/ai/prices/" + foreignPriceID, body: map[string]any{"inputPricePer1M": 6.5}},
		{method: http.MethodDelete, path: "/api/v1/ai/prices/" + foreignPriceID},
	}
	for _, tt := range tests {
		var rec *httptest.ResponseRecorder
		if tt.body != nil {
			rec = performJSON(t, f.router, tt.method, tt.path, tt.body)
		} else {
			rec = httptest.NewRecorder()
			f.router.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
		}
		assertStatus(t, rec, http.StatusBadRequest)
	}
}

// TestAdminHandler_QuotaPriceByIDForeignTenantIsNotFoundAndUnwritten pins the
// second state: a forged tenant gets the same 404 as a missing id and no row
// is written.
func TestAdminHandler_QuotaPriceByIDForeignTenantIsNotFoundAndUnwritten(t *testing.T) {
	forged := "?organizationId=" + foreignTenantOrg
	f := newQuotaPriceAdminFixture(t, strictTenantCtx(trustedTenantOrg))
	f.seedForeignTenantRows(t)
	beforeQuota := *f.quotas.items[foreignQuotaID]
	beforePrice := *f.prices.items[foreignPriceID]

	assertStatus(t, f.get(t, "/api/v1/ai/quotas/"+foreignQuotaID+forged), http.StatusNotFound)
	rec := performJSON(t, f.router, http.MethodPatch, "/api/v1/ai/quotas/"+foreignQuotaID+forged,
		map[string]any{"limitValue": 120})
	assertStatus(t, rec, http.StatusNotFound)
	rec = httptest.NewRecorder()
	f.router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/ai/quotas/"+foreignQuotaID+forged, nil))
	assertStatus(t, rec, http.StatusNotFound)

	assertStatus(t, f.get(t, "/api/v1/ai/prices/"+foreignPriceID+forged), http.StatusNotFound)
	rec = performJSON(t, f.router, http.MethodPatch, "/api/v1/ai/prices/"+foreignPriceID+forged,
		map[string]any{"inputPricePer1M": 6.5})
	assertStatus(t, rec, http.StatusNotFound)
	rec = httptest.NewRecorder()
	f.router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/ai/prices/"+foreignPriceID+forged, nil))
	assertStatus(t, rec, http.StatusNotFound)

	afterQuota, ok := f.quotas.items[foreignQuotaID]
	require.True(t, ok, "a forged delete must not remove the foreign quota")
	assert.Equal(t, beforeQuota.LimitValue, afterQuota.LimitValue)
	assert.Equal(t, beforeQuota.Status, afterQuota.Status)
	afterPrice, ok := f.prices.items[foreignPriceID]
	require.True(t, ok, "a forged delete must not remove the foreign price")
	assert.Equal(t, beforePrice.InputPricePer1M, afterPrice.InputPricePer1M)
	assert.Equal(t, beforePrice.Currency, afterPrice.Currency)
	assert.Zero(t, f.quotas.updateCalls, "a forged quota patch must not write")
	assert.Zero(t, f.quotas.deleteCalls, "a forged quota delete must not write")
	assert.Zero(t, f.prices.updateCalls, "a forged price patch must not write")
	assert.Zero(t, f.prices.deleteCalls, "a forged price delete must not write")
}

// TestAdminHandler_QuotaPriceByIDOwnTenantSucceeds pins the third state: the
// same six entries work inside the trusted tenant.
func TestAdminHandler_QuotaPriceByIDOwnTenantSucceeds(t *testing.T) {
	f := newQuotaPriceAdminFixture(t, strictTenantCtx(trustedTenantOrg))
	ctx := context.Background()
	require.NoError(t, f.quotas.Create(ctx, &aigatewaydomain.LlmQuotaPolicy{
		ID: trustedQuotaID, OrganizationID: trustedTenantOrg, Scope: "organization",
		LimitType: "rpm", LimitValue: 60, ResetCycle: "daily", Status: "active",
		CreatedAt: fixedNow, UpdatedAt: fixedNow, Revision: 1,
	}))
	require.NoError(t, f.prices.Create(ctx, &aigatewaydomain.LlmPriceVersion{
		ID: trustedPriceID, OrganizationID: trustedTenantOrg, ModelID: "m-trusted", VersionNo: 1,
		InputPricePer1M: 5, OutputPricePer1M: 15, Currency: "USD",
		EffectiveFrom: fixedNow.Add(-24 * time.Hour), CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}))

	rec := f.get(t, "/api/v1/ai/quotas/"+trustedQuotaID)
	assertStatus(t, rec, http.StatusOK)
	assert.Equal(t, trustedQuotaID, decodeResource(t, rec)["id"])

	rec = performJSON(t, f.router, http.MethodPatch, "/api/v1/ai/quotas/"+trustedQuotaID,
		map[string]any{"limitValue": 120})
	assertStatus(t, rec, http.StatusOK)
	assert.Equal(t, float64(120), decodeResource(t, rec)["limitValue"])
	assert.Equal(t, 1, f.quotas.updateCalls)

	rec = httptest.NewRecorder()
	f.router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/ai/quotas/"+trustedQuotaID, nil))
	assertStatus(t, rec, http.StatusNoContent)
	assert.Equal(t, 1, f.quotas.deleteCalls)
	assertStatus(t, f.get(t, "/api/v1/ai/quotas/"+trustedQuotaID), http.StatusNotFound)

	rec = f.get(t, "/api/v1/ai/prices/"+trustedPriceID)
	assertStatus(t, rec, http.StatusOK)
	assert.Equal(t, trustedPriceID, decodeResource(t, rec)["id"])

	rec = performJSON(t, f.router, http.MethodPatch, "/api/v1/ai/prices/"+trustedPriceID,
		map[string]any{"inputPricePer1M": 6.5})
	assertStatus(t, rec, http.StatusOK)
	assert.Equal(t, 6.5, decodeResource(t, rec)["inputPricePer1M"])
	assert.Equal(t, 1, f.prices.updateCalls)

	rec = httptest.NewRecorder()
	f.router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/ai/prices/"+trustedPriceID, nil))
	assertStatus(t, rec, http.StatusNoContent)
	assert.Equal(t, 1, f.prices.deleteCalls)
	assertStatus(t, f.get(t, "/api/v1/ai/prices/"+trustedPriceID), http.StatusNotFound)
}
