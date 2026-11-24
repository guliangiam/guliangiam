package webhooks

import (
	"testing"
	"time"
)

func TestWebhook_Fields(t *testing.T) {
	now := time.Now()
	wh := Webhook{
		ID:                    "wh-1",
		OrganizationID:        "org-1",
		Name:                  "my-webhook",
		DisplayName:           "My Webhook",
		URL:                   "https://example.com/hook",
		Method:                "POST",
		Headers:               `{"Content-Type":"application/json"}`,
		Events:                []string{"user.created", "user.updated"},
		IsEnabled:             true,
		Secret:                "whsec_abc123",
		RetryCount:            0,
		TimeoutMs:             5000,
		ContentType:           "application/json",
		MaxRetries:            3,
		RetryInterval:         60,
		UseExponentialBackoff: true,
		CreatedAt:             now,
		UpdatedAt:             now,
		Revision:              1,
	}

	if wh.ID != "wh-1" {
		t.Errorf("ID = %q, want wh-1", wh.ID)
	}
	if !wh.IsEnabled {
		t.Error("IsEnabled should be true")
	}
	if len(wh.Events) != 2 {
		t.Errorf("len(Events) = %d, want 2", len(wh.Events))
	}
	if !wh.UseExponentialBackoff {
		t.Error("UseExponentialBackoff should be true")
	}
}

func TestWebhook_Disabled(t *testing.T) {
	wh := Webhook{ID: "wh-2", IsEnabled: false}
	if wh.IsEnabled {
		t.Error("IsEnabled should be false")
	}
}

func TestWebhook_ErrorState(t *testing.T) {
	wh := Webhook{
		ID:        "wh-3",
		IsError:   true,
		ErrorText: "connection refused",
	}
	if !wh.IsError {
		t.Error("IsError should be true")
	}
	if wh.ErrorText != "connection refused" {
		t.Errorf("ErrorText = %q, want 'connection refused'", wh.ErrorText)
	}
}
