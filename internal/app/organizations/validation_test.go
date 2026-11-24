package organizations

import (
	"errors"
	"testing"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// assertAppError pins both the code and the HTTP status: 4.14.1 distinguishes
// a missing field (400, retrying the same body cannot help) from a rule that
// rejected a bound value (422, the caller succeeds by changing one field).
func assertAppError(t *testing.T, err error, code apperrors.Code) *apperrors.Error {
	t.Helper()
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T (%v)", err, err)
	}
	if appErr.Code != code {
		t.Fatalf("code = %q, want %q", appErr.Code, code)
	}
	status := 400
	if code == apperrors.CodeUnprocessableEntity {
		status = 422
	}
	if appErr.HTTPStatus() != status {
		t.Fatalf("HTTPStatus() = %d, want %d", appErr.HTTPStatus(), status)
	}
	return appErr
}

func TestValidateCreateOrganizationCommand(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		err := validateCreateOrganizationCommand(CreateOrganizationCommand{
			Slug: "my-org", DisplayName: "My Org", Status: "active",
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("missing_slug", func(t *testing.T) {
		err := validateCreateOrganizationCommand(CreateOrganizationCommand{
			DisplayName: "My Org", Status: "active",
		})
		assertAppError(t, err, apperrors.CodeInvalidArgument)
	})
	t.Run("missing_display_name", func(t *testing.T) {
		err := validateCreateOrganizationCommand(CreateOrganizationCommand{
			Slug: "my-org", Status: "active",
		})
		assertAppError(t, err, apperrors.CodeInvalidArgument)
	})
	t.Run("invalid_website", func(t *testing.T) {
		err := validateCreateOrganizationCommand(CreateOrganizationCommand{
			Slug: "my-org", DisplayName: "My Org", Status: "active", WebsiteURL: "not-a-url",
		})
		appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
		if appErr.Meta["format"] != "uri" || appErr.Meta["field"] != "websiteUrl" {
			t.Fatalf("meta = %v, want field/format", appErr.Meta)
		}
	})
	t.Run("unsupported_language", func(t *testing.T) {
		err := validateCreateOrganizationCommand(CreateOrganizationCommand{
			Slug: "my-org", DisplayName: "My Org", Status: "active", Languages: []string{"zh"},
		})
		appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
		if appErr.Meta["language"] != "zh" {
			t.Fatalf("meta = %v, want language", appErr.Meta)
		}
	})
	t.Run("duplicate_language", func(t *testing.T) {
		err := validateCreateOrganizationCommand(CreateOrganizationCommand{
			Slug: "my-org", DisplayName: "My Org", Status: "active", Languages: []string{"en", "en"},
		})
		assertAppError(t, err, apperrors.CodeUnprocessableEntity)
	})
	t.Run("supported_languages", func(t *testing.T) {
		err := validateCreateOrganizationCommand(CreateOrganizationCommand{
			Slug: "my-org", DisplayName: "My Org", Status: "active", Languages: []string{"en", "zh-CN"},
		})
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestValidatePatchOrganizationCommand(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		err := validatePatchOrganizationCommand(PatchOrganizationCommand{})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("blank_display_name", func(t *testing.T) {
		blank := "  "
		err := validatePatchOrganizationCommand(PatchOrganizationCommand{DisplayName: &blank})
		assertAppError(t, err, apperrors.CodeInvalidArgument)
	})
	t.Run("blank_status", func(t *testing.T) {
		blank := "  "
		err := validatePatchOrganizationCommand(PatchOrganizationCommand{Status: &blank})
		assertAppError(t, err, apperrors.CodeInvalidArgument)
	})
	t.Run("invalid_website", func(t *testing.T) {
		bad := "not-a-url"
		err := validatePatchOrganizationCommand(PatchOrganizationCommand{WebsiteURL: &bad})
		assertAppError(t, err, apperrors.CodeUnprocessableEntity)
	})
}

func TestValidateUpdateOrganizationSettingsCommand(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		err := validateUpdateOrganizationSettingsCommand(UpdateOrganizationSettingsCommand{})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("duplicate_language", func(t *testing.T) {
		err := validateUpdateOrganizationSettingsCommand(UpdateOrganizationSettingsCommand{
			Languages: []string{"en", "en"},
		})
		appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
		if appErr.Meta["language"] != "en" {
			t.Fatalf("meta = %v, want language", appErr.Meta)
		}
	})
	t.Run("empty_language", func(t *testing.T) {
		err := validateUpdateOrganizationSettingsCommand(UpdateOrganizationSettingsCommand{
			Languages: []string{""},
		})
		assertAppError(t, err, apperrors.CodeUnprocessableEntity)
	})
	t.Run("unsupported_language", func(t *testing.T) {
		err := validateUpdateOrganizationSettingsCommand(UpdateOrganizationSettingsCommand{
			Languages: []string{"xx"},
		})
		appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
		if appErr.Meta["language"] != "xx" {
			t.Fatalf("meta = %v, want language", appErr.Meta)
		}
		allowed, ok := appErr.Meta["allowed"].([]string)
		if !ok || len(allowed) != 14 {
			t.Fatalf("meta allowed = %v, want 14 supported languages", appErr.Meta["allowed"])
		}
	})
	t.Run("supported_language_subset", func(t *testing.T) {
		err := validateUpdateOrganizationSettingsCommand(UpdateOrganizationSettingsCommand{
			Languages: []string{"en", "zh-CN"},
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("invalid_country_code", func(t *testing.T) {
		err := validateUpdateOrganizationSettingsCommand(UpdateOrganizationSettingsCommand{
			CountryCodes: []string{"cn"},
		})
		appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
		if appErr.Meta["format"] != "iso3166-alpha2" || appErr.Meta["field"] != "countryCodes" {
			t.Fatalf("meta = %v, want format/field", appErr.Meta)
		}
	})
	t.Run("dial_code_is_not_country_code", func(t *testing.T) {
		err := validateUpdateOrganizationSettingsCommand(UpdateOrganizationSettingsCommand{
			CountryCodes: []string{"+86"},
		})
		assertAppError(t, err, apperrors.CodeUnprocessableEntity)
	})
	t.Run("duplicate_country_code", func(t *testing.T) {
		err := validateUpdateOrganizationSettingsCommand(UpdateOrganizationSettingsCommand{
			CountryCodes: []string{"CN", "CN"},
		})
		assertAppError(t, err, apperrors.CodeUnprocessableEntity)
	})
	t.Run("valid_country_codes", func(t *testing.T) {
		err := validateUpdateOrganizationSettingsCommand(UpdateOrganizationSettingsCommand{
			CountryCodes: []string{"CN", "US"},
		})
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestValidateOrganizationAccountItem(t *testing.T) {
	seen := map[string]struct{}{}
	t.Run("valid", func(t *testing.T) {
		err := validateOrganizationAccountItem(0, AccountItemDTO{Name: "email"}, seen)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("empty_name", func(t *testing.T) {
		err := validateOrganizationAccountItem(0, AccountItemDTO{Name: ""}, map[string]struct{}{})
		assertAppError(t, err, apperrors.CodeInvalidArgument)
	})
	t.Run("duplicate_name", func(t *testing.T) {
		s := map[string]struct{}{"email": {}}
		err := validateOrganizationAccountItem(1, AccountItemDTO{Name: "email"}, s)
		assertAppError(t, err, apperrors.CodeUnprocessableEntity)
	})
	t.Run("valid_rules_and_regex", func(t *testing.T) {
		err := validateOrganizationAccountItem(0, AccountItemDTO{
			Name: "Email", Visible: true, ViewRule: "Admin", ModifyRule: "Immutable", Regex: `^[^@]+@[^@]+$`,
		}, map[string]struct{}{})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("invalid_view_rule", func(t *testing.T) {
		err := validateOrganizationAccountItem(2, AccountItemDTO{Name: "Phone", ViewRule: "all"}, map[string]struct{}{})
		appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
		if appErr.Meta["field"] != "viewRule" || appErr.Meta["index"] != 2 {
			t.Fatalf("meta = %v, want field/index", appErr.Meta)
		}
	})
	t.Run("invalid_modify_rule", func(t *testing.T) {
		err := validateOrganizationAccountItem(0, AccountItemDTO{Name: "Phone", ViewRule: "Public", ModifyRule: "admin"}, map[string]struct{}{})
		appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
		if appErr.Meta["field"] != "modifyRule" {
			t.Fatalf("meta = %v, want field", appErr.Meta)
		}
	})
	t.Run("invalid_regex", func(t *testing.T) {
		err := validateOrganizationAccountItem(0, AccountItemDTO{Name: "Email", ViewRule: "Public", ModifyRule: "Self", Regex: "([unclosed"}, map[string]struct{}{})
		appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
		if appErr.Meta["field"] != "regex" {
			t.Fatalf("meta = %v, want field", appErr.Meta)
		}
	})
}

func TestValidateUpdateOrganizationThemeCommand(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		err := validateUpdateOrganizationThemeCommand(UpdateOrganizationThemeCommand{ThemeName: "dark"})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("empty_theme", func(t *testing.T) {
		err := validateUpdateOrganizationThemeCommand(UpdateOrganizationThemeCommand{ThemeName: ""})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestValidateUpdateOrganizationSecurityPolicyCommand(t *testing.T) {
	t.Run("valid_empty", func(t *testing.T) {
		err := validateUpdateOrganizationSecurityPolicyCommand(UpdateOrganizationSecurityPolicyCommand{})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("valid_positive", func(t *testing.T) {
		err := validateUpdateOrganizationSecurityPolicyCommand(UpdateOrganizationSecurityPolicyCommand{
			PasswordPolicy: map[string]any{"minLength": 8},
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("invalid_not_number", func(t *testing.T) {
		err := validateUpdateOrganizationSecurityPolicyCommand(UpdateOrganizationSecurityPolicyCommand{
			PasswordPolicy: map[string]any{"minLength": "bad"},
		})
		appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
		if appErr.Meta["expected"] != "number" {
			t.Fatalf("meta = %v, want expected=number", appErr.Meta)
		}
	})
	t.Run("invalid_zero", func(t *testing.T) {
		err := validateUpdateOrganizationSecurityPolicyCommand(UpdateOrganizationSecurityPolicyCommand{
			SessionPolicy: map[string]any{"idleTimeoutMinutes": 0},
		})
		appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
		if appErr.Meta["min"] != 1 {
			t.Fatalf("meta = %v, want min=1", appErr.Meta)
		}
	})
	t.Run("valid_sm3_derive", func(t *testing.T) {
		err := validateUpdateOrganizationSecurityPolicyCommand(UpdateOrganizationSecurityPolicyCommand{
			PasswordDeriveAlgorithm: orgdomain.PasswordDeriveAlgorithmHMACSM3,
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("invalid_derive_algorithm", func(t *testing.T) {
		err := validateUpdateOrganizationSecurityPolicyCommand(UpdateOrganizationSecurityPolicyCommand{
			PasswordDeriveAlgorithm: "md5",
		})
		appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
		if appErr.Meta["field"] != "passwordDeriveAlgorithm" {
			t.Fatalf("meta = %v, want field=passwordDeriveAlgorithm", appErr.Meta)
		}
		allowed, ok := appErr.Meta["allowed"].([]string)
		if !ok || len(allowed) != 2 || allowed[0] != orgdomain.PasswordDeriveAlgorithmHMACSHA256 || allowed[1] != orgdomain.PasswordDeriveAlgorithmHMACSM3 {
			t.Fatalf("meta allowed = %v, want [hmac_sha256 hmac_sm3]", appErr.Meta["allowed"])
		}
	})
}

func TestNumericValue(t *testing.T) {
	tests := []struct {
		input any
		want  float64
		ok    bool
	}{
		{float64(3.14), 3.14, true},
		{int(5), 5.0, true},
		{int64(10), 10.0, true},
		{uint(2), 2.0, true},
		{"bad", 0, false},
		{nil, 0, false},
	}
	for _, tt := range tests {
		got, ok := numericValue(tt.input)
		if ok != tt.ok || (ok && got != tt.want) {
			t.Fatalf("numericValue(%v) = (%f, %v), want (%f, %v)", tt.input, got, ok, tt.want, tt.ok)
		}
	}
}

func TestValidateOptionalURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"empty", "", false},
		{"valid", "https://example.com", false},
		{"invalid", "not-a-url", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOptionalURL(tt.value, "test")
			if tt.wantErr {
				assertAppError(t, err, apperrors.CodeUnprocessableEntity)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
