package notifications

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestToNotification(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		sentAt := now.Add(time.Minute)
		subject := "Welcome"
		templateID := "tmpl-001"
		errMsg := "timeout"
		vars := map[string]any{"name": "John", "code": "1234"}
		varsJSON, _ := json.Marshal(vars)

		row := notificationRow{
			ID:             "notif-001",
			OrganizationID: "org-001",
			ProviderID:     "prov-001",
			Recipient:      "user@example.com",
			Subject:        &subject,
			Content:        "Hello {{.name}}",
			TemplateID:     &templateID,
			Variables:      varsJSON,
			Status:         "sent",
			Error:          &errMsg,
			SentAt:         &sentAt,
			RetryCount:     2,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		result := toNotification(row)

		assert.Equal(t, "notif-001", result.ID)
		assert.Equal(t, "org-001", result.OrganizationID)
		assert.Equal(t, "prov-001", result.ProviderID)
		assert.Equal(t, "user@example.com", result.Recipient)
		assert.Equal(t, "Welcome", result.Subject)
		assert.Equal(t, "Hello {{.name}}", result.Content)
		assert.Equal(t, "tmpl-001", result.TemplateID)
		assert.Equal(t, "sent", result.Status)
		assert.Equal(t, "timeout", result.Error)
		assert.Equal(t, &sentAt, result.SentAt)
		assert.Equal(t, 2, result.RetryCount)
		assert.Equal(t, "John", result.Variables["name"])
	})

	t.Run("nil_optional_fields", func(t *testing.T) {
		now := time.Now()
		row := notificationRow{
			ID:             "notif-002",
			OrganizationID: "org-001",
			ProviderID:     "prov-001",
			Recipient:      "test@example.com",
			Subject:        nil,
			TemplateID:     nil,
			Variables:      nil,
			Error:          nil,
			SentAt:         nil,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		result := toNotification(row)

		assert.Equal(t, "", result.Subject)
		assert.Equal(t, "", result.TemplateID)
		assert.Nil(t, result.Variables)
		assert.Equal(t, "", result.Error)
		assert.Nil(t, result.SentAt)
	})
}
