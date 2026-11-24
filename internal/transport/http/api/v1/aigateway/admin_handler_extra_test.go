package aigateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	appgateway "github.com/guliangiam/guliangiam/internal/app/aigateway"
	aigatewaydomain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Fake ModelsRepo ──────────────────────────────────────────────────

type fakeModelsRepo struct {
	items       map[string]*aigatewaydomain.LlmModel
	updateCalls int
	deleteCalls int
}

func newFakeModelsRepo() *fakeModelsRepo {
	return &fakeModelsRepo{items: map[string]*aigatewaydomain.LlmModel{}}
}

func (f *fakeModelsRepo) GetByID(_ context.Context, id string) (*aigatewaydomain.LlmModel, error) {
	if m, ok := f.items[id]; ok {
		cp := *m
		return &cp, nil
	}
	return nil, apperrors.NewNotFound("model not found")
}
func (f *fakeModelsRepo) GetByIDForOrganization(_ context.Context, org, id string) (*aigatewaydomain.LlmModel, error) {
	m, ok := f.items[id]
	if !ok || m.OrganizationID != org {
		return nil, apperrors.NewNotFound("model not found")
	}
	cp := *m
	return &cp, nil
}
func (f *fakeModelsRepo) List(_ context.Context, org string) ([]*aigatewaydomain.LlmModel, error) {
	var out []*aigatewaydomain.LlmModel
	for _, m := range f.items {
		if m.OrganizationID == org {
			cp := *m
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (f *fakeModelsRepo) ListByProvider(_ context.Context, id string) ([]*aigatewaydomain.LlmModel, error) {
	var out []*aigatewaydomain.LlmModel
	for _, m := range f.items {
		if m.ProviderID == id {
			cp := *m
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (f *fakeModelsRepo) Create(_ context.Context, m *aigatewaydomain.LlmModel) error {
	cp := *m
	f.items[m.ID] = &cp
	return nil
}
func (f *fakeModelsRepo) Update(_ context.Context, m *aigatewaydomain.LlmModel) error {
	if _, ok := f.items[m.ID]; !ok {
		return apperrors.NewNotFound("model not found")
	}
	cp := *m
	f.items[m.ID] = &cp
	return nil
}
func (f *fakeModelsRepo) Delete(_ context.Context, id string) error {
	delete(f.items, id)
	return nil
}
func (f *fakeModelsRepo) UpdateForOrganization(_ context.Context, m *aigatewaydomain.LlmModel, org string) error {
	cur, ok := f.items[m.ID]
	if !ok || cur.OrganizationID != org {
		return apperrors.NewNotFound("model not found")
	}
	f.updateCalls++
	cp := *m
	f.items[m.ID] = &cp
	return nil
}
func (f *fakeModelsRepo) DeleteForOrganization(_ context.Context, org, id string) error {
	m, ok := f.items[id]
	if !ok || m.OrganizationID != org {
		return apperrors.NewNotFound("model not found")
	}
	f.deleteCalls++
	delete(f.items, id)
	return nil
}

// ── Fake AliasesRepo ─────────────────────────────────────────────────

type fakeAliasesRepo struct {
	items       map[string]*aigatewaydomain.LlmModelAlias
	updateCalls int
	deleteCalls int
}

func newFakeAliasesRepo() *fakeAliasesRepo {
	return &fakeAliasesRepo{items: map[string]*aigatewaydomain.LlmModelAlias{}}
}

func (f *fakeAliasesRepo) GetByID(_ context.Context, id string) (*aigatewaydomain.LlmModelAlias, error) {
	if a, ok := f.items[id]; ok {
		cp := *a
		return &cp, nil
	}
	return nil, apperrors.NewNotFound("alias not found")
}
func (f *fakeAliasesRepo) GetByIDForOrganization(_ context.Context, org, id string) (*aigatewaydomain.LlmModelAlias, error) {
	a, ok := f.items[id]
	if !ok || a.OrganizationID != org {
		return nil, apperrors.NewNotFound("alias not found")
	}
	cp := *a
	return &cp, nil
}
func (f *fakeAliasesRepo) GetByName(_ context.Context, org, name string) (*aigatewaydomain.LlmModelAlias, error) {
	for _, a := range f.items {
		if a.OrganizationID == org && a.Alias == name {
			cp := *a
			return &cp, nil
		}
	}
	return nil, nil
}
func (f *fakeAliasesRepo) List(_ context.Context, org string) ([]*aigatewaydomain.LlmModelAlias, error) {
	var out []*aigatewaydomain.LlmModelAlias
	for _, a := range f.items {
		if a.OrganizationID == org {
			cp := *a
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (f *fakeAliasesRepo) ListAll(context.Context) ([]*aigatewaydomain.LlmModelAlias, error) {
	var out []*aigatewaydomain.LlmModelAlias
	for _, a := range f.items {
		cp := *a
		out = append(out, &cp)
	}
	return out, nil
}
func (f *fakeAliasesRepo) Create(_ context.Context, a *aigatewaydomain.LlmModelAlias) error {
	cp := *a
	f.items[a.ID] = &cp
	return nil
}
func (f *fakeAliasesRepo) Update(_ context.Context, a *aigatewaydomain.LlmModelAlias) error {
	if _, ok := f.items[a.ID]; !ok {
		return apperrors.NewNotFound("alias not found")
	}
	cp := *a
	f.items[a.ID] = &cp
	return nil
}
func (f *fakeAliasesRepo) Delete(_ context.Context, id string) error {
	delete(f.items, id)
	return nil
}
func (f *fakeAliasesRepo) UpdateForOrganization(_ context.Context, a *aigatewaydomain.LlmModelAlias, org string) error {
	cur, ok := f.items[a.ID]
	if !ok || cur.OrganizationID != org {
		return apperrors.NewNotFound("alias not found")
	}
	f.updateCalls++
	cp := *a
	f.items[a.ID] = &cp
	return nil
}
func (f *fakeAliasesRepo) DeleteForOrganization(_ context.Context, org, id string) error {
	a, ok := f.items[id]
	if !ok || a.OrganizationID != org {
		return apperrors.NewNotFound("alias not found")
	}
	f.deleteCalls++
	delete(f.items, id)
	return nil
}

// ── Fake RoutesRepo ──────────────────────────────────────────────────

type fakeRoutesRepo struct {
	items map[string]*aigatewaydomain.LlmRoute
}

func newFakeRoutesRepo() *fakeRoutesRepo {
	return &fakeRoutesRepo{items: map[string]*aigatewaydomain.LlmRoute{}}
}

func (f *fakeRoutesRepo) GetByID(_ context.Context, id string) (*aigatewaydomain.LlmRoute, error) {
	if r, ok := f.items[id]; ok {
		cp := *r
		return &cp, nil
	}
	return nil, apperrors.NewNotFound("route not found")
}
func (f *fakeRoutesRepo) ListByAlias(_ context.Context, aliasID string) ([]*aigatewaydomain.LlmRoute, error) {
	var out []*aigatewaydomain.LlmRoute
	for _, r := range f.items {
		if r.AliasID == aliasID {
			cp := *r
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (f *fakeRoutesRepo) List(_ context.Context, org string) ([]*aigatewaydomain.LlmRoute, error) {
	var out []*aigatewaydomain.LlmRoute
	for _, r := range f.items {
		if r.OrganizationID == org {
			cp := *r
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (f *fakeRoutesRepo) Create(_ context.Context, r *aigatewaydomain.LlmRoute) error {
	cp := *r
	f.items[r.ID] = &cp
	return nil
}
func (f *fakeRoutesRepo) Update(_ context.Context, r *aigatewaydomain.LlmRoute) error {
	if _, ok := f.items[r.ID]; !ok {
		return apperrors.NewNotFound("route not found")
	}
	cp := *r
	f.items[r.ID] = &cp
	return nil
}
func (f *fakeRoutesRepo) Delete(_ context.Context, id string) error {
	delete(f.items, id)
	return nil
}

// ── New test wiring with real fake repos ──────────────────────────────

func newTestAdminRouterWithFakes(t *testing.T) (chi.Router, *fakeProvidersRepo, *fakeModelsRepo, *fakeAliasesRepo, *fakeRoutesRepo) {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY_CURRENT", "BX1Ab1Jz6cRDESuWCleZWGRv7RHQRAf0sqikOVcVzkg=")

	uuidGen := &sequentialUUIDGen{}
	providers := newFakeProvidersRepo()
	models := newFakeModelsRepo()
	aliases := newFakeAliasesRepo()
	routes := newFakeRoutesRepo()

	svc := appgateway.NewAdminService(
		uuidGen,
		fixedClock{},
		nil,
		providers,
		models,
		aliases,
		routes,
		stubApiKeysRepo{},
		stubQuotaPoliciesRepo{},
		stubUsageLogsRepo{},
		stubPriceVersionsRepo{},
		stubProviderHealthRepo{},
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
	return r, providers, models, aliases, routes
}

// ── Models CRUD ──────────────────────────────────────────────────────

func newTestAdminRouterWithUsageLogs(t *testing.T, usageLogs aigatewaydomain.UsageLogsRepo) chi.Router {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY_CURRENT", "BX1Ab1Jz6cRDESuWCleZWGRv7RHQRAf0sqikOVcVzkg=")

	svc := appgateway.NewAdminService(
		&sequentialUUIDGen{}, fixedClock{}, nil,
		newFakeProvidersRepo(), stubModelsRepo{}, stubAliasesRepo{}, stubRoutesRepo{},
		stubApiKeysRepo{}, stubQuotaPoliciesRepo{}, usageLogs,
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
	return r
}

func TestAdminHandler_ModelsCRUD(t *testing.T) {
	router, providers, models, _, _ := newTestAdminRouterWithFakes(t)

	// Seed a provider so ModelCreate's FK check passes
	providers.items["p-1"] = &aigatewaydomain.LlmProvider{
		ID: "p-1", OrganizationID: fixedOrgID, Name: "openai", ProviderType: "openai",
		BaseURL: "https://api.openai.com", Status: "active", CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}

	// List empty → 200, []
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/models", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assertStatus(t, rec, http.StatusOK)
	emptyList := decodeList(t, rec)
	assert.Empty(t, emptyList)

	// Create → 201
	createBody := map[string]any{
		"providerId":        "p-1",
		"providerModelName": "gpt-4o",
		"displayName":       "GPT-4o (Latest)",
		"status":            "active",
	}
	createRec := performJSON(t, router, http.MethodPost, "/api/v1/ai/models", createBody)
	assertStatus(t, createRec, http.StatusCreated)
	var created map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	modelID := created["id"].(string)
	assert.Equal(t, "gpt-4o", created["providerModelName"])
	assert.Equal(t, fixedOrgID, created["organizationId"])

	// List → 1 item
	listRec := performJSON(t, router, http.MethodGet, "/api/v1/ai/models", nil)
	assertStatus(t, listRec, http.StatusOK)
	listed := decodeList(t, listRec)
	assert.Len(t, listed, 1)

	// Get → 200
	getRec := performJSON(t, router, http.MethodGet, "/api/v1/ai/models/"+modelID, nil)
	assertStatus(t, getRec, http.StatusOK)

	// Get not found → 404
	nfRec := performJSON(t, router, http.MethodGet, "/api/v1/ai/models/00000000-0000-0000-0000-000000000000", nil)
	assertStatus(t, nfRec, http.StatusNotFound)

	// Partial PATCH → 200
	patchRec := performJSON(t, router, http.MethodPatch, "/api/v1/ai/models/"+modelID, map[string]any{
		"displayName": "GPT-4o Turbo",
	})
	assertStatus(t, patchRec, http.StatusOK)
	var patched map[string]any
	require.NoError(t, json.Unmarshal(patchRec.Body.Bytes(), &patched))
	assert.Equal(t, "GPT-4o Turbo", patched["displayName"])

	// PATCH not found → 404
	patchNf := performJSON(t, router, http.MethodPatch, "/api/v1/ai/models/00000000-0000-0000-0000-000000000000",
		map[string]any{"displayName": "x"})
	assertStatus(t, patchNf, http.StatusNotFound)

	// DELETE → 204
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/ai/models/"+modelID, nil)
	delRec := httptest.NewRecorder()
	router.ServeHTTP(delRec, delReq)
	assertStatus(t, delRec, http.StatusNoContent)

	// Verify gone
	gone := performJSON(t, router, http.MethodGet, "/api/v1/ai/models/"+modelID, nil)
	assertStatus(t, gone, http.StatusNotFound)

	// Verify repo state
	assert.Equal(t, 0, len(models.items))
}

// ── Aliases CRUD ─────────────────────────────────────────────────────

func TestAdminHandler_AliasesCRUD(t *testing.T) {
	router, _, _, aliases, _ := newTestAdminRouterWithFakes(t)

	// Create → 201
	createBody := map[string]any{
		"alias":       "fast",
		"displayName": "Fast model",
		"status":      "active",
	}
	createRec := performJSON(t, router, http.MethodPost, "/api/v1/ai/aliases", createBody)
	assertStatus(t, createRec, http.StatusCreated)
	var created map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	aliasID := created["id"].(string)
	assert.Equal(t, "fast", created["alias"])

	// List → 1 item
	listRec := performJSON(t, router, http.MethodGet, "/api/v1/ai/aliases", nil)
	assertStatus(t, listRec, http.StatusOK)
	listed := decodeList(t, listRec)
	assert.Len(t, listed, 1)

	// Get → 200
	getRec := performJSON(t, router, http.MethodGet, "/api/v1/ai/aliases/"+aliasID, nil)
	assertStatus(t, getRec, http.StatusOK)

	// PATCH → 200
	patchRec := performJSON(t, router, http.MethodPatch, "/api/v1/ai/aliases/"+aliasID, map[string]any{
		"displayName": "Turbo",
	})
	assertStatus(t, patchRec, http.StatusOK)
	var patched map[string]any
	require.NoError(t, json.Unmarshal(patchRec.Body.Bytes(), &patched))
	assert.Equal(t, "Turbo", patched["displayName"])

	// DELETE → 204
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/ai/aliases/"+aliasID, nil)
	delRec := httptest.NewRecorder()
	router.ServeHTTP(delRec, delReq)
	assertStatus(t, delRec, http.StatusNoContent)

	// Verify repo empty
	assert.Equal(t, 0, len(aliases.items))
}

// ── Routes CRUD (needs alias + model) ────────────────────────────────

func TestAdminHandler_RoutesCRUD(t *testing.T) {
	router, providers, models, aliases, routes := newTestAdminRouterWithFakes(t)

	// Seed provider → model
	providers.items["p-1"] = &aigatewaydomain.LlmProvider{
		ID: "p-1", OrganizationID: fixedOrgID, Name: "openai", ProviderType: "openai",
		BaseURL: "https://api.openai.com", Status: "active", CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}
	models.items["m-1"] = &aigatewaydomain.LlmModel{
		ID: "m-1", OrganizationID: fixedOrgID, ProviderID: "p-1", ProviderModelName: "gpt-4o",
		Status: "active", CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}
	aliases.items["a-1"] = &aigatewaydomain.LlmModelAlias{
		ID: "a-1", OrganizationID: fixedOrgID, Alias: "default",
		Status: "active", CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}

	// List empty → 200
	listRec := performJSON(t, router, http.MethodGet, "/api/v1/ai/routes", nil)
	assertStatus(t, listRec, http.StatusOK)
	emptyList := decodeList(t, listRec)
	assert.Empty(t, emptyList)

	// Create route → 201
	createBody := map[string]any{
		"aliasId":  "a-1",
		"modelId":  "m-1",
		"priority": 10,
		"weight":   80,
	}
	createRec := performJSON(t, router, http.MethodPost, "/api/v1/ai/routes", createBody)
	assertStatus(t, createRec, http.StatusCreated)
	var created map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	routeID := created["id"].(string)
	assert.Equal(t, float64(10), created["priority"])

	// Get → 200
	getRec := performJSON(t, router, http.MethodGet, "/api/v1/ai/routes/"+routeID, nil)
	assertStatus(t, getRec, http.StatusOK)

	// Get not found → 404
	nfRec := performJSON(t, router, http.MethodGet, "/api/v1/ai/routes/00000000-0000-0000-0000-000000000000", nil)
	assertStatus(t, nfRec, http.StatusNotFound)

	// PATCH → 200
	patchRec := performJSON(t, router, http.MethodPatch, "/api/v1/ai/routes/"+routeID, map[string]any{
		"weight": 50,
	})
	assertStatus(t, patchRec, http.StatusOK)
	var patched map[string]any
	require.NoError(t, json.Unmarshal(patchRec.Body.Bytes(), &patched))
	assert.Equal(t, float64(50), patched["weight"])

	// DELETE → 204
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/ai/routes/"+routeID, nil)
	delRec := httptest.NewRecorder()
	router.ServeHTTP(delRec, delReq)
	assertStatus(t, delRec, http.StatusNoContent)

	// Verify
	assert.Equal(t, 0, len(routes.items))
}

// ── Usage Logs (read-only) ───────────────────────────────────────────

func TestAdminHandler_UsageLogsList(t *testing.T) {
	router, _, _, _, _ := newTestAdminRouterWithFakes(t)

	// GET /api/v1/ai/usage → 200 (repo stub returns empty)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/usage", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assertStatus(t, rec, http.StatusOK)

	// GET /api/v1/ai/usage/stats?days=7 → 200
	statsReq := httptest.NewRequest(http.MethodGet, "/api/v1/ai/usage/stats?days=7", nil)
	statsRec := httptest.NewRecorder()
	router.ServeHTTP(statsRec, statsReq)
	assertStatus(t, statsRec, http.StatusOK)
}

// The management DTO exposes llm_usage_logs.error_code verbatim, so the column
// must contain a normalized code — never the raw upstream error text.
func TestAdminHandler_UsageLogsList_ErrorCodeIsNormalized(t *testing.T) {
	code := "ai_upstream_error"
	router := newTestAdminRouterWithUsageLogs(t, stubUsageLogsRepo{logs: []*aigatewaydomain.UsageLog{{
		ID: "u-1", OrganizationID: fixedOrgID, AliasID: "a-1", ProviderID: "p-1", ModelID: "m-1",
		ProtocolFace: "gemini", AttemptNo: 1, Status: "failed", ErrorCode: &code,
		RequestID: "req-1", CreatedAt: fixedNow,
	}}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/usage", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assertStatus(t, rec, http.StatusOK)

	var body struct {
		Items []struct {
			ErrorCode *string `json:"errorCode"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Items, 1)
	require.NotNil(t, body.Items[0].ErrorCode)
	assert.Equal(t, code, *body.Items[0].ErrorCode)
	assert.NotContains(t, rec.Body.String(), "gemini: request failed")
}

// Verify compile-time satisfaction for the Domain Port interfaces.
var (
	_ aigatewaydomain.ModelsRepo              = (*fakeModelsRepo)(nil)
	_ aigatewaydomain.TenantScopedModelsRepo  = (*fakeModelsRepo)(nil)
	_ aigatewaydomain.AliasesRepo             = (*fakeAliasesRepo)(nil)
	_ aigatewaydomain.TenantScopedAliasesRepo = (*fakeAliasesRepo)(nil)
	_ aigatewaydomain.RoutesRepo              = (*fakeRoutesRepo)(nil)
)

// ── API Keys Get / Update / Delete (reuses existing newTestAdminRouter fixture) ──

func TestAdminHandler_APIKeys_UpdateDelete(t *testing.T) {
	router, repos := newTestAdminRouter(t)

	// First create a key — fixture already supports this
	createBody := map[string]any{
		"name": "temp-key", "rpmLimit": 1000, "currency": "USD",
	}
	createRec := performJSON(t, router, http.MethodPost, "/api/v1/ai/api-keys", createBody)
	assertStatus(t, createRec, http.StatusCreated)
	var created map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	keyID := created["id"].(string)

	// GET by ID → 200
	getRec := performJSON(t, router, http.MethodGet, "/api/v1/ai/api-keys/"+keyID, nil)
	assertStatus(t, getRec, http.StatusOK)
	var got map[string]any
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &got))
	assert.Equal(t, "temp-key", got["name"])

	// GET non-existent → 404
	nfRec := performJSON(t, router, http.MethodGet, "/api/v1/ai/api-keys/00000000-0000-0000-0000-000000000000", nil)
	assertStatus(t, nfRec, http.StatusNotFound)

	// PATCH name → 200
	patchBody := map[string]any{"name": "renamed-key"}
	patchRec := performJSON(t, router, http.MethodPatch, "/api/v1/ai/api-keys/"+keyID, patchBody)
	assertStatus(t, patchRec, http.StatusOK)
	var patched map[string]any
	require.NoError(t, json.Unmarshal(patchRec.Body.Bytes(), &patched))
	assert.Equal(t, "renamed-key", patched["name"])

	// PATCH non-existent → 404
	patchNf := performJSON(t, router, http.MethodPatch, "/api/v1/ai/api-keys/00000000-0000-0000-0000-000000000000",
		map[string]any{"name": "x"})
	assertStatus(t, patchNf, http.StatusNotFound)

	// DELETE → 204
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/ai/api-keys/"+keyID, nil)
	delRec := httptest.NewRecorder()
	router.ServeHTTP(delRec, delReq)
	assertStatus(t, delRec, http.StatusNoContent)

	// Verify gone via repo
	assert.Equal(t, 0, len(repos.ApiKeys.items))
}

// ── Quotas CRUD (needs new fake repo + newTestAdminRouter variant) ──────

type fakeQuotasRepo struct {
	items map[string]*aigatewaydomain.LlmQuotaPolicy
}

func newFakeQuotasRepo() *fakeQuotasRepo {
	return &fakeQuotasRepo{items: map[string]*aigatewaydomain.LlmQuotaPolicy{}}
}

func (f *fakeQuotasRepo) GetByID(_ context.Context, id string) (*aigatewaydomain.LlmQuotaPolicy, error) {
	if q, ok := f.items[id]; ok {
		cp := *q
		return &cp, nil
	}
	return nil, apperrors.NewNotFound("quota not found")
}
func (f *fakeQuotasRepo) List(_ context.Context, org string) ([]*aigatewaydomain.LlmQuotaPolicy, error) {
	var out []*aigatewaydomain.LlmQuotaPolicy
	for _, q := range f.items {
		if q.OrganizationID == org {
			cp := *q
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (f *fakeQuotasRepo) Create(_ context.Context, q *aigatewaydomain.LlmQuotaPolicy) error {
	cp := *q
	f.items[q.ID] = &cp
	return nil
}
func (f *fakeQuotasRepo) Update(_ context.Context, q *aigatewaydomain.LlmQuotaPolicy) error {
	if _, ok := f.items[q.ID]; !ok {
		return apperrors.NewNotFound("quota not found")
	}
	cp := *q
	f.items[q.ID] = &cp
	return nil
}
func (f *fakeQuotasRepo) Delete(_ context.Context, id string) error {
	delete(f.items, id)
	return nil
}

// newTestAdminRouterWithQuotas wires everything including a real fake Quotas repo.
func newTestAdminRouterWithQuotas(t *testing.T) (chi.Router, *fakeQuotasRepo) {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY_CURRENT", "BX1Ab1Jz6cRDESuWCleZWGRv7RHQRAf0sqikOVcVzkg=")

	uuidGen := &sequentialUUIDGen{}
	quotas := newFakeQuotasRepo()

	svc := appgateway.NewAdminService(
		uuidGen, fixedClock{}, nil,
		newFakeProvidersRepo(), stubModelsRepo{}, stubAliasesRepo{}, stubRoutesRepo{},
		stubApiKeysRepo{}, quotas, stubUsageLogsRepo{}, stubPriceVersionsRepo{}, stubProviderHealthRepo{},
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
	return r, quotas
}

func TestAdminHandler_QuotasCRUD(t *testing.T) {
	router, quotas := newTestAdminRouterWithQuotas(t)

	// List empty → 200
	listRec := performJSON(t, router, http.MethodGet, "/api/v1/ai/quotas", nil)
	assertStatus(t, listRec, http.StatusOK)
	empty := decodeList(t, listRec)
	assert.Empty(t, empty)

	// Create → 201
	createBody := map[string]any{
		"scope":      "organization",
		"limitType":  "rpm",
		"limitValue": 60,
		"resetCycle": "daily",
	}
	createRec := performJSON(t, router, http.MethodPost, "/api/v1/ai/quotas", createBody)
	assertStatus(t, createRec, http.StatusCreated)
	var created map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	quotaID := created["id"].(string)
	assert.Equal(t, "organization", created["scope"])
	assert.Equal(t, fixedOrgID, created["organizationId"])

	// Get → 200
	getRec := performJSON(t, router, http.MethodGet, "/api/v1/ai/quotas/"+quotaID, nil)
	assertStatus(t, getRec, http.StatusOK)

	// Get not found → 404
	nfRec := performJSON(t, router, http.MethodGet, "/api/v1/ai/quotas/00000000-0000-0000-0000-000000000000", nil)
	assertStatus(t, nfRec, http.StatusNotFound)

	// PATCH limitValue → 200
	patchBody := map[string]any{"limitValue": 120}
	patchRec := performJSON(t, router, http.MethodPatch, "/api/v1/ai/quotas/"+quotaID, patchBody)
	assertStatus(t, patchRec, http.StatusOK)
	var patched map[string]any
	require.NoError(t, json.Unmarshal(patchRec.Body.Bytes(), &patched))
	assert.Equal(t, float64(120), patched["limitValue"])

	// DELETE → 204
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/ai/quotas/"+quotaID, nil)
	delRec := httptest.NewRecorder()
	router.ServeHTTP(delRec, delReq)
	assertStatus(t, delRec, http.StatusNoContent)
	assert.Equal(t, 0, len(quotas.items))
}

// Verify compile-time satisfaction for the Domain Port interfaces.
var _ aigatewaydomain.QuotaPoliciesRepo = (*fakeQuotasRepo)(nil)
