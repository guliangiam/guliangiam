package webhookevents

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockScanner struct {
	values []interface{}
	err    error
}

func (m *mockScanner) Scan(dest ...interface{}) error {
	if m.err != nil {
		return m.err
	}
	for i, d := range dest {
		switch v := m.values[i].(type) {
		case int64:
			if ptr, ok := d.(*int64); ok {
				*ptr = v
			}
		case int:
			if ptr, ok := d.(*int); ok {
				*ptr = v
			}
		case string:
			if ptr, ok := d.(*string); ok {
				*ptr = v
			}
		case []byte:
			if ptr, ok := d.(*[]byte); ok {
				*ptr = v
			}
		case time.Time:
			if ptr, ok := d.(*time.Time); ok {
				*ptr = v
			}
		case *string:
			if ptr, ok := d.(**string); ok {
				*ptr = v
			}
		case *int:
			if ptr, ok := d.(**int); ok {
				*ptr = v
			}
		case *time.Time:
			if ptr, ok := d.(**time.Time); ok {
				*ptr = v
			}
		case nil:
			// skip nil values
		}
	}
	return nil
}

func TestScanWebhookEvent(t *testing.T) {
	now := time.Now()
	nextRetry := now.Add(5 * time.Minute)
	responseCode := 200
	responseBody := "OK"
	idempotencyKey := "idem-001"

	t.Run("full_event", func(t *testing.T) {
		payload := []byte(`{"event":"user.created","userId":"user-001"}`)
		sc := &mockScanner{
			values: []interface{}{
				"event-001", "webhook-001", "user.created",
				payload,
				&responseCode, &responseBody,
				"delivered", 0,
				&nextRetry, &idempotencyKey,
				now, now, int64(1),
			},
		}

		event, err := scanWebhookEvent(sc)
		require.NoError(t, err)

		assert.Equal(t, "event-001", event.ID)
		assert.Equal(t, "webhook-001", event.WebhookID)
		assert.Equal(t, "user.created", event.EventType)
		assert.Equal(t, json.RawMessage(`{"event":"user.created","userId":"user-001"}`), event.Payload)
		assert.Equal(t, 200, event.ResponseCode)
		assert.Equal(t, "OK", event.ResponseBody)
		assert.Equal(t, "delivered", event.Status)
		assert.Equal(t, 0, event.RetryCount)
		assert.Equal(t, nextRetry, *event.NextRetryAt)
		assert.Equal(t, "idem-001", event.IdempotencyKey)
	})

	t.Run("null_fields", func(t *testing.T) {
		sc := &mockScanner{
			values: []interface{}{
				"event-002", "webhook-002", "user.deleted",
				[]byte("null"),
				nil, nil, // responseCode, responseBody
				"pending", 2,
				nil, nil, // nextRetryAt, idempotencyKey
				now, now, int64(1),
			},
		}

		event, err := scanWebhookEvent(sc)
		require.NoError(t, err)
		assert.Nil(t, event.Payload)
		assert.Equal(t, 0, event.ResponseCode)
		assert.Equal(t, "", event.ResponseBody)
		assert.Nil(t, event.NextRetryAt)
		assert.Equal(t, "", event.IdempotencyKey)
	})

	t.Run("scan_error", func(t *testing.T) {
		sc := &mockScanner{err: errors.New("db error")}
		_, err := scanWebhookEvent(sc)
		assert.Error(t, err)
	})
}
