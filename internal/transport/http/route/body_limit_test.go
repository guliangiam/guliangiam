package route

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type errorEnvelopeBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	RequestID string `json:"requestId"`
	TraceID   string `json:"traceId"`
}

func serveWithRequestID(handler http.Handler, request *http.Request) (*httptest.ResponseRecorder, errorEnvelopeBody) {
	recorder := httptest.NewRecorder()
	chimw.RequestID(handler).ServeHTTP(recorder, request)

	var body errorEnvelopeBody
	if recorder.Body.Len() > 0 {
		_ = json.Unmarshal(recorder.Body.Bytes(), &body)
	}
	return recorder, body
}

// A JSON body over the bootstrap MaxBytesReader wall must be the registered
// 413, not the invalid_argument 400 DecodeJSON used to return for every decode
// failure.
func TestDecodeJSON_OverBodyLimitIs413(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 32)
		var target map[string]string
		if err := DecodeJSON(r, &target); err != nil {
			WriteError(w, r, err)
			return
		}
		WriteJSON(w, http.StatusOK, target)
	})

	payload := `{"name":"` + strings.Repeat("x", 64) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(payload))
	recorder, body := serveWithRequestID(handler, request)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	assert.Equal(t, "request_entity_too_large", body.Error.Code)
	assert.Contains(t, body.Error.Message, "32")
	assert.NotEmpty(t, body.RequestID, "the unified envelope must carry requestId")
	assert.NotEmpty(t, body.TraceID, "the unified envelope must carry traceId")
}

// Multipart parsing returns the raw *http.MaxBytesError; WriteError must map it
// too so an oversized import never surfaces as a 500 (and never relies on the
// handler to know about MaxBytesReader).
func TestWriteError_MapsMultipartMaxBytesErrorTo413(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 64)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			WriteError(w, r, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	part, err := writer.CreateFormFile("file", "users.xlsx")
	require.NoError(t, err)
	_, err = part.Write(bytes.Repeat([]byte("x"), 128))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	request := httptest.NewRequest(http.MethodPost, "/api/v1/users/upload", &payload)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder, body := serveWithRequestID(handler, request)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	assert.Equal(t, "request_entity_too_large", body.Error.Code)
	assert.NotEmpty(t, body.RequestID)
	assert.NotEmpty(t, body.TraceID)
}
