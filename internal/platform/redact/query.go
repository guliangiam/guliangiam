package redact

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// maxQueryParams bounds how many parameter names one log line may carry. A
// request with thousands of distinct keys must not turn a single blocked-request
// record into a record-sized payload.
const maxQueryParams = 16

// queryParamName keeps a parameter name printable and short. A name outside
// this shape may itself be credential material (an unnamed `?<token>` request),
// so it is replaced rather than echoed.
var queryParamName = regexp.MustCompile(`^[A-Za-z0-9_.\[\]-]{1,32}$`)

// SanitizeQuery renders a raw query string for a log sink with parameter names
// preserved and every value replaced by Placeholder. WAF and access-path logs
// must be able to say "this request carried access_token/api_key/signature"
// without writing the value: `?access_token=…` becomes `access_token=***`.
// Names are sorted for stable records, capped, and rejected when they do not
// look like ordinary parameter names. An unparsable query collapses to
// Placeholder instead of being echoed back.
func SanitizeQuery(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return Placeholder
	}
	if len(values) == 0 {
		return ""
	}

	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, safeQueryParamName(name))
	}
	sort.Strings(names)
	if len(names) > maxQueryParams {
		names = names[:maxQueryParams]
	}

	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+"="+Placeholder)
	}
	return Bound(strings.Join(parts, "&"))
}

// safeQueryParamName drops control characters and rejects names that could be
// the secret itself (long random runs, URL-shaped values, empty names).
func safeQueryParamName(name string) string {
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(name))
	if name == "" || !queryParamName.MatchString(name) {
		return Placeholder
	}
	if sanitized := SanitizeText(name); sanitized != name {
		return Placeholder
	}
	return name
}
