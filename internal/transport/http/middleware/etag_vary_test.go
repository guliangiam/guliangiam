package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BE-E-08: cached GET responses must vary on credentials so a shared browser
// cannot replay user A's body for user B after a logout/login switch.
func TestETagCacheMiddleware_VaryOnCredentials(t *testing.T) {
	cfg := ETagConfig{HMACKey: makeTestHMACKey()}
	handler := ETagCacheMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"users":[]}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Values("Vary"), "Cookie")
	assert.Contains(t, rec.Header().Values("Vary"), "Authorization")

	// The 304 revalidation branch must carry Vary too.
	etag := rec.Header().Get("ETag")
	assert.NotEmpty(t, etag)
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusNotModified, rec2.Code)
	assert.Contains(t, rec2.Header().Values("Vary"), "Cookie")
	assert.Contains(t, rec2.Header().Values("Vary"), "Authorization")
}
