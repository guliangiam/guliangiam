package geo

import (
	"testing"
)

func TestParseRegionResult(t *testing.T) {
	t.Run("full", func(t *testing.T) {
		r := parseRegionResult("中国|华南|广东省|深圳市|电信")
		if r == nil {
			t.Fatal("nil")
		}
		if r.Country != "中国" || r.Province != "广东省" || r.City != "深圳市" {
			t.Fatalf("unexpected: %+v", r)
		}
	})
	t.Run("with_zero_placeholder", func(t *testing.T) {
		r := parseRegionResult("中国|0|广东省|0|电信")
		if r == nil {
			t.Fatal("nil")
		}
		if r.Region != "" || r.City != "" {
			t.Fatalf("unexpected: %+v", r)
		}
	})
	t.Run("short", func(t *testing.T) {
		r := parseRegionResult("中国|华南")
		if r == nil {
			t.Fatal("nil")
		}
		if r.Country != "中国" || r.Province != "" {
			t.Fatalf("unexpected: %+v", r)
		}
	})
	t.Run("empty", func(t *testing.T) {
		r := parseRegionResult("")
		if r != nil {
			t.Fatalf("expected nil, got %+v", r)
		}
	})
}

func TestCleanField(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"0", ""},
		{" 0 ", ""},
		{"中国", "中国"},
		{"  中国  ", "中国"},
	}
	for _, tt := range tests {
		got := cleanField(tt.input)
		if got != tt.want {
			t.Fatalf("cleanField(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatLocation(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := FormatLocation(nil); got != "" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("country_only", func(t *testing.T) {
		r := &LookupResult{Country: "中国"}
		if got := FormatLocation(r); got != "中国" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("country_and_province", func(t *testing.T) {
		r := &LookupResult{Country: "中国", Province: "广东省", City: "深圳市"}
		if got := FormatLocation(r); got != "中国 广东省 深圳市" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("province_same_as_country", func(t *testing.T) {
		r := &LookupResult{Country: "新加坡", Province: "新加坡", City: ""}
		if got := FormatLocation(r); got != "新加坡" {
			t.Fatalf("got %q", got)
		}
	})
}
