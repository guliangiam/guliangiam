package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/platform/logging"
)

// captureLogs 把默认 slog 换成写入 buffer 的生产日志 handler，返回 buffer。
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(logging.NewJSONHandler(buf, logging.Options{
		ServiceVersion: "test-sha",
		Environment:    "test",
		Level:          slog.LevelDebug,
	})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return buf
}

func traceIDWithSample(t *testing.T, keep bool) string {
	t.Helper()
	for i := 0; i < 1000; i++ {
		traceID := fmt.Sprintf("trace-%d", i)
		if logging.KeepInfoSample(traceID) == keep {
			return traceID
		}
	}
	t.Fatalf("no trace id with keep=%v", keep)
	return ""
}

func decodeLastLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.NotEmpty(t, lines)
	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[len(lines)-1]), &entry))
	return entry
}

func TestAccessLogMiddleware_LogsStatusAndRoutePattern(t *testing.T) {
	buf := captureLogs(t)

	router := chi.NewRouter()
	router.Use(chimw.RequestID)
	router.Use(AccessLogMiddleware)
	router.Get("/api/v1/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/0f8fad5b-d9cb-469f-a165-70867728950e?token=secret", http.NoBody)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	entry := decodeLastLine(t, buf)
	assert.Equal(t, "http access", entry["msg"])
	assert.Equal(t, http.MethodGet, entry["method"])
	// 路由模板而不是原始 URL：UUID 与 query 都不进日志。
	assert.Equal(t, "/api/v1/users/{id}", entry["path"])
	assert.Equal(t, float64(http.StatusCreated), entry["status"])
	assert.Equal(t, float64(len(`{"status":"ok"}`)), entry["bytes"])
	assert.NotEmpty(t, entry["request_id"])
	assert.NotEmpty(t, entry["trace_id"])
	assert.Contains(t, entry, "duration_ms")
	assert.NotContains(t, buf.String(), "secret")
}

func TestAccessLogMiddleware_DefaultStatusIsOK(t *testing.T) {
	buf := captureLogs(t)

	handler := AccessLogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/ping", http.NoBody))

	assert.Equal(t, float64(http.StatusOK), decodeLastLine(t, buf)["status"])
}

func TestAccessLogMiddleware_ServerErrorsLogAtErrorLevel(t *testing.T) {
	buf := captureLogs(t)

	handler := AccessLogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/boom", http.NoBody))

	entry := decodeLastLine(t, buf)
	assert.Equal(t, "ERROR", entry["level"])
	assert.Equal(t, float64(http.StatusBadGateway), entry["status"])
}

func TestAccessLogMiddleware_ClientErrorsLogAtInfoLevel(t *testing.T) {
	buf := captureLogs(t)

	handler := AccessLogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/missing", http.NoBody))

	entry := decodeLastLine(t, buf)
	assert.Equal(t, "INFO", entry["level"])
	assert.Equal(t, float64(http.StatusNotFound), entry["status"])
}

func TestAccessLogMiddleware_SkipsProbes(t *testing.T) {
	buf := captureLogs(t)

	handler := AccessLogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for _, path := range []string{"/healthz", "/livez", "/readyz", "/metrics"} {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, http.NoBody))
	}

	assert.Empty(t, buf.String())
}

// SSE / WebSocket 依赖 writer 能力断言；包装器不透传时 AI 网关流式会直接 500。
func TestAccessLogMiddleware_PreservesFlusher(t *testing.T) {
	handler := AccessLogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		flusher, ok := w.(http.Flusher)
		require.True(t, ok, "access log wrapper must forward http.Flusher")
		flusher.Flush()
		require.NotNil(t, http.NewResponseController(w))
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/stream", http.NoBody))
}

func TestAccessLogMiddleware_SnakeCaseFieldsAndSpanID(t *testing.T) {
	buf := captureLogs(t)

	handler := AccessLogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ctx := logging.WithRequestIDs(context.Background(), logging.RequestIDs{
		RequestID: "req-e3b",
		TraceID:   traceIDWithSample(t, true),
		SpanID:    "span-e3b",
	})
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody).WithContext(ctx))

	entry := decodeLastLine(t, buf)
	assert.Equal(t, "req-e3b", entry["request_id"])
	assert.NotEmpty(t, entry["trace_id"])
	assert.Equal(t, "span-e3b", entry["span_id"])
	assert.Contains(t, entry, "duration_ms")
	assert.NotContains(t, entry, "requestId")
	assert.NotContains(t, entry, "traceId")
	assert.NotContains(t, entry, "durationMs")
}

func TestAccessLogMiddleware_SamplesInfoByTrace(t *testing.T) {
	tests := []struct {
		name  string
		keep  bool
		wants bool
	}{
		{"sampled-out trace", false, false},
		{"sampled-in trace", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := captureLogs(t)
			handler := AccessLogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			ctx := logging.WithRequestIDs(context.Background(), logging.RequestIDs{
				TraceID: traceIDWithSample(t, tt.keep),
			})
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/ping", http.NoBody).WithContext(ctx))
			if tt.wants {
				assert.NotEmpty(t, buf.String())
				return
			}
			assert.Empty(t, buf.String())
		})
	}
}

func TestAccessLogMiddleware_LogsSlowRequestsAtFullRate(t *testing.T) {
	buf := captureLogs(t)
	handler := AccessLogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(510 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	ctx := logging.WithRequestIDs(context.Background(), logging.RequestIDs{
		TraceID: traceIDWithSample(t, false),
	})
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/slow", http.NoBody).WithContext(ctx))

	entry := decodeLastLine(t, buf)
	assert.Equal(t, "INFO", entry["level"])
	duration, ok := entry["duration_ms"].(float64)
	require.True(t, ok)
	assert.GreaterOrEqual(t, duration, float64(500))
}

func TestAccessLogMiddleware_LogsServerErrorsAtFullRate(t *testing.T) {
	buf := captureLogs(t)
	handler := AccessLogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	ctx := logging.WithRequestIDs(context.Background(), logging.RequestIDs{
		TraceID: traceIDWithSample(t, false),
	})
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/boom", http.NoBody).WithContext(ctx))

	entry := decodeLastLine(t, buf)
	assert.Equal(t, "ERROR", entry["level"])
	assert.Equal(t, float64(http.StatusBadGateway), entry["status"])
	assert.Equal(t, "http access", entry["msg"])
}

func TestAccessLogMiddleware_LogsFailedProbes(t *testing.T) {
	tests := []struct {
		status int
		level  string
	}{
		{http.StatusNotFound, "WARN"},
		{http.StatusServiceUnavailable, "ERROR"},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			buf := captureLogs(t)
			handler := AccessLogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
			}))
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody))

			entry := decodeLastLine(t, buf)
			assert.Equal(t, tt.level, entry["level"])
			assert.Equal(t, float64(tt.status), entry["status"])
		})
	}
}

func TestPrometheusMiddleware_PreservesFlusher(t *testing.T) {
	handler := PrometheusMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, ok := w.(http.Flusher)
		require.True(t, ok, "prometheus wrapper must forward http.Flusher")
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/v1/chat/completions", http.NoBody))
}
