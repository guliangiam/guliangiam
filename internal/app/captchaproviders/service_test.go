package captchaproviders

import (
	"testing"
	"time"
)

type fakeClock struct{ fixedTime time.Time }

func (c fakeClock) Now() time.Time { return c.fixedTime }

type fakeUUIDGenerator struct{ fixedID string }

func (g fakeUUIDGenerator) NewString() (string, error) { return g.fixedID, nil }

func TestService_NewCaptchaProvider_DefaultEnabled(t *testing.T) {
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test-id"},
	}
	p, err := svc.NewCaptchaProvider(CreateCaptchaProviderCommand{
		OrganizationID: "org-1", Name: "recaptcha", ProviderType: "recaptcha",
		SiteKey: "site-key", SecretKey: "secret-key",
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

func TestService_NewCaptchaProvider_ExplicitDisabled(t *testing.T) {
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test-id"},
	}
	disabled := false
	p, err := svc.NewCaptchaProvider(CreateCaptchaProviderCommand{
		OrganizationID: "org-1", Name: "hcaptcha", ProviderType: "hcaptcha",
		IsEnabled: &disabled,
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if p.IsEnabled {
		t.Errorf("IsEnabled = %v, want false", p.IsEnabled)
	}
}

func TestService_NewCaptchaProvider_WithConfig(t *testing.T) {
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test-id"},
	}
	config := map[string]any{"theme": "dark"}
	p, err := svc.NewCaptchaProvider(CreateCaptchaProviderCommand{
		OrganizationID: "org-1", Name: "test", ProviderType: "turnstile", Config: config,
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if p.Config["theme"] != "dark" {
		t.Errorf("Config = %v", p.Config)
	}
}

func TestService_NewCaptchaProvider_NilConfig(t *testing.T) {
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test-id"},
	}
	p, err := svc.NewCaptchaProvider(CreateCaptchaProviderCommand{
		OrganizationID: "org-1", Name: "test", ProviderType: "recaptcha",
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if p.Config == nil {
		t.Error("Config should not be nil")
	}
}
