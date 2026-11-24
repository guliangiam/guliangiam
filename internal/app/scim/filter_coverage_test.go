package scim

import (
	"testing"
)

func TestIsWordChar(t *testing.T) {
	tests := []struct {
		ch   byte
		want bool
	}{
		{'a', true},
		{'Z', true},
		{'0', true},
		{'_', true},
		{'-', true},
		{' ', false},
		{'(', false},
		{')', false},
		{'.', false},
	}
	for _, tt := range tests {
		if got := isWordChar(tt.ch); got != tt.want {
			t.Errorf("isWordChar(%q) = %v, want %v", string(tt.ch), got, tt.want)
		}
	}
}

func TestStringToFilterOp_Coverage(t *testing.T) {
	tests := []struct {
		s      string
		wantOk bool
	}{
		{"eq", true},
		{"ne", true},
		{"co", true},
		{"sw", true},
		{"ew", true},

		{"gt", true},
		{"lt", true},
		{"ge", true},
		{"le", true},
		{"unknown", false},
	}
	for _, tt := range tests {
		op, ok := stringToFilterOp(tt.s)
		if ok != tt.wantOk {
			t.Errorf("stringToFilterOp(%q) = (%v, %v), want ok=%v", tt.s, op, ok, tt.wantOk)
		}
	}
}

func TestParseFilter_ComplexAnd(t *testing.T) {
	node, err := ParseFilter(`userName eq "john" and active eq true`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterAnd {
		t.Errorf("expected FilterAnd, got %v", node.Op)
	}
}

func TestParseFilter_ComplexOr(t *testing.T) {
	node, err := ParseFilter(`userName eq "john" or displayName eq "John"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterOr {
		t.Errorf("expected FilterOr, got %v", node.Op)
	}
}

func TestParseFilter_Grouped(t *testing.T) {
	node, err := ParseFilter(`(userName eq "john") and (active eq true)`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Op != FilterAnd {
		t.Errorf("expected FilterAnd, got %v", node.Op)
	}
}

func TestTokenize(t *testing.T) {
	tokens := tokenize(`userName eq "john"`)
	if len(tokens) == 0 {
		t.Error("expected non-empty tokens")
	}
}

func TestTokenize_Complex(t *testing.T) {
	tokens := tokenize(`(userName eq "john" or active eq true) and displayName sw "J"`)
	if len(tokens) < 5 {
		t.Errorf("expected at least 5 tokens, got %d", len(tokens))
	}
}
