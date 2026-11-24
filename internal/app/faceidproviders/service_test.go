package faceidproviders

import (
	"testing"
	"time"
)

type fakeClock struct{ fixedTime time.Time }

func (c fakeClock) Now() time.Time { return c.fixedTime }

type fakeUUIDGenerator struct{ fixedID string }

func (g fakeUUIDGenerator) NewString() (string, error) { return g.fixedID, nil }

func TestService_NewFaceIdProvider_DefaultEnabled(t *testing.T) {
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test-id"},
	}
	p, err := svc.NewFaceIdProvider(CreateFaceIdProviderCommand{
		OrganizationID: "org-1", Name: "aliyun", ProviderType: "Aliyun",
		AccessKeyID: "ak-id", AccessKeySecret: "ak-secret", Endpoint: "https://faceid.aliyun.com",
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
	if p.AccessKeyID != "ak-id" {
		t.Errorf("AccessKeyID = %q", p.AccessKeyID)
	}
}

func TestService_NewFaceIdProvider_ExplicitDisabled(t *testing.T) {
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test-id"},
	}
	disabled := false
	p, err := svc.NewFaceIdProvider(CreateFaceIdProviderCommand{
		OrganizationID: "org-1", Name: "test", ProviderType: "Aliyun",
		IsEnabled: &disabled,
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if p.IsEnabled {
		t.Errorf("IsEnabled = %v, want false", p.IsEnabled)
	}
}

func TestService_NewFaceIdProvider_WithConfig(t *testing.T) {
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test-id"},
	}
	p, err := svc.NewFaceIdProvider(CreateFaceIdProviderCommand{
		OrganizationID: "org-1", Name: "test", ProviderType: "Aliyun",
		Config: map[string]any{"region": "cn-shanghai"},
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if p.Config["region"] != "cn-shanghai" {
		t.Errorf("Config = %v", p.Config)
	}
}
