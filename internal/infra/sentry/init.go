package sentry

import (
	"log/slog"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/guliangiam/guliangiam/internal/platform/config"
)

// Config holds Sentry initialization parameters.
type Config struct {
	DSN         string
	Environment string
	Release     string
}

// ConfigFromEnv builds a Config from well-known environment variables.
func ConfigFromEnv() Config {
	return Config{
		DSN:         config.SentryDSN(),
		Environment: config.SentryEnvironment(),
		Release:     config.SentryRelease(),
	}
}

// Init initializes the global Sentry SDK. If DSN is empty the call is a
// no-op, so the application works fine without a Sentry backend.
func Init(cfg Config) {
	if cfg.DSN == "" {
		slog.Info("sentry: DSN not configured, SDK disabled")
		return
	}

	if cfg.Environment == "" {
		cfg.Environment = "development"
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              cfg.DSN,
		Environment:      cfg.Environment,
		Release:          cfg.Release,
		TracesSampleRate: 0.1,
		// Attach stacktraces to all captured events
		AttachStacktrace: true,
	})
	if err != nil {
		slog.Error("sentry: failed to initialize SDK", "error", err)
		return
	}

	slog.Info("sentry: SDK initialized", "environment", cfg.Environment, "release", cfg.Release)
}

// Flush waits up to the given duration for Sentry events to be sent.
// Call this before process exit (e.g. in a defer after Init).
func Flush(timeout time.Duration) {
	sentry.Flush(timeout)
}
