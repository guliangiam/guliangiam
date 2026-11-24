package mcpservers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSyncIntranetHTTP_RejectsOversizedSurface pins the transport contract for
// E-01: the caps are enforced centrally and surface as the standard error envelope
// with 422, before any probe or repository write.
func TestSyncIntranetHTTP_RejectsOversizedSurface(t *testing.T) {
	cidrs := make([]string, 9)
	for i := range cidrs {
		cidrs[i] = "127.0.0.1/32"
	}
	body, err := json.Marshal(map[string]any{
		"cidrs":       cidrs,
		"ports":       []int{80},
		"concurrency": 1,
	})
	require.NoError(t, err)

	f := newTenantFixture(nil, nil)
	recorder := serveTenant(f, tenantRequest(http.MethodPost,
		"/api/v1/mcp-servers/sync-intranet", string(body), tenantSessionOrgID))

	assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"unprocessable_entity"`)
	assert.Empty(t, f.servers.servers, "a rejected scan must not register servers")
}
