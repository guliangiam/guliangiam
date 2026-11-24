package phonefmt

import (
	"errors"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestToE164(t *testing.T) {
	tests := []struct {
		name        string
		number      string
		countryHint string
		want        string
		wantErr     bool
	}{
		{name: "already_e164", number: "+8613800138000", countryHint: "CN", want: "+8613800138000"},
		{name: "local_with_iso", number: "13800138000", countryHint: "CN", wantErr: true},
		{name: "local_with_legacy_dial", number: "13800138000", countryHint: "86", want: "+8613800138000"},
		{name: "local_with_plus_dial", number: "13800138000", countryHint: "+86", want: "+8613800138000"},
		{name: "local_without_hint", number: "13800138000", wantErr: true},
		{name: "spaced_number", number: "+86 138 0013 8000", want: "+8613800138000"},
		{name: "local_with_spaces", number: "138 0013 8000", countryHint: "86", want: "+8613800138000"},
		{name: "too_short", number: "123", countryHint: "86", wantErr: true},
		{name: "malformed_plus", number: "+86abc", wantErr: true},
		{name: "empty", number: "", countryHint: "86", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToE164(tt.number, tt.countryHint)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				var appErr *apperrors.Error
				if !errors.As(err, &appErr) {
					t.Fatalf("expected *apperrors.Error, got %T", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsISOCountryCode(t *testing.T) {
	if !IsISOCountryCode("CN") || !IsISOCountryCode("US") {
		t.Fatal("expected uppercase alpha-2 codes to be accepted")
	}
	for _, value := range []string{"cn", "+86", "86", "CHN", ""} {
		if IsISOCountryCode(value) {
			t.Fatalf("%q must not be accepted as ISO alpha-2", value)
		}
	}
}

func TestValidateCountryCode(t *testing.T) {
	if err := ValidateCountryCode("", "profile.countryCode"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCountryCode("CN", "profile.countryCode"); err != nil {
		t.Fatal(err)
	}
	err := ValidateCountryCode("+86", "profile.countryCode")
	if err == nil {
		t.Fatal("expected error")
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr.Meta["format"] != "iso3166-alpha2" {
		t.Fatalf("error = %v, want format meta", err)
	}
}

func TestValidatePhone(t *testing.T) {
	for _, value := range []string{"", "+8613800138000", "13800138000", "138 0013 8000"} {
		if err := ValidatePhone(value, "profile.phone"); err != nil {
			t.Fatalf("ValidatePhone(%q) = %v", value, err)
		}
	}
	if err := ValidatePhone("CN13800138000", "profile.phone"); err == nil {
		t.Fatal("expected error")
	}
}
