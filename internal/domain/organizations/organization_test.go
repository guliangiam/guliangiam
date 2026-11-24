package organizations

import (
	"testing"
	"time"
)

func TestOrganization_Fields(t *testing.T) {
	now := time.Now()
	org := Organization{
		ID:          "019test-org-id",
		Slug:        "test-org",
		DisplayName: "Test Organization",
		WebsiteURL:  "https://example.com",
		Status:      "active",
		InitScore:   100,
		Currency:    "USD",
		CreatedAt:   now,
		UpdatedAt:   now,
		Revision:    1,
	}

	if org.ID != "019test-org-id" {
		t.Errorf("ID = %q, want %q", org.ID, "019test-org-id")
	}
	if org.Slug != "test-org" {
		t.Errorf("Slug = %q, want %q", org.Slug, "test-org")
	}
	if org.DisplayName != "Test Organization" {
		t.Errorf("DisplayName = %q, want %q", org.DisplayName, "Test Organization")
	}
	if org.Status != "active" {
		t.Errorf("Status = %q, want %q", org.Status, "active")
	}
}

func TestOrganization_BooleanFlags(t *testing.T) {
	org := Organization{
		DisableSignin:      true,
		DisableSignup:      true,
		EnableSoftDeletion: true,
		EnableDeposit:      false,
		IsProfilePublic:    true,
		UseEmailAsUsername: false,
		EnableTour:         true,
	}

	if !org.DisableSignin {
		t.Error("DisableSignin should be true")
	}
	if !org.DisableSignup {
		t.Error("DisableSignup should be true")
	}
	if !org.EnableSoftDeletion {
		t.Error("EnableSoftDeletion should be true")
	}
	if org.EnableDeposit {
		t.Error("EnableDeposit should be false")
	}
	if !org.IsProfilePublic {
		t.Error("IsProfilePublic should be true")
	}
	if org.UseEmailAsUsername {
		t.Error("UseEmailAsUsername should be false")
	}
}

func TestOrganization_PasswordOptions(t *testing.T) {
	org := Organization{
		PasswordOptions: []string{" minLength", "requireUpper", "requireDigit"},
	}
	if len(org.PasswordOptions) != 3 {
		t.Errorf("len(PasswordOptions) = %d, want 3", len(org.PasswordOptions))
	}
}

func TestOrganization_SecurityPolicy(t *testing.T) {
	policy := OrganizationSecurityPolicy{
		PasswordPolicy:          map[string]any{"minLength": 8, "requireUpper": true},
		MfaPolicy:               map[string]any{"enabled": true},
		IPPolicy:                map[string]any{"whitelist": []string{"10.0.0.0/8"}},
		SessionPolicy:           map[string]any{"maxConcurrent": 5},
		RiskPolicy:              map[string]any{"enabled": false},
		LockoutPolicy:           map[string]any{"maxAttempts": 5, "lockoutMinutes": 30},
		ConcurrentSessionPolicy: map[string]any{"maxSessions": 3},
		HasMasterKey:            true,
	}

	if !policy.HasMasterKey {
		t.Error("HasMasterKey should be true")
	}
	if policy.PasswordPolicy["minLength"] != 8 {
		t.Errorf("PasswordPolicy minLength = %v, want 8", policy.PasswordPolicy["minLength"])
	}
}

func TestOrganization_Settings(t *testing.T) {
	settings := OrganizationSettings{
		DefaultApplicationID: "app-1",
		FeatureFlags:         map[string]any{"darkMode": true},
	}

	if settings.DefaultApplicationID != "app-1" {
		t.Errorf("DefaultApplicationID = %q, want %q", settings.DefaultApplicationID, "app-1")
	}
}

func TestOrganization_Theme(t *testing.T) {
	theme := OrganizationThemeSettings{
		ThemeName: "dark",
	}

	if theme.ThemeName != "dark" {
		t.Errorf("ThemeName = %q, want %q", theme.ThemeName, "dark")
	}
}

func TestOrganization_KerberosSettings(t *testing.T) {
	org := Organization{
		KerberosRealm:       "EXAMPLE.COM",
		KerberosKdcHost:     "kdc.example.com",
		KerberosServiceName: "HTTP",
	}

	if org.KerberosRealm != "EXAMPLE.COM" {
		t.Errorf("KerberosRealm = %q, want %q", org.KerberosRealm, "EXAMPLE.COM")
	}
}
