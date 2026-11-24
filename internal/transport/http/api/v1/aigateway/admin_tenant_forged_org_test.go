package aigateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appgateway "github.com/guliangiam/guliangiam/internal/app/aigateway"
	aigatewaydomain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	"github.com/guliangiam/guliangiam/internal/infra/aiupstream"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	trustedTenantOrg  = "11111111-1111-1111-1111-111111111111"
	foreignTenantOrg  = "22222222-2222-2222-2222-222222222222"
	foreignProviderID = "p-foreign"
)

// tenantScopedAdminFixture wires the real handler onto chi with one injectable
// context middleware, plus in-memory repos so tenant predicates are observable
// in the response (foreign rows are filtered out by the fake repos exactly as
// the SQL repositories filter by organization_id).
type tenantScopedAdminFixture struct {
	router    chi.Router
	providers *fakeProvidersRepo
	models    *fakeModelsRepo
	aliases   *fakeAliasesRepo
	routes    *fakeRoutesRepo
	apiKeys   *fakeApiKeysRepo
	quotas    *fakeQuotasRepo
	prices    *fakePricesRepo
	health    *fakeHealthRepo
}

func newTenantScopedAdminFixture(t *testing.T, scopeCtx func(context.Context) context.Context) *tenantScopedAdminFixture {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY_CURRENT", "BX1Ab1Jz6cRDESuWCleZWGRv7RHQRAf0sqikOVcVzkg=")

	f := &tenantScopedAdminFixture{
		providers: newFakeProvidersRepo(),
		models:    newFakeModelsRepo(),
		aliases:   newFakeAliasesRepo(),
		routes:    newFakeRoutesRepo(),
		apiKeys:   newFakeApiKeysRepo(),
		quotas:    newFakeQuotasRepo(),
		prices:    newFakePricesRepo(),
		health:    newFakeHealthRepo(),
	}
	svc := appgateway.NewAdminService(
		&sequentialUUIDGen{}, fixedClock{}, nil,
		f.providers, f.models, f.aliases, f.routes, f.apiKeys, f.quotas,
		stubUsageLogsRepo{}, f.prices, f.health,
	)
	svc.Upstreams = aiupstream.NewAdapterFactory()
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

// strictTenantCtx mirrors TenantFilterMiddleware for a regular tenant user:
// the authenticated scope and the session agree on the tenant.
func strictTenantCtx(sessionOrg string) func(context.Context) context.Context {
	return func(ctx context.Context) context.Context {
		ctx = tenantctx.WithTenantScope(ctx, sessionOrg, "acme", tenantctx.TenantFilterStrict)
		return auth.WithSessionContext(ctx, auth.SessionContext{
			ID: "sess-test", OrganizationID: sessionOrg, OrganizationSlug: "acme",
			Username: "tenant-admin", AccessToken: "fake-token", Source: "repository", AuthTime: fixedNow,
		})
	}
}

// scopeOnlyTenantCtx keeps the tenant scope authoritative but omits the session
// org. It pins that the scope alone is enough to bind a request, so a forged
// query cannot win by being the only org-shaped value on the request.
func scopeOnlyTenantCtx(scopeOrg string) func(context.Context) context.Context {
	return func(ctx context.Context) context.Context {
		ctx = tenantctx.WithTenantScope(ctx, scopeOrg, "acme", tenantctx.TenantFilterStrict)
		return auth.WithSessionContext(ctx, auth.SessionContext{
			ID: "sess-test", Username: "tenant-admin", AccessToken: "fake-token",
			Source: "repository", AuthTime: fixedNow,
		})
	}
}

// platformAdminCtx mirrors the platform-admin escalation: TenantFilter promotes
// the request to super_admin so an explicit organizationId is honoured.
func platformAdminCtx() func(context.Context) context.Context {
	return func(ctx context.Context) context.Context {
		ctx = tenantctx.WithTenantScope(ctx, "", "", tenantctx.TenantFilterSuperAdmin)
		return auth.WithSessionContext(ctx, auth.SessionContext{
			ID: "sess-admin", OrganizationID: trustedTenantOrg, OrganizationSlug: "built-in",
			Username: "platform-admin", AccessToken: "fake-token", Source: "repository", AuthTime: fixedNow,
		})
	}
}

func (f *tenantScopedAdminFixture) seedForeignTenantRows(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, f.providers.Create(ctx, &aigatewaydomain.LlmProvider{
		ID: foreignProviderID, OrganizationID: foreignTenantOrg, Name: "foreign-provider",
		ProviderType: "openai", PreferredProtocol: "openai",
		BaseURL: "https://api.foreign.example", Status: "active",
		RequestTimeoutMs: 5000, APIKeySealed: "sealed", CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}))
	require.NoError(t, f.models.Create(ctx, &aigatewaydomain.LlmModel{
		ID: "m-foreign", OrganizationID: foreignTenantOrg, ProviderID: foreignProviderID,
		ProviderModelName: "gpt-4o", DisplayName: "foreign model", Status: "active",
		CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}))
	require.NoError(t, f.aliases.Create(ctx, &aigatewaydomain.LlmModelAlias{
		ID: "a-foreign", OrganizationID: foreignTenantOrg, Alias: "foreign-alias",
		DisplayName: "foreign alias", Status: "active",
		CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}))
	require.NoError(t, f.routes.Create(ctx, &aigatewaydomain.LlmRoute{
		ID: "rt-foreign", OrganizationID: foreignTenantOrg, AliasID: "a-foreign", ModelID: "m-foreign",
		Priority: 1, Weight: 100, Status: "active", CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}))
	require.NoError(t, f.apiKeys.Create(ctx, &aigatewaydomain.LlmApiKey{
		ID: "k-foreign", OrganizationID: foreignTenantOrg, Name: "foreign-key",
		KeyPrefix: "glk_foreign", Status: "active", CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}))
	require.NoError(t, f.quotas.Create(ctx, &aigatewaydomain.LlmQuotaPolicy{
		ID: "q-foreign", OrganizationID: foreignTenantOrg, Scope: "organization",
		LimitType: "rpm", LimitValue: 60, ResetCycle: "daily", Status: "active",
		CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}))
	require.NoError(t, f.prices.Create(ctx, &aigatewaydomain.LlmPriceVersion{
		ID: "pv-foreign", OrganizationID: foreignTenantOrg, ModelID: "m-foreign", VersionNo: 1,
		InputPricePer1M: 1, OutputPricePer1M: 2, Currency: "USD",
		EffectiveFrom: fixedNow.Add(-24 * time.Hour), CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}))
	require.NoError(t, f.health.Upsert(ctx, &aigatewaydomain.ProviderHealth{
		ID: "h-foreign", ProviderID: foreignProviderID, State: aigatewaydomain.HealthStateHealthy,
		UpdatedAt: fixedNow,
	}))
	f.health.providerOrg[foreignProviderID] = foreignTenantOrg
}

func (f *tenantScopedAdminFixture) get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// TestAdminHandler_ForgedQueryOrganizationIDIgnored pins that the query value
// only requests a tenant; the authenticated scope decides which repository
// predicate is used for providers/models/routes/api-keys/quotas/prices.
func TestAdminHandler_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	guard := strictTenantCtx(trustedTenantOrg)

	t.Run("session tenant wins over forged query", func(t *testing.T) {
		f := newTenantScopedAdminFixture(t, guard)
		f.seedForeignTenantRows(t)

		listPaths := []string{
			"/api/v1/ai/providers?organizationId=" + foreignTenantOrg,
			"/api/v1/ai/models?organizationId=" + foreignTenantOrg,
			"/api/v1/ai/routes?organizationId=" + foreignTenantOrg,
			"/api/v1/ai/api-keys?organizationId=" + foreignTenantOrg,
			"/api/v1/ai/quotas?organizationId=" + foreignTenantOrg,
			"/api/v1/ai/prices?organizationId=" + foreignTenantOrg,
		}
		for _, path := range listPaths {
			rec := f.get(t, path)
			assertStatus(t, rec, http.StatusOK)
			assert.Empty(t, decodeList(t, rec), "foreign tenant rows must stay out of %s", path)
		}
	})

	t.Run("forged query cannot create a model in another tenant", func(t *testing.T) {
		f := newTenantScopedAdminFixture(t, guard)
		require.NoError(t, f.providers.Create(context.Background(), &aigatewaydomain.LlmProvider{
			ID: "p-owned", OrganizationID: trustedTenantOrg, Name: "owned-provider",
			ProviderType: "openai", PreferredProtocol: "openai",
			BaseURL: "https://api.owned.example", Status: "active",
			RequestTimeoutMs: 5000, APIKeySealed: "sealed", CreatedAt: fixedNow, UpdatedAt: fixedNow,
		}))

		rec := performJSON(t, f.router, http.MethodPost,
			"/api/v1/ai/models?organizationId="+foreignTenantOrg,
			map[string]any{
				"providerId": "p-owned", "providerModelName": "gpt-4o-mini",
				"displayName": "mini", "status": "active",
			})
		assertStatus(t, rec, http.StatusCreated)

		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, trustedTenantOrg, body["organizationId"])
		for _, m := range f.models.items {
			assert.Equal(t, trustedTenantOrg, m.OrganizationID,
				"forged query must never reach the persisted model")
		}
	})

	t.Run("scope alone binds the tenant without a session org", func(t *testing.T) {
		f := newTenantScopedAdminFixture(t, scopeOnlyTenantCtx(trustedTenantOrg))
		f.seedForeignTenantRows(t)

		rec := f.get(t, "/api/v1/ai/providers?organizationId="+foreignTenantOrg)
		assertStatus(t, rec, http.StatusOK)
		assert.Empty(t, decodeList(t, rec), "scope tenant must win over the forged query")
	})
}

// TestAdminHandler_SuperAdminRequestedOrganizationHonoured pins the single
// legitimate override: a platform administrator acting on an explicit tenant.
func TestAdminHandler_SuperAdminRequestedOrganizationHonoured(t *testing.T) {
	f := newTenantScopedAdminFixture(t, platformAdminCtx())
	f.seedForeignTenantRows(t)

	rec := f.get(t, "/api/v1/ai/providers?organizationId="+foreignTenantOrg)
	assertStatus(t, rec, http.StatusOK)
	items := decodeList(t, rec)
	require.Len(t, items, 1)
	assert.Equal(t, foreignProviderID, items[0]["id"])
	assert.Equal(t, foreignTenantOrg, items[0]["organizationId"])

	priceRec := f.get(t, "/api/v1/ai/prices?organizationId="+foreignTenantOrg)
	assertStatus(t, priceRec, http.StatusOK)
	require.Len(t, decodeList(t, priceRec), 1)
}

// TestAdminHandler_WithoutTrustedTenantRejected pins the fail-fast: with neither
// a tenant scope nor a session org, no handler may fall back to the forged query.
func TestAdminHandler_WithoutTrustedTenantRejected(t *testing.T) {
	f := newTenantScopedAdminFixture(t, nil)
	f.seedForeignTenantRows(t)

	tests := []struct {
		name string
		path string
	}{
		{name: "providers", path: "/api/v1/ai/providers?organizationId=" + foreignTenantOrg},
		{name: "models", path: "/api/v1/ai/models?organizationId=" + foreignTenantOrg},
		{name: "routes", path: "/api/v1/ai/routes?organizationId=" + foreignTenantOrg},
		{name: "api_keys", path: "/api/v1/ai/api-keys?organizationId=" + foreignTenantOrg},
		{name: "quotas", path: "/api/v1/ai/quotas?organizationId=" + foreignTenantOrg},
		{name: "prices", path: "/api/v1/ai/prices?organizationId=" + foreignTenantOrg},
		{name: "usage", path: "/api/v1/ai/usage?organizationId=" + foreignTenantOrg},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			f.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
			assertStatus(t, rec, http.StatusBadRequest)
		})
	}
}

// TestAdminHandler_HealthListScopedToTrustedTenant pins the read-only health
// surface after IDOR B2: llm_provider_health has no tenant column, so ownership
// is resolved through its provider. A forged organizationId cannot widen the
// list beyond the authenticated tenant, and a tenant probe for another
// organization's rows is reported as an empty list.
func TestAdminHandler_HealthListScopedToTrustedTenant(t *testing.T) {
	f := newTenantScopedAdminFixture(t, strictTenantCtx(trustedTenantOrg))
	f.seedForeignTenantRows(t)
	f.seedTrustedTenantHealth(t)

	plain := f.get(t, "/api/v1/ai/health")
	forged := f.get(t, "/api/v1/ai/health?organizationId="+foreignTenantOrg)
	assertStatus(t, plain, http.StatusOK)
	assertStatus(t, forged, http.StatusOK)

	for _, rec := range []*httptest.ResponseRecorder{plain, forged} {
		items := decodeList(t, rec)
		require.Len(t, items, 1, "only the trusted tenant's provider health may be listed")
		assert.Equal(t, "h-trusted", items[0]["id"])
		assert.Equal(t, trustedProviderID, items[0]["providerId"])
	}
}
