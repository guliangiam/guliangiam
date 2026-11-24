package webhooks

import (
	"encoding/json"
	"testing"
	"time"

	eventdomain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	domain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
)

func TestToListItem(t *testing.T) {
	now := time.Now()
	w := &domain.Webhook{
		ID: "wh-1", OrganizationID: "org-1", Name: "hook", DisplayName: "My Hook",
		URL: "https://example.com", Method: "POST", IsEnabled: true,
		Organization: "acme", ContentType: "application/json",
		CreatedAt: now, UpdatedAt: now,
	}
	item := toListItem(w)
	if item.ID != "wh-1" {
		t.Errorf("ID = %q, want wh-1", item.ID)
	}
	if !item.IsEnabled {
		t.Error("IsEnabled should be true")
	}
}

func TestToDetail(t *testing.T) {
	now := time.Now()
	w := &domain.Webhook{
		ID: "wh-1", OrganizationID: "org-1", Name: "hook",
		Events:                []string{"user.created", "user.updated"},
		RetryCount:            2,
		TimeoutMs:             5000,
		MaxRetries:            3,
		RetryInterval:         30,
		UseExponentialBackoff: true,
		Revision:              5,
		CreatedAt:             now, UpdatedAt: now,
	}
	d := toDetail(w)
	if len(d.Events) != 2 {
		t.Errorf("len(Events) = %d, want 2", len(d.Events))
	}
	if d.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", d.MaxRetries)
	}
	if !d.UseExponentialBackoff {
		t.Error("UseExponentialBackoff should be true")
	}
}

func TestToEventListItem(t *testing.T) {
	now := time.Now()
	e := &eventdomain.WebhookEvent{
		ID: "evt-1", WebhookID: "wh-1", EventType: "user.created",
		Status: "delivered", RetryCount: 0, ResponseCode: 200,
		CreatedAt: now, UpdatedAt: now,
	}
	item := toEventListItem(e)
	if item.EventType != "user.created" {
		t.Errorf("EventType = %q, want user.created", item.EventType)
	}
	if item.ResponseCode != 200 {
		t.Errorf("ResponseCode = %d, want 200", item.ResponseCode)
	}
}

func TestToEventDetail(t *testing.T) {
	now := time.Now()
	payload := json.RawMessage(`{"action":"create"}`)
	e := &eventdomain.WebhookEvent{
		ID: "evt-1", WebhookID: "wh-1", EventType: "user.created",
		Status: "delivered", Payload: payload, ResponseBody: "OK",
		CreatedAt: now, UpdatedAt: now,
	}
	d := toEventDetail(e)
	if d.ResponseBody != "OK" {
		t.Errorf("ResponseBody = %q, want OK", d.ResponseBody)
	}
	if len(d.Payload) == 0 {
		t.Error("expected non-empty payload")
	}
}

func TestToEventDetail_NilPayload(t *testing.T) {
	now := time.Now()
	e := &eventdomain.WebhookEvent{
		ID: "evt-1", WebhookID: "wh-1", EventType: "user.created",
		Status: "pending", Payload: nil,
		CreatedAt: now, UpdatedAt: now,
	}
	d := toEventDetail(e)
	if d.Payload != nil {
		t.Error("expected nil payload")
	}
}
