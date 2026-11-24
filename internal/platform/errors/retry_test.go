package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestWithRetryablePreservesMeta(t *testing.T) {
	err := NewWithMeta(CodeUnavailable, "upstream down", map[string]any{"component": "channel"})
	annotated := WithRetryable(err, true)

	if retryable, ok := IsRetryable(annotated); !ok || !retryable {
		t.Fatalf("IsRetryable = (%v, %v), want (true, true)", retryable, ok)
	}
	if annotated.Meta["component"] != "channel" || annotated.Code != CodeUnavailable {
		t.Fatalf("WithRetryable mutated the error: %+v", annotated)
	}
}

func TestIsRetryableRequiresExplicitMarker(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
		ok        bool
	}{
		{name: "nil"},
		{name: "plain error", err: errors.New("boom")},
		{name: "no meta", err: NewUnavailable("down")},
		{name: "explicit false", err: WithRetryable(NewInvalidArgument("bad"), false), ok: true},
		{name: "explicit true", err: WithRetryable(NewUnavailable("down"), true), retryable: true, ok: true},
		{name: "wrapped", err: fmt.Errorf("outer: %w", WithRetryable(NewUnavailable("down"), true)), retryable: true, ok: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retryable, ok := IsRetryable(tt.err)
			if ok != tt.ok || retryable != tt.retryable {
				t.Fatalf("IsRetryable = (%v, %v), want (%v, %v)", retryable, ok, tt.retryable, tt.ok)
			}
		})
	}
}

func TestRetryAfterSecondsRoundTrip(t *testing.T) {
	err := WithRetryAfterSeconds(NewTooManyRequests("limited"), 12)
	for _, target := range []error{err, fmt.Errorf("outer: %w", err)} {
		if seconds, ok := RetryAfterSeconds(target); !ok || seconds != 12 {
			t.Fatalf("RetryAfterSeconds = (%d, %v), want (12, true)", seconds, ok)
		}
	}
	// Non-positive values are rejected at the producer, never written back.
	for _, seconds := range []int{0, -5} {
		if _, ok := RetryAfterSeconds(WithRetryAfterSeconds(NewTooManyRequests("limited"), seconds)); ok {
			t.Fatalf("seconds=%d must not be recorded", seconds)
		}
	}
	// Go duration strings are not a valid Retry-After representation.
	if _, ok := RetryAfterSeconds(NewWithMeta(CodeTooManyRequests, "limited", map[string]any{MetaRetryAfterSeconds: "1s"})); ok {
		t.Fatal("duration strings must be rejected")
	}
}
