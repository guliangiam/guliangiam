package webhooks

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToWebhook(t *testing.T) {
	now := time.Now()

	t.Run("basic_webhook", func(t *testing.T) {
		// DB-058: the confidential column is persisted as ciphertext.
		encSecret, encErr := webhookEncryptedMapper.EncryptColumn(webhookAAD("wh-001"), "wh-secret")
		require.NoError(t, encErr)
		row := webhookRow{
			ID:                    "wh-001",
			OrganizationID:        "org-001",
			Name:                  "user-created",
			DisplayName:           "User Created Webhook",
			URL:                   "https://example.com/webhook",
			Method:                "POST",
			Headers:               `{"X-Custom":"value"}`,
			Events:                []byte(`["user.created","user.updated"]`),
			IsEnabled:             true,
			Secret:                encSecret,
			ContentType:           "application/json",
			MaxRetries:            5,
			RetryInterval:         10,
			UseExponentialBackoff: true,
			CreatedAt:             now,
			UpdatedAt:             now,
			Revision:              1,
		}

		wh, err := toWebhook(row)
		require.NoError(t, err)

		assert.Equal(t, "wh-001", wh.ID)
		assert.Equal(t, "org-001", wh.OrganizationID)
		assert.Equal(t, "user-created", wh.Name)
		assert.Equal(t, "https://example.com/webhook", wh.URL)
		assert.Equal(t, "POST", wh.Method)
		assert.Equal(t, "wh-secret", wh.Secret)
		assert.True(t, wh.IsEnabled)
		assert.Equal(t, "application/json", wh.ContentType)
		assert.Equal(t, 5, wh.MaxRetries)
		assert.True(t, wh.UseExponentialBackoff)
		assert.Equal(t, int64(1), wh.Revision)
	})

	t.Run("nil_events_defaults_to_empty", func(t *testing.T) {
		row := webhookRow{
			ID:             "wh-002",
			OrganizationID: "org-001",
			Name:           "test",
			Events:         nil,
			TokenFields:    nil,
			ObjectFields:   nil,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		wh, err := toWebhook(row)
		require.NoError(t, err)
		assert.Equal(t, []string{}, wh.Events)
		assert.Equal(t, []string{}, wh.TokenFields)
		assert.Equal(t, []string{}, wh.ObjectFields)
	})

	t.Run("disabled_webhook", func(t *testing.T) {
		row := webhookRow{
			ID:             "wh-003",
			OrganizationID: "org-001",
			Name:           "disabled",
			IsEnabled:      false,
			Events:         []byte(`[]`),
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		wh, err := toWebhook(row)
		require.NoError(t, err)
		assert.False(t, wh.IsEnabled)
	})

	t.Run("custom_body_webhook", func(t *testing.T) {
		row := webhookRow{
			ID:             "wh-004",
			OrganizationID: "org-001",
			Name:           "custom-body",
			URL:            "https://example.com/hook",
			CustomBody:     `{"event":"{{.EventType}}","data":{{.Payload}}}`,
			CustomHeaders:  `{"X-Signature":"{{.Signature}}"}`,
			CertID:         "cert-001",
			Events:         []byte(`["order.paid"]`),
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		wh, err := toWebhook(row)
		require.NoError(t, err)
		assert.Equal(t, `{"event":"{{.EventType}}","data":{{.Payload}}}`, wh.CustomBody)
		assert.Equal(t, "cert-001", wh.CertID)
	})
}
