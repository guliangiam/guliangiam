package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateNotificationRequest_Fields(t *testing.T) {
	t.Run("full_fields", func(t *testing.T) {
		req := CreateNotificationRequest{
			OrganizationID: "org-001",
			ProviderID:     "prov-001",
			Recipient:      "user@example.com",
			Subject:        "Welcome",
			Content:        "Hello {{.name}}",
			TemplateID:     "tmpl-001",
			Variables:      map[string]any{"name": "John"},
		}

		assert.Equal(t, "org-001", req.OrganizationID)
		assert.Equal(t, "prov-001", req.ProviderID)
		assert.Equal(t, "Welcome", req.Subject)
		assert.Equal(t, "John", req.Variables["name"])
	})
}

func TestListNotificationsRequest_Fields(t *testing.T) {
	t.Run("full_fields", func(t *testing.T) {
		req := ListNotificationsRequest{
			Page:           1,
			PageSize:       20,
			OrganizationID: "org-001",
			Status:         "pending",
		}

		assert.Equal(t, 1, req.Page)
		assert.Equal(t, "pending", req.Status)
	})
}
