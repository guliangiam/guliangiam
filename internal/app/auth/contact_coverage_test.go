package auth

import (
	"testing"
)

func TestMaskEmail_StandardEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  string
	}{
		{"standard_email", "johndoe@gmail.com", "j******@gmail.com"},
		{"short_local", "a@example.com", "*@example.com"},
		{"two_char_local", "ab@example.com", "a*@example.com"},
		{"long_local", "verylongemail@company.org", "v************@company.org"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskEmail(tt.email)
			if got != tt.want {
				t.Errorf("maskEmail(%q) = %q, want %q", tt.email, got, tt.want)
			}
		})
	}
}

func TestMaskEmail_NoAtSign(t *testing.T) {
	got := maskEmail("noemail")
	if got != "***" {
		t.Errorf("maskEmail(%q) = %q, want %q", "noemail", got, "***")
	}
}

func TestMaskEmail_EmptyString(t *testing.T) {
	got := maskEmail("")
	if got != "***" {
		t.Errorf("maskEmail(%q) = %q, want %q", "", got, "***")
	}
}

func TestMaskEmail_SingleCharLocal(t *testing.T) {
	got := maskEmail("x@test.com")
	if got != "*@test.com" {
		t.Errorf("maskEmail(%q) = %q, want %q", "x@test.com", got, "*@test.com")
	}
}

func TestMaskPhone_StandardPhone(t *testing.T) {
	tests := []struct {
		name  string
		phone string
		want  string
	}{
		{"chinese_mobile", "13812345678", "138****5678"},
		{"us_phone", "12345678901", "123****8901"},
		{"long_number", "1381234567890", "138******7890"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskPhone(tt.phone)
			if got != tt.want {
				t.Errorf("maskPhone(%q) = %q, want %q", tt.phone, got, tt.want)
			}
		})
	}
}

func TestMaskPhone_ShortPhone(t *testing.T) {
	tests := []struct {
		name  string
		phone string
		want  string
	}{
		{"7_digits", "1234567", "*******"},
		{"5_digits", "12345", "*****"},
		{"1_digit", "1", "*"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskPhone(tt.phone)
			if got != tt.want {
				t.Errorf("maskPhone(%q) = %q, want %q", tt.phone, got, tt.want)
			}
		})
	}
}

func TestMaskPhone_Exactly8Digits(t *testing.T) {
	// 8 digits: first 3 + mask 1 + last 4
	got := maskPhone("12345678")
	if got != "123*5678" {
		t.Errorf("maskPhone(%q) = %q, want %q", "12345678", got, "123*5678")
	}
}
