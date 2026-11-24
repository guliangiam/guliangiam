package payments

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The notify endpoint is an external webhook: every supported gateway retries
// any non-2xx answer, so an oversized body stays a retryable failure while
// using the registered 413 instead of the old generic 400.
func TestNotify_OverBodyLimitIs413(t *testing.T) {
	handler := NewNotifyHandler(&mockOrderNotifier{}, NotifyConfig{})
	request := newNotifyRequest("stripe", "/api/v1/payments/notify/stripe",
		bytes.Repeat([]byte("x"), 2048))
	recorder := httptest.NewRecorder()
	request.Body = http.MaxBytesReader(recorder, request.Body, 1024)

	handler.Notify(recorder, request)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code, recorder.Body.String())
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, "request_entity_too_large", body.Error.Code)
}
