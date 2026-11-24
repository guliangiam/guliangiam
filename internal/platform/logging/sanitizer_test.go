package logging

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
)

func TestLogSanitizer_MixedKeys(t *testing.T) {
	s := NewLogSanitizer()
	tests := []struct {
		key      string
		value    any
		expected any
	}{
		{"password", "secret", "***"},
		{"username", "alice", "alice"},
		{"access_token", "tok123", "***"},
		{"email", "bob@test.com", "bob@test.com"},
		{"clientSecret", "cs", "***"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := s.SanitizeValue(tt.key, tt.value)
			if result != tt.expected {
				t.Errorf("SanitizeValue(%q) = %v, want %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestLogSanitizer_HyphenAndUnderscoreNormalization(t *testing.T) {
	s := NewLogSanitizer()
	tests := []struct {
		key   string
		value string
	}{
		{"password-hash", "val"},
		{"password_hash", "val"},
		{"PASSWORD_HASH", "val"},
		{"access-secret", "val"},
		{"refresh_token", "val"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := s.SanitizeValue(tt.key, tt.value)
			if result != "***" {
				t.Errorf("SanitizeValue(%q) = %v, want ***", tt.key, result)
			}
		})
	}
}

func TestSanitizingHandler_GroupAttrs(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := NewSanitizingHandler(baseHandler)
	logger := slog.New(handler)

	logger.Info("test", "user", slog.GroupValue(
		slog.String("name", "alice"),
		slog.String("password", "secret123"),
	))

	output := buf.String()
	if bytes.Contains(buf.Bytes(), []byte("secret123")) {
		t.Errorf("expected password in group to be sanitized, got: %s", output)
	}
	if !bytes.Contains(buf.Bytes(), []byte("alice")) {
		t.Errorf("expected name in group to be preserved, got: %s", output)
	}
}

func TestSanitizingHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := NewSanitizingHandler(baseHandler)

	childHandler := handler.WithAttrs([]slog.Attr{
		slog.String("password", "my-secret"),
		slog.String("user", "alice"),
	})
	logger := slog.New(childHandler)

	logger.Info("test message")

	output := buf.String()
	if bytes.Contains(buf.Bytes(), []byte("my-secret")) {
		t.Errorf("expected password to be sanitized in WithAttrs, got: %s", output)
	}
	if !bytes.Contains(buf.Bytes(), []byte("alice")) {
		t.Errorf("expected username to be preserved, got: %s", output)
	}
}

func TestSanitizingHandler_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := NewSanitizingHandler(baseHandler)

	childHandler := handler.WithGroup("auth")
	logger := slog.New(childHandler)

	logger.Info("test", "password", "secret123")

	output := buf.String()
	if bytes.Contains(buf.Bytes(), []byte("secret123")) {
		t.Errorf("expected password in group to be sanitized, got: %s", output)
	}
}

func TestSanitizingHandler_MultipleSensitiveFields(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := NewSanitizingHandler(baseHandler)
	logger := slog.New(handler)

	logger.Info("login",
		"password", "pass123",
		"accessToken", "at123",
		"refreshToken", "rt123",
		"clientSecret", "cs123",
		"username", "alice",
	)

	output := buf.String()
	for _, secret := range []string{"pass123", "at123", "rt123", "cs123"} {
		if bytes.Contains(buf.Bytes(), []byte(secret)) {
			t.Errorf("expected %q to be sanitized, got: %s", secret, output)
		}
	}
	if !bytes.Contains(buf.Bytes(), []byte("alice")) {
		t.Errorf("expected username to be preserved, got: %s", output)
	}
}

func TestSanitizingHandler_EnabledDelegates(t *testing.T) {
	tests := []struct {
		name  string
		level slog.Level
		want  bool
	}{
		{"info enabled", slog.LevelInfo, true},
		{"warn enabled", slog.LevelWarn, true},
		{"debug disabled", slog.LevelDebug, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseHandler := slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelInfo})
			handler := NewSanitizingHandler(baseHandler)
			got := handler.Enabled(context.Background(), tt.level)
			if got != tt.want {
				t.Errorf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
