package aigateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appgateway "github.com/guliangiam/guliangiam/internal/app/aigateway"
	aigatewaydomain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	"github.com/guliangiam/guliangiam/internal/infra/aiupstream"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Fake Repos (PriceVersionsRepo + ProviderHealthRepo)
// ============================================================================

type fakePricesRepo struct {
	items map[string]*aigatewaydomain.LlmPriceVersion
}

func newFakePricesRepo() *fakePricesRepo {
	return &fakePricesRepo{items: map[string]*aigatewaydomain.LlmPriceVersion{}}
}

func (f *fakePricesRepo) GetByID(_ context.Context, id string) (*aigatewaydomain.LlmPriceVersion, error) {
	if v, ok := f.items[id]; ok {
		cp := *v
		return &cp, nil
	}
	return nil, apperrors.NewNotFound("price version not found")
}
func (f *fakePricesRepo) GetByModel(_ context.Context, modelID string) (*aigatewaydomain.LlmPriceVersion, error) {
	// return version with highest EffectiveFrom for that model
	var best *aigatewaydomain.LlmPriceVersion
	for _, v := range f.items {
		if v.ModelID != modelID || v.DeletedAt != nil {
			continue
		}
		if best == nil || v.EffectiveFrom.After(best.EffectiveFrom) {
			cp := *v
			best = &cp
		}
	}
	if best == nil {
		return nil, apperrors.NewNotFound("price version not found")
	}
	return best, nil
}
func (f *fakePricesRepo) List(_ context.Context, orgID string) ([]*aigatewaydomain.LlmPriceVersion, error) {
	var out []*aigatewaydomain.LlmPriceVersion
	for _, v := range f.items {
		if v.OrganizationID == orgID && v.DeletedAt == nil {
			cp := *v
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (f *fakePricesRepo) ListByModel(_ context.Context, modelID string) ([]*aigatewaydomain.LlmPriceVersion, error) {
	var out []*aigatewaydomain.LlmPriceVersion
	for _, v := range f.items {
		if v.ModelID == modelID && v.DeletedAt == nil {
			cp := *v
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (f *fakePricesRepo) Create(_ context.Context, v *aigatewaydomain.LlmPriceVersion) error {
	cp := *v
	f.items[v.ID] = &cp
	return nil
}
func (f *fakePricesRepo) Update(_ context.Context, v *aigatewaydomain.LlmPriceVersion) error {
	if _, ok := f.items[v.ID]; !ok {
		return apperrors.NewNotFound("price version not found")
	}
	cp := *v
	f.items[v.ID] = &cp
	return nil
}
func (f *fakePricesRepo) Delete(_ context.Context, id string) error {
	delete(f.items, id)
	return nil
}

// ── Fake ProviderHealthRepo ──────────────────────────────────────────

type fakeHealthRepo struct {
	items map[string]*aigatewaydomain.ProviderHealth
	// providerOrg mirrors the provider→organization predicate the SQL
	// repositories apply: llm_provider_health has no organization_id, so a
	// record is addressable only when its provider belongs to the tenant.
	providerOrg map[string]string
}

func newFakeHealthRepo() *fakeHealthRepo {
	return &fakeHealthRepo{
		items:       map[string]*aigatewaydomain.ProviderHealth{},
		providerOrg: map[string]string{},
	}
}

func (f *fakeHealthRepo) GetByID(_ context.Context, id string) (*aigatewaydomain.ProviderHealth, error) {
	if h, ok := f.items[id]; ok {
		cp := *h
		return &cp, nil
	}
	return nil, apperrors.NewNotFound("health not found")
}

func (f *fakeHealthRepo) GetByIDForOrganization(_ context.Context, orgID, id string) (*aigatewaydomain.ProviderHealth, error) {
	h, ok := f.items[id]
	if !ok || f.providerOrg[h.ProviderID] != orgID {
		return nil, apperrors.NewNotFound("provider health not found")
	}
	cp := *h
	return &cp, nil
}

func (f *fakeHealthRepo) ListByOrganization(_ context.Context, orgID string) ([]*aigatewaydomain.ProviderHealth, error) {
	var out []*aigatewaydomain.ProviderHealth
	for _, h := range f.items {
		if f.providerOrg[h.ProviderID] != orgID {
			continue
		}
		cp := *h
		out = append(out, &cp)
	}
	return out, nil
}
func (f *fakeHealthRepo) GetByProvider(_ context.Context, providerID string) (*aigatewaydomain.ProviderHealth, error) {
	for _, h := range f.items {
		if h.ProviderID == providerID {
			cp := *h
			return &cp, nil
		}
	}
	return nil, apperrors.NewNotFound("health not found")
}
func (f *fakeHealthRepo) ListAll(_ context.Context) ([]*aigatewaydomain.ProviderHealth, error) {
	var out []*aigatewaydomain.ProviderHealth
	for _, h := range f.items {
		cp := *h
		out = append(out, &cp)
	}
	return out, nil
}
func (f *fakeHealthRepo) Upsert(_ context.Context, h *aigatewaydomain.ProviderHealth) error {
	cp := *h
	f.items[h.ID] = &cp
	return nil
}
func (f *fakeHealthRepo) ListOpen(_ context.Context) ([]*aigatewaydomain.ProviderHealth, error) {
	var out []*aigatewaydomain.ProviderHealth
	for _, h := range f.items {
		if h.State == aigatewaydomain.HealthStateOpen {
			cp := *h
			out = append(out, &cp)
		}
	}
	return out, nil
}

// ============================================================================
// Fixture: Admin router with Prices + Health + Models fakes
// ============================================================================

func newTestAdminRouterWithPricesAndHealth(t *testing.T) (chi.Router, *fakePricesRepo, *fakeHealthRepo, *fakeModelsRepo) {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY_CURRENT", "BX1Ab1Jz6cRDESuWCleZWGRv7RHQRAf0sqikOVcVzkg=")

	uuidGen := &sequentialUUIDGen{}
	models := newFakeModelsRepo()
	prices := newFakePricesRepo()
	health := newFakeHealthRepo()

	// Pre-populate a model the org owns — CreatePrice needs Models.GetByID
	models.Create(context.Background(), &aigatewaydomain.LlmModel{
		ID: "m-1", OrganizationID: fixedOrgID,
		ProviderModelName: "gpt-4o", DisplayName: "GPT-4o",
		Status: "active", CreatedAt: fixedNow, UpdatedAt: fixedNow,
	})
	models.Create(context.Background(), &aigatewaydomain.LlmModel{
		ID: "m-2", OrganizationID: fixedOrgID,
		ProviderModelName: "claude-sonnet", DisplayName: "Claude Sonnet",
		Status: "active", CreatedAt: fixedNow, UpdatedAt: fixedNow,
	})

	// Pre-populate a price version for m-1
	prices.Create(context.Background(), &aigatewaydomain.LlmPriceVersion{
		ID: "pv-1", OrganizationID: fixedOrgID, ModelID: "m-1",
		VersionNo: 1, InputPricePer1M: 5.0, OutputPricePer1M: 15.0, Currency: "USD",
		EffectiveFrom: fixedNow.Add(-24 * time.Hour),
		CreatedAt:     fixedNow.Add(-24 * time.Hour), UpdatedAt: fixedNow.Add(-24 * time.Hour),
	})
	// Pre-populate a health record
	health.Upsert(context.Background(), &aigatewaydomain.ProviderHealth{
		ID: "h-1", ProviderID: "p-1", State: aigatewaydomain.HealthStateHealthy,
		ConsecutiveFailures: 0, UpdatedAt: fixedNow,
	})
	health.providerOrg["p-1"] = fixedOrgID
	openUntil := fixedNow.Add(10 * time.Minute)
	health.Upsert(context.Background(), &aigatewaydomain.ProviderHealth{
		ID: "h-2", ProviderID: "p-2", State: aigatewaydomain.HealthStateOpen,
		ConsecutiveFailures: 5, OpenUntil: &openUntil, UpdatedAt: fixedNow,
	})
	health.providerOrg["p-2"] = fixedOrgID

	svc := appgateway.NewAdminService(
		uuidGen, fixedClock{}, nil,
		newFakeProvidersRepo(),
		models,
		stubAliasesRepo{},
		stubRoutesRepo{},
		stubApiKeysRepo{},
		stubQuotaPoliciesRepo{},
		stubUsageLogsRepo{},
		prices,
		health,
	)

	handler := NewAdminHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			s := auth.SessionContext{
				ID: "sess-test", OrganizationID: fixedOrgID, OrganizationSlug: fixedOrgSlug,
				Username: "admin", AccessToken: "fake-token", Source: "repository", AuthTime: fixedNow,
			}
			req = req.WithContext(auth.WithSessionContext(req.Context(), s))
			next.ServeHTTP(w, req)
		})
	})
	RegisterAdminChiRoutes(r, handler)
	return r, prices, health, models
}

// ============================================================================
// Tests — Prices CRUD
// ============================================================================

func TestAdminHandler_ListPrices(t *testing.T) {
	router, _, _, _ := newTestAdminRouterWithPricesAndHealth(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/prices", nil)
	req.Header.Set("Authorization", "Bearer fake-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Items []map[string]any `json:"items"`
		Page  struct {
			Total int `json:"total"`
		} `json:"page"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Len(t, body.Items, 1, "pre-populated pv-1 should show up")
	assert.Equal(t, 1, body.Page.Total)

	first := body.Items[0]
	assert.Equal(t, "pv-1", first["id"])
	assert.Equal(t, float64(1), first["versionNo"])
	assert.Equal(t, "USD", first["currency"])
}

func TestAdminHandler_CreatePrice(t *testing.T) {
	router, prices, _, _ := newTestAdminRouterWithPricesAndHealth(t)

	payload := map[string]any{
		"modelId":          "m-1", // owned by org
		"inputPricePer1M":  3.5,
		"outputPricePer1M": 12.0,
		"currency":         "USD",
		"effectiveFrom":    fixedNow.Format(time.RFC3339),
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/prices", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer fake-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())
	var created map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	assert.Equal(t, fixedOrgID, created["organizationId"])
	assert.Equal(t, "m-1", created["modelId"])
	// Versioning: existing pv-1 is version 1 → new one should be version 2
	assert.Equal(t, float64(2), created["versionNo"])
	assert.Equal(t, 3.5, created["inputPricePer1M"])

	// Verify persisted
	assert.Len(t, prices.items, 2)
}

func TestAdminHandler_CreatePrice_SemanticViolations(t *testing.T) {
	router, _, _, _ := newTestAdminRouterWithPricesAndHealth(t)

	tests := []struct {
		name    string
		payload map[string]any
	}{
		{
			name: "effectiveFrom not RFC3339",
			payload: map[string]any{
				"modelId": "m-1", "inputPricePer1M": 1.0, "outputPricePer1M": 2.0, "effectiveFrom": "not-a-time",
			},
		},
		{
			name: "negative input price",
			payload: map[string]any{
				"modelId": "m-1", "inputPricePer1M": -1.0, "outputPricePer1M": 2.0, "effectiveFrom": fixedNow.Format(time.RFC3339),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/prices", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer fake-token")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, "body=%s", rec.Body.String())
		})
	}
}

func TestAdminHandler_CreatePrice_ModelNotFound(t *testing.T) {
	router, _, _, _ := newTestAdminRouterWithPricesAndHealth(t)

	payload := map[string]any{
		"modelId":          "nonexistent",
		"inputPricePer1M":  3.5,
		"outputPricePer1M": 12.0,
		"currency":         "USD",
		"effectiveFrom":    fixedNow.Format(time.RFC3339),
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/prices", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer fake-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminHandler_GetPrice(t *testing.T) {
	router, _, _, _ := newTestAdminRouterWithPricesAndHealth(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/prices/pv-1", nil)
	req.Header.Set("Authorization", "Bearer fake-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "pv-1", body["id"])
	assert.Equal(t, float64(1), body["versionNo"])
	assert.Equal(t, float64(5.0), body["inputPricePer1M"])
	assert.Equal(t, float64(15.0), body["outputPricePer1M"])
}

func TestAdminHandler_GetPrice_NotFound(t *testing.T) {
	router, _, _, _ := newTestAdminRouterWithPricesAndHealth(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/prices/pv-999", nil)
	req.Header.Set("Authorization", "Bearer fake-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminHandler_UpdatePrice(t *testing.T) {
	router, prices, _, _ := newTestAdminRouterWithPricesAndHealth(t)

	payload := map[string]any{
		"inputPricePer1M":  7.5,
		"outputPricePer1M": 20.0,
		"effectiveFrom":    fixedNow.Add(-48 * time.Hour).Format(time.RFC3339),
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/ai/prices/pv-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer fake-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var updated map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updated))
	assert.Equal(t, float64(7.5), updated["inputPricePer1M"])
	assert.Equal(t, float64(20.0), updated["outputPricePer1M"])

	// Persisted
	stored := prices.items["pv-1"]
	assert.Equal(t, 7.5, stored.InputPricePer1M)
	assert.Equal(t, 20.0, stored.OutputPricePer1M)
}

func TestAdminHandler_DeletePrice(t *testing.T) {
	router, prices, _, _ := newTestAdminRouterWithPricesAndHealth(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/ai/prices/pv-1", nil)
	req.Header.Set("Authorization", "Bearer fake-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)

	// Gone
	_, ok := prices.items["pv-1"]
	assert.False(t, ok, "price should be deleted")
}

// ============================================================================
// Tests — Health endpoints
// ============================================================================

func TestAdminHandler_ListHealth(t *testing.T) {
	router, _, _, _ := newTestAdminRouterWithPricesAndHealth(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/health", nil)
	req.Header.Set("Authorization", "Bearer fake-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Items []map[string]any `json:"items"`
		Page  struct {
			Total int `json:"total"`
		} `json:"page"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Len(t, body.Items, 2, "pre-populated h-1 and h-2")
	assert.Equal(t, 2, body.Page.Total)

	// Verify both states present
	states := map[string]string{}
	for _, it := range body.Items {
		states[it["id"].(string)] = it["state"].(string)
	}
	assert.Equal(t, aigatewaydomain.HealthStateHealthy, states["h-1"])
	assert.Equal(t, aigatewaydomain.HealthStateOpen, states["h-2"])
}

func TestAdminHandler_GetHealth(t *testing.T) {
	router, _, _, _ := newTestAdminRouterWithPricesAndHealth(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/health/h-1", nil)
	req.Header.Set("Authorization", "Bearer fake-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "h-1", body["id"])
	assert.Equal(t, "p-1", body["providerId"])
	assert.Equal(t, aigatewaydomain.HealthStateHealthy, body["state"])
	assert.Equal(t, float64(0), body["consecutiveFailures"])
}

func TestAdminHandler_GetHealth_NotFound(t *testing.T) {
	router, _, _, _ := newTestAdminRouterWithPricesAndHealth(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/health/h-999", nil)
	req.Header.Set("Authorization", "Bearer fake-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// ============================================================================
// Tests — handleTestProvider (needs httptest.Server)
// ============================================================================

const testFieldEncryptionKey = "BX1Ab1Jz6cRDESuWCleZWGRv7RHQRAf0sqikOVcVzkg="

// newProviderTestRouter mounts the admin routes with a fixed session context.
func newProviderTestRouter(svc *appgateway.AdminService) chi.Router {
	handler := NewAdminHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			s := auth.SessionContext{
				ID: "sess-test", OrganizationID: fixedOrgID, OrganizationSlug: fixedOrgSlug,
				Username: "admin", AccessToken: "fake-token", Source: "repository", AuthTime: fixedNow,
			}
			req = req.WithContext(auth.WithSessionContext(req.Context(), s))
			next.ServeHTTP(w, req)
		})
	})
	RegisterAdminChiRoutes(r, handler)
	return r
}

// newProviderTestService wires AdminService with the same AdapterFactory
// construction path bootstrap uses, so TestProvider decrypts credentials.
func newProviderTestService(t *testing.T, providers *fakeProvidersRepo) (*appgateway.AdminService, *platformcrypto.FieldEncryptor) {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY_CURRENT", testFieldEncryptionKey)
	enc, err := platformcrypto.NewFieldEncryptorChecked()
	require.NoError(t, err)

	svc := appgateway.NewAdminService(
		&sequentialUUIDGen{}, fixedClock{}, nil,
		providers,
		stubModelsRepo{}, stubAliasesRepo{}, stubRoutesRepo{},
		stubApiKeysRepo{}, stubQuotaPoliciesRepo{}, stubUsageLogsRepo{},
		stubPriceVersionsRepo{}, stubProviderHealthRepo{},
	)
	factory := aiupstream.NewAdapterFactory()
	factory.Decryptor = enc
	svc.Upstreams = factory
	return svc, enc
}

func TestAdminHandler_TestProvider_Success(t *testing.T) {
	const plaintextKey = "sk-live-probe-fixture"
	providers := newFakeProvidersRepo()
	svc, enc := newProviderTestService(t, providers)
	sealed, err := enc.EncryptField(plaintextKey)
	require.NoError(t, err)

	type probeRequest struct {
		method string
		path   string
		auth   string
		body   string
	}
	captured := make(chan probeRequest, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		captured <- probeRequest{
			method: r.Method,
			path:   r.URL.Path,
			auth:   r.Header.Get("Authorization"),
			body:   string(raw),
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer ts.Close()

	require.NoError(t, providers.Create(context.Background(), &aigatewaydomain.LlmProvider{
		ID: "p-test", OrganizationID: fixedOrgID, Name: "test-llm",
		ProviderType: "openai", PreferredProtocol: "openai",
		BaseURL:          ts.URL,
		Status:           "active",
		RequestTimeoutMs: 5000,
		APIKeySealed:     sealed,
		CreatedAt:        fixedNow, UpdatedAt: fixedNow,
	}))

	r := newProviderTestRouter(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/providers/p-test/test", nil)
	req.Header.Set("Authorization", "Bearer fake-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "p-test", body["providerId"])
	assert.Equal(t, true, body["reachable"], "TestProvider response should report reachable=true")
	sc, ok := body["statusCode"].(float64)
	assert.True(t, ok, "statusCode should be present and numeric")
	assert.Equal(t, float64(http.StatusOK), sc)

	select {
	case got := <-captured:
		assert.Equal(t, http.MethodGet, got.method)
		assert.Equal(t, "/models", got.path)
		assert.Equal(t, "Bearer "+plaintextKey, got.auth,
			"decrypted credential must reach the upstream request")
		assert.Empty(t, got.body)
	case <-time.After(2 * time.Second):
		t.Fatal("upstream probe request was not observed")
	}
	assert.NotContains(t, rec.Body.String(), plaintextKey)
	assert.NotContains(t, rec.Body.String(), sealed)
}

func TestAdminHandler_TestProvider_RejectsLinkLocalMetadata(t *testing.T) {
	// 管理面盲 SSRF 回归：BaseURL 指向云元数据（link-local）必须被 SSRF 防护拒绝，
	// 不允许用 TestProvider 探测内网元数据服务。
	providers := newFakeProvidersRepo()
	svc, _ := newProviderTestService(t, providers)

	require.NoError(t, providers.Create(context.Background(), &aigatewaydomain.LlmProvider{
		ID: "p-meta", OrganizationID: fixedOrgID, Name: "meta-probe",
		ProviderType: "openai", PreferredProtocol: "openai",
		BaseURL:          "http://169.254.169.254/latest/meta-data/",
		Status:           "active",
		RequestTimeoutMs: 3000,
		APIKeySealed:     "",
		CreatedAt:        fixedNow, UpdatedAt: fixedNow,
	}))

	r := newProviderTestRouter(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/providers/p-meta/test", nil)
	req.Header.Set("Authorization", "Bearer fake-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, false, body["reachable"], "link-local metadata target must not be reachable")
	_, hasError := body["error"].(string)
	assert.True(t, hasError, "blocked probe must report a structured error")
}

func TestAdminHandler_TestProvider_DecryptFailureIsInternalAndRedacted(t *testing.T) {
	const sealed = "v1:not-decryptable-probe-fixture"
	providers := newFakeProvidersRepo()
	svc, _ := newProviderTestService(t, providers)

	require.NoError(t, providers.Create(context.Background(), &aigatewaydomain.LlmProvider{
		ID: "p-sealed", OrganizationID: fixedOrgID, Name: "sealed-provider",
		ProviderType: "openai", PreferredProtocol: "openai",
		BaseURL:          "https://api.openai.com/v1",
		Status:           "active",
		RequestTimeoutMs: 3000,
		APIKeySealed:     sealed,
		CreatedAt:        fixedNow, UpdatedAt: fixedNow,
	}))

	var logBuf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	r := newProviderTestRouter(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/providers/p-sealed/test", nil)
	req.Header.Set("Authorization", "Bearer fake-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code, "body=%s", rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	errBody, ok := body["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, string(apperrors.CodeInternal), errBody["code"])

	assert.NotContains(t, rec.Body.String(), sealed)
	assert.NotContains(t, rec.Body.String(), "not-decryptable")
	assert.NotContains(t, logBuf.String(), sealed)
	assert.NotContains(t, logBuf.String(), "not-decryptable")
}

func TestAdminHandler_TestProvider_UnsupportedProviderType(t *testing.T) {
	providers := newFakeProvidersRepo()
	svc, _ := newProviderTestService(t, providers)

	require.NoError(t, providers.Create(context.Background(), &aigatewaydomain.LlmProvider{
		ID: "p-custom", OrganizationID: fixedOrgID, Name: "custom-provider",
		ProviderType: "custom", PreferredProtocol: "custom",
		BaseURL:          "https://upstream.example.com",
		Status:           "active",
		RequestTimeoutMs: 3000,
		APIKeySealed:     "",
		CreatedAt:        fixedNow, UpdatedAt: fixedNow,
	}))

	r := newProviderTestRouter(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/providers/p-custom/test", nil)
	req.Header.Set("Authorization", "Bearer fake-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code, "body=%s", rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	errBody, ok := body["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, string(apperrors.CodeUnavailable), errBody["code"])
}

func TestAdminHandler_TestProvider_UpstreamRegistryNotConfigured(t *testing.T) {
	providers := newFakeProvidersRepo()
	require.NoError(t, providers.Create(context.Background(), &aigatewaydomain.LlmProvider{
		ID: "p-noreg", OrganizationID: fixedOrgID, Name: "unwired-provider",
		ProviderType: "openai", PreferredProtocol: "openai",
		BaseURL:          "https://upstream.example.com",
		Status:           "active",
		RequestTimeoutMs: 3000,
		APIKeySealed:     "",
		CreatedAt:        fixedNow, UpdatedAt: fixedNow,
	}))

	svc := appgateway.NewAdminService(
		&sequentialUUIDGen{}, fixedClock{}, nil,
		providers,
		stubModelsRepo{}, stubAliasesRepo{}, stubRoutesRepo{},
		stubApiKeysRepo{}, stubQuotaPoliciesRepo{}, stubUsageLogsRepo{},
		stubPriceVersionsRepo{}, stubProviderHealthRepo{},
	)

	r := newProviderTestRouter(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/providers/p-noreg/test", nil)
	req.Header.Set("Authorization", "Bearer fake-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code, "body=%s", rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	errBody, ok := body["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, string(apperrors.CodeUnavailable), errBody["code"])
}

func TestAdminHandler_TestProvider_NotFound(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", "BX1Ab1Jz6cRDESuWCleZWGRv7RHQRAf0sqikOVcVzkg=")

	svc := appgateway.NewAdminService(
		&sequentialUUIDGen{}, fixedClock{}, nil,
		newFakeProvidersRepo(), // empty
		stubModelsRepo{}, stubAliasesRepo{}, stubRoutesRepo{},
		stubApiKeysRepo{}, stubQuotaPoliciesRepo{}, stubUsageLogsRepo{},
		stubPriceVersionsRepo{}, stubProviderHealthRepo{},
	)

	handler := NewAdminHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			s := auth.SessionContext{
				ID: "sess-test", OrganizationID: fixedOrgID, OrganizationSlug: fixedOrgSlug,
				Username: "admin", AccessToken: "fake-token", Source: "repository", AuthTime: fixedNow,
			}
			req = req.WithContext(auth.WithSessionContext(req.Context(), s))
			next.ServeHTTP(w, req)
		})
	})
	RegisterAdminChiRoutes(r, handler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/providers/nonexistent/test", nil)
	req.Header.Set("Authorization", "Bearer fake-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
