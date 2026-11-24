package webhookevents

import (
	"encoding/json"
	"time"
)

type WebhookEvent struct {
	ID             string          `json:"id"`
	WebhookID      string          `json:"webhookId"`
	EventType      string          `json:"eventType"`
	Payload        json.RawMessage `json:"payload,omitempty"`
	ResponseCode   int             `json:"responseCode,omitempty"`
	ResponseBody   string          `json:"responseBody,omitempty"`
	Status         string          `json:"status"`
	RetryCount     int             `json:"retryCount"`
	NextRetryAt    *time.Time      `json:"nextRetryAt,omitempty"`
	IdempotencyKey string          `json:"idempotencyKey,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	Revision       int64           `json:"revision"`
}
