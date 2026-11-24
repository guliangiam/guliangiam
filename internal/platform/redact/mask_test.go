package redact

import "testing"

func TestMaskRecipient_Email(t *testing.T) {
	tests := []struct {
		name      string
		recipient string
		want      string
	}{
		{"ordinary", "john.doe@example.com", "j*******@example.com"},
		{"single local rune", "a@b.com", "*@b.com"},
		{"plus tag", "user.name+tag@example.com", "u************@example.com"},
		{"unicode local", "张三@example.com", "张*@example.com"},
		{"trimmed", "  john@example.com  ", "j***@example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaskRecipient(tt.recipient); got != tt.want {
				t.Errorf("MaskRecipient(%q) = %q, want %q", tt.recipient, got, tt.want)
			}
		})
	}
}

func TestMaskRecipient_Phone(t *testing.T) {
	tests := []struct {
		name      string
		recipient string
		want      string
	}{
		{"mobile", "13812345678", "138****5678"},
		{"e164", "+8613912345678", "+86*******5678"},
		{"spaced", "138 1234 5678", "138******5678"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaskRecipient(tt.recipient); got != tt.want {
				t.Errorf("MaskRecipient(%q) = %q, want %q", tt.recipient, got, tt.want)
			}
		})
	}
}

func TestMaskRecipient_UnknownShapeBecomesPlaceholder(t *testing.T) {
	for _, recipient := range []string{
		"",
		"@example.com",
		"user@",
		"telegram:1234567",
		"12345",
		"aGVsbG8gdGhlcmU",
	} {
		if got := MaskRecipient(recipient); got != Placeholder && recipient != "" {
			t.Errorf("MaskRecipient(%q) = %q, want %q", recipient, got, Placeholder)
		}
	}
}

func TestMaskRecipient_EmptyStaysEmpty(t *testing.T) {
	if got := MaskRecipient(""); got != "" {
		t.Fatalf("MaskRecipient(\"\") = %q, want empty", got)
	}
}

func TestMaskRecipient_NeverEchoesFullRecipient(t *testing.T) {
	for _, recipient := range []string{
		"jane.roberts@corp.example.com",
		"+15551234567",
		"15551234567",
	} {
		got := MaskRecipient(recipient)
		if got == recipient {
			t.Errorf("MaskRecipient(%q) returned the input unchanged", recipient)
		}
	}
}
