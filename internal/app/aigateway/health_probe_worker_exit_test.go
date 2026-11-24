package aigateway

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newExitTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestHealthProbeWorker_Start_LogsNonCancelError(t *testing.T) {
	var buf bytes.Buffer
	w := &HealthProbeWorker{
		Interval: time.Hour,
		logger:   newExitTestLogger(&buf),
		runFn:    func(context.Context) error { return errors.New("probe loop exploded") },
	}
	w.Start(context.Background())
	defer w.Stop()

	require.Eventually(t, func() bool {
		return strings.Contains(buf.String(), "stopped with error") &&
			strings.Contains(buf.String(), "probe loop exploded")
	}, 2*time.Second, 10*time.Millisecond)
}

func TestHealthProbeWorker_Start_ContextCancelIsNotAnError(t *testing.T) {
	var buf bytes.Buffer
	w := &HealthProbeWorker{
		Interval: time.Hour,
		logger:   newExitTestLogger(&buf),
		runFn:    func(context.Context) error { return context.Canceled },
	}
	w.Start(context.Background())
	defer w.Stop()

	require.Eventually(t, func() bool {
		return strings.Contains(buf.String(), "loop exited")
	}, 2*time.Second, 10*time.Millisecond)
	require.NotContains(t, buf.String(), "stopped with error")
}
