package workload

import "testing"

func TestValidateSubjectPatternMatrix(t *testing.T) {
	tests := []struct {
		pattern string
		valid   bool
	}{
		{pattern: "repo:org/app:ref:main", valid: true},
		{pattern: "Repo:Org/App:ref:main", valid: true},
		{pattern: "repo:org/*", valid: true},
		{pattern: "*", valid: false},
		{pattern: "", valid: false},
		{pattern: "/*", valid: false},
		{pattern: "repo:*", valid: false},
		{pattern: "repo:org*", valid: false},
		{pattern: "repo:*/app", valid: false},
		{pattern: "repo:org/**", valid: false},
		{pattern: "repo:org/*/*", valid: false},
		{pattern: " repo:org/app", valid: false},
		{pattern: "repo:org/app ", valid: false},
		{pattern: "repo:org/app\n", valid: false},
		{pattern: "repo:\u0000org", valid: false},
		{pattern: "repo:\u2003org", valid: false},
		{pattern: "repo:\u200borg", valid: false},
	}
	for _, test := range tests {
		t.Run(test.pattern, func(t *testing.T) {
			err := ValidateSubjectPattern(test.pattern)
			if (err == nil) != test.valid {
				t.Fatalf("ValidateSubjectPattern(%q) error = %v, valid = %v", test.pattern, err, test.valid)
			}
		})
	}
}

func TestMatchSubjectMatrix(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		subject  string
		allowed  bool
	}{
		{name: "exact", patterns: []string{"repo:org/app:ref:main"}, subject: "repo:org/app:ref:main", allowed: true},
		{name: "exact is case sensitive", patterns: []string{"repo:Org/app"}, subject: "repo:org/app", allowed: false},
		{name: "prefix", patterns: []string{"repo:org/*"}, subject: "repo:org/app:ref:main", allowed: true},
		{name: "prefix includes slash", patterns: []string{"repo:org/*"}, subject: "repo:organization/app", allowed: false},
		{name: "second pattern matches", patterns: []string{"spiffe://example/*", "repo:org/app"}, subject: "repo:org/app", allowed: true},
		{name: "empty patterns fail closed", patterns: nil, subject: "repo:org/app", allowed: false},
		{name: "empty subject fail closed", patterns: []string{"repo:org/*"}, subject: "", allowed: false},
		{name: "blank subject fail closed", patterns: []string{"repo:org/*"}, subject: "repo:org/app\n", allowed: false},
		{name: "invalid pattern fails entire match", patterns: []string{"repo:org/app", "*"}, subject: "repo:org/app", allowed: false},
		{name: "bare wildcard rejected", patterns: []string{"*"}, subject: "anything", allowed: false},
		{name: "no substring wildcard", patterns: []string{"repo:org/*"}, subject: "prefix/repo:org/app", allowed: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := MatchSubject(test.patterns, test.subject); got != test.allowed {
				t.Fatalf("MatchSubject(%q, %q) = %v, want %v", test.patterns, test.subject, got, test.allowed)
			}
		})
	}
}

func TestValidateSubjectPatternsRequiresNonEmptyList(t *testing.T) {
	if err := ValidateSubjectPatterns(nil); err == nil {
		t.Fatal("empty subject pattern list must be rejected")
	}
}

func TestValidateHTTPSURI(t *testing.T) {
	for _, test := range []struct {
		value string
		valid bool
	}{
		{value: "https://issuer.example.com", valid: true},
		{value: "HTTPS://issuer.example.com/path", valid: true},
		{value: "https://issuer.example.com:8443/jwks.json", valid: true},
		{value: "http://issuer.example.com", valid: false},
		{value: "issuer.example.com", valid: false},
		{value: "https://", valid: false},
		{value: "", valid: false},
		{value: "https://issuer.example.com/\n", valid: false},
	} {
		if err := ValidateHTTPSURI(test.value); (err == nil) != test.valid {
			t.Fatalf("ValidateHTTPSURI(%q) error = %v, valid = %v", test.value, err, test.valid)
		}
	}
}
