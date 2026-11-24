package notification

import (
	"testing"
)

func TestFormatGoogleChatMessage(t *testing.T) {
	n := &Notification{Subject: "Alert", Content: "Something happened"}
	result := formatGoogleChatMessage(n)
	expected := "*Alert*\nSomething happened"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}

	n2 := &Notification{Subject: "", Content: "Just content"}
	result2 := formatGoogleChatMessage(n2)
	if result2 != "Just content" {
		t.Errorf("got %q, want %q", result2, "Just content")
	}
}
