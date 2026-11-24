package saml

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// errBadRequest used to return a package-local type that carried only a message.
// The protocol face renders through route.WriteError → OAuthErrorFrom, and an
// error that is neither an *apperrors.Error nor an HTTPStatus() implementer
// falls through to StatusCodeFromError's 500 default — so every 400 these
// endpoints meant to send was answered as 500 server_error. These assertions
// pin the property that was violated, not the wording of a message: a future
// refactor can rename the errors without weakening the guarantee.
func TestErrBadRequest_IsRenderedAs400InvalidRequest(t *testing.T) {
	err := errBadRequest("SAMLRequest is required")

	require.EqualError(t, err, "SAMLRequest is required")
	assert.Equal(t, http.StatusBadRequest, route.StatusCodeFromError(err))

	status, code, description := route.OAuthErrorFrom(err)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "invalid_request", string(code))
	assert.Equal(t, "SAMLRequest is required", description)
}

// The end-to-end form of the same guarantee: the body and status a client
// actually receives when the handler hands the error to the protocol face.
func TestErrBadRequest_ProtocolFaceResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/saml/sso/app-1", nil)
	request = request.WithContext(route.WithRequestMetadata(request.Context(), route.RequestMetadata{
		Surface: route.SurfaceProtocol,
	}))

	route.WriteError(recorder, request, errBadRequest("applicationId is required"))

	assert.Equal(t, http.StatusBadRequest, recorder.Code)

	body := map[string]any{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, "invalid_request", body["error"])
	assert.NotEqual(t, "server_error", body["error"])
}

// Every errBadRequest call site must land on 400. A regression that reintroduced
// a bare error type would show up here as a 500 while the message stayed equal,
// which is exactly how the defect stayed invisible before.
func TestErrBadRequest_EveryCallSiteIsA400(t *testing.T) {
	messages := []string{
		"applicationId is required",
		"authentication required for SLO",
		"SAMLRequest is required",
		"invalid form data",
	}
	for _, message := range messages {
		t.Run(message, func(t *testing.T) {
			assert.Equal(t, http.StatusBadRequest, route.StatusCodeFromError(errBadRequest(message)))
		})
	}
}
