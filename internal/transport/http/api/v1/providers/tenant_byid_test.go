package providers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readTrackingBody counts reads so a test can prove the handler rejected the
// request before decoding the payload.
type readTrackingBody struct {
	reads int
}

func (b *readTrackingBody) Read([]byte) (int, error) {
	b.reads++
	return 0, io.EOF
}

// TestProviderByID_MissingTenantRejectsBeforeBodyRead pins the fail-fast order:
// the trusted tenant is resolved before DecodeJSON, so a body-carrying write
// without a tenant never reaches the decoder or the repository.
func TestProviderByID_MissingTenantRejectsBeforeBodyRead(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
	}{
		{"patch", http.MethodPatch, "/api/v1/providers/" + ownProviderID},
		{"update-config", http.MethodPut, "/api/v1/providers/" + ownProviderID + "/config"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newTenantScopeRepo(ownProviderID, forgedOrgSessionID)
			service, _, _ := tenantScopeService(repo)

			body := &readTrackingBody{}
			req := httptest.NewRequest(tt.method, tt.target, body)
			req.Header.Set("Content-Type", "application/json")
			recorder := serveProviderRequest(t, tenantScopeRouter(service), req)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, body.reads, "missing tenant must be rejected before body decoding")
			assert.Zero(t, repo.getCalls, "missing tenant must not reach the repository")
		})
	}
}

// TestProviderByID_ConfigAndSecretStatusSurface pins the read shape of the two
// sub-resources: the config payload never mixes in the provider credential,
// and secret status exposes lifecycle metadata only.
func TestProviderByID_ConfigAndSecretStatusSurface(t *testing.T) {
	t.Run("config", func(t *testing.T) {
		repo := newTenantScopeRepo(ownProviderID, forgedOrgSessionID)
		service, _, _ := tenantScopeService(repo)
		recorder := serveProviderRequest(t, tenantScopeRouter(service), forgedOrgRequest(http.MethodGet,
			"/api/v1/providers/"+ownProviderID+"/config", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
		assert.NotContains(t, payload, "clientSecret")
		assert.NotContains(t, payload, "password")
		assert.NotContains(t, recorder.Body.String(), tenantScopeSecret)
	})

	t.Run("secret-status", func(t *testing.T) {
		repo := newTenantScopeRepo(ownProviderID, forgedOrgSessionID)
		service, _, _ := tenantScopeService(repo)
		recorder := serveProviderRequest(t, tenantScopeRouter(service), forgedOrgRequest(http.MethodGet,
			"/api/v1/providers/"+ownProviderID+"/secret-status", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		var payload struct {
			Items []map[string]any `json:"items"`
		}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
		for _, item := range payload.Items {
			for _, forbidden := range []string{"value", "secret", "fingerprint", "masked", "plaintext"} {
				assert.NotContains(t, item, forbidden)
			}
		}
		assert.NotContains(t, recorder.Body.String(), tenantScopeSecret)
	})
}

// TestProviderByID_ForeignConfigAndSecretStatusNotRead pins the ordering for
// the two sub-resource reads: ownership is decided on the provider row before
// the config or secret-status repository method is called.
func TestProviderByID_ForeignConfigAndSecretStatusNotRead(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{"config", "/api/v1/providers/" + foreignProviderID + "/config"},
		{"secret-status", "/api/v1/providers/" + foreignProviderID + "/secret-status"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newTenantScopeRepo(foreignProviderID, forgedOrgRequestID)
			service, _, _ := tenantScopeService(repo)
			recorder := serveProviderRequest(t, tenantScopeRouter(service), forgedOrgRequest(http.MethodGet,
				tt.target+"?organizationId="+forgedOrgRequestID, nil))

			assert.Equal(t, http.StatusNotFound, recorder.Code)
			assert.Zero(t, repo.configCalls)
			assert.Zero(t, repo.secretStatusCalls)
			assert.NotContains(t, strings.ToLower(recorder.Body.String()), tenantScopeSecret)
		})
	}
}
