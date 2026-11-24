package payments

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

func newWorkerExitLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestQueryWorker_Start_LogsNonCancelError(t *testing.T) {
	var buf bytes.Buffer
	w := &QueryWorker{
		Interval: time.Hour,
		logger:   newWorkerExitLogger(&buf),
		runFn:    func(context.Context) error { return errors.New("query loop exploded") },
	}
	w.Start(context.Background())
	defer w.Stop()

	require.Eventually(t, func() bool {
		return strings.Contains(buf.String(), "stopped with error") &&
			strings.Contains(buf.String(), "query loop exploded")
	}, 2*time.Second, 10*time.Millisecond)
}

func TestQueryWorker_Start_ContextCancelIsNotAnError(t *testing.T) {
	var buf bytes.Buffer
	w := &QueryWorker{
		Interval: time.Hour,
		logger:   newWorkerExitLogger(&buf),
		runFn:    func(context.Context) error { return context.Canceled },
	}
	w.Start(context.Background())
	defer w.Stop()

	require.Eventually(t, func() bool {
		return strings.Contains(buf.String(), "loop exited")
	}, 2*time.Second, 10*time.Millisecond)
	require.NotContains(t, buf.String(), "stopped with error")
}

func TestReconcileWorker_Start_LogsNonCancelError(t *testing.T) {
	var buf bytes.Buffer
	w := &ReconcileWorker{
		Interval: time.Hour,
		logger:   newWorkerExitLogger(&buf),
		runFn:    func(context.Context) error { return errors.New("reconcile loop exploded") },
	}
	w.Start(context.Background())
	defer w.Stop()

	require.Eventually(t, func() bool {
		return strings.Contains(buf.String(), "stopped with error") &&
			strings.Contains(buf.String(), "reconcile loop exploded")
	}, 2*time.Second, 10*time.Millisecond)
}

func TestReconcileWorker_Start_ContextCancelIsNotAnError(t *testing.T) {
	var buf bytes.Buffer
	w := &ReconcileWorker{
		Interval: time.Hour,
		logger:   newWorkerExitLogger(&buf),
		runFn:    func(context.Context) error { return context.DeadlineExceeded },
	}
	w.Start(context.Background())
	defer w.Stop()

	require.Eventually(t, func() bool {
		return strings.Contains(buf.String(), "loop exited")
	}, 2*time.Second, 10*time.Millisecond)
	require.NotContains(t, buf.String(), "stopped with error")
}
