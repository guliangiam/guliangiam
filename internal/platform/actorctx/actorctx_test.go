package actorctx

import (
	"context"
	"testing"
)

func TestWithActorID_RoundTrip(t *testing.T) {
	ctx := WithActorID(context.Background(), "user-1")
	if got := FromContext(ctx); got != "user-1" {
		t.Fatalf("FromContext = %q, want user-1", got)
	}
}

func TestWithActorID_EmptyDoesNotInject(t *testing.T) {
	ctx := WithActorID(context.Background(), "")
	if got := FromContext(ctx); got != "" {
		t.Fatalf("FromContext = %q, want empty", got)
	}
}

func TestFromContext_Missing(t *testing.T) {
	if got := FromContext(context.Background()); got != "" {
		t.Fatalf("FromContext = %q, want empty", got)
	}
}

func TestWithActorID_Overwrites(t *testing.T) {
	ctx := WithActorID(WithActorID(context.Background(), "user-1"), "user-2")
	if got := FromContext(ctx); got != "user-2" {
		t.Fatalf("FromContext = %q, want user-2", got)
	}
}
