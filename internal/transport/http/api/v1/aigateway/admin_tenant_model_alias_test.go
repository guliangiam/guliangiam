package aigateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	aigatewaydomain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	trustedModelID = "m-trusted"
	trustedAliasID = "a-trusted"
	foreignModelID = "m-foreign"
	foreignAliasID = "a-foreign"
)

// TestAdminHandler_ModelAliasByIDEmptyTenantIsRejected pins the first state of
// the E2a contract: with no trusted tenant all six by-ID entries answer 400
// before a path id or request body is decoded.
func TestAdminHandler_ModelAliasByIDEmptyTenantIsRejected(t *testing.T) {
	f := newTenantScopedAdminFixture(t, nil)
	f.seedForeignTenantRows(t)

	tests := []struct {
		method string
		path   string
		body   any
	}{
		{method: http.MethodGet, path: "/api/v1/ai/models/" + foreignModelID},
		{method: http.MethodPatch, path: "/api/v1/ai/models/" + foreignModelID, body: map[string]any{"displayName": "forged"}},
		{method: http.MethodDelete, path: "/api/v1/ai/models/" + foreignModelID},
		{method: http.MethodGet, path: "/api/v1/ai/aliases/" + foreignAliasID},
		{method: http.MethodPatch, path: "/api/v1/ai/aliases/" + foreignAliasID, body: map[string]any{"displayName": "forged"}},
		{method: http.MethodDelete, path: "/api/v1/ai/aliases/" + foreignAliasID},
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

// TestAdminHandler_ModelAliasByIDForeignTenantIsNotFoundAndUnwritten pins the
// second state: a regular tenant forging another organization's id (path or
// query) gets the same 404 as a missing id, no row is written, and the foreign
// row keeps its content.
func TestAdminHandler_ModelAliasByIDForeignTenantIsNotFoundAndUnwritten(t *testing.T) {
	forged := "?organizationId=" + foreignTenantOrg
	f := newTenantScopedAdminFixture(t, strictTenantCtx(trustedTenantOrg))
	f.seedForeignTenantRows(t)
	beforeModel := *f.models.items[foreignModelID]
	beforeAlias := *f.aliases.items[foreignAliasID]

	assertStatus(t, f.get(t, "/api/v1/ai/models/"+foreignModelID+forged), http.StatusNotFound)
	rec := performJSON(t, f.router, http.MethodPatch, "/api/v1/ai/models/"+foreignModelID+forged,
		map[string]any{"displayName": "pwned"})
	assertStatus(t, rec, http.StatusNotFound)
	rec = httptest.NewRecorder()
	f.router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/ai/models/"+foreignModelID+forged, nil))
	assertStatus(t, rec, http.StatusNotFound)

	assertStatus(t, f.get(t, "/api/v1/ai/aliases/"+foreignAliasID+forged), http.StatusNotFound)
	rec = performJSON(t, f.router, http.MethodPatch, "/api/v1/ai/aliases/"+foreignAliasID+forged,
		map[string]any{"displayName": "pwned"})
	assertStatus(t, rec, http.StatusNotFound)
	rec = httptest.NewRecorder()
	f.router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/ai/aliases/"+foreignAliasID+forged, nil))
	assertStatus(t, rec, http.StatusNotFound)

	afterModel, ok := f.models.items[foreignModelID]
	require.True(t, ok, "a forged delete must not remove the foreign model")
	assert.Equal(t, beforeModel.DisplayName, afterModel.DisplayName)
	assert.Equal(t, beforeModel.Status, afterModel.Status)
	afterAlias, ok := f.aliases.items[foreignAliasID]
	require.True(t, ok, "a forged delete must not remove the foreign alias")
	assert.Equal(t, beforeAlias.Alias, afterAlias.Alias)
	assert.Equal(t, beforeAlias.DisplayName, afterAlias.DisplayName)
	assert.Zero(t, f.models.updateCalls, "a forged model patch must not write")
	assert.Zero(t, f.models.deleteCalls, "a forged model delete must not write")
	assert.Zero(t, f.aliases.updateCalls, "a forged alias patch must not write")
	assert.Zero(t, f.aliases.deleteCalls, "a forged alias delete must not write")
}

// TestAdminHandler_ModelAliasByIDOwnTenantSucceeds pins the third state: the
// same six entries work inside the trusted tenant.
func TestAdminHandler_ModelAliasByIDOwnTenantSucceeds(t *testing.T) {
	f := newTenantScopedAdminFixture(t, strictTenantCtx(trustedTenantOrg))
	ctx := context.Background()
	require.NoError(t, f.models.Create(ctx, &aigatewaydomain.LlmModel{
		ID: trustedModelID, OrganizationID: trustedTenantOrg, ProviderID: "p-trusted",
		ProviderModelName: "gpt-4o", DisplayName: "trusted model", Status: "active",
		CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}))
	require.NoError(t, f.aliases.Create(ctx, &aigatewaydomain.LlmModelAlias{
		ID: trustedAliasID, OrganizationID: trustedTenantOrg, Alias: "trusted-alias",
		DisplayName: "trusted alias", Status: "active",
		CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}))

	rec := f.get(t, "/api/v1/ai/models/"+trustedModelID)
	assertStatus(t, rec, http.StatusOK)
	assert.Equal(t, trustedModelID, decodeResource(t, rec)["id"])

	rec = performJSON(t, f.router, http.MethodPatch, "/api/v1/ai/models/"+trustedModelID,
		map[string]any{"displayName": "model-renamed"})
	assertStatus(t, rec, http.StatusOK)
	assert.Equal(t, "model-renamed", decodeResource(t, rec)["displayName"])
	assert.Equal(t, 1, f.models.updateCalls)

	rec = httptest.NewRecorder()
	f.router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/ai/models/"+trustedModelID, nil))
	assertStatus(t, rec, http.StatusNoContent)
	assert.Equal(t, 1, f.models.deleteCalls)
	assertStatus(t, f.get(t, "/api/v1/ai/models/"+trustedModelID), http.StatusNotFound)

	rec = f.get(t, "/api/v1/ai/aliases/"+trustedAliasID)
	assertStatus(t, rec, http.StatusOK)
	assert.Equal(t, trustedAliasID, decodeResource(t, rec)["id"])

	rec = performJSON(t, f.router, http.MethodPatch, "/api/v1/ai/aliases/"+trustedAliasID,
		map[string]any{"displayName": "alias-renamed"})
	assertStatus(t, rec, http.StatusOK)
	assert.Equal(t, "alias-renamed", decodeResource(t, rec)["displayName"])
	assert.Equal(t, 1, f.aliases.updateCalls)

	rec = httptest.NewRecorder()
	f.router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/ai/aliases/"+trustedAliasID, nil))
	assertStatus(t, rec, http.StatusNoContent)
	assert.Equal(t, 1, f.aliases.deleteCalls)
	assertStatus(t, f.get(t, "/api/v1/ai/aliases/"+trustedAliasID), http.StatusNotFound)
}
