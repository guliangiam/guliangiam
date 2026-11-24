package verifications

import (
	"errors"
	"testing"

	"github.com/guliangiam/guliangiam/internal/domain/verifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

func TestToVerificationDTO_Nil(t *testing.T) {
	result := toVerificationDTO(nil)
	assert.Nil(t, result)
}

func TestToVerificationDTO_Valid(t *testing.T) {
	v := &verifications.Verification{
		ID:             "id-1",
		OrganizationID: "org-1",
		Type:           "email",
		Recipient:      "test@example.com",
		Code:           "123456",
		Status:         "pending",
	}
	result := toVerificationDTO(v)
	assert.NotNil(t, result)
	assert.Equal(t, "id-1", result.ID)
	assert.Equal(t, "org-1", result.OrganizationID)
	assert.Equal(t, "email", result.Type)
	assert.Equal(t, "test@example.com", result.Recipient)
}

func TestValidateSendVerificationCommand_EmptyRecipient(t *testing.T) {
	err := validateSendVerificationCommand(SendVerificationCommand{Recipient: "", Type: "email"})
	assert.Error(t, err)
}

func TestValidateSendVerificationCommand_InvalidType(t *testing.T) {
	err := validateSendVerificationCommand(SendVerificationCommand{Recipient: "test@example.com", Type: "invalid"})
	assert.Error(t, err)
}

func TestValidateSendVerificationCommand_ValidEmail(t *testing.T) {
	err := validateSendVerificationCommand(SendVerificationCommand{Recipient: "test@example.com", Type: "email"})
	assert.NoError(t, err)
}

func TestValidateSendVerificationCommand_ValidPhone(t *testing.T) {
	err := validateSendVerificationCommand(SendVerificationCommand{Recipient: "+1234567890", Type: "phone"})
	assert.NoError(t, err)
}

func TestValidateVerifyCodeCommand_EmptyRecipient(t *testing.T) {
	err := validateVerifyCodeCommand(VerifyCodeCommand{Recipient: "", Code: "123456", Type: "email"})
	assert.Error(t, err)
}

func TestValidateVerifyCodeCommand_EmptyCode(t *testing.T) {
	err := validateVerifyCodeCommand(VerifyCodeCommand{Recipient: "test@example.com", Code: "", Type: "email"})
	assert.Error(t, err)
}

func TestValidateVerifyCodeCommand_InvalidType(t *testing.T) {
	err := validateVerifyCodeCommand(VerifyCodeCommand{Recipient: "test@example.com", Code: "123456", Type: "invalid"})
	assert.Error(t, err)
}

func TestValidateVerifyCodeCommand_Valid(t *testing.T) {
	err := validateVerifyCodeCommand(VerifyCodeCommand{Recipient: "test@example.com", Code: "123456", Type: "email"})
	assert.NoError(t, err)
}

func TestGenerateNumericCode(t *testing.T) {
	code, err := generateNumericCode(6)
	assert.NoError(t, err)
	assert.Len(t, code, 6)
	for _, c := range code {
		assert.True(t, c >= '0' && c <= '9')
	}
}

func TestFormatVerificationCode_SixDigits(t *testing.T) {
	result := FormatVerificationCode("123456")
	assert.Equal(t, "123-456", result)
}

func TestFormatVerificationCode_OtherLength(t *testing.T) {
	result := FormatVerificationCode("12345")
	assert.Equal(t, "12345", result)

	result = FormatVerificationCode("1234567")
	assert.Equal(t, "1234567", result)
}

func TestValidateVerificationType_Answers422(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"send", validateSendVerificationCommand(SendVerificationCommand{Recipient: "user@example.com", Type: "sms"})},
		{"verify", validateVerifyCodeCommand(VerifyCodeCommand{Recipient: "user@example.com", Code: "123456", Type: "sms"})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var appErr *apperrors.Error
			if !errors.As(tt.err, &appErr) {
				t.Fatalf("expected *apperrors.Error, got %T (%v)", tt.err, tt.err)
			}
			assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
			assert.Equal(t, "type", appErr.Meta["field"])
			assert.NotNil(t, appErr.Meta["allowed"])
		})
	}
}
