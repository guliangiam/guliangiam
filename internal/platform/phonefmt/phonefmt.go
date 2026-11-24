// Package phonefmt holds the narrow phone/country formatting rules shared by
// app-level validators. `countryCode` means ISO 3166-1 alpha-2; a phone number
// is either E.164 (`+` + digits) or a legacy local number paired with a dial
// prefix (`86` / `+86`). ISO codes are never treated as calling codes.
package phonefmt

import (
	"regexp"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

var (
	e164Pattern       = regexp.MustCompile(`^\+[1-9]\d{3,14}$`)
	localPattern      = regexp.MustCompile(`^\d{4,15}$`)
	dialCodePattern   = regexp.MustCompile(`^[1-9]\d{0,2}$`)
	isoCountryPattern = regexp.MustCompile(`^[A-Z]{2}$`)
	separators        = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "")
)

// IsE164 reports whether value is a canonical E.164 number (4-15 digits after
// a `+` and non-zero leading digit).
func IsE164(value string) bool {
	return e164Pattern.MatchString(strings.TrimSpace(value))
}

// IsISOCountryCode reports whether value is a 2-letter uppercase ISO 3166-1
// alpha-2 code. Membership is intentionally not checked: the repository has no
// shared country directory.
func IsISOCountryCode(value string) bool {
	return isoCountryPattern.MatchString(strings.TrimSpace(value))
}

// IsDialCode reports whether value is a legacy E.164 calling code (`86`/`+86`).
func IsDialCode(value string) bool {
	trimmed := strings.TrimPrefix(strings.TrimSpace(value), "+")
	return dialCodePattern.MatchString(trimmed)
}

// NormalizeDialCode returns a canonical `+digits` calling code, or "".
func NormalizeDialCode(value string) string {
	trimmed := strings.TrimPrefix(strings.TrimSpace(value), "+")
	if !dialCodePattern.MatchString(trimmed) {
		return ""
	}
	return "+" + trimmed
}

// ValidateCountryCode accepts empty or an uppercase ISO alpha-2 code.
func ValidateCountryCode(value, fieldName string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || IsISOCountryCode(trimmed) {
		return nil
	}
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		fieldName+" must be an ISO 3166-1 alpha-2 code (uppercase)",
		map[string]any{"field": fieldName, "value": trimmed, "format": "iso3166-alpha2"},
	)
}

// ValidatePhone accepts empty, an E.164 number, or a bare local number that the
// caller pairs with its ISO country selector.
func ValidatePhone(value, fieldName string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	normalized := separators.Replace(trimmed)
	if IsE164(normalized) || localPattern.MatchString(normalized) {
		return nil
	}
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		fieldName+" must be E.164 (e.g. +8613800138000) or a local number of 4-15 digits",
		map[string]any{"field": fieldName, "value": trimmed, "format": "e164"},
	)
}

// ToE164 builds the E.164 recipient used by SMS verification. An already-E.164
// number wins because it carries its own country prefix. Otherwise the number
// must be a local number and the hint must be a dial prefix; an ISO alpha-2
// hint is rejected instead of being silently concatenated (the old `CN138...`
// bug).
func ToE164(number, countryHint string) (string, error) {
	trimmedNumber := strings.TrimSpace(number)
	if trimmedNumber == "" {
		return "", apperrors.NewInvalidArgument("phone number is required")
	}

	normalizedNumber := separators.Replace(trimmedNumber)
	if strings.HasPrefix(normalizedNumber, "+") {
		if IsE164(normalizedNumber) {
			return normalizedNumber, nil
		}
		return "", invalidNumberError(trimmedNumber)
	}

	dialCode := NormalizeDialCode(countryHint)
	if dialCode == "" {
		return "", apperrors.NewWithMeta(
			apperrors.CodeInvalidArgument,
			"countryCode must be an E.164 calling code; an ISO 3166-1 alpha-2 code cannot prefix a phone number",
			map[string]any{"field": "countryCode", "value": strings.TrimSpace(countryHint), "format": "e164-calling-code"},
		)
	}
	if !localPattern.MatchString(normalizedNumber) {
		return "", invalidNumberError(trimmedNumber)
	}

	return dialCode + normalizedNumber, nil
}

func invalidNumberError(value string) error {
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"phone number must be E.164 or a local number of 4-15 digits",
		map[string]any{"field": "phone", "value": value, "format": "e164"},
	)
}
