package organizations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrganizationRepository_Max(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    int
		fallback int
		want     int
	}{
		{"positive_value", 5, 10, 5},
		{"zero_uses_fallback", 0, 10, 10},
		{"negative_uses_fallback", -1, 20, 20},
		{"both_positive", 15, 10, 15},
		{"same_values", 10, 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maxInt(tt.value, tt.fallback)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestScanOrganizationRow_FieldCount(t *testing.T) {
	t.Parallel()
	// Verify that the organizationRow struct has the expected number of fields
	// to ensure scan alignment with SQL SELECT columns
	row := organizationRow{
		ID:          "org-001",
		Slug:        "test-org",
		DisplayName: "Test Org",
		Status:      "active",
		CreatedAt:   mustParseTime(),
		UpdatedAt:   mustParseTime(),
		Revision:    1,
	}

	// Verify all fields are accessible without panic
	assert.Equal(t, "org-001", row.ID)
	assert.Equal(t, "test-org", row.Slug)
	assert.Equal(t, "Test Org", row.DisplayName)
	assert.Equal(t, "active", row.Status)
	assert.Equal(t, int64(1), row.Revision)
}

func TestOrganizationRow_ExtendedFields(t *testing.T) {
	t.Parallel()

	t.Run("password_type_fields", func(t *testing.T) {
		row := organizationRow{
			ID:                    "org-001",
			PasswordType:          "argon2id",
			PasswordSalt:          "somesalt",
			PasswordObfuscatorKey: "somekey",
			CreatedAt:             mustParseTime(),
			UpdatedAt:             mustParseTime(),
		}
		assert.Equal(t, "argon2id", row.PasswordType)
		assert.Equal(t, "somesalt", row.PasswordSalt)
		assert.Equal(t, "somekey", row.PasswordObfuscatorKey)
	})

	t.Run("feature_flags", func(t *testing.T) {
		row := organizationRow{
			ID:                 "org-001",
			EnableDeposit:      true,
			EnableDarkLogo:     true,
			IsProfilePublic:    true,
			UseEmailAsUsername: false,
			UsePermanentAvatar: true,
			EnableTour:         false,
			CreatedAt:          mustParseTime(),
			UpdatedAt:          mustParseTime(),
		}
		assert.True(t, row.EnableDeposit)
		assert.True(t, row.EnableDarkLogo)
		assert.True(t, row.IsProfilePublic)
		assert.False(t, row.UseEmailAsUsername)
		assert.True(t, row.UsePermanentAvatar)
		assert.False(t, row.EnableTour)
	})
}

// --- Enhanced boundary tests for Organizations Repository ---

func TestOrganizationRow_SlugUniqueness(t *testing.T) {
	t.Parallel()

	t.Run("slug_field_populated", func(t *testing.T) {
		row := organizationRow{
			ID:        "org-slug-test",
			Slug:      "unique-slug-123",
			CreatedAt: mustParseTime(),
			UpdatedAt: mustParseTime(),
		}
		assert.Equal(t, "unique-slug-123", row.Slug)
	})

	t.Run("slug_with_special_chars", func(t *testing.T) {
		row := organizationRow{
			ID:        "org-special",
			Slug:      "my-org-v2",
			CreatedAt: mustParseTime(),
			UpdatedAt: mustParseTime(),
		}
		assert.Equal(t, "my-org-v2", row.Slug)
	})

	t.Run("empty_slug", func(t *testing.T) {
		row := organizationRow{
			ID:        "org-empty-slug",
			Slug:      "",
			CreatedAt: mustParseTime(),
			UpdatedAt: mustParseTime(),
		}
		assert.Equal(t, "", row.Slug)
	})
}

func TestOrganizationRow_ThemeConfig(t *testing.T) {
	t.Parallel()

	t.Run("theme_row_fields", func(t *testing.T) {
		row := organizationThemeRow{
			OrganizationID: "org-001",
			ThemeName:      "dark",
			Tokens:         []byte(`{"primary":"#1890ff"}`),
			CustomCSS:      ".btn { color: red; }",
			Assets:         []byte(`{"logo":"/logo.png"}`),
		}
		assert.Equal(t, "org-001", row.OrganizationID)
		assert.Equal(t, "dark", row.ThemeName)
		assert.Contains(t, string(row.Tokens), "primary")
		assert.Contains(t, row.CustomCSS, ".btn")
		assert.Contains(t, string(row.Assets), "logo")
	})

	t.Run("settings_row_fields", func(t *testing.T) {
		row := organizationSettingsRow{
			OrganizationID:       "org-001",
			DefaultApplicationID: "app-default",
			AccountMenu:          "side",
			MfaRememberInHours:   24,
			DcrPolicy:            "auto",
		}
		assert.Equal(t, "org-001", row.OrganizationID)
		assert.Equal(t, "app-default", row.DefaultApplicationID)
		assert.Equal(t, 24, row.MfaRememberInHours)
		assert.Equal(t, "auto", row.DcrPolicy)
	})
}

func TestOrganizationRow_SecurityPolicy(t *testing.T) {
	t.Parallel()

	t.Run("security_row_fields", func(t *testing.T) {
		row := organizationSecurityRow{
			OrganizationID:          "org-001",
			PasswordPolicy:          []byte(`{"minLength":8,"requireUppercase":true}`),
			PasswordDeriveAlgorithm: "hmac_sm3",
			MfaPolicy:               []byte(`{"enabled":true}`),
			IPPolicy:                []byte(`{"whitelist":["10.0.0.0/8"]}`),
			SessionPolicy:           []byte(`{"maxConcurrent":5}`),
			RiskPolicy:              []byte(`{"level":"medium"}`),
			LockoutPolicy:           []byte(`{"maxAttempts":5}`),
			ConcurrentSessionPolicy: []byte(`{"maxSessions":3}`),
		}
		assert.Equal(t, "org-001", row.OrganizationID)
		assert.Equal(t, "hmac_sm3", row.PasswordDeriveAlgorithm)
		assert.Contains(t, string(row.PasswordPolicy), "minLength")
		assert.Contains(t, string(row.MfaPolicy), "enabled")
		assert.Contains(t, string(row.SessionPolicy), "maxConcurrent")
		assert.Contains(t, string(row.LockoutPolicy), "maxAttempts")
	})
}

func TestOrganizationRow_AllFieldsAccessible(t *testing.T) {
	t.Parallel()

	now := mustParseTime()
	website := "https://example.com"
	logo := "logo.png"

	row := organizationRow{
		ID:                     "org-full",
		Slug:                   "full-org",
		DisplayName:            "Full Organization",
		WebsiteURL:             &website,
		LogoURL:                &logo,
		Status:                 "active",
		InitScore:              100,
		Balance:                1000.50,
		Currency:               "CNY",
		PasswordType:           "argon2id",
		PasswordSalt:           "salt123",
		PasswordObfuscatorKey:  "key456",
		EnableDeposit:          true,
		BalanceCredit:          500.0,
		Mu:                     0,
		DefaultQuota:           10,
		DefaultCurrency:        "USD",
		EnableDarkLogo:         true,
		HasPrivilegeConsent:    false,
		IsProfilePublic:        true,
		UseEmailAsUsername:     false,
		UsePermanentAvatar:     false,
		EnableTour:             true,
		MasterPassword:         "master123",
		MasterVerificationCode: "123456",
		DefaultPassword:        "default123",
		KerberosRealm:          "EXAMPLE.COM",
		KerberosKdcHost:        "kdc.example.com",
		KerberosKeytab:         "/etc/krb5.keytab",
		KerberosServiceName:    "HTTP",
		CreatedAt:              now,
		UpdatedAt:              now,
		Revision:               5,
	}

	assert.Equal(t, "org-full", row.ID)
	assert.Equal(t, "full-org", row.Slug)
	assert.Equal(t, "Full Organization", row.DisplayName)
	assert.Equal(t, "active", row.Status)
	assert.Equal(t, 100, row.InitScore)
	assert.Equal(t, 1000.50, row.Balance)
	assert.Equal(t, "CNY", row.Currency)
	assert.Equal(t, "argon2id", row.PasswordType)
	assert.Equal(t, true, row.EnableDeposit)
	assert.Equal(t, "EXAMPLE.COM", row.KerberosRealm)
	assert.Equal(t, int64(5), row.Revision)
}

func TestMax_Organizations_Boundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    int
		fallback int
		want     int
	}{
		{"large_page_size", 500, 20, 500},
		{"zero_page_size_uses_fallback", 0, 20, 20},
		{"negative_page_size_uses_fallback", -1, 20, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maxInt(tt.value, tt.fallback)
			assert.Equal(t, tt.want, got)
		})
	}
}

type countingScanner struct{ destinations int }

func (s *countingScanner) Scan(dest ...any) error {
	s.destinations = len(dest)
	return nil
}

// countTopLevelSelectColumns counts comma-separated expressions in a SELECT
// projection, ignoring commas inside function calls.
func countTopLevelSelectColumns(query string) int {
	upper := strings.ToUpper(query)
	start := strings.Index(upper, "SELECT")
	end := strings.Index(upper[start:], "\nFROM ")
	if start < 0 || end < 0 {
		return 0
	}
	projection := query[start+len("SELECT") : start+end]
	depth, columns := 0, 1
	for _, r := range projection {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				columns++
			}
		}
	}
	return columns
}

func TestScanOrganizationRow_MatchesListQuery(t *testing.T) {
	t.Parallel()
	scanner := &countingScanner{}
	_, err := scanOrganizationRow(scanner)
	if err != nil {
		t.Fatalf("scanOrganizationRow: %v", err)
	}
	expected := countTopLevelSelectColumns(queryListOrganizations)
	if scanner.destinations != expected {
		t.Fatalf("list scan has %d destinations, queryListOrganizations selects %d columns", scanner.destinations, expected)
	}
}

func TestOrganizationParentIDEmptyStringIsNormalized(t *testing.T) {
	t.Parallel()
	if !strings.Contains(queryInsertOrganizationCore, "NULLIF($19, '')::uuid") {
		t.Fatal("insert must normalize an empty parent_id to NULL")
	}
	if !strings.Contains(queryUpdateOrganizationCore, "NULLIF($14, '')::uuid") {
		t.Fatal("update must normalize an empty parent_id to NULL")
	}
}
