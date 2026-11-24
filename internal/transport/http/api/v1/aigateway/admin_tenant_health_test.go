package aigateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	aigatewaydomain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	trustedProviderID = "p-trusted"
	trustedHealthID   = "h-trusted"
)

// seedTrustedTenantHealth adds a provider owned by trustedTenantOrg plus its
// circuit-breaker row. The fake health repo resolves ownership through the same
// provider→organization predicate the SQL repositories use.
func (f *tenantScopedAdminFixture) seedTrustedTenantHealth(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, f.providers.Create(ctx, &aigatewaydomain.LlmProvider{
		ID: trustedProviderID, OrganizationID: trustedTenantOrg, Name: "trusted-provider",
		ProviderType: "openai", PreferredProtocol: "openai",
		BaseURL: "https://api.trusted.example", Status: "active",
		RequestTimeoutMs: 5000, APIKeySealed: "sealed", CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}))
	f.health.providerOrg[trustedProviderID] = trustedTenantOrg
	require.NoError(t, f.health.Upsert(ctx, &aigatewaydomain.ProviderHealth{
		ID: trustedHealthID, ProviderID: trustedProviderID,
		State: aigatewaydomain.HealthStateDegraded, ConsecutiveFailures: 2, UpdatedAt: fixedNow,
	}))
}

func decodeResource(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body
}

// TestAdminHandler_GetHealth_OwnTenantSucceeds pins both accepted addresses:
// the console addresses the provider (GET /ai/health/{providerId}) while the
// list payload exposes the health record id, which stays readable inside the
// tenant.
func TestAdminHandler_GetHealth_OwnTenantSucceeds(t *testing.T) {
	f := newTenantScopedAdminFixture(t, strictTenantCtx(trustedTenantOrg))
	f.seedForeignTenantRows(t)
	f.seedTrustedTenantHealth(t)

	byProvider := f.get(t, "/api/v1/ai/health/"+trustedProviderID)
	assertStatus(t, byProvider, http.StatusOK)
	body := decodeResource(t, byProvider)
	assert.Equal(t, trustedHealthID, body["id"])
	assert.Equal(t, trustedProviderID, body["providerId"])
	assert.Equal(t, aigatewaydomain.HealthStateDegraded, body["state"])
	assert.Equal(t, float64(2), body["consecutiveFailures"])

	byRecord := f.get(t, "/api/v1/ai/health/"+trustedHealthID)
	assertStatus(t, byRecord, http.StatusOK)
	record := decodeResource(t, byRecord)
	assert.Equal(t, trustedHealthID, record["id"])
	assert.Equal(t, trustedProviderID, record["providerId"])
}

// TestAdminHandler_GetHealth_ForeignTenantIsNotFound pins that a forged query
// cannot widen the tenant: both a foreign provider id and a foreign health
// record id answer 404, the same shape as a missing id.
func TestAdminHandler_GetHealth_ForeignTenantIsNotFound(t *testing.T) {
	f := newTenantScopedAdminFixture(t, strictTenantCtx(trustedTenantOrg))
	f.seedForeignTenantRows(t)

	tests := []struct {
		name string
		path string
	}{
		{name: "foreign provider id", path: "/api/v1/ai/health/" + foreignProviderID},
		{name: "foreign provider id with forged org", path: "/api/v1/ai/health/" + foreignProviderID + "?organizationId=" + foreignTenantOrg},
		{name: "foreign health record id", path: "/api/v1/ai/health/h-foreign"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := f.get(t, tt.path)
			assertStatus(t, rec, http.StatusNotFound)
		})
	}
}

// TestAdminHandler_GetHealth_WithoutTrustedTenantRejected pins the fail-fast:
// with neither a scope nor a session org the handler answers 400 before reading
// the id, so a forged organizationId can never be the only org-shaped value.
func TestAdminHandler_GetHealth_WithoutTrustedTenantRejected(t *testing.T) {
	f := newTenantScopedAdminFixture(t, nil)
	f.seedTrustedTenantHealth(t)

	for _, path := range []string{
		"/api/v1/ai/health/" + trustedProviderID + "?organizationId=" + foreignTenantOrg,
		"/api/v1/ai/health?organizationId=" + foreignTenantOrg,
	} {
		rec := f.get(t, path)
		assertStatus(t, rec, http.StatusBadRequest)
	}
}

// TestAdminHandler_GetHealth_PlatformAdminTargetHonoured pins the single
// legitimate cross-tenant read: a platform administrator acting on an explicit
// organization sees that organization's provider health.
func TestAdminHandler_GetHealth_PlatformAdminTargetHonoured(t *testing.T) {
	f := newTenantScopedAdminFixture(t, platformAdminCtx())
	f.seedForeignTenantRows(t)

	byProvider := f.get(t, "/api/v1/ai/health/"+foreignProviderID+"?organizationId="+foreignTenantOrg)
	assertStatus(t, byProvider, http.StatusOK)
	body := decodeResource(t, byProvider)
	assert.Equal(t, "h-foreign", body["id"])
	assert.Equal(t, foreignProviderID, body["providerId"])

	byRecord := f.get(t, "/api/v1/ai/health/h-foreign?organizationId="+foreignTenantOrg)
	assertStatus(t, byRecord, http.StatusOK)

	list := f.get(t, "/api/v1/ai/health?organizationId="+foreignTenantOrg)
	assertStatus(t, list, http.StatusOK)
	items := decodeList(t, list)
	require.Len(t, items, 1)
	assert.Equal(t, "h-foreign", items[0]["id"])
}

// TestAdminHandler_HealthPayloadHasNoProviderCredentials pins that only
// circuit-breaker state crosses the tenant boundary; provider secrets stay on
// the provider aggregate.
func TestAdminHandler_HealthPayloadHasNoProviderCredentials(t *testing.T) {
	sensitive := []string{"apiKey", "apiKeySealed", "apiKeyFingerprint", "baseUrl"}

	f := newTenantScopedAdminFixture(t, strictTenantCtx(trustedTenantOrg))
	f.seedTrustedTenantHealth(t)

	detail := decodeResource(t, f.get(t, "/api/v1/ai/health/"+trustedProviderID))
	for _, key := range sensitive {
		_, found := detail[key]
		assert.False(t, found, "provider health payload must not expose %s", key)
	}

	list := f.get(t, "/api/v1/ai/health")
	assertStatus(t, list, http.StatusOK)
	items := decodeList(t, list)
	require.Len(t, items, 1)
	for _, key := range sensitive {
		_, found := items[0][key]
		assert.False(t, found, "provider health list must not expose %s", key)
	}
}
