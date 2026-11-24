package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/platform/logging"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

func newTestLogger(t *testing.T) (*slog.Logger, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}
	return slog.New(logging.NewJSONHandler(buf, logging.Options{
		ServiceVersion: "test-sha",
		Environment:    "test",
		Level:          slog.LevelDebug,
	})), buf
}

func decodeSingleLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	line := strings.TrimSpace(buf.String())
	require.NotEmpty(t, line)
	require.NotContains(t, line, "\n", "a log record must be a single JSON line")
	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(line), &entry))
	return entry
}

func TestJSONHandlerInjectsResourceAttributesAndCaller(t *testing.T) {
	logger, buf := newTestLogger(t)
	logger.Info("hello")

	entry := decodeSingleLine(t, buf)
	assert.Equal(t, "guliangiam", entry["service.name"])
	assert.Equal(t, "test-sha", entry["service.version"])
	assert.Equal(t, "test", entry["env"])
	caller, _ := entry["caller"].(string)
	assert.NotEmpty(t, caller)
	assert.NotEqual(t, "unknown", caller)
}

func TestJSONHandlerInjectsCorrelationIDs(t *testing.T) {
	logger, buf := newTestLogger(t)
	ctx := logging.WithRequestIDs(context.Background(), logging.RequestIDs{
		RequestID: "req-e3b",
		TraceID:   "4bf92f3577b34da6a3ce929d0e0e4736",
		SpanID:    "00f067aa0ba902b7",
	})
	logger.InfoContext(ctx, "correlated")

	entry := decodeSingleLine(t, buf)
	assert.Equal(t, "req-e3b", entry["request_id"])
	assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", entry["trace_id"])
	assert.Equal(t, "00f067aa0ba902b7", entry["span_id"])
}

func TestKeepInfoSampleIsStablePerTrace(t *testing.T) {
	for i := 0; i < 100; i++ {
		traceID := fmt.Sprintf("trace-%d", i)
		want := logging.KeepInfoSample(traceID)
		for j := 0; j < 5; j++ {
			require.Equal(t, want, logging.KeepInfoSample(traceID),
				"sampling decision must be stable for trace %q", traceID)
		}
	}
}

func TestKeepInfoSampleRateIsTenPercent(t *testing.T) {
	const total = 10000
	kept := 0
	for i := 0; i < total; i++ {
		if logging.KeepInfoSample(fmt.Sprintf("trace-%d", i)) {
			kept++
		}
	}
	ratio := float64(kept) / float64(total)
	assert.GreaterOrEqual(t, ratio, 0.08, "kept %d/%d", kept, total)
	assert.LessOrEqual(t, ratio, 0.12, "kept %d/%d", kept, total)
}

func TestHandlerSamplingRules(t *testing.T) {
	sampledOut, sampledIn := "", ""
	for i := 0; i < 1000 && (sampledOut == "" || sampledIn == ""); i++ {
		traceID := fmt.Sprintf("trace-%d", i)
		if logging.KeepInfoSample(traceID) {
			sampledIn = traceID
		} else {
			sampledOut = traceID
		}
	}
	require.NotEmpty(t, sampledOut)
	require.NotEmpty(t, sampledIn)

	tests := []struct {
		name     string
		traceID  string
		duration time.Duration
		level    slog.Level
		want     bool
	}{
		{"sampled-out access info", sampledOut, 10 * time.Millisecond, slog.LevelInfo, false},
		{"sampled-in access info", sampledIn, 10 * time.Millisecond, slog.LevelInfo, true},
		{"slow access info is kept", sampledOut, logging.SlowRequestThreshold + time.Millisecond, slog.LevelInfo, true},
		{"access error is kept", sampledOut, 10 * time.Millisecond, slog.LevelError, true},
		{"access warn is kept", sampledOut, 10 * time.Millisecond, slog.LevelWarn, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, buf := newTestLogger(t)
			ctx := logging.WithRequestIDs(context.Background(), logging.RequestIDs{TraceID: tt.traceID})
			ctx = logging.WithAccessLog(ctx, tt.duration)
			logger.Log(ctx, tt.level, "access")
			if tt.want {
				require.NotEmpty(t, buf.String())
				return
			}
			assert.Empty(t, buf.String())
		})
	}
}

func TestHandlerDoesNotSampleApplicationInfo(t *testing.T) {
	sampledOut := ""
	for i := 0; i < 1000; i++ {
		traceID := fmt.Sprintf("trace-%d", i)
		if !logging.KeepInfoSample(traceID) {
			sampledOut = traceID
			break
		}
	}
	require.NotEmpty(t, sampledOut)

	logger, buf := newTestLogger(t)
	ctx := logging.WithRequestIDs(context.Background(), logging.RequestIDs{TraceID: sampledOut})
	logger.InfoContext(ctx, "security event")
	assert.NotEmpty(t, buf.String(), "§12.5 security events are not sampled")
}

func TestJSONHandlerTruncatesOversizedRecord(t *testing.T) {
	logger, buf := newTestLogger(t)
	logger.Info("big", "blob", strings.Repeat("x", 40*1024), slog.Any("error", errors.New("boom")))

	line := strings.TrimSpace(buf.String())
	require.LessOrEqual(t, len(line), logging.MaxRecordBytes)

	entry := decodeSingleLine(t, buf)
	assert.Equal(t, true, entry["log.truncated"])
	original, ok := entry["log.original_bytes"].(float64)
	require.True(t, ok)
	assert.Greater(t, original, float64(logging.MaxRecordBytes))
	assert.Equal(t, "boom", entry["error"])
	assert.Equal(t, "guliangiam", entry["service.name"])
}

func TestJSONHandlerKeepsShortenedErrorValue(t *testing.T) {
	logger, buf := newTestLogger(t)
	logger.Error("big error", slog.Any("error", errors.New(strings.Repeat("e", 40*1024))))

	line := strings.TrimSpace(buf.String())
	require.LessOrEqual(t, len(line), logging.MaxRecordBytes)

	entry := decodeSingleLine(t, buf)
	assert.Equal(t, true, entry["log.truncated"])
	value, ok := entry["error"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, value)
	assert.Less(t, len(value), 40*1024)
}

func TestRequestAuditFieldsIncludesCorrelationIDs(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	request = request.WithContext(logging.WithRequestIDs(request.Context(), logging.RequestIDs{
		RequestID: "req-audit",
		TraceID:   "trace-audit",
	}))

	fields := audit.RequestAuditFields(request, "actor-1", "users")
	require.Equal(t, 0, len(fields)%2, "audit fields must be key/value pairs")
	got := make(map[string]any, len(fields)/2)
	for i := 0; i < len(fields); i += 2 {
		key, ok := fields[i].(string)
		require.True(t, ok)
		got[key] = fields[i+1]
	}

	assert.Equal(t, "req-audit", got["request_id"])
	assert.Equal(t, "trace-audit", got["trace_id"])
	assert.Equal(t, http.MethodGet, got["http.method"])
	assert.Equal(t, "actor-1", got["actor.id"])
	assert.Equal(t, "users", got["audit.resource"])
}
