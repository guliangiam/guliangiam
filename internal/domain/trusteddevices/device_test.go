package trusteddevices

import (
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCredentialInput(t *testing.T) {
	valid := map[string]string{
		"hex 64":             "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"uppercase hex 64":   "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF",
		"base64url 32":       "abcdefghijklmnopqrstuvwxyz012345",
		"base64url -_":       "abcdefghijklmnopqrstuvwxyz01234-_",
		"base64url 32 bytes": "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8",
	}
	for name, value := range valid {
		t.Run("accept "+name, func(t *testing.T) {
			require.NoError(t, ValidateCredentialInput(value))
		})
	}

	missing := map[string]string{
		"empty":      "",
		"whitespace": "   ",
	}
	for name, value := range missing {
		t.Run("reject "+name, func(t *testing.T) {
			err := ValidateCredentialInput(value)
			require.Error(t, err)
			assertErrorCode(t, err, apperrors.CodeInvalidArgument)
		})
	}

	invalid := map[string]string{
		"short hex":      "0123456789abcdef",
		"31 base64url":   "abcdefghijklmnopqrstuvwxyz01234",
		"plus":           "abcdefghijklmnopqrstuvwxyz0123+4",
		"slash":          "abcdefghijklmnopqrstuvwxyz0123/4",
		"padding":        "abcdefghijklmnopqrstuvwxyz0123=",
		"internal space": "abcdefghijklmnopqrstuvwxyz 1234",
	}
	for name, value := range invalid {
		t.Run("reject "+name, func(t *testing.T) {
			err := ValidateCredentialInput(value)
			require.Error(t, err)
			assertErrorCode(t, err, apperrors.CodeUnprocessableEntity)
		})
	}
}

func TestValidateCredentialDigest(t *testing.T) {
	require.NoError(t, ValidateCredentialDigest("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"))

	invalid := []string{
		"",
		"0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0",
		"zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
	}
	for _, value := range invalid {
		err := ValidateCredentialDigest(value)
		require.Error(t, err)
		assertErrorCode(t, err, apperrors.CodeUnprocessableEntity)
	}
}

func assertErrorCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, code, appErr.Code)
}
