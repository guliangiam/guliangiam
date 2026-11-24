package middleware

import (
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultCompressionConfig(t *testing.T) {
	cfg := DefaultCompressionConfig()
	assert.Equal(t, 1024, cfg.MinSize)
	assert.Equal(t, 6, cfg.Level)
	assert.Contains(t, cfg.SkipSuffixes, ".jpg")
	assert.Contains(t, cfg.SkipSuffixes, ".png")
	assert.Contains(t, cfg.SkipSuffixes, ".gif")
	assert.Contains(t, cfg.SkipSuffixes, ".woff2")
}

func TestCompressionMiddleware_SkipsWithoutAcceptEncoding(t *testing.T) {
	cfg := DefaultCompressionConfig()
	handler := CompressionMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello world"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	// No Accept-Encoding header
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "hello world", rec.Body.String())
	// Should not have Content-Encoding since client doesn't accept gzip
	assert.NotEqual(t, "gzip", rec.Header().Get("Content-Encoding"))
}

func TestCompressionMiddleware_SkipsHeadRequests(t *testing.T) {
	cfg := DefaultCompressionConfig()
	handler := CompressionMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodHead, "/api/v1/users", http.NoBody)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCompressionMiddleware_SkipsSkipSuffixes(t *testing.T) {
	cfg := DefaultCompressionConfig()
	handler := CompressionMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("image data"))
	}))

	tests := []struct {
		name string
		path string
	}{
		{"jpg", "/static/photo.jpg"},
		{"png", "/static/icon.png"},
		{"gif", "/static/anim.gif"},
		{"webp", "/static/photo.webp"},
		{"woff2", "/static/font.woff2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, http.NoBody)
			req.Header.Set("Accept-Encoding", "gzip")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.NotEqual(t, "gzip", rec.Header().Get("Content-Encoding"))
		})
	}
}

func TestCompressionMiddleware_PassthroughSmallResponse(t *testing.T) {
	cfg := CompressionConfig{
		MinSize:      1024,
		Level:        6,
		SkipSuffixes: []string{},
	}

	handler := CompressionMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("small")) // less than MinSize
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	// Small response should not be compressed
	assert.Equal(t, "small", rec.Body.String())
}

func TestAcceptsGzip(t *testing.T) {
	tests := []struct {
		name           string
		acceptEncoding string
		want           bool
	}{
		{"gzip_only", "gzip", true},
		{"gzip_with_quality", "gzip;q=1.0", true},
		{"multiple_encodings", "br, gzip, deflate", true},
		{"no_gzip", "br, deflate", false},
		{"empty", "", false},
		{"identity_only", "identity", false},
		{"gzip_after_space", "gzip, br", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			if tt.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}
			got := acceptsGzip(req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCompressionConfig_Defaults(t *testing.T) {
	t.Run("min_size_zero_defaults_to_1024", func(t *testing.T) {
		cfg := CompressionConfig{MinSize: 0, Level: 6}
		handler := CompressionMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		assert.NotNil(t, handler)
	})

	t.Run("invalid_level_defaults", func(t *testing.T) {
		cfg := CompressionConfig{MinSize: 1024, Level: 99}
		handler := CompressionMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		assert.NotNil(t, handler)
	})
}

func TestCompressionResponseWriter_CloseWritesBufferedBody(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &compressionResponseWriter{
		ResponseWriter: rec,
		cfg:            DefaultCompressionConfig(),
		buffer:         []byte("hello"),
		status:         http.StatusCreated,
	}
	rw.close()

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "hello", rec.Body.String())
}

func TestCompressionResponseWriter_CloseWritesStatusOnly(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &compressionResponseWriter{
		ResponseWriter: rec,
		cfg:            DefaultCompressionConfig(),
		status:         http.StatusNoContent,
	}
	rw.close()

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func TestCompressionResponseWriter_CloseReleasesGzipWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &compressionResponseWriter{
		ResponseWriter: rec,
		cfg:            DefaultCompressionConfig(),
		compress:       true,
	}
	pw, ok := gzipWriterPool.Get().(*pooledGzipWriter)
	assert.True(t, ok)
	pw.Writer, _ = gzip.NewWriterLevel(rec, rw.cfg.Level)
	pw.level = rw.cfg.Level
	rw.gzWriter = pw

	rw.close()
	assert.Nil(t, rw.gzWriter)
}
