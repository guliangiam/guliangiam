package http

import "strings"

// ParseNegotiateHeader extracts the SPNEGO token from an HTTP Authorization header
// of the form "Negotiate <base64-token>". Returns the raw token string and whether
// the header was a Negotiate scheme at all.
func ParseNegotiateHeader(header string) (string, bool) {
	if header == "" {
		return "", false
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "negotiate") {
		return "", false
	}
	return strings.TrimSpace(parts[1]), true
}

// BuildNegotiateChallenge builds a WWW-Authenticate: Negotiate header value.
// Pass an empty token for the initial SPNEGO challenge.
func BuildNegotiateChallenge(token string) string {
	if token == "" {
		return "Negotiate"
	}
	return "Negotiate " + token
}
