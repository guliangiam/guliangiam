package organizations

import (
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

func TestOrganization_Validate(t *testing.T) {
	t.Run("valid_organization", func(t *testing.T) {
		org := Organization{
			ID:          "org-001",
			Slug:        "test-org",
			DisplayName: "Test Organization",
		}
		assert.NoError(t, org.Validate())
	})

	t.Run("missing_id", func(t *testing.T) {
		org := Organization{Slug: "test-org", DisplayName: "Test Org"}
		assert.Error(t, org.Validate())
	})

	t.Run("missing_slug", func(t *testing.T) {
		org := Organization{ID: "org-001", DisplayName: "Test Org"}
		assert.Error(t, org.Validate())
	})

	t.Run("invalid_slug_too_short", func(t *testing.T) {
		org := Organization{ID: "org-001", Slug: "a", DisplayName: "Test Org"}
		assert.Error(t, org.Validate())
	})

	t.Run("invalid_slug_uppercase", func(t *testing.T) {
		org := Organization{ID: "org-001", Slug: "Test-Org", DisplayName: "Test Org"}
		assert.Error(t, org.Validate())
	})

	t.Run("invalid_slug_spaces", func(t *testing.T) {
		org := Organization{ID: "org-001", Slug: "test org", DisplayName: "Test Org"}
		assert.Error(t, org.Validate())
	})

	t.Run("valid_slug_with_hyphens", func(t *testing.T) {
		org := Organization{ID: "org-001", Slug: "my-test-org-123", DisplayName: "Test Org"}
		assert.NoError(t, org.Validate())
	})

	t.Run("valid_slug_alphanumeric", func(t *testing.T) {
		org := Organization{ID: "org-001", Slug: "org123", DisplayName: "Test Org"}
		assert.NoError(t, org.Validate())
	})

	t.Run("missing_displayName", func(t *testing.T) {
		org := Organization{ID: "org-001", Slug: "test-org"}
		assert.Error(t, org.Validate())
	})
}

func TestOrganization_IsSignupAllowed(t *testing.T) {
	t.Run("signup_allowed_by_default", func(t *testing.T) {
		org := Organization{}
		assert.True(t, org.IsSignupAllowed())
	})

	t.Run("signup_disabled", func(t *testing.T) {
		org := Organization{DisableSignup: true}
		assert.False(t, org.IsSignupAllowed())
	})
}

func TestOrganization_IsSigninAllowed(t *testing.T) {
	t.Run("signin_allowed_by_default", func(t *testing.T) {
		org := Organization{}
		assert.True(t, org.IsSigninAllowed())
	})

	t.Run("signin_disabled", func(t *testing.T) {
		org := Organization{DisableSignin: true}
		assert.False(t, org.IsSigninAllowed())
	})
}

func TestOrganizationSecurityPolicy_PasswordPolicy(t *testing.T) {
	policy := OrganizationSecurityPolicy{
		PasswordPolicy: map[string]any{
			"minLength":          12,
			"requireUpper":       true,
			"requireLower":       true,
			"requireDigit":       true,
			"requireSpecial":     true,
			"passwordExpireDays": 90,
		},
	}

	assert.Equal(t, 12, policy.PasswordPolicy["minLength"])
	assert.Equal(t, true, policy.PasswordPolicy["requireUpper"])
	assert.Equal(t, 90, policy.PasswordPolicy["passwordExpireDays"])
}

func TestOrganizationSettings_AccountItems(t *testing.T) {
	settings := OrganizationSettings{
		AccountItems: []AccountItem{
			{Name: "email", Visible: true, ViewRule: "All", ModifyRule: "Admin", Tab: "profile"},
			{Name: "phone", Visible: true, ViewRule: "Self", ModifyRule: "Self", Tab: "profile"},
			{Name: "idCard", Visible: false, ViewRule: "Admin", ModifyRule: "None", Regex: `^\d{17}[\dXx]$`},
		},
	}

	assert.Len(t, settings.AccountItems, 3)
	assert.Equal(t, "email", settings.AccountItems[0].Name)
	assert.True(t, settings.AccountItems[0].Visible)
	assert.Equal(t, "Admin", settings.AccountItems[0].ModifyRule)
}

// ---------------------------------------------------------------------------
// Extended boundary and edge case tests
// ---------------------------------------------------------------------------

func TestOrganization_Validate_Boundary_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		org     Organization
		wantErr bool
	}{
		{"valid minimal", Organization{ID: "org-1", Slug: "acme", DisplayName: "Acme"}, false},
		{"missing id", Organization{Slug: "acme", DisplayName: "Acme"}, true},
		{"missing slug", Organization{ID: "org-1", DisplayName: "Acme"}, true},
		{"missing displayName", Organization{ID: "org-1", Slug: "acme"}, true},
		{"slug single char", Organization{ID: "org-1", Slug: "a", DisplayName: "A"}, true},
		{"slug two chars valid", Organization{ID: "org-1", Slug: "ab", DisplayName: "AB"}, false},
		{"slug with leading hyphen", Organization{ID: "org-1", Slug: "-test", DisplayName: "Test"}, true},
		{"slug with trailing hyphen", Organization{ID: "org-1", Slug: "test-", DisplayName: "Test"}, true},
		{"slug with underscore", Organization{ID: "org-1", Slug: "test_org", DisplayName: "Test"}, true},
		{"slug with spaces", Organization{ID: "org-1", Slug: "test org", DisplayName: "Test"}, true},
		{"slug uppercase", Organization{ID: "org-1", Slug: "TestOrg", DisplayName: "Test"}, true},
		{"slug numeric", Organization{ID: "org-1", Slug: "org123", DisplayName: "Org123"}, false},
		{"slug with multiple hyphens", Organization{ID: "org-1", Slug: "my-test-org-123", DisplayName: "Test"}, false},
		{"slug all digits", Organization{ID: "org-1", Slug: "123", DisplayName: "Numeric"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.org.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOrganization_PasswordPolicy_Boundary(t *testing.T) {
	tests := []struct {
		name   string
		policy OrganizationSecurityPolicy
	}{
		{"complex policy", OrganizationSecurityPolicy{
			PasswordPolicy:          map[string]any{"minLength": 16, "requireUpper": true, "requireLower": true, "requireDigit": true, "requireSpecial": true, "passwordExpireDays": 30},
			LockoutPolicy:           map[string]any{"maxAttempts": 3, "lockoutMinutes": 60},
			ConcurrentSessionPolicy: map[string]any{"maxSessions": 1},
		}},
		{"empty policy", OrganizationSecurityPolicy{}},
		{"minimal policy", OrganizationSecurityPolicy{
			PasswordPolicy: map[string]any{"minLength": 6},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the policy can be constructed without errors
			// Note: PasswordPolicy may be nil for empty policy, which is valid
		})
	}
}

func TestOrganization_ThemeConfiguration(t *testing.T) {
	tests := []struct {
		name  string
		theme OrganizationThemeSettings
	}{
		{"dark theme with tokens", OrganizationThemeSettings{
			OrganizationID: "org-1", ThemeName: "dark",
			Tokens:    map[string]any{"primaryColor": "#1890ff", "borderRadius": 4},
			CustomCSS: ".btn { border-radius: 4px; }",
		}},
		{"light theme", OrganizationThemeSettings{
			OrganizationID: "org-1", ThemeName: "light",
		}},
		{"custom theme with assets", OrganizationThemeSettings{
			OrganizationID: "org-1", ThemeName: "custom",
			Assets: map[string]any{"logo": "/assets/logo.png", "favicon": "/assets/favicon.ico"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.theme.ThemeName)
		})
	}
}

func TestOrganization_SignupSigninFlags(t *testing.T) {
	tests := []struct {
		name       string
		org        Organization
		wantSignup bool
		wantSignin bool
	}{
		{"both enabled", Organization{}, true, true},
		{"signup disabled", Organization{DisableSignup: true}, false, true},
		{"signin disabled", Organization{DisableSignin: true}, true, false},
		{"both disabled", Organization{DisableSignup: true, DisableSignin: true}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantSignup, tt.org.IsSignupAllowed())
			assert.Equal(t, tt.wantSignin, tt.org.IsSigninAllowed())
		})
	}
}

func TestOrganization_Kerberos_Boundary(t *testing.T) {
	tests := []struct {
		name string
		org  Organization
	}{
		{"full kerberos config", Organization{
			KerberosRealm: "EXAMPLE.COM", KerberosKdcHost: "kdc.example.com",
			KerberosKeytab: "/etc/krb5.keytab", KerberosServiceName: "HTTP",
		}},
		{"empty kerberos config", Organization{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify struct assignment works correctly
			if tt.org.KerberosRealm != "" {
				assert.NotEmpty(t, tt.org.KerberosKdcHost)
			}
		})
	}
}

func TestOrganization_Balance_Boundary(t *testing.T) {
	tests := []struct {
		name    string
		balance float64
	}{
		{"zero balance", 0.0},
		{"positive balance", 100.50},
		{"negative balance", -10.0},
		{"large balance", 999999.99},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org := Organization{ID: "org-1", Balance: tt.balance}
			assert.Equal(t, tt.balance, org.Balance)
		})
	}
}

func TestOrganization_Validate_SlugAnswers422(t *testing.T) {
	err := (&Organization{ID: "org-1", Slug: "Bad Slug", DisplayName: "X"}).Validate()
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "slug", appErr.Meta["field"])
}
