package notificationproviders

import (
	"testing"
	"time"
)

type fakeClock struct{ fixedTime time.Time }

func (c fakeClock) Now() time.Time { return c.fixedTime }

type fakeUUIDGenerator struct{ fixedID string }

func (g fakeUUIDGenerator) NewString() (string, error) { return g.fixedID, nil }

func TestService_NewNotificationProvider_DefaultEnabled(t *testing.T) {
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test-id"},
	}
	p, err := svc.NewNotificationProvider(CreateNotificationProviderCommand{
		OrganizationID: "org-1", Name: "smtp", ProviderType: "SMTP",
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if p.ID != "test-id" {
		t.Errorf("ID = %q", p.ID)
	}
	if !p.IsEnabled {
		t.Errorf("IsEnabled = %v, want true", p.IsEnabled)
	}
	if p.Revision != 1 {
		t.Errorf("Revision = %d, want 1", p.Revision)
	}
}

func TestService_NewNotificationProvider_ExplicitDisabled(t *testing.T) {
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test-id"},
	}
	disabled := false
	p, err := svc.NewNotificationProvider(CreateNotificationProviderCommand{
		OrganizationID: "org-1", Name: "webhook", ProviderType: "Webhook",
		IsEnabled: &disabled,
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if p.IsEnabled {
		t.Errorf("IsEnabled = %v, want false", p.IsEnabled)
	}
}

func TestService_NewNotificationProvider_WithConfig(t *testing.T) {
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test-id"},
	}
	p, err := svc.NewNotificationProvider(CreateNotificationProviderCommand{
		OrganizationID: "org-1", Name: "test", ProviderType: "DingTalk",
		WebhookURL:  "https://example.com/webhook",
		AccessToken: "token-123",
		Config:      map[string]any{"region": "cn"},
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if p.WebhookURL != "https://example.com/webhook" {
		t.Errorf("WebhookURL = %q", p.WebhookURL)
	}
	if p.AccessToken != "token-123" {
		t.Errorf("AccessToken = %q", p.AccessToken)
	}
}
