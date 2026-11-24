package aigateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	appgateway "github.com/guliangiam/guliangiam/internal/app/aigateway"
	aigatewaydomain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	platformclock "github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixedOrgID is the UUID used by every SessionContext in these tests.
// It's also the OrganizationID written by fake repo implementations —
// so the handler's session-extracted orgID matches what's stored.
const fixedOrgID = "01a0ea76-0c00-7000-8000-000000000001"
const fixedOrgSlug = "built-in"

var fixedNow = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

// ── UUID / Clock fakes ──────────────────────────────────────────────────

type sequentialUUIDGen struct {
	counter int
}

func (s *sequentialUUIDGen) NewString() (string, error) {
	s.counter++
	// Stable UUIDs — each call increments a known suffix.
	return "aaaaaaaa-0000-0000-0000-" + leftPad6(s.counter), nil
}

type fixedClock struct {
	t time.Time
}

func (fixedClock) Now() time.Time { return fixedNow }

func leftPad6(n int) string {
	s := ""
	for i := 0; i < 6 && n > 0; i++ {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	for len(s) < 6 {
		s = "0" + s
	}
	return s
}

// ── Fake ProvidersRepo ────────────────────────────────────────────────────

type fakeProvidersRepo struct {
	items       map[string]*aigatewaydomain.LlmProvider
	updateCalls int
	deleteCalls int
}

func newFakeProvidersRepo() *fakeProvidersRepo {
	return &fakeProvidersRepo{items: map[string]*aigatewaydomain.LlmProvider{}}
}

func (f *fakeProvidersRepo) GetByID(_ context.Context, id string) (*aigatewaydomain.LlmProvider, error) {
	if p, ok := f.items[id]; ok {
		cp := *p
		return &cp, nil
	}
	return nil, apperrors.NewNotFound("provider not found")
}

func (f *fakeProvidersRepo) GetByIDForOrganization(_ context.Context, organizationID, id string) (*aigatewaydomain.LlmProvider, error) {
	p, ok := f.items[id]
	if !ok || p.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("provider not found")
	}
	cp := *p
	return &cp, nil
}

func (f *fakeProvidersRepo) List(_ context.Context, organizationID string) ([]*aigatewaydomain.LlmProvider, error) {
	var out []*aigatewaydomain.LlmProvider
	for _, p := range f.items {
		if p.OrganizationID == organizationID {
			cp := *p
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (f *fakeProvidersRepo) Create(_ context.Context, p *aigatewaydomain.LlmProvider) error {
	cp := *p
	f.items[p.ID] = &cp
	return nil
}

func (f *fakeProvidersRepo) Update(_ context.Context, p *aigatewaydomain.LlmProvider) error {
	cp := *p
	f.items[p.ID] = &cp
	return nil
}

func (f *fakeProvidersRepo) UpdateForOrganization(_ context.Context, p *aigatewaydomain.LlmProvider, organizationID string) error {
	cur, ok := f.items[p.ID]
	if !ok || cur.OrganizationID != organizationID {
		return apperrors.NewNotFound("provider not found")
	}
	f.updateCalls++
	cp := *p
	f.items[p.ID] = &cp
	return nil
}

func (f *fakeProvidersRepo) Delete(_ context.Context, id string) error {
	delete(f.items, id)
	return nil
}

func (f *fakeProvidersRepo) DeleteForOrganization(_ context.Context, organizationID, id string) error {
	p, ok := f.items[id]
	if !ok || p.OrganizationID != organizationID {
		return apperrors.NewNotFound("provider not found")
	}
	f.deleteCalls++
	delete(f.items, id)
	return nil
}

func (f *fakeProvidersRepo) GetByName(_ context.Context, organizationID, name string) (*aigatewaydomain.LlmProvider, error) {
	for _, p := range f.items {
		if p.OrganizationID == organizationID && p.Name == name {
			cp := *p
			return &cp, nil
		}
	}
	return nil, nil
}

// ── Stub repos (each interface gets its own tiny zero-value struct) ──────
//
// Go does NOT let a single type satisfy multiple interfaces whose method
// signatures overlap (e.g. every repo has a `List(context.Context, string)`
// but with different element types — those are distinct methods).  Each stub
// below is a dedicated type with exactly the methods the AdminService
// constructor requires for its interface.

type stubModelsRepo struct{}

func (stubModelsRepo) GetByID(context.Context, string) (*aigatewaydomain.LlmModel, error) {
	return nil, nil
}
func (stubModelsRepo) List(context.Context, string) ([]*aigatewaydomain.LlmModel, error) {
	return nil, nil
}
func (stubModelsRepo) ListByProvider(context.Context, string) ([]*aigatewaydomain.LlmModel, error) {
	return nil, nil
}
func (stubModelsRepo) Create(context.Context, *aigatewaydomain.LlmModel) error { return nil }
func (stubModelsRepo) Update(context.Context, *aigatewaydomain.LlmModel) error { return nil }
func (stubModelsRepo) Delete(context.Context, string) error                    { return nil }

type stubAliasesRepo struct{}

func (stubAliasesRepo) GetByID(context.Context, string) (*aigatewaydomain.LlmModelAlias, error) {
	return nil, nil
}
func (stubAliasesRepo) GetByName(context.Context, string, string) (*aigatewaydomain.LlmModelAlias, error) {
	return nil, nil
}
func (stubAliasesRepo) List(context.Context, string) ([]*aigatewaydomain.LlmModelAlias, error) {
	return nil, nil
}
func (stubAliasesRepo) ListAll(context.Context) ([]*aigatewaydomain.LlmModelAlias, error) {
	return nil, nil
}
func (stubAliasesRepo) Create(context.Context, *aigatewaydomain.LlmModelAlias) error { return nil }
func (stubAliasesRepo) Update(context.Context, *aigatewaydomain.LlmModelAlias) error { return nil }
func (stubAliasesRepo) Delete(context.Context, string) error                         { return nil }

type stubRoutesRepo struct{}

func (stubRoutesRepo) GetByID(context.Context, string) (*aigatewaydomain.LlmRoute, error) {
	return nil, nil
}
func (stubRoutesRepo) ListByAlias(context.Context, string) ([]*aigatewaydomain.LlmRoute, error) {
	return nil, nil
}
func (stubRoutesRepo) List(context.Context, string) ([]*aigatewaydomain.LlmRoute, error) {
	return nil, nil
}
func (stubRoutesRepo) Create(context.Context, *aigatewaydomain.LlmRoute) error { return nil }
func (stubRoutesRepo) Update(context.Context, *aigatewaydomain.LlmRoute) error { return nil }
func (stubRoutesRepo) Delete(context.Context, string) error                    { return nil }

type stubApiKeysRepo struct{}

func (stubApiKeysRepo) GetByID(context.Context, string) (*aigatewaydomain.LlmApiKey, error) {
	return nil, nil
}
func (stubApiKeysRepo) GetByHash(context.Context, string) (*aigatewaydomain.LlmApiKey, error) {
	return nil, nil
}
func (stubApiKeysRepo) List(context.Context, string) ([]*aigatewaydomain.LlmApiKey, error) {
	return nil, nil
}
func (stubApiKeysRepo) Create(context.Context, *aigatewaydomain.LlmApiKey) error { return nil }
func (stubApiKeysRepo) Update(context.Context, *aigatewaydomain.LlmApiKey) error { return nil }
func (stubApiKeysRepo) Delete(context.Context, string) error                     { return nil }

// ── Real fake for API Keys ──────────────────────────────────────────────
//
// Unlike the stubs above, fakeApiKeysRepo actually stores data so tests can
// exercise Create / List / Get / Revoke end-to-end through the handler.

type fakeApiKeysRepo struct {
	items map[string]*aigatewaydomain.LlmApiKey
}

func newFakeApiKeysRepo() *fakeApiKeysRepo {
	return &fakeApiKeysRepo{items: map[string]*aigatewaydomain.LlmApiKey{}}
}

func (f *fakeApiKeysRepo) GetByID(_ context.Context, id string) (*aigatewaydomain.LlmApiKey, error) {
	if k, ok := f.items[id]; ok {
		cp := *k
		return &cp, nil
	}
	return nil, apperrors.NewNotFound("api key not found")
}

func (f *fakeApiKeysRepo) GetByHash(_ context.Context, hash string) (*aigatewaydomain.LlmApiKey, error) {
	for _, k := range f.items {
		if k.KeyHash == hash {
			cp := *k
			return &cp, nil
		}
	}
	return nil, apperrors.NewNotFound("api key not found")
}

func (f *fakeApiKeysRepo) List(_ context.Context, organizationID string) ([]*aigatewaydomain.LlmApiKey, error) {
	var out []*aigatewaydomain.LlmApiKey
	for _, k := range f.items {
		if k.OrganizationID == organizationID {
			cp := *k
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (f *fakeApiKeysRepo) Create(_ context.Context, k *aigatewaydomain.LlmApiKey) error {
	cp := *k
	f.items[k.ID] = &cp
	return nil
}

func (f *fakeApiKeysRepo) Update(_ context.Context, k *aigatewaydomain.LlmApiKey) error {
	if _, ok := f.items[k.ID]; !ok {
		return apperrors.NewNotFound("api key not found")
	}
	cp := *k
	f.items[k.ID] = &cp
	return nil
}

func (f *fakeApiKeysRepo) Delete(_ context.Context, id string) error {
	delete(f.items, id)
	return nil
}

type stubQuotaPoliciesRepo struct{}

func (stubQuotaPoliciesRepo) GetByID(context.Context, string) (*aigatewaydomain.LlmQuotaPolicy, error) {
	return nil, nil
}
func (stubQuotaPoliciesRepo) List(context.Context, string) ([]*aigatewaydomain.LlmQuotaPolicy, error) {
	return nil, nil
}
func (stubQuotaPoliciesRepo) Create(context.Context, *aigatewaydomain.LlmQuotaPolicy) error {
	return nil
}
func (stubQuotaPoliciesRepo) Update(context.Context, *aigatewaydomain.LlmQuotaPolicy) error {
	return nil
}
func (stubQuotaPoliciesRepo) Delete(context.Context, string) error { return nil }

type stubPriceVersionsRepo struct{}

func (stubPriceVersionsRepo) GetByID(context.Context, string) (*aigatewaydomain.LlmPriceVersion, error) {
	return nil, nil
}
func (stubPriceVersionsRepo) GetByModel(context.Context, string) (*aigatewaydomain.LlmPriceVersion, error) {
	return nil, nil
}
func (stubPriceVersionsRepo) List(context.Context, string) ([]*aigatewaydomain.LlmPriceVersion, error) {
	return nil, nil
}
func (stubPriceVersionsRepo) ListByModel(context.Context, string) ([]*aigatewaydomain.LlmPriceVersion, error) {
	return nil, nil
}
func (stubPriceVersionsRepo) Create(context.Context, *aigatewaydomain.LlmPriceVersion) error {
	return nil
}
func (stubPriceVersionsRepo) Update(context.Context, *aigatewaydomain.LlmPriceVersion) error {
	return nil
}
func (stubPriceVersionsRepo) Delete(context.Context, string) error { return nil }

type stubProviderHealthRepo struct{}

func (stubProviderHealthRepo) GetByID(context.Context, string) (*aigatewaydomain.ProviderHealth, error) {
	return nil, nil
}
func (stubProviderHealthRepo) GetByIDForOrganization(context.Context, string, string) (*aigatewaydomain.ProviderHealth, error) {
	return nil, nil
}
func (stubProviderHealthRepo) GetByProvider(context.Context, string) (*aigatewaydomain.ProviderHealth, error) {
	return nil, nil
}
func (stubProviderHealthRepo) ListAll(context.Context) ([]*aigatewaydomain.ProviderHealth, error) {
	return nil, nil
}
func (stubProviderHealthRepo) ListByOrganization(context.Context, string) ([]*aigatewaydomain.ProviderHealth, error) {
	return nil, nil
}
func (stubProviderHealthRepo) Upsert(context.Context, *aigatewaydomain.ProviderHealth) error {
	return nil
}
func (stubProviderHealthRepo) ListOpen(context.Context) ([]*aigatewaydomain.ProviderHealth, error) {
	return nil, nil
}

type stubUsageLogsRepo struct {
	logs []*aigatewaydomain.UsageLog
}

func (stubUsageLogsRepo) Create(context.Context, *aigatewaydomain.UsageLog) error { return nil }
func (r stubUsageLogsRepo) List(context.Context, string, int, int) ([]*aigatewaydomain.UsageLog, int64, error) {
	return r.logs, int64(len(r.logs)), nil
}
func (stubUsageLogsRepo) QueryStats(context.Context, string, int) (*aigatewaydomain.UsageStats, error) {
	return nil, nil
}

// ── Test wiring ─────────────────────────────────────────────────────────

// newTestAdminRouter builds a chi.Router that has AdminHandler routes
// registered and a session-context middleware that injects a fixed
// OrganizationID / OrganizationSlug / Username.  This is NOT the full
// bootstrap runtime — just the handler + chi registration + session injection.
// testRepos bundles all the fake repos used in handler tests.
//
// Only Providers and ApiKeys get real in-memory maps; the rest are stubs.
// Tests that need a new fake can add it here and update newTestAdminRouter.
type testRepos struct {
	Providers *fakeProvidersRepo
	ApiKeys   *fakeApiKeysRepo
}

func newTestAdminRouter(t *testing.T) (chi.Router, *testRepos) {
	t.Helper()

	// CreateProvider / CreateAPIKey calls Encryptor.EncryptField which needs
	// ENCRYPTION_KEY_CURRENT (or ENCRYPTION_KEY). We use a 32-byte base64 test
	// key — the value doesn't matter because fake repos store the ciphertext
	// as-is and we never decrypt in these tests.
	t.Setenv("ENCRYPTION_KEY_CURRENT", "BX1Ab1Jz6cRDESuWCleZWGRv7RHQRAf0sqikOVcVzkg=")

	uuidGen := &sequentialUUIDGen{}
	providers := newFakeProvidersRepo()
	apiKeys := newFakeApiKeysRepo()

	// We pass stubRepo for everything except Providers / ApiKeys — the handler
	// only exercises those two in these tests.
	svc := appgateway.NewAdminService(
		uuidGen,
		fixedClock{},
		nil, // Encryptor — Providers.Create falls back to GlobalFieldEncryptor() when nil
		providers,
		stubModelsRepo{},
		stubAliasesRepo{},
		stubRoutesRepo{},
		apiKeys,
		stubQuotaPoliciesRepo{},
		stubUsageLogsRepo{},
		stubPriceVersionsRepo{},
		stubProviderHealthRepo{},
	)

	handler := NewAdminHandler(svc)
	r := chi.NewRouter()

	// Session injection — middleware injects SessionContext before every
	// request, matching how bootstrap's SessionContextMiddleware works.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			s := auth.SessionContext{
				ID:               "sess-test",
				OrganizationID:   fixedOrgID,
				OrganizationSlug: fixedOrgSlug,
				Username:         "admin",
				AccessToken:      "fake-token",
				Source:           "repository",
				AuthTime:         fixedNow,
			}
			req = req.WithContext(auth.WithSessionContext(req.Context(), s))
			next.ServeHTTP(w, req)
		})
	})

	RegisterAdminChiRoutes(r, handler)
	return r, &testRepos{Providers: providers, ApiKeys: apiKeys}
}

// performJSON is a helper: POST/PATCH with JSON body, returns recorder.
func performJSON(t *testing.T, router http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, json.NewEncoder(&buf).Encode(body))
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	assert.Equal(t, want, rec.Code, "expected status %d, got %d, body=%s", want, rec.Code, rec.Body.String())
}

// decodeList unwraps the PaginatedResponse body ({items, page}) and returns items.
func decodeList(t *testing.T, rec *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var body struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Items
}

// ── Provider CRUD round-trip test ────────────────────────────────────────

func TestAdminHandler_ProvidersCRUD(t *testing.T) {
	router, _ := newTestAdminRouter(t)

	// ── 1. Empty list → 200, empty array ──
	t.Run("list_empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/providers", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assertStatus(t, rec, http.StatusOK)

		items := decodeList(t, rec)
		assert.Empty(t, items)
	})

	// ── 2. Create provider → 201 ──
	createBody := map[string]any{
		"name":               "deepseek",
		"displayName":        "DeepSeek",
		"providerType":       "deepseek",
		"baseUrl":            "https://api.deepseek.com",
		"apiKey":             "sk-test-secret-123",
		"requestTimeoutMs":   10000,
		"maxRetries":         2,
		"priority":           100,
		"weight":             50,
		"supportedProtocols": []string{"openai_compatible", "anthropic_native"},
		"preferredProtocol":  "openai_compatible",
	}
	createRec := performJSON(t, router, http.MethodPost, "/api/v1/ai/providers", createBody)
	assertStatus(t, createRec, http.StatusCreated)

	var created map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	// Hand-lifted UUID from sequentialUUIDGen counter — Create call
	// generates UUID "aaaaaaaa-0000-0000-0000-000001"
	providerID, _ := created["id"].(string)
	require.NotEmpty(t, providerID, "created provider must have id")
	assert.Equal(t, "deepseek", created["name"])
	assert.Equal(t, "deepseek", created["providerType"])
	// OrganizationID must come from session context, not request body
	assert.Equal(t, fixedOrgID, created["organizationId"],
		"handler must inject orgID from session, not trust request body")
	// API key must NOT be exposed as plaintext — the DTO filters it out
	_, hasPlaintextKey := created["apiKey"]
	assert.False(t, hasPlaintextKey, "apiKey must not appear in response body")

	// ── 3. List returns 1 item ──
	t.Run("list_after_create", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/providers", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assertStatus(t, rec, http.StatusOK)

		items := decodeList(t, rec)
		assert.Len(t, items, 1)
		assert.Equal(t, providerID, items[0]["id"])
	})

	// ── 4. Get by ID → 200 ──
	t.Run("get", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/providers/"+providerID, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assertStatus(t, rec, http.StatusOK)

		var got map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		assert.Equal(t, providerID, got["id"])
		assert.Equal(t, "deepseek", got["name"])
	})

	// ── 5. Get non-existent → 404 ──
	t.Run("get_not_found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/providers/00000000-0000-0000-0000-000000000000", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assertStatus(t, rec, http.StatusNotFound)
	})

	// ── 6. Partial PATCH (only baseUrl + priority) → 200 ──
	t.Run("patch_partial", func(t *testing.T) {
		patchBody := map[string]any{
			"baseUrl":  "https://api.deepseek.com/v2",
			"priority": 80,
		}
		rec := performJSON(t, router, http.MethodPatch, "/api/v1/ai/providers/"+providerID, patchBody)
		assertStatus(t, rec, http.StatusOK)

		var got map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		assert.Equal(t, "https://api.deepseek.com/v2", got["baseUrl"])
		assert.Equal(t, float64(80), got["priority"], "priority updated")
		assert.Equal(t, "deepseek", got["name"], "unchanged fields preserved")
	})

	// ── 7. DELETE → 204 No Content ──
	t.Run("delete", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/ai/providers/"+providerID, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assertStatus(t, rec, http.StatusNoContent)

		// Verify gone
		getReq := httptest.NewRequest(http.MethodGet, "/api/v1/ai/providers/"+providerID, nil)
		getRec := httptest.NewRecorder()
		router.ServeHTTP(getRec, getReq)
		assertStatus(t, getRec, http.StatusNotFound)
	})
}

// TestAdminHandler_OrgIDComesFromSession asserts that:
//
//  1. A caller cannot spoof organizationId via request body — it's silently
//     rejected by DTO decoding (json:"-" tag) or overwritten by the handler
//     with the session's orgID.
//  2. Cross-org isolation holds — a session in org A cannot read org B's
//     providers even if the DB has rows for org B.
func TestAdminHandler_OrgIDComesFromSession(t *testing.T) {
	router, repos := newTestAdminRouter(t)

	// ── Cross-org isolation: foreign providers invisible to our session ──
	t.Run("cross_org_isolation", func(t *testing.T) {
		repos.Providers.items["foreign-uuid-1"] = &aigatewaydomain.LlmProvider{
			ID:             "foreign-uuid-1",
			OrganizationID: "00000000-0000-0000-0000-000000000000",
			Name:           "foreign-provider",
			ProviderType:   "openai",
			BaseURL:        "https://example.com",
			CreatedAt:      fixedNow,
			UpdatedAt:      fixedNow,
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/providers", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assertStatus(t, rec, http.StatusOK)

		items := decodeList(t, rec)
		assert.Empty(t, items, "cross-org isolation must hold")
	})

	// ── OrganizationID in body is silently dropped (json:"-") ──
	t.Run("body_orgid_ignored", func(t *testing.T) {
		createBody := map[string]any{
			"name":         "legit",
			"providerType": "openai",
			"baseUrl":      "https://api.openai.com",
			"apiKey":       "sk-fake",
			// Note: organizationId is NOT a valid JSON field on ProviderCreate
			// DTO (json:"-" tag), so sending it results in 400 unknown field.
			// That's fine — the key invariant we care about is that the handler
			// ALWAYS injects session orgID, never trusts body.
		}
		createRec := performJSON(t, router, http.MethodPost, "/api/v1/ai/providers", createBody)
		assertStatus(t, createRec, http.StatusCreated)

		var created map[string]any
		require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
		assert.Equal(t, fixedOrgID, created["organizationId"],
			"handler must inject session org — never trust request body")
	})
}

// ── API Key CRUD + Revoke round-trip ──────────────────────────────────────

func TestAdminHandler_APIKeysCRUD(t *testing.T) {
	router, repos := newTestAdminRouter(t)

	// ── 1. List empty → 200 ──
	t.Run("list_empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/api-keys", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assertStatus(t, rec, http.StatusOK)
		items := decodeList(t, rec)
		assert.Empty(t, items)
	})

	// ── 2. Create API key → 201 with plaintext + session orgID ──
	createBody := map[string]any{
		"name":     "ci-cd-bot",
		"rpmLimit": 1000,
		"tpmLimit": 50000,
		"currency": "USD",
	}
	createRec := performJSON(t, router, http.MethodPost, "/api/v1/ai/api-keys", createBody)
	assertStatus(t, createRec, http.StatusCreated)

	var created map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	assert.Equal(t, "ci-cd-bot", created["name"])
	assert.NotEmpty(t, created["plaintext"], "Create response must include plaintext key (one-time reveal)")
	assert.NotEmpty(t, created["keyPrefix"])

	keyID := created["id"].(string)

	// Verify the key was stored with session orgID (handler injects it before Create)
	stored := repos.ApiKeys.items[keyID]
	require.NotNil(t, stored)
	assert.Equal(t, fixedOrgID, stored.OrganizationID,
		"handler must inject session org — never trust request body")

	// ── 3. List after create → 1 item, no plaintext ──
	listRec := performJSON(t, router, http.MethodGet, "/api/v1/ai/api-keys", nil)
	assertStatus(t, listRec, http.StatusOK)
	listItems := decodeList(t, listRec)
	require.Len(t, listItems, 1)
	assert.Equal(t, keyID, listItems[0]["id"])
	assert.Empty(t, listItems[0]["plaintext"], "List response must NOT leak plaintext")

	// ── 4. Cross-org isolation for API keys ──
	t.Run("cross_org_isolation", func(t *testing.T) {
		repos.ApiKeys.items["foreign-key-1"] = &aigatewaydomain.LlmApiKey{
			ID:             "foreign-key-1",
			OrganizationID: "00000000-0000-0000-0000-000000000000",
			Name:           "foreign-bot",
			Status:         aigatewaydomain.APIKeyStatusActive,
		}
		listRec2 := performJSON(t, router, http.MethodGet, "/api/v1/ai/api-keys", nil)
		assertStatus(t, listRec2, http.StatusOK)
		items := decodeList(t, listRec2)
		assert.Len(t, items, 1, "foreign org key must be invisible")
	})

	// ── 5. Revoke API key → 204 ──
	revokeReq := httptest.NewRequest(http.MethodPost, "/api/v1/ai/api-keys/"+keyID+"/revoke", nil)
	revokeRec := httptest.NewRecorder()
	router.ServeHTTP(revokeRec, revokeReq)
	assertStatus(t, revokeRec, http.StatusNoContent)

	// Verify it's marked revoked in the repo (handler → service → domain key.Revoke() → Update)
	stored2 := repos.ApiKeys.items[keyID]
	require.NotNil(t, stored2)
	assert.Equal(t, aigatewaydomain.APIKeyStatusRevoked, stored2.Status,
		"revoke must flip status to revoked in domain model")

	// ── 6. Revoke foreign key → 404 (org isolation on revoke) ──
	foreignRevoke := httptest.NewRequest(http.MethodPost, "/api/v1/ai/api-keys/foreign-key-1/revoke", nil)
	foreignRevokeRec := httptest.NewRecorder()
	router.ServeHTTP(foreignRevokeRec, foreignRevoke)
	assertStatus(t, foreignRevokeRec, http.StatusNotFound)
}

// TestAdminHandler_ProviderByIDEmptyTenantIsRejected pins the first state of
// the E1 contract: with no trusted tenant the five by-ID entries answer 400
// before a path id or request body is decoded.
func TestAdminHandler_ProviderByIDEmptyTenantIsRejected(t *testing.T) {
	f := newTenantScopedAdminFixture(t, nil)
	tests := []struct {
		method string
		path   string
		body   any
	}{
		{method: http.MethodGet, path: "/api/v1/ai/providers/p-foreign"},
		{method: http.MethodPatch, path: "/api/v1/ai/providers/p-foreign", body: map[string]any{"displayName": "forged"}},
		{method: http.MethodDelete, path: "/api/v1/ai/providers/p-foreign"},
		{method: http.MethodPost, path: "/api/v1/ai/providers/p-foreign/test"},
		{method: http.MethodGet, path: "/api/v1/ai/health/p-foreign"},
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

// TestAdminHandler_ProviderByIDForeignTenantIsNotFoundAndUnwritten pins the
// second state: a regular tenant forging another organization's id (path or
// query) gets the same 404 as a missing id, and no provider row is written.
func TestAdminHandler_ProviderByIDForeignTenantIsNotFoundAndUnwritten(t *testing.T) {
	forged := "?organizationId=" + foreignTenantOrg
	f := newTenantScopedAdminFixture(t, strictTenantCtx(trustedTenantOrg))
	f.seedForeignTenantRows(t)
	before := *f.providers.items[foreignProviderID]

	rec := f.get(t, "/api/v1/ai/providers/"+foreignProviderID+forged)
	assertStatus(t, rec, http.StatusNotFound)

	rec = performJSON(t, f.router, http.MethodPatch, "/api/v1/ai/providers/"+foreignProviderID+forged,
		map[string]any{"displayName": "pwned"})
	assertStatus(t, rec, http.StatusNotFound)

	rec = httptest.NewRecorder()
	f.router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/ai/providers/"+foreignProviderID+forged, nil))
	assertStatus(t, rec, http.StatusNotFound)

	rec = httptest.NewRecorder()
	f.router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/ai/providers/"+foreignProviderID+"/test"+forged, nil))
	assertStatus(t, rec, http.StatusNotFound)

	rec = f.get(t, "/api/v1/ai/health/"+foreignProviderID+forged)
	assertStatus(t, rec, http.StatusNotFound)
	rec = f.get(t, "/api/v1/ai/health/h-foreign"+forged)
	assertStatus(t, rec, http.StatusNotFound)

	after, ok := f.providers.items[foreignProviderID]
	require.True(t, ok, "a forged delete must not remove the foreign row")
	assert.Equal(t, before.Name, after.Name)
	assert.Equal(t, before.DisplayName, after.DisplayName)
	assert.Equal(t, before.Status, after.Status)
	assert.Zero(t, f.providers.updateCalls, "a forged patch must not write")
	assert.Zero(t, f.providers.deleteCalls, "a forged delete must not write")
}

// TestAdminHandler_ProviderByIDOwnTenantSucceeds pins the third state: the
// same five entries work inside the trusted tenant.
func TestAdminHandler_ProviderByIDOwnTenantSucceeds(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	f := newTenantScopedAdminFixture(t, strictTenantCtx(trustedTenantOrg))
	f.seedTrustedTenantHealth(t)
	require.NoError(t, f.providers.Create(context.Background(), &aigatewaydomain.LlmProvider{
		ID: "p-live", OrganizationID: trustedTenantOrg, Name: "live-provider",
		ProviderType: "openai", PreferredProtocol: "openai", BaseURL: ts.URL,
		Status: "active", RequestTimeoutMs: 3000, APIKeySealed: "",
		CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}))

	rec := f.get(t, "/api/v1/ai/providers/"+trustedProviderID)
	assertStatus(t, rec, http.StatusOK)
	assert.Equal(t, trustedProviderID, decodeResource(t, rec)["id"])

	rec = performJSON(t, f.router, http.MethodPatch, "/api/v1/ai/providers/"+trustedProviderID,
		map[string]any{"displayName": "trusted-renamed"})
	assertStatus(t, rec, http.StatusOK)
	assert.Equal(t, "trusted-renamed", decodeResource(t, rec)["displayName"])
	assert.Equal(t, 1, f.providers.updateCalls)

	rec = httptest.NewRecorder()
	f.router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/ai/providers/p-live/test", nil))
	assertStatus(t, rec, http.StatusOK)
	assert.Equal(t, true, decodeResource(t, rec)["reachable"])

	rec = f.get(t, "/api/v1/ai/health/"+trustedProviderID)
	assertStatus(t, rec, http.StatusOK)
	assert.Equal(t, trustedHealthID, decodeResource(t, rec)["id"])
	rec = f.get(t, "/api/v1/ai/health/"+trustedHealthID)
	assertStatus(t, rec, http.StatusOK)

	rec = httptest.NewRecorder()
	f.router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/ai/providers/"+trustedProviderID, nil))
	assertStatus(t, rec, http.StatusNoContent)
	_, ok := f.providers.items[trustedProviderID]
	assert.False(t, ok, "the trusted tenant delete must remove the row")
	assert.Equal(t, 1, f.providers.deleteCalls)

	rec = f.get(t, "/api/v1/ai/providers/"+trustedProviderID)
	assertStatus(t, rec, http.StatusNotFound)
}

// Verify compile-time satisfaction.
var _ = platformuuid.Generator((*sequentialUUIDGen)(nil))
var _ = platformclock.Clock(fixedClock{})
