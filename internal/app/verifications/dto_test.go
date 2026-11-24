package verifications

import (
	"testing"
	"time"

	domainverifications "github.com/guliangiam/guliangiam/internal/domain/verifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToVerificationDTO_RedactsOTPCode(t *testing.T) {
	expiresAt := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	createdAt := expiresAt.Add(-10 * time.Minute)

	verification := &domainverifications.Verification{
		ID:             "verification-1",
		OrganizationID: "org-1",
		Type:           "email",
		Recipient:      "user@example.com",
		Code:           "123456",
		Status:         "pending",
		ExpiresAt:      expiresAt,
		CreatedAt:      createdAt,
	}

	dto := toVerificationDTO(verification)

	require.NotNil(t, dto)
	assert.Empty(t, dto.Code)
	assert.Equal(t, verification.ID, dto.ID)
	assert.Equal(t, verification.OrganizationID, dto.OrganizationID)
	assert.Equal(t, verification.Type, dto.Type)
	assert.Equal(t, verification.Recipient, dto.Recipient)
	assert.Equal(t, verification.Status, dto.Status)
	assert.Equal(t, verification.ExpiresAt, dto.ExpiresAt)
	assert.Equal(t, verification.CreatedAt, dto.CreatedAt)
}
