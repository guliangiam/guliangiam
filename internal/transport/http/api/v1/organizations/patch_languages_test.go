package organizations

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// PATCH /api/v1/organizations/{id} uses an independent DTO with
// DecodeJSON's DisallowUnknownFields wall; languages must decode and map to
// the app command instead of being rejected as an unknown field.
func TestPatchOrganizationRequest_DecodesLanguages(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/organizations/org-1",
		strings.NewReader(`{"languages":["en","zh-CN"],"revision":2}`),
	)
	req.Header.Set("Content-Type", "application/json")

	var body PatchOrganizationRequest
	require.NoError(t, route.DecodeJSON(req, &body))
	assert.Equal(t, []string{"en", "zh-CN"}, body.Languages)

	command := body.toCommand("org-1")
	assert.Equal(t, []string{"en", "zh-CN"}, command.Languages)
}
