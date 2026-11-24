package usernotifications

import "testing"

func TestPayloadStringFormatsNumbers(t *testing.T) {
	payload := decodePayload([]byte(`{"api_key_id":"key-1","current_value":1000,"limit_type":"tpm"}`))
	if payload == nil {
		t.Fatal("payload must decode")
	}
	if got := payloadString(payload, "api_key_id"); got != "key-1" {
		t.Fatalf("string payload = %q, want key-1", got)
	}
	// JSON 数字统一为 float64；整数值必须去掉小数尾巴。
	if got := payloadString(payload, "current_value"); got != "1000" {
		t.Fatalf("integer payload = %q, want 1000", got)
	}
	if got := payloadString(payload, "missing"); got != "-" {
		t.Fatalf("missing payload = %q, want -", got)
	}
	if got := payloadString(decodePayload([]byte("not-json")), "any"); got != "-" {
		t.Fatalf("invalid payload = %q, want -", got)
	}
}
