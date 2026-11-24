package payment

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestMapAdyenStatus(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"AUTHORISED", "paid"},
		{"authorised", "paid"},
		{"REFUSED", "failed"},
		{"ERROR", "failed"},
		{"CANCELLED", "failed"},
		{"REFUNDED", "refunded"},
		{"PENDING", "pending"},
		{"UNKNOWN", "pending"},
		{"", "pending"},
	}
	for _, tt := range tests {
		if got := mapAdyenStatus(tt.input); got != tt.want {
			t.Errorf("mapAdyenStatus(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBuildAdyenSignData(t *testing.T) {
	params := map[string]string{
		"pspReference":        "PSP123",
		"originalReference":   "ORIG456",
		"merchantAccountCode": "MAC789",
		"merchantReference":   "MR001",
		"value":               "1000",
		"currency":            "EUR",
		"eventCode":           "AUTHORISATION",
		"success":             "true",
	}
	result := buildAdyenSignData(params)
	expected := "PSP123ORIG456MAC789MR0011000EURAUTHORISATIONtrue"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestBuildAdyenSignData_MissingKeys(t *testing.T) {
	params := map[string]string{"pspReference": "PSP123"}
	result := buildAdyenSignData(params)
	if result != "PSP123" {
		t.Errorf("expected partial result, got %q", result)
	}
}

func TestVerifyAdyenHMAC(t *testing.T) {
	data := "test-data"
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	mac := hmac.New(sha256.New, func() []byte {
		b, _ := hex.DecodeString(key)
		return b
	}())
	mac.Write([]byte(data))
	validSig := hex.EncodeToString(mac.Sum(nil))

	if !verifyAdyenHMAC(data, validSig, key) {
		t.Error("expected HMAC verification to succeed")
	}
	if verifyAdyenHMAC(data, "invalid-signature", key) {
		t.Error("expected HMAC verification to fail with wrong signature")
	}
	if verifyAdyenHMAC(data, validSig, "invalid-hex-key") {
		t.Error("expected HMAC verification to fail with invalid key")
	}
}

func TestMapAirwallexStatus(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"SUCCEEDED", "paid"},
		{"succeeded", "paid"},
		{"AUTHENTICATED", "paid"},
		{"FAILED", "failed"},
		{"CANCELLED", "failed"},
		{"REFUNDED", "refunded"},
		{"PENDING", "pending"},
		{"", "pending"},
	}
	for _, tt := range tests {
		if got := mapAirwallexStatus(tt.input); got != tt.want {
			t.Errorf("mapAirwallexStatus(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
