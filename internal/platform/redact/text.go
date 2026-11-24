package redact

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxTextRunes bounds every sanitized text that leaves the process.
const MaxTextRunes = 200

// The URL rule runs first so credentials cannot hide in a query.
var (
	urlTokenPattern  = regexp.MustCompile(`(?i)\b(?:https?|wss?)://[^\s"'<>]+`)
	headerPattern    = regexp.MustCompile(`(?i)\b(authorization|proxy-authorization|x-api-key|api-key|api_key|x-auth-token|cookie|set-cookie)(\s*["']?\s*[:=]\s*)("[^"]*"|'[^']*'|\b(?:Bearer|Basic)\s+[^\s"',;]+|[^\s"',;]+)`)
	bearerPattern    = regexp.MustCompile(`(?i)\b(bearer|basic)\s+[A-Za-z0-9._~+/=-]+`)
	queryPattern     = regexp.MustCompile(`(?i)([?&;]|\b)((?:api[_-]?key|access[_-]?token|refresh[_-]?token|client[_-]?secret|secret|token|key|password|signature|sig)=)([^&\s"',;]+)`)
	longTokenPattern = regexp.MustCompile(`[A-Za-z0-9+/=_-]{32,}`)
)

// SanitizeText renders free text without credentials and bounds it to
// MaxTextRunes. URLs keep scheme+host only (userinfo/path/query/fragment are
// dropped), so ?key= / ?api_key= / ?token= payloads cannot survive. Credential
// headers, Bearer/Basic tokens, sensitive key=value pairs and 32+ character
// base64/hex runs become Placeholder; unparsable URLs are replaced wholesale.
func SanitizeText(text string) string {
	if text == "" {
		return ""
	}
	text = urlTokenPattern.ReplaceAllStringFunc(text, sanitizeURL)
	text = headerPattern.ReplaceAllStringFunc(text, sanitizeHeader)
	text = bearerPattern.ReplaceAllString(text, "${1} "+Placeholder)
	text = queryPattern.ReplaceAllString(text, "${1}${2}"+Placeholder)
	text = longTokenPattern.ReplaceAllStringFunc(text, sanitizeLongToken)
	return Bound(text)
}

// Bound trims whitespace and caps s at MaxTextRunes with an ellipsis.
func Bound(s string) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= MaxTextRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:MaxTextRunes-3]) + "..."
}

// sanitizedError renders a safe message while keeping the original error
// reachable through errors.Is/As.
type sanitizedError struct {
	text  string
	cause error
}

func (e *sanitizedError) Error() string { return e.text }
func (e *sanitizedError) Unwrap() error { return e.cause }

// WrapError sanitizes Error() while preserving the errors.Is/As chain.
func WrapError(err error) error {
	if err == nil {
		return nil
	}
	text := SanitizeText(err.Error())
	if text == "" {
		text = Placeholder
	}
	return &sanitizedError{text: text, cause: err}
}

func sanitizeURL(raw string) string {
	trailing := ""
	for len(raw) > 0 && strings.ContainsRune(".,;:!)]}", rune(raw[len(raw)-1])) {
		trailing = string(raw[len(raw)-1]) + trailing
		raw = raw[:len(raw)-1]
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return Placeholder + trailing
	}
	// parsed.User and parsed.RawQuery are deliberately dropped.
	return parsed.Scheme + "://" + parsed.Host + trailing
}

func sanitizeHeader(match string) string {
	parts := headerPattern.FindStringSubmatch(match)
	if len(parts) != 4 {
		return Placeholder
	}
	replacement := Placeholder
	if value := parts[3]; len(value) >= 2 && (value[0] == '"' || value[0] == '\'') {
		replacement = string(value[0]) + Placeholder + string(value[0])
	}
	return parts[1] + parts[2] + replacement
}

// sanitizeLongToken keeps ordinary long words readable; a run counts as a
// secret only when it looks random (has a digit, or mixes upper/lower case).
func sanitizeLongToken(token string) string {
	var hasDigit, hasUpper, hasLower bool
	for _, r := range token {
		switch {
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		}
	}
	if !hasDigit && !(hasUpper && hasLower) {
		return token
	}
	return Placeholder
}
