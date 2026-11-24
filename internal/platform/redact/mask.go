package redact

import (
	"strings"
	"unicode"
)

// MaskRecipient renders an email address or phone number so delivery logs can
// be correlated per notification without writing the real contact to the sink.
// It mirrors the read-face masks used for password-reset hints: the email local
// part and the phone middle digits are replaced, the domain / last four digits
// stay readable for support. Anything that is neither email-shaped nor
// phone-shaped (a chat id, a free-form handle) collapses to Placeholder rather
// than being partially echoed.
func MaskRecipient(recipient string) string {
	value := strings.TrimSpace(recipient)
	if value == "" {
		return ""
	}
	if at := strings.LastIndex(value, "@"); at > 0 && at < len(value)-1 {
		return maskEmail(value, at)
	}
	if isPhoneLike(value) {
		return maskPhone(value)
	}
	return Placeholder
}

func maskEmail(value string, at int) string {
	local := []rune(value[:at])
	domain := value[at+1:]
	if len(local) == 0 || domain == "" {
		return Placeholder
	}
	if len(local) == 1 {
		return "*@" + domain
	}
	return string(local[0]) + strings.Repeat("*", len(local)-1) + "@" + domain
}

// isPhoneLike accepts digits plus the separators an E.164-style contact may
// carry. Fewer than seven digits is not a deliverable phone number, so such a
// value falls through to Placeholder instead of a misleading partial mask.
func isPhoneLike(value string) bool {
	digits := 0
	for _, r := range value {
		switch {
		case unicode.IsDigit(r):
			digits++
		case r == '+', r == '-', r == ' ', r == '(', r == ')', r == '.':
		default:
			return false
		}
	}
	return digits >= 7
}

func maskPhone(value string) string {
	runes := []rune(value)
	if len(runes) <= 7 {
		return Placeholder
	}
	return string(runes[:3]) + strings.Repeat("*", len(runes)-7) + string(runes[len(runes)-4:])
}
