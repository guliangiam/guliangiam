package organizations

import (
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ---------------------------------------------------------------------------
// Fakes for service_test.go
// ---------------------------------------------------------------------------

type svcFakeClock struct {
	fixedTime time.Time
}

func (c svcFakeClock) Now() time.Time { return c.fixedTime }

type svcFakeUUID struct {
	fixedID string
}

func (g svcFakeUUID) NewString() (string, error) { return g.fixedID, nil }

type svcFakeUUIDErr struct{}

func (g svcFakeUUIDErr) NewString() (string, error) {
	return "", apperrors.NewInternal("uuid generation failed")
}

// ---------------------------------------------------------------------------
// Tests for Service.NewOrganization
// ---------------------------------------------------------------------------

func TestService_NewOrganization_UUIDError(t *testing.T) {
	svc := Service{
		Clock: svcFakeClock{fixedTime: time.Now()},
		UUID:  svcFakeUUIDErr{},
	}

	_, err := svc.NewOrganization(CreateOrganizationCommand{
		Slug:        "test-org",
		DisplayName: "Test Org",
		Status:      "active",
	})
	if err == nil {
		t.Fatal("NewOrganization() expected error when UUID fails, got nil")
	}
}

func TestService_NewOrganization_AllFields(t *testing.T) {
	fixedNow := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	fixedID := "019org-test-id-0001"

	svc := Service{
		Clock: svcFakeClock{fixedTime: fixedNow},
		UUID:  svcFakeUUID{fixedID: fixedID},
	}

	org, err := svc.NewOrganization(CreateOrganizationCommand{
		Slug:        "guliang",
		DisplayName: "GuliangTech",
		WebsiteURL:  "https://guliang.tech",
		LogoURL:     "https://guliang.tech/logo.png",
		FaviconURL:  "https://guliang.tech/favicon.ico",
		Status:      "active",
	})
	if err != nil {
		t.Fatalf("NewOrganization() error = %v", err)
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"ID", org.ID, fixedID},
		{"Slug", org.Slug, "guliang"},
		{"DisplayName", org.DisplayName, "GuliangTech"},
		{"WebsiteURL", org.WebsiteURL, "https://guliang.tech"},
		{"LogoURL", org.LogoURL, "https://guliang.tech/logo.png"},
		{"FaviconURL", org.FaviconURL, "https://guliang.tech/favicon.ico"},
		{"Status", org.Status, "active"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}

	if !org.CreatedAt.Equal(fixedNow) {
		t.Errorf("CreatedAt = %v, want %v", org.CreatedAt, fixedNow)
	}
	if org.Revision != 1 {
		t.Errorf("Revision = %d, want 1", org.Revision)
	}
}

func TestService_NewOrganization_MinimalFields(t *testing.T) {
	fixedNow := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)

	svc := Service{
		Clock: svcFakeClock{fixedTime: fixedNow},
		UUID:  svcFakeUUID{fixedID: "test-id"},
	}

	org, err := svc.NewOrganization(CreateOrganizationCommand{
		Slug:        "minimal",
		DisplayName: "Minimal",
		Status:      "active",
	})
	if err != nil {
		t.Fatalf("NewOrganization() error = %v", err)
	}

	if org.WebsiteURL != "" {
		t.Errorf("WebsiteURL = %q, want empty", org.WebsiteURL)
	}
	if org.LogoURL != "" {
		t.Errorf("LogoURL = %q, want empty", org.LogoURL)
	}
}
