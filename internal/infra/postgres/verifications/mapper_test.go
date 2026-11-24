package verifications

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestToVerification(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		row := verificationRow{
			ID:             "ver-001",
			OrganizationID: "org-001",
			Type:           "email",
			Recipient:      "user@example.com",
			Code:           "123456",
			Status:         "pending",
			ExpiresAt:      now.Add(10 * time.Minute),
			CreatedAt:      now,
		}

		result := toVerification(row)

		assert.Equal(t, "ver-001", result.ID)
		assert.Equal(t, "org-001", result.OrganizationID)
		assert.Equal(t, "email", result.Type)
		assert.Equal(t, "user@example.com", result.Recipient)
		assert.Equal(t, "123456", result.Code)
		assert.Equal(t, "pending", result.Status)
	})

	t.Run("minimal_fields", func(t *testing.T) {
		now := time.Now()
		row := verificationRow{
			ID:             "ver-002",
			OrganizationID: "org-002",
			CreatedAt:      now,
			ExpiresAt:      now,
		}

		result := toVerification(row)

		assert.Equal(t, "ver-002", result.ID)
		assert.Equal(t, "", result.Type)
		assert.Equal(t, "", result.Status)
	})
}
