package payment

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestMapPolarStatus(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"paid", "paid"},
		{"PAID", "paid"},
		{"succeeded", "paid"},
		{"delivered", "paid"},
		{"failed", "failed"},
		{"canceled", "failed"},
		{"refunded", "refunded"},
		{"pending", "pending"},
		{"", "pending"},
	}
	for _, tt := range tests {
		if got := mapPolarStatus(tt.input); got != tt.want {
			t.Errorf("mapPolarStatus(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMapPaddleStatus(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"completed", "paid"},
		{"COMPLETED", "paid"},
		{"paid", "paid"},
		{"ready", "paid"},
		{"canceled", "failed"},
		{"failed", "failed"},
		{"refunded", "refunded"},
		{"pending", "pending"},
		{"", "pending"},
	}
	for _, tt := range tests {
		if got := mapPaddleStatus(tt.input); got != tt.want {
			t.Errorf("mapPaddleStatus(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMapPaddleRefundStatus(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"approved", "success"},
		{"APPROVED", "success"},
		{"completed", "success"},
		{"declined", "failed"},
		{"failed", "failed"},
		{"pending", "processing"},
		{"", "processing"},
	}
	for _, tt := range tests {
		if got := mapPaddleRefundStatus(tt.input); got != tt.want {
			t.Errorf("mapPaddleRefundStatus(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestVerifyPaddleSignature(t *testing.T) {
	secret := "test-secret"
	ts := "1700000000"
	payload := ts + ":"
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	h1 := hex.EncodeToString(mac.Sum(nil))
	sig := "ts=" + ts + ";h1=" + h1

	if !verifyPaddleSignature(sig, secret) {
		t.Error("expected valid signature")
	}
	if verifyPaddleSignature("ts=;h1=", secret) {
		t.Error("expected failure for empty ts/h1")
	}
	if verifyPaddleSignature("invalid", secret) {
		t.Error("expected failure for malformed signature")
	}
	if verifyPaddleSignature("ts=1700000000;h1=wrong", secret) {
		t.Error("expected failure for wrong h1")
	}
}

func TestMapWeChatTradeState(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"SUCCESS", "paid"},
		{"REFUND", "refunded"},
		{"NOTPAY", "pending"},
		{"CLOSED", "failed"},
		{"REVOKED", "failed"},
		{"USERPAYING", "pending"},
		{"PAYERROR", "failed"},
		{"UNKNOWN", "pending"},
		{"", "pending"},
	}
	for _, tt := range tests {
		if got := mapWeChatTradeState(tt.input); got != tt.want {
			t.Errorf("mapWeChatTradeState(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
