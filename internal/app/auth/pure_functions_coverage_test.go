package auth

import (
	"testing"
)

// ---------------------------------------------------------------------------
// authorize.go pure function tests
// ---------------------------------------------------------------------------

func TestContainsString(t *testing.T) {
	tests := []struct {
		name     string
		values   []string
		expected string
		want     bool
	}{
		{"found", []string{"openid", "profile", "email"}, "profile", true},
		{"not_found", []string{"openid", "profile"}, "admin", false},
		{"empty_slice", []string{}, "openid", false},
		{"nil_slice", nil, "openid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsString(tt.values, tt.expected)
			if got != tt.want {
				t.Errorf("containsString() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// backchannel_logout.go pure function tests
// ---------------------------------------------------------------------------

func TestUrlEncode(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"alphanumeric", "hello123", "hello123"},
		{"unreserved_chars", "a-b.c_d~", "a-b.c_d~"},
		{"space", " ", "%20"},
		{"special_chars", "hello world!", "hello%20world%21"},
		{"equals_and_ampersand", "a=b&c=d", "a%3Db%26c%3Dd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := urlEncode(tt.input)
			if got != tt.want {
				t.Errorf("urlEncode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsUnreservedByte(t *testing.T) {
	tests := []struct {
		byte byte
		want bool
	}{
		{'A', true},
		{'Z', true},
		{'a', true},
		{'z', true},
		{'0', true},
		{'9', true},
		{'-', true},
		{'.', true},
		{'_', true},
		{'~', true},
		{' ', false},
		{'!', false},
		{'@', false},
		{'#', false},
		{'/', false},
	}

	for _, tt := range tests {
		name := string([]byte{tt.byte})
		t.Run(name, func(t *testing.T) {
			got := isUnreservedByte(tt.byte)
			if got != tt.want {
				t.Errorf("isUnreservedByte(%q) = %v, want %v", name, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// anomaly_detection.go pure function tests
// ---------------------------------------------------------------------------

func TestBoolPtr(t *testing.T) {
	val := true
	ptr := boolPtr(val)
	if ptr == nil {
		t.Fatal("boolPtr returned nil")
	}
	if *ptr != val {
		t.Errorf("boolPtr(true) = %v, want %v", *ptr, val)
	}

	val2 := false
	ptr2 := boolPtr(val2)
	if *ptr2 != val2 {
		t.Errorf("boolPtr(false) = %v, want %v", *ptr2, val2)
	}
}
