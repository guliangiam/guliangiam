package applications

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// 脚本能力已废弃（治理裁定）：customJs 不再是 PATCH 请求的合法字段，
// DecodeJSON 的 DisallowUnknownFields 墙必须拒绝携带它的请求体。
func TestPatchApplicationRequest_RejectsCustomJs(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/applications/app-1",
		strings.NewReader(`{"customJs":"alert(1)","revision":1}`),
	)
	req.Header.Set("Content-Type", "application/json")

	var body PatchApplicationRequest
	err := route.DecodeJSON(req, &body)
	require.Error(t, err, "customJs must be rejected as an unknown field")
	assert.Contains(t, err.Error(), "customJs")
}

// 同批废弃不得误伤相邻合法字段：customCss/customHeader 仍正常解码并映射。
func TestPatchApplicationRequest_StillDecodesCustomCssAndHeader(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/applications/app-1",
		strings.NewReader(`{"customCss":".x{color:red}","customHeader":"<meta name=\"description\" content=\"app\">","revision":1}`),
	)
	req.Header.Set("Content-Type", "application/json")

	var body PatchApplicationRequest
	require.NoError(t, route.DecodeJSON(req, &body))
	require.NotNil(t, body.CustomCss)
	assert.Equal(t, ".x{color:red}", *body.CustomCss)
	require.NotNil(t, body.CustomHeader)
	assert.Equal(t, `<meta name="description" content="app">`, *body.CustomHeader)
}
