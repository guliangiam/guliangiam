package phonefmt

import "testing"

func TestCallingCodeKnownCountries(t *testing.T) {
	cases := map[string]string{
		"CN": "86",
		"US": "1",
		"GB": "44",
		"DE": "49",
		"JP": "81",
		"RU": "7",
		"KZ": "7", // shared +7
		"IT": "39",
		"VA": "39", // shared +39
		"AC": "247",
		"TA": "290",
		"XK": "383",
	}
	for iso, want := range cases {
		got, ok := CallingCode(iso)
		if !ok {
			t.Fatalf("CallingCode(%q) = not found", iso)
		}
		if got != want {
			t.Fatalf("CallingCode(%q) = %q, want %q", iso, got, want)
		}
	}
}

func TestCallingCodeNormalizesISO(t *testing.T) {
	for _, input := range []string{"cn", " CN ", "Cn"} {
		got, ok := CallingCode(input)
		if !ok || got != "86" {
			t.Fatalf("CallingCode(%q) = %q, %v; want 86, true", input, got, ok)
		}
	}
}

func TestCallingCodeRejectsNonISO(t *testing.T) {
	for _, input := range []string{"", "ZZ", "+86", "86", "CHN"} {
		if got, ok := CallingCode(input); ok {
			t.Fatalf("CallingCode(%q) = %q; want not found", input, got)
		}
	}
}

// The table must stay at parity with organization_settings.country_codes
// (245 = libphonenumber-js getCountries()). A drift here means the server
// cannot resolve a calling code for a country the UI offers.
func TestCallingCodeTableCoverage(t *testing.T) {
	if len(callingCodes) != 245 {
		t.Fatalf("callingCodes has %d entries, want 245", len(callingCodes))
	}
	for iso, code := range callingCodes {
		if !isoCountryPattern.MatchString(iso) {
			t.Fatalf("invalid ISO alpha-2 key %q", iso)
		}
		if !dialCodePattern.MatchString(code) {
			t.Fatalf("invalid calling code %q for %s", code, iso)
		}
	}
	nanp := 0
	for _, code := range callingCodes {
		if code == "1" {
			nanp++
		}
	}
	if nanp != 25 {
		t.Fatalf("+1 members = %d, want 25", nanp)
	}
}
