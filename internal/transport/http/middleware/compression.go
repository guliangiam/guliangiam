package middleware

import (
	"bufio"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
)

type CompressionConfig struct {
	MinSize      int
	Level        int
	SkipSuffixes []string
}

func DefaultCompressionConfig() CompressionConfig {
	return CompressionConfig{
		MinSize:      1024,
		Level:        6,
		SkipSuffixes: []string{".jpg", ".png", ".gif", ".webp", ".woff2", ".br", ".gz"},
	}
}

type pooledGzipWriter struct {
	*gzip.Writer
	level int
}

var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		return &pooledGzipWriter{}
	},
}

func CompressionMiddleware(cfg CompressionConfig) func(http.Handler) http.Handler {
	if cfg.MinSize <= 0 {
		cfg.MinSize = 1024
	}
	if cfg.Level < gzip.HuffmanOnly || cfg.Level > gzip.BestCompression {
		cfg.Level = gzip.DefaultCompression
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !shouldCompress(r, cfg) {
				next.ServeHTTP(w, r)
				return
			}

			if !acceptsGzip(r) {
				next.ServeHTTP(w, r)
				return
			}

			rw := &compressionResponseWriter{
				ResponseWriter: w,
				cfg:            cfg,
				buffer:         make([]byte, 0, cfg.MinSize),
			}
			defer rw.close()

			next.ServeHTTP(rw, r)
		})
	}
}

func shouldCompress(r *http.Request, cfg CompressionConfig) bool {
	if r.Method == http.MethodHead {
		return false
	}

	routePattern := chi.RouteContext(r.Context()).RoutePattern()
	if routePattern != "" {
		for _, suffix := range cfg.SkipSuffixes {
			if strings.HasSuffix(routePattern, suffix) {
				return false
			}
		}
	}

	path := r.URL.Path
	for _, suffix := range cfg.SkipSuffixes {
		if strings.HasSuffix(path, suffix) {
			return false
		}
	}

	ae := r.Header.Get("Accept-Encoding")
	return ae != ""
}

func acceptsGzip(r *http.Request) bool {
	ae := r.Header.Get("Accept-Encoding")
	for _, enc := range strings.Split(ae, ",") {
		enc = strings.TrimSpace(enc)
		if enc == "gzip" || strings.HasPrefix(enc, "gzip;") {
			return true
		}
	}
	return false
}

type compressionResponseWriter struct {
	http.ResponseWriter
	cfg         CompressionConfig
	buffer      []byte
	gzWriter    *pooledGzipWriter
	compress    bool
	written     bool
	wroteHeader bool
	status      int
	hijacked    bool
}

func (rw *compressionResponseWriter) Write(b []byte) (int, error) {
	if rw.hijacked {
		return rw.ResponseWriter.Write(b)
	}

	if !rw.compress {
		rw.buffer = append(rw.buffer, b...)
		if len(rw.buffer) >= rw.cfg.MinSize {
			rw.startCompression()
			n, err := rw.gzWriter.Write(rw.buffer)
			rw.buffer = nil
			if err != nil {
				return n, err
			}
		}
		return len(b), nil
	}

	return rw.gzWriter.Write(b)
}

func (rw *compressionResponseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.written = true
		rw.status = code
	}
}

func (rw *compressionResponseWriter) startCompression() {
	rw.compress = true
	rw.Header().Del("Content-Length")
	rw.Header().Set("Content-Encoding", "gzip")
	rw.Header().Add("Vary", "Accept-Encoding")

	if rw.status > 0 {
		rw.ResponseWriter.WriteHeader(rw.status)
	}

	pw := gzipWriterPool.Get().(*pooledGzipWriter)
	if pw.Writer == nil || pw.level != rw.cfg.Level {
		pw.Writer, _ = gzip.NewWriterLevel(rw.ResponseWriter, rw.cfg.Level)
		pw.level = rw.cfg.Level
	} else {
		pw.Reset(rw.ResponseWriter)
	}
	rw.gzWriter = pw
}

func (rw *compressionResponseWriter) close() {
	switch {
	case rw.compress && rw.gzWriter != nil:
		_ = rw.gzWriter.Close()
		rw.gzWriter.Reset(io.Discard)
		gzipWriterPool.Put(rw.gzWriter)
		rw.gzWriter = nil
	case len(rw.buffer) > 0:
		if rw.status > 0 {
			rw.ResponseWriter.WriteHeader(rw.status)
		}
		_, _ = rw.ResponseWriter.Write(rw.buffer)
		rw.buffer = nil
	case rw.status > 0 && !rw.wroteHeader:
		rw.ResponseWriter.WriteHeader(rw.status)
	}
}

func (rw *compressionResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	rw.hijacked = true
	return rw.ResponseWriter.(http.Hijacker).Hijack()
}

func (rw *compressionResponseWriter) Flush() {
	if rw.compress && rw.gzWriter != nil {
		_ = rw.gzWriter.Flush()
	}
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (rw *compressionResponseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}
