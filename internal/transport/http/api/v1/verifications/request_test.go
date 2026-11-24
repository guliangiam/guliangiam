package verifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSendVerificationCodeRequest_Fields(t *testing.T) {
	t.Run("full_fields", func(t *testing.T) {
		req := SendVerificationCodeRequest{
			OrganizationID: "org-001",
			Type:           "email",
			Recipient:      "user@example.com",
		}

		assert.Equal(t, "org-001", req.OrganizationID)
		assert.Equal(t, "email", req.Type)
		assert.Equal(t, "user@example.com", req.Recipient)
	})
}

func TestVerifyCodeRequest_Fields(t *testing.T) {
	t.Run("full_fields", func(t *testing.T) {
		req := VerifyCodeRequest{
			Type:      "sms",
			Recipient: "+1234567890",
			Code:      "123456",
			ClientIP:  "192.168.1.1",
		}

		assert.Equal(t, "sms", req.Type)
		assert.Equal(t, "+1234567890", req.Recipient)
		assert.Equal(t, "123456", req.Code)
		assert.Equal(t, "192.168.1.1", req.ClientIP)
	})
}

func TestListVerificationsRequest_Fields(t *testing.T) {
	t.Run("full_fields", func(t *testing.T) {
		req := ListVerificationsRequest{
			Page:           1,
			PageSize:       20,
			OrganizationID: "org-001",
			Type:           "email",
			Status:         "pending",
			Recipient:      "user@example.com",
		}

		assert.Equal(t, 1, req.Page)
		assert.Equal(t, "pending", req.Status)
	})
}
