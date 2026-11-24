package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/guliangiam/guliangiam/internal/platform/logging"
)

// benchLogSink counts the access-log records that survive sampling and reach
// the sink. Benchmarks are single-goroutine, so no mutex is needed.
type benchLogSink struct {
	records int
}

func (s *benchLogSink) Write(p []byte) (int, error) {
	s.records++
	return len(p), nil
}

// benchResponseWriter is a zero-allocation ResponseWriter; httptest.Recorder
// would add its own allocations and hide the middleware overhead.
type benchResponseWriter struct {
	header http.Header
	status int
	bytes  int
}

func (w *benchResponseWriter) Header() http.Header { return w.header }

func (w *benchResponseWriter) WriteHeader(code int) { w.status = code }

func (w *benchResponseWriter) Write(p []byte) (int, error) {
	w.bytes += len(p)
	return len(p), nil
}

func benchOK(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func benchRouter(withAccessLog bool) http.Handler {
	router := chi.NewRouter()
	if withAccessLog {
		router.Use(AccessLogMiddleware)
	}
	router.Get("/api/v1/users/{id}", benchOK)
	router.Get("/healthz", benchOK)
	return router
}

func benchAccessRequest(path, traceID string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	ctx := logging.WithRequestIDs(request.Context(), logging.RequestIDs{
		RequestID: "req-bench",
		TraceID:   traceID,
		SpanID:    "span-bench",
	})
	return request.WithContext(ctx)
}

func benchTraceID(keep bool) string {
	for i := 0; i < 10000; i++ {
		traceID := "trace-" + strconv.Itoa(i)
		if logging.KeepInfoSample(traceID) == keep {
			return traceID
		}
	}
	return ""
}

func runAccessLogBench(b *testing.B, sink *benchLogSink, handler http.Handler, request *http.Request) {
	writer := &benchResponseWriter{header: make(http.Header, 4)}
	start := sink.records
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.ServeHTTP(writer, request)
	}
	b.StopTimer()
	b.ReportMetric(float64(sink.records-start)*100/float64(b.N), "%written")
}

// BenchmarkAccessLogMiddleware measures the production access-log chain:
// chi routing + AccessLogMiddleware + statusRecorder + sanitizing JSON handler.
// The logger is the exact composition bootstrap installs (SanitizingHandler
// around NewJSONHandler), and correlation ids are pre-stamped as the
// RequestMetadataMiddleware would stamp them.
func BenchmarkAccessLogMiddleware(b *testing.B) {
	sink := &benchLogSink{}
	previous := slog.Default()
	slog.SetDefault(slog.New(logging.NewSanitizingHandler(logging.NewJSONHandler(sink, logging.Options{
		ServiceVersion: "bench-sha",
		Environment:    "benchmark",
		Level:          slog.LevelInfo,
	}))))
	b.Cleanup(func() { slog.SetDefault(previous) })

	sampledOut := benchTraceID(false)
	sampledIn := benchTraceID(true)
	if sampledOut == "" || sampledIn == "" {
		b.Fatal("no trace id found for both sampling buckets")
	}

	// NoAccessLog / NoAccessLogProbe are controls; subtract the matching one
	// to isolate the middleware cost for the same chi route shape.
	b.Run("NoAccessLog", func(b *testing.B) {
		runAccessLogBench(b, sink, benchRouter(false), benchAccessRequest("/api/v1/users/bench", sampledOut))
	})
	b.Run("SampledIn", func(b *testing.B) {
		runAccessLogBench(b, sink, benchRouter(true), benchAccessRequest("/api/v1/users/bench", sampledIn))
	})
	b.Run("SampledOut", func(b *testing.B) {
		runAccessLogBench(b, sink, benchRouter(true), benchAccessRequest("/api/v1/users/bench", sampledOut))
	})
	b.Run("NoAccessLogProbe", func(b *testing.B) {
		runAccessLogBench(b, sink, benchRouter(false), benchAccessRequest("/healthz", sampledOut))
	})
	b.Run("ProbeSkipped", func(b *testing.B) {
		runAccessLogBench(b, sink, benchRouter(true), benchAccessRequest("/healthz", sampledOut))
	})
}
