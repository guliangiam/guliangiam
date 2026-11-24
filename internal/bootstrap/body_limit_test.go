package bootstrap

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/guliangiam/guliangiam/internal/transport/http/route"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestBodyLimit_PathTable(t *testing.T) {
	t.Setenv("STORAGE_UPLOAD_MAX_BYTES", "33554432")

	assert.Equal(t, int64(33554432), requestBodyLimit("/api/v1/upload-resource"))
	for _, path := range []string{
		"/api/v1/users/upload",
		"/api/v1/users/upload/preview",
		"/api/v1/users/upload-csv",
		"/api/v1/groups/upload",
		"/api/v1/roles/upload",
		"/api/v1/permissions/upload",
		"/api/v1/enforcers/enf-001/upload-policies",
	} {
		assert.Equal(t, int64(33554432), requestBodyLimit(path), path)
	}
	for _, path := range []string{
		"/api/v1/entries/traces",
		"/api/v1/entries/metrics",
		"/api/v1/entries/logs",
	} {
		assert.Equal(t, otlpRequestBodyLimit, requestBodyLimit(path), path)
	}
	// The old `/entries/otlp/v1/*` spelling matches no registered route; it
	// must not keep a larger wall than the default.
	assert.Equal(t, defaultRequestBodyLimit, requestBodyLimit("/api/v1/entries/otlp/v1/logs"))
	assert.Equal(t, defaultRequestBodyLimit, requestBodyLimit("/api/v1/users"))
	// The enforcer whitelist is a suffix rule; other enforcer paths keep the
	// default wall.
	assert.Equal(t, defaultRequestBodyLimit, requestBodyLimit("/api/v1/enforcers/enf-001"))
	assert.Equal(t, defaultRequestBodyLimit, requestBodyLimit("/api/v1/enforcers/enf-001/export-policies"))
}

func multipartUploadOverOneMiB(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()
	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	part, err := writer.CreateFormFile("file", "import.xlsx")
	require.NoError(t, err)
	// One MiB plus the multipart envelope: over the 1 MiB default wall, under
	// the 2 MiB configured cap used by the accept subtest.
	_, err = part.Write(bytes.Repeat([]byte("x"), 1<<20))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return &payload, writer.FormDataContentType()
}

func multipartImportHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		route.WriteError(w, r, err)
		return
	}
	defer r.MultipartForm.RemoveAll()
	route.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// The users import whitelist must be the real gate: a body over the 1 MiB
// default but under STORAGE_UPLOAD_MAX_BYTES parses, and one over the configured
// cap is the unified 413. Before this fix both outcomes were 413 at 1 MiB no
// matter what the handler declared.
func TestRequestBodyLimitMiddleware_UsersImportUsesConfiguredLimit(t *testing.T) {
	t.Run("under configured cap parses", func(t *testing.T) {
		t.Setenv("STORAGE_UPLOAD_MAX_BYTES", strconv.Itoa(2<<20))
		payload, contentType := multipartUploadOverOneMiB(t)
		request := httptest.NewRequest(http.MethodPost, "/api/v1/users/upload", payload)
		request.Header.Set("Content-Type", contentType)
		recorder := httptest.NewRecorder()

		requestBodyLimitMiddleware(http.HandlerFunc(multipartImportHandler)).ServeHTTP(recorder, request)

		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	})

	t.Run("over configured cap is 413", func(t *testing.T) {
		t.Setenv("STORAGE_UPLOAD_MAX_BYTES", strconv.Itoa(1<<20))
		payload, contentType := multipartUploadOverOneMiB(t)
		request := httptest.NewRequest(http.MethodPost, "/api/v1/users/upload", payload)
		request.Header.Set("Content-Type", contentType)
		recorder := httptest.NewRecorder()

		requestBodyLimitMiddleware(http.HandlerFunc(multipartImportHandler)).ServeHTTP(recorder, request)

		require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code, recorder.Body.String())
		var body struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
		assert.Equal(t, "request_entity_too_large", body.Error.Code)
	})
}

// The remaining Excel import endpoints share the same whitelist and the same
// two-direction contract as the users imports: above 1 MiB but below the
// configured cap parses; above the configured cap is the unified 413.
func TestRequestBodyLimitMiddleware_OtherImportPathsUseConfiguredLimit(t *testing.T) {
	paths := []string{
		"/api/v1/groups/upload",
		"/api/v1/roles/upload",
		"/api/v1/permissions/upload",
		"/api/v1/enforcers/enf-001/upload-policies",
	}
	for _, path := range paths {
		path := path
		t.Run(path+" under configured cap parses", func(t *testing.T) {
			t.Setenv("STORAGE_UPLOAD_MAX_BYTES", strconv.Itoa(2<<20))
			payload, contentType := multipartUploadOverOneMiB(t)
			request := httptest.NewRequest(http.MethodPost, path, payload)
			request.Header.Set("Content-Type", contentType)
			recorder := httptest.NewRecorder()

			requestBodyLimitMiddleware(http.HandlerFunc(multipartImportHandler)).ServeHTTP(recorder, request)

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		})

		t.Run(path+" over configured cap is 413", func(t *testing.T) {
			t.Setenv("STORAGE_UPLOAD_MAX_BYTES", strconv.Itoa(1<<20))
			payload, contentType := multipartUploadOverOneMiB(t)
			request := httptest.NewRequest(http.MethodPost, path, payload)
			request.Header.Set("Content-Type", contentType)
			recorder := httptest.NewRecorder()

			requestBodyLimitMiddleware(http.HandlerFunc(multipartImportHandler)).ServeHTTP(recorder, request)

			require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code, recorder.Body.String())
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
			assert.Equal(t, "request_entity_too_large", body.Error.Code)
		})
	}
}

// The default remains the 1 MiB wall for everything not in the whitelist.
func TestRequestBodyLimitMiddleware_DefaultPathRejectsOverOneMiB(t *testing.T) {
	t.Setenv("STORAGE_UPLOAD_MAX_BYTES", strconv.Itoa(32<<20))
	payload, contentType := multipartUploadOverOneMiB(t)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/users", payload)
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()

	requestBodyLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			route.WriteError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(recorder, request)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code, recorder.Body.String())
}
