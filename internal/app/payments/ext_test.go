package payments

import (
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
)

// ReceiptNormalized + nowPtr — 两个 0% helper

func TestReceiptNormalized(t *testing.T) {
	p := &CallbackPipeline{}
	receipt := &domain.CallbackReceipt{
		RawEvent: map[string]any{"key": "value", "n": float64(1)},
	}
	s, err := p.ReceiptNormalized(receipt)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if s == "" {
		t.Fatal("expected non-empty JSON")
	}
}

func TestNowPtr(t *testing.T) {
	tm := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	got := nowPtr(tm)
	if got == nil || !got.Equal(tm) {
		t.Errorf("expected pointer to %v, got %v", tm, got)
	}
}
