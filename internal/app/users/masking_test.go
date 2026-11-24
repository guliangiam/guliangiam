package users

import (
	"testing"
)

func TestGetMaskedEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  string
	}{
		{"empty", "", ""},
		{"no at sign", "abcdefgh", "a******h"},
		{"simple email", "alice@example.com", "a***e@e*****e.com"},
		{"short username", "ab@example.com", "ab@e*****e.com"},
		{"subdomain", "user@mail.example.co.uk", "u**r@mail.example.co.uk"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getMaskedEmail(tt.email)
			if got != tt.want {
				t.Errorf("getMaskedEmail(%q) = %q, want %q", tt.email, got, tt.want)
			}
		})
	}
}

func TestGetMaskedPhone(t *testing.T) {
	tests := []struct {
		name  string
		phone string
		want  string
	}{
		{"standard", "13812345678", "138****5678"},
		{"short", "1234", "1234"},
		{"international", "+8613812345678", "+861****5678"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getMaskedPhone(tt.phone)
			if got != tt.want {
				t.Errorf("getMaskedPhone(%q) = %q, want %q", tt.phone, got, tt.want)
			}
		})
	}
}

func TestMaskString(t *testing.T) {
	tests := []struct {
		name string
		str  string
		want string
	}{
		{"empty", "", ""},
		{"single", "a", "a"},
		{"two", "ab", "ab"},
		{"three", "abc", "a*c"},
		{"long", "abcdef", "a****f"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskString(tt.str)
			if got != tt.want {
				t.Errorf("maskString(%q) = %q, want %q", tt.str, got, tt.want)
			}
		})
	}
}
