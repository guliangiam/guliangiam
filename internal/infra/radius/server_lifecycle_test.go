package radius

import (
	"context"
	"testing"
	"time"
)

// The transport layer binds SetAcctHandler through this structural interface
// (BE-B-13). AccountingHandler must stay a type ALIAS so a plain func literal
// satisfies it without the transport importing infra.
var _ interface {
	SetAcctHandler(func(ctx context.Context, username, sessionID, eventType string, bytesIn, bytesOut int64))
} = (*Server)(nil)

// BE-B-13 regression: Start honours the lifecycle context — cancelling it shuts
// the server down (observed via the internal done channel).
func TestServer_Start_LifecycleContextCancellation(t *testing.T) {
	s := NewServer("127.0.0.1:0", "secret")
	ctx, cancel := context.WithCancel(context.Background())
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	cancel()
	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
		t.Fatal("lifecycle context cancellation did not stop the server")
	}

	// Stop remains idempotent after a context-driven shutdown.
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop after cancellation: %v", err)
	}
}

// An explicit Stop also releases the lifecycle watcher (no goroutine leak).
func TestServer_Stop_ReleasesLifecycleWatcher(t *testing.T) {
	s := NewServer("127.0.0.1:0", "secret")
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not close the lifecycle done channel")
	}
}
