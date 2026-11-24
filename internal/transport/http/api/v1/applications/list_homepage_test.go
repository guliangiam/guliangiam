package applications

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
)

// TestListApplications_ResponseIncludesHomepageURL pins A1 (D4M-08): the list
// projection carries the application homepage so the portal does not need a
// per-card detail request.
func TestListApplications_ResponseIncludesHomepageURL(t *testing.T) {
	app := tenantApplication(byIDSessionOrg)
	app.HomepageURL = "https://portal.example.com"
	router, _ := newByIDRouter(&byIDApplicationRepo{apps: []*domain.Application{app}})

	recorder := serveByID(router, byIDRequest(http.MethodGet, "/api/v1/applications", ""))
	require.Equal(t, http.StatusOK, recorder.Code)

	var payload struct {
		Items []struct {
			ID          string `json:"id"`
			HomepageURL string `json:"homepageUrl"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Len(t, payload.Items, 1)
	assert.Equal(t, byIDApplicationID, payload.Items[0].ID)
	assert.Equal(t, "https://portal.example.com", payload.Items[0].HomepageURL)
}

// TestListApplications_NoSecretMaterial pins the read-side contract for the list
// endpoint: the list projection never carries a client secret, its plaintext or
// its hash. Detail responses expose only the masked pair, and the list endpoint
// exposes nothing at all.
func TestListApplications_NoSecretMaterial(t *testing.T) {
	app := tenantApplication(byIDSessionOrg)
	app.ClientSecretHash = "$argon2id$v=19$m=65536,t=3,p=2$list-sentinel$list-sentinel"
	router, _ := newByIDRouter(&byIDApplicationRepo{apps: []*domain.Application{app}})

	recorder := serveByID(router, byIDRequest(http.MethodGet, "/api/v1/applications", ""))
	require.Equal(t, http.StatusOK, recorder.Code)

	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	var items []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(payload["items"], &items), "items must be an array of objects")
	require.Len(t, items, 1)

	for _, item := range items {
		for _, key := range []string{"clientSecret", "clientSecretPlain", "clientSecretHash"} {
			_, exists := item[key]
			assert.Falsef(t, exists, "list item 不得包含字段 %q", key)
		}
	}
	assert.NotContains(t, recorder.Body.String(), "$argon2id$", "list 响应不得回显 secret 哈希")
}
