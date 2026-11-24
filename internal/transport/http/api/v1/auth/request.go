package auth

import (
	"unicode/utf8"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	maxStringLength = 1024
	maxPasswordLen  = 256
	maxSliceLength  = 64
)

// validateOptionalString checks that an optional string field is within length bounds if provided.
func validateOptionalString(field, value string) error {
	if value == "" {
		return nil
	}
	if utf8.RuneCountInString(value) > maxStringLength {
		return apperrors.NewInvalidArgument(field + " exceeds maximum length")
	}
	return nil
}

// validateStringSlice checks that a string slice is within length bounds and each element is valid.
func validateStringSlice(field string, values []string) error {
	if len(values) > maxSliceLength {
		return apperrors.NewInvalidArgument(field + " exceeds maximum number of entries")
	}
	for _, v := range values {
		if utf8.RuneCountInString(v) > maxStringLength {
			return apperrors.NewInvalidArgument(field + " contains an entry exceeding maximum length")
		}
	}
	return nil
}
