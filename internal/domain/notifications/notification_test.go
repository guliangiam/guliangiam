package notifications

import (
	"testing"
	"time"
)

func TestNotification_Fields(t *testing.T) {
	now := time.Now()
	n := Notification{
		ID:             "notif-1",
		OrganizationID: "org-1",
		ProviderID:     "prov-1",
		Recipient:      "user@example.com",
		Subject:        "Welcome",
		Content:        "Hello, World!",
		Status:         "pending",
		RetryCount:     0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if n.ID != "notif-1" {
		t.Errorf("ID = %q, want notif-1", n.ID)
	}
	if n.Status != "pending" {
		t.Errorf("Status = %q, want pending", n.Status)
	}
}

func TestNotification_SentStatus(t *testing.T) {
	now := time.Now()
	n := Notification{
		ID:     "notif-2",
		Status: "sent",
		SentAt: &now,
	}
	if n.Status != "sent" {
		t.Errorf("Status = %q, want sent", n.Status)
	}
	if n.SentAt == nil {
		t.Error("SentAt should not be nil")
	}
}

func TestNotification_FailedStatus(t *testing.T) {
	n := Notification{
		ID:     "notif-3",
		Status: "failed",
		Error:  "connection timeout",
	}
	if n.Status != "failed" {
		t.Errorf("Status = %q, want failed", n.Status)
	}
	if n.Error != "connection timeout" {
		t.Errorf("Error = %q, want 'connection timeout'", n.Error)
	}
}

func TestProviderType_Constants(t *testing.T) {
	tests := []struct {
		name     string
		pt       ProviderType
		expected string
	}{
		{"email", ProviderEmail, "email"},
		{"sms", ProviderSMS, "sms"},
		{"dingtalk", ProviderDingTalk, "dingtalk"},
		{"wecom", ProviderWeCom, "wecom"},
		{"slack", ProviderSlack, "slack"},
		{"webhook", ProviderWebhook, "webhook"},
		{"telegram", ProviderTelegram, "telegram"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.pt) != tt.expected {
				t.Errorf("ProviderType = %q, want %q", tt.pt, tt.expected)
			}
		})
	}
}

func TestSendNotification_Fields(t *testing.T) {
	sn := &SendNotification{
		Type:           "email",
		OrganizationID: "org-1",
		Recipient:      "test@example.com",
		Subject:        "Test",
		Content:        "Body",
		Metadata:       map[string]interface{}{"key": "value"},
	}
	if sn.Type != "email" {
		t.Errorf("Type = %q, want email", sn.Type)
	}
	if sn.Metadata["key"] != "value" {
		t.Errorf("Metadata[key] = %v, want value", sn.Metadata["key"])
	}
}
