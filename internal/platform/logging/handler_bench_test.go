package logging_test

import (
	"context"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/logging"
)

// benchSink counts complete records written by the handler. Benchmarks are
// single-goroutine; the counters stay in the measured loop so a sampling
// regression shows up as %written instead of a silently faster number.
type benchSink struct {
	records int
	bytes   int64
}

func (s *benchSink) Write(p []byte) (int, error) {
	s.records++
	s.bytes += int64(len(p))
	return len(p), nil
}

func newBenchLogger(sink io.Writer) *slog.Logger {
	return slog.New(logging.NewJSONHandler(sink, logging.Options{
		ServiceVersion: "bench-sha",
		Environment:    "benchmark",
		Level:          slog.LevelInfo,
	}))
}

// benchTraceID finds a trace id whose stable hash lands inside/outside the
// fixed 10% sample, mirroring the ids transport stamps in production.
func benchTraceID(keep bool) string {
	for i := 0; i < 10000; i++ {
		traceID := "trace-" + strconv.Itoa(i)
		if logging.KeepInfoSample(traceID) == keep {
			return traceID
		}
	}
	return ""
}

// accessAttrs mirrors the key/value pairs emitted by
// internal/platform/middleware.writeAccessLog.
func accessAttrs(traceID string) []any {
	return []any{
		"request_id", "req-bench",
		"trace_id", traceID,
		"span_id", "span-bench",
		"method", "GET",
		"path", "/api/v1/users/{id}",
		"surface", "management",
		"status", 200,
		"duration_ms", int64(12),
		"bytes", int64(2),
	}
}

func appInfoAttrs() []any {
	return []any{
		"user.id", "user-bench",
		"organization.id", "org-bench",
		"application.id", "app-bench",
		"method", "password",
		"outcome", "success",
		"duration_ms", int64(12),
	}
}

// BenchmarkJSONHandler measures the §12.10 handler for the four record classes
// called out by the E3-b receipt: normal INFO, sampled-in trace, slow request
// and oversized (truncated) records. Sampled-out is included as the
// complementary case because it is the sampling fast path.
func BenchmarkJSONHandler(b *testing.B) {
	sink := &benchSink{}
	logger := newBenchLogger(sink)
	sampledOut := benchTraceID(false)
	sampledIn := benchTraceID(true)
	if sampledOut == "" || sampledIn == "" {
		b.Fatal("no trace id found for both sampling buckets")
	}

	cases := []struct {
		name string
		ctx  context.Context
		args []any
	}{
		{
			// Application INFO must stay at 100% even when its trace would be
			// sampled out; sampled-out is used here on purpose so a regression
			// that starts sampling app logs shows up as %written=0.
			name: "Info",
			ctx: logging.WithRequestIDs(context.Background(),
				logging.RequestIDs{RequestID: "req-bench", TraceID: sampledOut}),
			args: appInfoAttrs(),
		},
		{
			name: "AccessInfoSampledIn",
			ctx: logging.WithAccessLog(logging.WithRequestIDs(context.Background(),
				logging.RequestIDs{RequestID: "req-bench", TraceID: sampledIn}), 12*time.Millisecond),
			args: accessAttrs(sampledIn),
		},
		{
			name: "AccessInfoSampledOut",
			ctx: logging.WithAccessLog(logging.WithRequestIDs(context.Background(),
				logging.RequestIDs{RequestID: "req-bench", TraceID: sampledOut}), 12*time.Millisecond),
			args: accessAttrs(sampledOut),
		},
		{
			name: "AccessInfoSlow",
			ctx: logging.WithAccessLog(logging.WithRequestIDs(context.Background(),
				logging.RequestIDs{RequestID: "req-bench", TraceID: sampledOut}),
				logging.SlowRequestThreshold+10*time.Millisecond),
			args: accessAttrs(sampledOut),
		},
	}

	for _, tc := range cases {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			start := sink.records
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				logger.InfoContext(tc.ctx, "bench", tc.args...)
			}
			b.StopTimer()
			b.ReportMetric(float64(sink.records-start)*100/float64(b.N), "%written")
		})
	}
}

// BenchmarkJSONHandlerTruncate32KB forces renderTruncated: the raw record is
// ~40KB, so the handler re-renders with shortened values until it fits 32KB.
func BenchmarkJSONHandlerTruncate32KB(b *testing.B) {
	sink := &benchSink{}
	logger := newBenchLogger(sink)
	blob := strings.Repeat("x", 40*1024)
	traceID := benchTraceID(true)
	if traceID == "" {
		b.Fatal("no sampled-in trace id found")
	}
	ctx := logging.WithRequestIDs(context.Background(),
		logging.RequestIDs{RequestID: "req-bench", TraceID: traceID})

	start := sink.records
	startBytes := sink.bytes
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.InfoContext(ctx, "oversized payload", "payload", blob, "bytes", len(blob))
	}
	b.StopTimer()
	b.ReportMetric(float64(sink.records-start)*100/float64(b.N), "%written")
	if records := sink.records - start; records > 0 {
		b.ReportMetric(float64(sink.bytes-startBytes)/float64(records), "B/record")
	}
}
