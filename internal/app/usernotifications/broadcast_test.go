package usernotifications

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestDedupeUserIDs(t *testing.T) {
	got := dedupeUserIDs([]string{" u1 ", "u2", "u1", "", "  "})
	if len(got) != 2 || got[0] != "u1" || got[1] != "u2" {
		t.Fatalf("dedupeUserIDs = %#v, want [u1 u2]", got)
	}
}

func TestBroadcastRejectsInvalidInput(t *testing.T) {
	service := Service{}
	ctx := context.Background()

	if _, err := service.Broadcast(ctx, BroadcastCommand{UserIDs: []string{"u1"}}); err == nil {
		t.Fatal("empty title must be rejected")
	}
	if _, err := service.Broadcast(ctx, BroadcastCommand{Title: "t", Category: "nope", UserIDs: []string{"u1"}}); err == nil {
		t.Fatal("unknown category must be rejected")
	}
	if _, err := service.Broadcast(ctx, BroadcastCommand{Title: "t", Category: "system"}); err == nil {
		t.Fatal("empty recipients must be rejected")
	}
	overLimit := make([]string, broadcastMaxRecipients+1)
	for i := range overLimit {
		overLimit[i] = fmt.Sprintf("user-%d", i)
	}
	if _, err := service.Broadcast(ctx, BroadcastCommand{
		Title:    "t",
		Category: "system",
		UserIDs:  overLimit,
	}); err == nil || !strings.Contains(err.Error(), "too many recipients") {
		t.Fatalf("over-limit recipients error = %v, want too many recipients", err)
	}
}

func TestIsKnownCategory(t *testing.T) {
	for _, category := range []string{"system", "security", "business", "feedback", "announcement"} {
		if !isKnownCategory(category) {
			t.Fatalf("%q must be a known category", category)
		}
	}
	if isKnownCategory("other") {
		t.Fatal("unknown category accepted")
	}
}
