package sentry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigFromEnv(t *testing.T) {
	cfg := ConfigFromEnv()
	assert.NotNil(t, cfg)
}

func TestInit_EmptyDSN(t *testing.T) {
	// Should be a no-op when DSN is empty
	Init(Config{DSN: ""})
}

func TestInit_WithEnvironment(t *testing.T) {
	// Should use provided environment
	Init(Config{DSN: "", Environment: "test"})
}

func TestFlush(t *testing.T) {
	// Should not panic
	Flush(0)
}
