package radiusconfigs

import (
	"testing"
	"time"
)

type fakeClock struct{ fixedTime time.Time }

func (c fakeClock) Now() time.Time { return c.fixedTime }

type fakeUUIDGenerator struct{ fixedID string }

func (g fakeUUIDGenerator) NewString() (string, error) { return g.fixedID, nil }

func TestService_NewRadiusConfig_DefaultEnabled(t *testing.T) {
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test-id"},
	}
	c, err := svc.NewRadiusConfig(CreateRadiusConfigCommand{
		OrganizationID: "org-1", Name: "freeradius", Host: "radius.example.com",
		Port: 1812, SharedSecret: "shared-secret", AuthProtocol: "PAP",
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if c.ID != "test-id" {
		t.Errorf("ID = %q", c.ID)
	}
	if !c.IsEnabled {
		t.Errorf("IsEnabled = %v, want true", c.IsEnabled)
	}
	if c.Revision != 1 {
		t.Errorf("Revision = %d, want 1", c.Revision)
	}
	if c.Port != 1812 {
		t.Errorf("Port = %d, want 1812", c.Port)
	}
}

func TestService_NewRadiusConfig_ExplicitDisabled(t *testing.T) {
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "test-id"},
	}
	disabled := false
	c, err := svc.NewRadiusConfig(CreateRadiusConfigCommand{
		OrganizationID: "org-1", Name: "test", Host: "localhost",
		Port: 1812, SharedSecret: "secret", AuthProtocol: "CHAP",
		IsEnabled: &disabled,
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if c.IsEnabled {
		t.Errorf("IsEnabled = %v, want false", c.IsEnabled)
	}
}
