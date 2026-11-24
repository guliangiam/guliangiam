package webhookevents

import (
	"encoding/json"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
)

// scanWebhookEvent scans a full webhook_event row into a domain WebhookEvent.
func scanWebhookEvent(sc interface{ Scan(...interface{}) error }) (*domain.WebhookEvent, error) {
	var e domain.WebhookEvent
	var payload []byte
	var responseCode *int
	var responseBody *string
	var nextRetryAt *time.Time
	var idempotencyKey *string
	err := sc.Scan(&e.ID, &e.WebhookID, &e.EventType, &payload, &responseCode, &responseBody, &e.Status, &e.RetryCount, &nextRetryAt, &idempotencyKey, &e.CreatedAt, &e.UpdatedAt, &e.Revision)
	if err != nil {
		return nil, err
	}
	if len(payload) > 0 && string(payload) != "null" {
		e.Payload = json.RawMessage(payload)
	}
	if responseCode != nil {
		e.ResponseCode = *responseCode
	}
	if responseBody != nil {
		e.ResponseBody = *responseBody
	}
	e.NextRetryAt = nextRetryAt
	if idempotencyKey != nil {
		e.IdempotencyKey = *idempotencyKey
	}
	return &e, nil
}
