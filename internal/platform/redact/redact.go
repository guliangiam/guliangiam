// Package redact holds the canonical list of sensitive field names and the
// placeholder used to replace their values.
//
// The list is the single source of truth for every redaction boundary in the
// system: the log sanitizer (internal/platform/logging) and the read surfaces
// that must not emit plaintext credentials (设计报告 42.3.7 / 42.3.13). Two
// copies of this list would drift, and the drift would be invisible: the copy
// that lags behind silently stops redacting a field the other copy protects.
package redact

import (
	"strings"
	"unicode"
)

// Placeholder replaces every redacted value. It is deliberately not empty: an
// absent-looking value is indistinguishable from a field that was never
// collected, which makes the redaction itself unverifiable for the reader.
const Placeholder = "***"

// sensitiveFields is matched against a field name after removing separators and
// lowering the case, so `access_token`, `accessToken` and `Access-Token` are one
// entry. Add an entry here only when the name is sensitive in every context it
// can appear in — the list is shared, so a narrow name costs every caller.
var sensitiveFields = []string{
	"password",
	"passwordhash",
	"passwordsalt",
	"accesssecret",
	"accesssecrethash",
	"accesstoken",
	"refreshtoken",
	"clientsecret",
	"bindpassword",
	"totpsecret",
	"recoverycodes",
	"mfapushreceiver",
	"masterpassword",
	"kerberoskeytab",
	"privatekey",
	"token",
	"idcard",
	"passportno",
	"driverlicenseno",
	"cookie",
	"setcookie",
	"authorization",
	"apikey",
	"credential",
	"sessionsecret",
	"secret",
}

// Fields returns the canonical sensitive field names, normalized the same way
// IsSensitiveField normalizes its argument. Callers that must publish or audit
// the rule should use this rather than reading sensitiveFields.
func Fields() []string {
	out := make([]string, len(sensitiveFields))
	copy(out, sensitiveFields)
	return out
}

// IsSensitiveField reports whether name designates a sensitive field.
//
// Matching runs over the name's tokens rather than the raw string, so the
// spellings the conventions produce — `access_token`, `accessToken`,
// `X-Access-Token`, `PASSWORD_HASH` — all resolve to the same canonical entry
// without one entry per spelling.
func IsSensitiveField(name string) bool {
	tokens := tokenize(name)
	for start := range tokens {
		for end := start; end < len(tokens); end++ {
			if isCanonicalField(strings.Join(tokens[start:end+1], "")) {
				return true
			}
		}
	}
	return false
}

// isCanonicalField reports whether a joined token run names a sensitive field,
// including its plural / hash / salt variants.
func isCanonicalField(candidate string) bool {
	if candidate == "" {
		return false
	}
	for _, field := range sensitiveFields {
		if candidate == field {
			return true
		}
		if strings.HasPrefix(candidate, field) && isVariantSuffix(candidate[len(field):]) {
			return true
		}
	}
	return false
}

// isVariantSuffix reports whether suffix turns a canonical field name into a
// variant of the same field: plural, hash, salt, encrypted or wrapped.
func isVariantSuffix(suffix string) bool {
	switch suffix {
	case "", "s", "es", "hash", "salt", "hashes", "value", "values", "list", "data":
		return true
	default:
		return false
	}
}

// tokenize splits a field name on separators and camel-case boundaries and
// lowercases the result. Only token boundaries are considered, so a field whose
// name merely contains a sensitive word (`tokenizer`, `secretary`) stays
// readable.
func tokenize(name string) []string {
	runes := []rune(name)
	tokens := make([]string, 0, 4)
	current := make([]rune, 0, len(runes))

	flush := func() {
		if len(current) > 0 {
			tokens = append(tokens, strings.ToLower(string(current)))
			current = current[:0]
		}
	}

	for i, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			flush()
			continue
		}
		if unicode.IsUpper(r) && len(current) > 0 {
			previous := runes[i-1]
			nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			// `accessToken` breaks before `T`; `APIToken` breaks before `T` too,
			// because the run of capitals ends where the lowercase run begins.
			if unicode.IsLower(previous) || unicode.IsDigit(previous) || nextIsLower {
				flush()
			}
		}
		current = append(current, r)
	}
	flush()

	return tokens
}
