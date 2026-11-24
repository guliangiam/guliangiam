package applications

import (
	"errors"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// applicationErrorCode extracts the application-namespace code so a test can
// assert the error class without depending on the message wording.
func applicationErrorCode(t *testing.T, err error) string {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "error %v is not an *apperrors.Error", err)
	return string(appErr.Code)
}

// redirectURI is a shorthand so the table below reads as the rule it asserts.
func redirectURI(requested string, registered ...string) bool {
	return MatchRedirectURI(requested, registered)
}

// The two matchers this replaced disagreed, and both were permissive in a way
// the design report calls out (14.6.3): one admitted a
// `*.example.com` registration for any depth of subdomain and ignored the query,
// the other returned true for an empty registered set. Each case below names the
// rule it pins.
func TestMatchRedirectURI(t *testing.T) {
	cases := []struct {
		name       string
		requested  string
		registered []string
		want       bool
	}{
		// Rule 2: full-string equality.
		{"exact match", "https://example.com/cb", []string{"https://example.com/cb"}, true},
		{"exact match among several", "https://example.com/cb", []string{"https://a.com/cb", "https://example.com/cb"}, true},
		{"surrounding whitespace is tolerated", "https://example.com/cb", []string{" https://example.com/cb "}, true},
		{"different host", "https://evil.com/cb", []string{"https://example.com/cb"}, false},
		{"trailing slash is significant", "https://example.com/cb/", []string{"https://example.com/cb"}, false},
		{"trailing slash is significant in reverse", "https://example.com/cb", []string{"https://example.com/cb/"}, false},
		{"different scheme", "http://127.0.0.1/cb", []string{"https://127.0.0.1/cb"}, false},
		{"port must match exactly", "https://example.com:8443/cb", []string{"https://example.com/cb"}, false},

		// Rule 1: the requested URI must be one we would ever compare.
		{"empty request", "", []string{"https://example.com/cb"}, false},
		{"blank request", "   ", []string{"https://example.com/cb"}, false},
		{"relative request", "/cb", []string{"/cb"}, false},
		{"garbage request", "://invalid", []string{"https://example.com/cb"}, false},
		{"fragment is rejected", "https://example.com/cb#frag", []string{"https://example.com/cb"}, false},
		{"registered value with a fragment never matches", "https://example.com/cb", []string{"https://example.com/cb#frag"}, false},
		{"userinfo is rejected", "https://user@example.com/cb", []string{"https://user@example.com/cb"}, false},
		{"registered value with userinfo never matches", "https://example.com/cb", []string{"https://user@example.com/cb"}, false},
		{"plain http on a public host is rejected", "http://example.com/cb", []string{"http://example.com/cb"}, false},

		// Rule 4: no wildcard, of any depth, and not even for the bare domain.
		{"wildcard admits no subdomain", "https://app.example.com/cb", []string{"https://*.example.com/cb"}, false},
		{"wildcard admits no deep subdomain", "https://a.b.example.com/cb", []string{"https://*.example.com/cb"}, false},
		{"wildcard admits not even the bare domain", "https://example.com/cb", []string{"https://*.example.com/cb"}, false},
		{"wildcard in the path", "https://example.com/cb", []string{"https://example.com/*"}, false},
		{"suffix match is not substring match", "https://notexample.com/cb", []string{"https://example.com/cb"}, false},
		{"subdomain is not admitted by a bare registration", "https://app.example.com/cb", []string{"https://example.com/cb"}, false},

		// Rule 4: an empty registered set admits nothing (the open redirect).
		{"nil registered set", "https://any.com/cb", nil, false},
		{"empty registered set", "https://any.com/cb", []string{}, false},
		{"blank entries only", "https://any.com/cb", []string{"", "   "}, false},

		// Rule 2 continued: the query is part of the value.
		{"registered without query admits no query", "https://example.com/cb?a=1", []string{"https://example.com/cb"}, false},
		{"registered with query requires the same query", "https://example.com/cb?a=1", []string{"https://example.com/cb?a=1"}, true},
		{"registered with query rejects a different query", "https://example.com/cb?b=2", []string{"https://example.com/cb?a=1"}, false},
		{"registered with query rejects a missing query", "https://example.com/cb", []string{"https://example.com/cb?a=1"}, false},
		{"query key order matters", "https://example.com/cb?b=2&a=1", []string{"https://example.com/cb?a=1&b=2"}, false},

		// Rule 3: the RFC 8252 §7 loopback exception.
		{"loopback port is variable", "http://127.0.0.1:54321/cb", []string{"http://127.0.0.1/cb"}, true},
		{"loopback ipv6 port is variable", "http://[::1]:54321/cb", []string{"http://[::1]/cb"}, true},
		{"loopback without a port still matches", "http://127.0.0.1/cb", []string{"http://127.0.0.1/cb"}, true},
		{"loopback path must still match", "http://127.0.0.1:54321/other", []string{"http://127.0.0.1/cb"}, false},
		{"loopback query must still match", "http://127.0.0.1:54321/cb?a=1", []string{"http://127.0.0.1/cb"}, false},
		{"loopback does not conflate the two literals", "http://[::1]:1/cb", []string{"http://127.0.0.1/cb"}, false},
		{"loopback does not conflate in reverse", "http://127.0.0.1:1/cb", []string{"http://[::1]/cb"}, false},
		{"a pinned port is exact, not variable", "http://127.0.0.1:54321/cb", []string{"http://127.0.0.1:8080/cb"}, false},
		{"a pinned port still matches exactly", "http://127.0.0.1:8080/cb", []string{"http://127.0.0.1:8080/cb"}, true},
		{"https loopback is exact", "https://127.0.0.1:54321/cb", []string{"https://127.0.0.1/cb"}, false},
		{"https loopback with no port matches", "https://127.0.0.1/cb", []string{"https://127.0.0.1/cb"}, true},

		// Rule 1 continued: a DNS name is not a loopback interface. `localhost`
		// resolves through the resolver (and /etc/hosts), so it carries no
		// guarantee that the code stays on the machine.
		{"localhost the name is not the loopback interface", "http://localhost:3000/cb", []string{"http://localhost/cb"}, false},
		{"localhost does not match the ip literal", "http://localhost:3000/cb", []string{"http://127.0.0.1/cb"}, false},
		{"suffixed localhost is not the loopback interface", "http://app.localhost/cb", []string{"http://app.localhost/cb"}, false},
		{"any-host wildcard localhost", "http://foo.localhost/cb", []string{"http://*.localhost/cb"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, redirectURI(tc.requested, tc.registered...))
		})
	}
}

// Registration is the second half of the fix: the matcher must never be handed a
// wildcard in the first place, because a wildcard registration is either dead
// (today) or a security hole (as soon as someone relaxes the matcher again).
func TestValidateRedirectURI(t *testing.T) {
	valid := []string{
		"https://example.com/cb",
		"https://example.com:8443/cb?tenant=1",
		"https://example.com/cb/",
		"http://127.0.0.1/cb",
		"http://[::1]/cb",
		"http://127.0.0.1:8080/cb",
	}
	for _, uri := range valid {
		t.Run("accepts "+uri, func(t *testing.T) {
			assert.NoError(t, ValidateRedirectURI(uri))
		})
	}

	invalid := map[string]string{
		"empty":             "",
		"blank":             "   ",
		"wildcard host":     "https://*.example.com/cb",
		"wildcard deep":     "https://*.a.example.com/cb",
		"wildcard path":     "https://example.com/*",
		"wildcard port":     "https://example.com:*/cb",
		"relative":          "/cb",
		"no host":           "https://",
		"fragment":          "https://example.com/cb#frag",
		"userinfo":          "https://user@example.com/cb",
		"plain http":        "http://example.com/cb",
		"localhost":         "http://localhost/cb",
		"unknown scheme":    "ftp://example.com/cb",
		"javascript":        "javascript:alert(1)",
		"protocol relative": "//example.com/cb",
	}
	for name, uri := range invalid {
		t.Run("rejects "+name, func(t *testing.T) {
			err := ValidateRedirectURI(uri)
			require.Error(t, err)
			assert.Equal(t, "unprocessable_entity", applicationErrorCode(t, err))
		})
	}
}

// A registered value that the matcher could never admit would be a silent
// no-op registration, so the validator and the matcher must agree on the set of
// acceptable shapes.
func TestValidateRedirectURI_AgreesWithTheMatcher(t *testing.T) {
	for _, uri := range []string{
		"https://example.com/cb",
		"http://127.0.0.1/cb",
		"http://[::1]/cb",
	} {
		t.Run(uri, func(t *testing.T) {
			require.NoError(t, ValidateRedirectURI(uri))
			// A value accepted at registration must match itself at authorize time.
			assert.True(t, redirectURI(uri, uri))
		})
	}
}
