package users

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func multipartUploadBody(t *testing.T, size int) (*bytes.Buffer, string) {
	t.Helper()
	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	part, err := writer.CreateFormFile("file", "users.xlsx")
	require.NoError(t, err)
	_, err = part.Write(bytes.Repeat([]byte("x"), size))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return &payload, writer.FormDataContentType()
}

// Each import endpoint reads its multipart body through the bootstrap
// MaxBytesReader wall. Over the wall the answer is the unified 413 envelope,
// never a 500 or a dropped connection.
func TestUploadRoutes_OverBodyLimitIs413(t *testing.T) {
	paths := []string{
		"/api/v1/users/upload",
		"/api/v1/users/upload/preview",
		"/api/v1/users/upload-csv",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			router := testRouter()
			RegisterChiRoutes(router, newHandlerWithNilService())

			payload, contentType := multipartUploadBody(t, 2048)
			request := httptest.NewRequest(http.MethodPost, path, payload)
			request.Header.Set("Content-Type", contentType)
			recorder := httptest.NewRecorder()

			// Stand-in for bootstrap.requestBodyLimitMiddleware: the real wall is
			// STORAGE_UPLOAD_MAX_BYTES, a tiny cap makes the crossing observable.
			request.Body = http.MaxBytesReader(recorder, request.Body, 1024)
			router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)

			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
				RequestID string `json:"requestId"`
				TraceID   string `json:"traceId"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
			assert.Equal(t, "request_entity_too_large", body.Error.Code)
			assert.NotEmpty(t, body.RequestID)
			assert.NotEmpty(t, body.TraceID)
		})
	}
}
