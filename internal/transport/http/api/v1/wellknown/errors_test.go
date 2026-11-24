package wellknown

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errBadRequest used to return a package-local type carrying only a message.
// route.WriteError renders the protocol face through OAuthErrorFrom, and an
// error with neither an application code nor HTTPStatus() falls through to the
// 500 default — so the discovery/protected-resource 400s were answered as 500
// server_error. These assertions pin the violated property.
func TestErrBadRequest_IsRenderedAs400InvalidRequest(t *testing.T) {
	err := errBadRequest("application is required")

	require.EqualError(t, err, "application is required")
	assert.Equal(t, http.StatusBadRequest, route.StatusCodeFromError(err))

	status, code, description := route.OAuthErrorFrom(err)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "invalid_request", string(code))
	assert.Equal(t, "application is required", description)
}

func TestErrBadRequest_ProtocolFaceResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/.well-known/app-1/oauth-protected-resource", nil)
	request = request.WithContext(route.WithRequestMetadata(request.Context(), route.RequestMetadata{
		Surface: route.SurfaceProtocol,
	}))

	route.WriteError(recorder, request, errBadRequest("application is required"))

	assert.Equal(t, http.StatusBadRequest, recorder.Code)

	body := map[string]any{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, "invalid_request", body["error"])
}

// The UserInfo endpoint used to hand err.Error() straight to error_description,
// which made every internal failure readable by the caller. A 401 from a
// protected resource must keep the RFC 6750 §3.1 code (invalid_token) while the
// description is sanitized.
func TestUserInfo_InvalidTokenDoesNotEchoInternalMessage(t *testing.T) {
	recorder := httptest.NewRecorder()

	route.WriteResourceError(recorder, apperrors.NewInternal("SELECT * FROM guliangiam.access_tokens failed"),
		route.OAuthErrInvalidToken, http.StatusUnauthorized)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)

	body := map[string]any{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, "invalid_token", body["error"])

	description, _ := body["error_description"].(string)
	assert.NotEmpty(t, description)
	assert.NotContains(t, description, "SELECT")
	assert.NotContains(t, description, "access_tokens")
}

// A 4xx application message is safe to show and is what makes a client-side
// failure debuggable, so the sanitizer must only blank out the 5xx class.
func TestUserInfo_KeepsSafeClientMessage(t *testing.T) {
	recorder := httptest.NewRecorder()

	route.WriteResourceError(recorder, apperrors.NewForbidden("token has no openid scope"),
		route.OAuthErrInvalidToken, http.StatusUnauthorized)

	body := map[string]any{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	// The code stays invalid_token even though the service raised forbidden:
	// RFC 6750 §3.1 does not define insufficient_scope for a 401 here.
	assert.Equal(t, "invalid_token", body["error"])
	assert.Equal(t, "token has no openid scope", body["error_description"])
}
