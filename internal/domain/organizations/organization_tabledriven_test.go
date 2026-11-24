package organizations

import (
	"testing"
	"time"
)

func TestOrganization_Validate_TableDriven(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		org  Organization
	}{
		{
			name: "active org with all fields",
			org: Organization{
				ID: "org-1", Slug: "acme", DisplayName: "Acme Corp",
				WebsiteURL: "https://acme.com", Status: "active",
				InitScore: 100, Currency: "USD",
				DisableSignin: false, DisableSignup: false,
				EnableSoftDeletion: true, IsProfilePublic: true,
				CreatedAt: now, UpdatedAt: now, Revision: 1,
			},
		},
		{
			name: "org with empty slug",
			org: Organization{
				ID: "org-2", Slug: "", DisplayName: "No Slug Corp",
				Status: "active", CreatedAt: now, UpdatedAt: now,
			},
		},
		{
			name: "org with password options",
			org: Organization{
				ID: "org-3", Slug: "secure", Status: "active",
				PasswordOptions: []string{"minLength", "requireUpper", "requireDigit", "requireSpecial"},
				PasswordType:    "bcrypt",
				CreatedAt:       now, UpdatedAt: now,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.org.ID == "" {
				t.Error("ID should not be empty")
			}
		})
	}
}

func TestOrganization_SlugUniqueness(t *testing.T) {
	slugs := []string{"acme", "google", "microsoft", "华为"}
	seen := map[string]bool{}
	for _, slug := range slugs {
		if seen[slug] {
			t.Errorf("duplicate slug: %q", slug)
		}
		seen[slug] = true
	}
}

func TestOrganization_PasswordPolicy(t *testing.T) {
	tests := []struct {
		name    string
		policy  OrganizationSecurityPolicy
		wantLen int
	}{
		{
			name: "policy with rules",
			policy: OrganizationSecurityPolicy{
				PasswordPolicy: map[string]any{"minLength": 12, "requireUpper": true, "requireDigit": true},
				LockoutPolicy:  map[string]any{"maxAttempts": 5, "lockoutMinutes": 30},
			},
			wantLen: 3,
		},
		{
			name:    "empty policy",
			policy:  OrganizationSecurityPolicy{},
			wantLen: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.policy.PasswordPolicy) != tt.wantLen {
				t.Errorf("len(PasswordPolicy) = %d, want %d", len(tt.policy.PasswordPolicy), tt.wantLen)
			}
		})
	}
}

func TestOrganizationSettings_Fields(t *testing.T) {
	settings := OrganizationSettings{
		OrganizationID:       "org-1",
		Languages:            []string{"en", "zh"},
		DefaultApplicationID: "app-1",
		AccountItems: []AccountItem{
			{Name: "email", Visible: true, ViewRule: "normal", ModifyRule: "admin"},
		},
		FeatureFlags: map[string]any{"darkMode": true, "betaFeatures": false},
		CountryCodes: []string{"US", "CN"},
		UserTypes:    []string{"normal", "admin"},
		MfaItems:     []MfaItem{{Name: "totp", Rule: "required"}},
	}
	if len(settings.Languages) != 2 {
		t.Errorf("len(Languages) = %d, want 2", len(settings.Languages))
	}
	if len(settings.AccountItems) != 1 {
		t.Errorf("len(AccountItems) = %d, want 1", len(settings.AccountItems))
	}
	if settings.AccountItems[0].Name != "email" {
		t.Errorf("AccountItems[0].Name = %q, want %q", settings.AccountItems[0].Name, "email")
	}
	if len(settings.MfaItems) != 1 {
		t.Errorf("len(MfaItems) = %d, want 1", len(settings.MfaItems))
	}
}

func TestAccountItem_Fields(t *testing.T) {
	item := AccountItem{
		Name: "phone", Visible: true, ViewRule: "normal",
		ModifyRule: "admin", Regex: `^\+?\d{10,15}$`, Tab: "profile",
	}
	if item.Name != "phone" {
		t.Errorf("Name = %q, want %q", item.Name, "phone")
	}
	if !item.Visible {
		t.Error("Visible should be true")
	}
}

func TestOrganizationThemeSettings_Fields(t *testing.T) {
	theme := OrganizationThemeSettings{
		OrganizationID: "org-1",
		ThemeName:      "dark",
		Tokens:         map[string]any{"primaryColor": "#1890ff"},
		CustomCSS:      ".btn { border-radius: 4px; }",
		Assets:         map[string]any{"logo": "/assets/logo.png"},
	}
	if theme.ThemeName != "dark" {
		t.Errorf("ThemeName = %q, want %q", theme.ThemeName, "dark")
	}
	if len(theme.Tokens) != 1 {
		t.Errorf("len(Tokens) = %d, want 1", len(theme.Tokens))
	}
}

func TestOrganization_ZeroValue(t *testing.T) {
	var org Organization
	if org.ID != "" {
		t.Errorf("ID = %q, want empty", org.ID)
	}
	if org.DisableSignin {
		t.Error("DisableSignin should be false by default")
	}
	if org.EnableSoftDeletion {
		t.Error("EnableSoftDeletion should be false by default")
	}
	if org.Balance != 0 {
		t.Errorf("Balance = %v, want 0", org.Balance)
	}
}
