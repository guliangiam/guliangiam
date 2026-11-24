package faceid

import (
	"context"
	"testing"
)

func TestFakeVerifier_VerifyLiveness(t *testing.T) {
	v := NewFakeVerifier()
	result, err := v.VerifyLiveness(context.Background(), "img", "device")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsLive {
		t.Error("expected IsLive=true")
	}
	if result.Score != 0.99 {
		t.Errorf("expected Score=0.99, got %f", result.Score)
	}
	if result.SessionID != "fake-session-id" {
		t.Errorf("expected SessionID=fake-session-id, got %s", result.SessionID)
	}
}

func TestFakeVerifier_CompareFace(t *testing.T) {
	v := NewFakeVerifier()
	result, err := v.CompareFace(context.Background(), "img", "template")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsMatch {
		t.Error("expected IsMatch=true")
	}
	if result.Confidence != 0.95 {
		t.Errorf("expected Confidence=0.95, got %f", result.Confidence)
	}
}
