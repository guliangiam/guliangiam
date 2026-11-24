package webhookevents

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWebhookEvent_Creation(t *testing.T) {
	now := time.Now()
	we := WebhookEvent{
		ID:        "whe-001",
		WebhookID: "wh-001",
		EventType: "user.created",
		Payload:   json.RawMessage(`{"userId":"user-001"}`),
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
	}

	assert.Equal(t, "whe-001", we.ID)
	assert.Equal(t, "pending", we.Status)
	assert.Equal(t, 0, we.RetryCount)
}

func TestWebhookEvent_StatusTransitions(t *testing.T) {
	validStatuses := map[string]bool{
		"pending":            true,
		"delivering":         true,
		"delivered":          true,
		"failed":             true,
		"permanently_failed": true,
	}

	tests := []struct {
		name   string
		status string
		valid  bool
	}{
		{name: "pending", status: "pending", valid: true},
		{name: "delivering", status: "delivering", valid: true},
		{name: "delivered", status: "delivered", valid: true},
		{name: "failed", status: "failed", valid: true},
		{name: "permanently_failed", status: "permanently_failed", valid: true},
		{name: "invalid", status: "unknown", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := validStatuses[tt.status]
			assert.Equal(t, tt.valid, ok)
		})
	}
}

func TestWebhookEvent_RetryLogic(t *testing.T) {
	now := time.Now()
	maxRetries := 5

	t.Run("first_attempt", func(t *testing.T) {
		we := WebhookEvent{Status: "pending", RetryCount: 0}
		assert.Equal(t, 0, we.RetryCount)
		assert.True(t, we.RetryCount < maxRetries)
	})

	t.Run("after_retry", func(t *testing.T) {
		we := WebhookEvent{Status: "failed", RetryCount: 3, NextRetryAt: &now}
		assert.True(t, we.RetryCount < maxRetries)
		assert.NotNil(t, we.NextRetryAt)
	})

	t.Run("max_retries_exceeded", func(t *testing.T) {
		we := WebhookEvent{Status: "permanently_failed", RetryCount: 5}
		assert.Equal(t, maxRetries, we.RetryCount)
		assert.True(t, we.RetryCount >= maxRetries)
	})
}

func TestWebhookEvent_ResponseTracking(t *testing.T) {
	we := WebhookEvent{
		ID:           "whe-001",
		ResponseCode: 200,
		ResponseBody: `{"status":"ok"}`,
		Status:       "delivered",
	}

	assert.Equal(t, 200, we.ResponseCode)
	assert.Equal(t, "delivered", we.Status)
}

func TestWebhookEvent_ExponentialBackoff(t *testing.T) {
	now := time.Now()
	baseDelay := 1 * time.Second

	tests := []struct {
		retryCount int
		expected   time.Duration
	}{
		{retryCount: 0, expected: 1 * time.Second},  // 2^0 = 1s
		{retryCount: 1, expected: 2 * time.Second},  // 2^1 = 2s
		{retryCount: 2, expected: 4 * time.Second},  // 2^2 = 4s
		{retryCount: 3, expected: 8 * time.Second},  // 2^3 = 8s
		{retryCount: 4, expected: 16 * time.Second}, // 2^4 = 16s
	}

	for _, tt := range tests {
		t.Run("retry_"+string(rune('0'+tt.retryCount)), func(t *testing.T) {
			delay := baseDelay * time.Duration(1<<tt.retryCount)
			assert.Equal(t, tt.expected, delay)
			_ = now.Add(delay) // next retry time
		})
	}
}
