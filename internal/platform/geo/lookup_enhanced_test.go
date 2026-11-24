package geo

import (
	"testing"
)

func TestIsNewCountry_Disabled(t *testing.T) {
	geo := &LookupService{}
	if IsNewCountry(geo, "8.8.8.8", []string{"US"}) {
		t.Error("expected false when lookup is disabled")
	}
}

func TestExtractCountriesFromIPs_Disabled(t *testing.T) {
	geo := &LookupService{}
	result := ExtractCountriesFromIPs(geo, []string{"8.8.8.8", "1.1.1.1"})
	if len(result) != 0 {
		t.Errorf("expected empty result when disabled, got %v", result)
	}
}

func TestLookupService_String(t *testing.T) {
	s := &LookupService{}
	if s.String() != "LookupService(disabled)" {
		t.Errorf("unexpected: %s", s.String())
	}
}

func TestLookup_Disabled(t *testing.T) {
	s := &LookupService{}
	if s.Lookup("8.8.8.8") != nil {
		t.Error("expected nil when disabled")
	}
}

func TestLookupCountry_Disabled(t *testing.T) {
	s := &LookupService{}
	if s.LookupCountry("8.8.8.8") != "" {
		t.Error("expected empty when disabled")
	}
}

func TestLookup_InvalidIP(t *testing.T) {
	s := &LookupService{}
	if s.Lookup("not-an-ip") != nil {
		t.Error("expected nil for invalid IP")
	}
}

func TestClose_Disabled(t *testing.T) {
	s := &LookupService{}
	s.Close()
}
