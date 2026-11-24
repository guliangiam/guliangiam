package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultETagConfig(t *testing.T) {
	cfg := DefaultETagConfig()
	assert.NotEmpty(t, cfg.HMACKey)
	assert.Len(t, cfg.HMACKey, 32)
}

func TestETagCacheMiddleware_SkipsNonGetRequests(t *testing.T) {
	cfg := ETagConfig{HMACKey: makeTestHMACKey()}

	methods := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			handler := ETagCacheMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("response"))
			}))

			req := httptest.NewRequest(method, "/api/v1/users", http.NoBody)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "response", rec.Body.String())
			// Non-GET should not set ETag
			assert.Empty(t, rec.Header().Get("ETag"))
		})
	}
}

func TestETagCacheMiddleware_SkipsWriteOperationPaths(t *testing.T) {
	cfg := ETagConfig{HMACKey: makeTestHMACKey()}

	writePaths := []string{
		"/api/v1/auth/login",
		"/api/v1/auth/logout",
		"/api/v1/auth/signup",
	}

	for _, path := range writePaths {
		t.Run(path, func(t *testing.T) {
			handler := ETagCacheMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			}))

			req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Empty(t, rec.Header().Get("ETag"))
		})
	}
}

func TestETagCacheMiddleware_SessionScopedAuthReadsAreNoStore(t *testing.T) {
	cfg := ETagConfig{HMACKey: makeTestHMACKey()}

	paths := []string{"/api/v1/auth/account", "/api/v1/auth/permissions", "/api/v1/auth/contact"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			handler := ETagCacheMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"account":"admin"}`))
			}))

			req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			// 会话态读不得带 ETag / max-age，否则登出后浏览器会回放旧响应。
			assert.Empty(t, rec.Header().Get("ETag"))
			assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
		})
	}
}

func TestETagCacheMiddleware_SetsETagOnGetResponse(t *testing.T) {
	cfg := ETagConfig{HMACKey: makeTestHMACKey()}

	handler := ETagCacheMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello world"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("ETag"))
	assert.Equal(t, "private, no-cache", rec.Header().Get("Cache-Control"))
}

func TestETagCacheMiddleware_SetsETagConsistently(t *testing.T) {
	cfg := ETagConfig{HMACKey: makeTestHMACKey()}

	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello world"))
	})

	handler := ETagCacheMiddleware(cfg)(innerHandler)

	// First request to get the ETag
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	etag1 := rec1.Header().Get("ETag")
	assert.NotEmpty(t, etag1)

	// Second request without If-None-Match should produce the same ETag
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	etag2 := rec2.Header().Get("ETag")
	assert.Equal(t, etag1, etag2, "ETag should be consistent for the same content")
}

func TestETagCacheMiddleware_MatchETagLogic(t *testing.T) {
	// Test the matchETag function directly since the middleware's WriteHeader
	// behavior may prevent 304 in certain flow paths.
	// matchETag is the core logic for conditional requests.
	tests := []struct {
		name        string
		ifNoneMatch string
		etag        string
		want        bool
	}{
		{"matching_etag", `"abc123"`, `"abc123"`, true},
		{"non_matching_etag", `"def456"`, `"abc123"`, false},
		{"wildcard_match", "*", `"abc123"`, true},
		{"empty_if_none_match", "", `"abc123"`, false},
		{"multiple_values_with_match", `"def456", "abc123"`, `"abc123"`, true},
		{"multiple_values_no_match", `"def456", "ghi789"`, `"abc123"`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchETag(tt.ifNoneMatch, tt.etag)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsWriteOperationPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/api/v1/auth/login", true},
		{"/api/v1/auth/logout", true},
		{"/api/v1/auth/signup", true},
		{"/api/v1/auth/login/callback", true}, // prefix match
		{"/api/v1/users", false},
		{"/api/v1/organizations", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isWriteOperationPath(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchETag(t *testing.T) {
	tests := []struct {
		name        string
		ifNoneMatch string
		etag        string
		want        bool
	}{
		{"empty_if_none_match", "", `"abc"`, false},
		{"exact_match", `"abc"`, `"abc"`, true},
		{"no_match", `"def"`, `"abc"`, false},
		{"wildcard", "*", `"abc"`, true},
		{"multiple_values_match", `"def", "abc"`, `"abc"`, true},
		{"multiple_values_no_match", `"def", "ghi"`, `"abc"`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchETag(tt.ifNoneMatch, tt.etag)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildCacheControl(t *testing.T) {
	// 新鲜期必须为 0（no-cache = 每次复用回源校验）：max-age 会让写后 refetch
	// 在新鲜期内回放旧 body，破坏列表的读-after-写 同步。
	assert.Equal(t, "private, no-cache", buildCacheControl())
}

func TestETagCacheMiddleware_DefaultConfigRevalidates(t *testing.T) {
	cfg := ETagConfig{HMACKey: makeTestHMACKey()}

	handler := ETagCacheMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "private, no-cache", rec.Header().Get("Cache-Control"))
}

// 读-after-写 回归：body 变化后，携带旧 ETag 的条件请求必须拿到 200 + 新 body，
// 且 Cache-Control 不允许浏览器跳过回源（no-cache）。
func TestETagCacheMiddleware_ReadAfterWriteRevalidates(t *testing.T) {
	cfg := ETagConfig{HMACKey: makeTestHMACKey()}
	body := `{"items":["a"]}`

	handler := ETagCacheMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/api/v1/applications", http.NoBody))
	oldETag := first.Header().Get("ETag")
	assert.NotEmpty(t, oldETag)

	body = `{"items":["a","b"]}`
	second := httptest.NewRequest(http.MethodGet, "/api/v1/applications", http.NoBody)
	second.Header.Set("If-None-Match", oldETag)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, second)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, `{"items":["a","b"]}`, rec.Body.String())
	assert.NotEqual(t, oldETag, rec.Header().Get("ETag"))
	assert.Equal(t, "private, no-cache", rec.Header().Get("Cache-Control"))
}

// makeTestHMACKey returns a deterministic HMAC key for tests.
func makeTestHMACKey() []byte {
	return []byte("test-hmac-key-0123456789abcdef")
}

func TestETagCacheMiddleware_HonoursHandlerDeclaredNoStore(t *testing.T) {
	cfg := ETagConfig{HMACKey: makeTestHMACKey()}

	handler := ETagCacheMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"messages":[]}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entries/entry-1/transcript", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// The body must still be delivered: opting out of caching is not opting out
	// of the response.
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, `{"messages":[]}`, rec.Body.String())
	// The handler's declaration wins, and no validator is attached for a
	// conditional request to replay it with.
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Empty(t, rec.Header().Get("ETag"))
}

func TestETagCacheMiddleware_StillValidatesCacheableResponses(t *testing.T) {
	cfg := ETagConfig{HMACKey: makeTestHMACKey()}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[]}`))
	})
	handler := ETagCacheMiddleware(cfg)(next)

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody))
	etag := first.Header().Get("ETag")
	assert.NotEmpty(t, etag)
	assert.Equal(t, "private, no-cache", first.Header().Get("Cache-Control"))

	second := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	second.Header.Set("If-None-Match", etag)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, second)
	assert.Equal(t, http.StatusNotModified, rec.Code)
}

func TestETagCacheMiddleware_DeliversNonSuccessStatusWithItsBody(t *testing.T) {
	cfg := ETagConfig{HMACKey: makeTestHMACKey()}

	handler := ETagCacheMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"not_found"}`))
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/entries/missing", http.NoBody))

	// The body is buffered to compute the validator, so the status has to be
	// replayed with it; a buffered write alone would be answered 200.
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, `{"code":"not_found"}`, rec.Body.String())
	assert.Empty(t, rec.Header().Get("ETag"))
}
