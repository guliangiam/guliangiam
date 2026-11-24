package auth

import (
	"testing"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
)

// ---------------------------------------------------------------------------
// Password policy resolution edge cases
// ---------------------------------------------------------------------------

func TestResolvePasswordPolicy_Float64Values(t *testing.T) {
	org := &orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{
			PasswordPolicy: map[string]interface{}{
				"minLength": float64(16),
			},
		},
	}
	policy := resolvePasswordPolicy(org)
	if policy.MinLength != 16 {
		t.Errorf("MinLength = %d, want 16 (from float64)", policy.MinLength)
	}
}

func TestResolvePasswordPolicy_StringIntValues(t *testing.T) {
	org := &orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{
			PasswordPolicy: map[string]interface{}{
				"minLength": "20",
			},
		},
	}
	policy := resolvePasswordPolicy(org)
	if policy.MinLength != 20 {
		t.Errorf("MinLength = %d, want 20 (from string)", policy.MinLength)
	}
}

func TestResolvePasswordPolicy_BoolStringValues(t *testing.T) {
	org := &orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{
			PasswordPolicy: map[string]interface{}{
				"requireUppercase": "true",
				"requireDigit":     "false",
			},
		},
	}
	policy := resolvePasswordPolicy(org)
	if !policy.RequireUppercase {
		t.Error("RequireUppercase should be true (from string 'true')")
	}
	if policy.RequireDigit {
		t.Error("RequireDigit should be false (from string 'false')")
	}
}

func TestResolvePasswordPolicy_InvalidStringInt(t *testing.T) {
	org := &orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{
			PasswordPolicy: map[string]interface{}{
				"minLength": "not-a-number",
			},
		},
	}
	policy := resolvePasswordPolicy(org)
	// Invalid string should be ignored, keeping default
	if policy.MinLength != orgdomain.DefaultPasswordMinLength {
		t.Errorf("MinLength = %d, want %d (default, invalid string ignored)", policy.MinLength, orgdomain.DefaultPasswordMinLength)
	}
}

func TestResolvePasswordPolicy_InvalidBoolType(t *testing.T) {
	org := &orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{
			PasswordPolicy: map[string]interface{}{
				"requireUppercase": 42, // not a bool or string
			},
		},
	}
	policy := resolvePasswordPolicy(org)
	// Invalid type should be ignored, keeping default
	if !policy.RequireUppercase {
		t.Error("RequireUppercase should remain true (default, invalid type ignored)")
	}
}

// ---------------------------------------------------------------------------
// Password validation edge cases
// ---------------------------------------------------------------------------

func TestValidatePasswordAgainstPolicy_UsernameCaseInsensitive(t *testing.T) {
	policy := PasswordPolicyConfig{MinLength: 8, RejectUsername: true, RequireUppercase: false, RequireDigit: false, RequireSpecial: false}
	err := validatePasswordAgainstPolicy("ADMIN123456", "admin", policy)
	if err == nil {
		t.Error("expected error for password containing username (case-insensitive)")
	}
}

func TestValidatePasswordAgainstPolicy_EmptyUsername(t *testing.T) {
	policy := PasswordPolicyConfig{MinLength: 8, RejectUsername: true, RequireUppercase: false, RequireDigit: false, RequireSpecial: false}
	err := validatePasswordAgainstPolicy("validpass", "", policy)
	if err != nil {
		t.Errorf("expected valid when username is empty, got: %v", err)
	}
}

func TestValidatePasswordAgainstPolicy_PasswordExactlyMinLength(t *testing.T) {
	policy := PasswordPolicyConfig{MinLength: 8, RequireUppercase: false, RequireDigit: false, RequireSpecial: false}
	err := validatePasswordAgainstPolicy("a1b2c3d4", "user", policy)
	if err != nil {
		t.Errorf("expected valid for exactly minLength, got: %v", err)
	}
}

func TestValidatePasswordAgainstPolicy_PasswordOneBelowMinLength(t *testing.T) {
	policy := PasswordPolicyConfig{MinLength: 8, RequireUppercase: false, RequireDigit: false, RequireSpecial: false}
	err := validatePasswordAgainstPolicy("abcdefg", "user", policy)
	if err == nil {
		t.Error("expected error for password below minLength")
	}
}

func TestValidatePasswordAgainstPolicy_ZeroMaxLength(t *testing.T) {
	policy := PasswordPolicyConfig{MinLength: 8, MaxLength: 0, RequireUppercase: false, RequireDigit: false, RequireSpecial: false}
	err := validatePasswordAgainstPolicy("a_very_long_password_that_exceeds_normal_limits", "user", policy)
	if err != nil {
		t.Errorf("expected valid when maxLength=0 (no limit), got: %v", err)
	}
}

func TestValidatePasswordAgainstPolicy_CommonPasswordCaseInsensitive(t *testing.T) {
	policy := PasswordPolicyConfig{
		MinLength:             8,
		RejectCommonPasswords: true,
		RequireUppercase:      false,
		RequireDigit:          false,
		RequireSpecial:        false,
	}
	err := validatePasswordAgainstPolicy("PASSWORD", "user", policy)
	if err == nil {
		t.Error("expected error for common password (case-insensitive)")
	}
}

func TestValidatePasswordAgainstPolicy_NotCommonPassword(t *testing.T) {
	policy := PasswordPolicyConfig{
		MinLength:             8,
		RejectCommonPasswords: true,
		RequireUppercase:      false,
		RequireDigit:          false,
		RequireSpecial:        false,
	}
	err := validatePasswordAgainstPolicy("Xk9mL2pQ", "user", policy)
	if err != nil {
		t.Errorf("expected valid for non-common password, got: %v", err)
	}
}

func TestHasSequentialChars_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		password string
		length   int
		want     bool
	}{
		{"exact_4_sequential", "abcd", 4, true},
		{"3_sequential_not_4", "abc1", 4, false},
		{"reverse_sequential", "zyxw", 4, true},
		{"mixed_with_sequential", "xabcde", 4, true},
		{"single_char", "a", 4, false},
		{"empty", "", 4, false},
		{"numbers_sequential", "1234", 4, true},
		{"numbers_reverse", "4321", 4, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasSequentialChars(tt.password, tt.length)
			if got != tt.want {
				t.Errorf("hasSequentialChars(%q, %d) = %v, want %v", tt.password, tt.length, got, tt.want)
			}
		})
	}
}

func TestHasRepeatedChars_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		password string
		length   int
		want     bool
	}{
		{"exact_4_repeated", "aaaa", 4, true},
		{"3_repeated_not_4", "aaa1", 4, false},
		{"mixed_repeated", "xaaaab", 4, true},
		{"single_char", "a", 4, false},
		{"empty", "", 4, false},
		{"numbers_repeated", "1111", 4, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasRepeatedChars(tt.password, tt.length)
			if got != tt.want {
				t.Errorf("hasRepeatedChars(%q, %d) = %v, want %v", tt.password, tt.length, got, tt.want)
			}
		})
	}
}

func TestIsCommonPassword_EdgeCases(t *testing.T) {
	tests := []struct {
		password string
		want     bool
	}{
		{"123456", true},
		{"password", true},
		{"qwerty123", true},
		{"p@ssw0rd", true},
		{"passw0rd", true},
		{"admin", true},
		{"welcome", true},
		{"MyStr0ng!Pass", false},
		{"uniqueP@ss99", false},
	}

	for _, tt := range tests {
		t.Run(tt.password, func(t *testing.T) {
			got := isCommonPassword(tt.password)
			if got != tt.want {
				t.Errorf("isCommonPassword(%q) = %v, want %v", tt.password, got, tt.want)
			}
		})
	}
}

func TestIntFromMap_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		m       map[string]any
		key     string
		wantVal int
		wantOk  bool
	}{
		{"int32_value", map[string]any{"k": int32(42)}, "k", 42, true},
		{"int64_value", map[string]any{"k": int64(100)}, "k", 100, true},
		{"negative_int", map[string]any{"k": -5}, "k", -5, true},
		{"zero_int", map[string]any{"k": 0}, "k", 0, true},
		{"invalid_type_bool", map[string]any{"k": true}, "k", 0, false},
		{"invalid_string", map[string]any{"k": "abc"}, "k", 0, false},
		{"whitespace_string", map[string]any{"k": "  42  "}, "k", 42, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := intFromMap(tt.m, tt.key)
			if val != tt.wantVal || ok != tt.wantOk {
				t.Errorf("intFromMap() = (%d, %v), want (%d, %v)", val, ok, tt.wantVal, tt.wantOk)
			}
		})
	}
}

func TestBoolFromMap_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		m       map[string]any
		key     string
		wantVal bool
		wantOk  bool
	}{
		{"false_string", map[string]any{"k": "false"}, "k", false, true},
		{"TRUE_string", map[string]any{"k": "TRUE"}, "k", true, true},
		{"invalid_type_int", map[string]any{"k": 1}, "k", false, false},
		{"empty_string", map[string]any{"k": ""}, "k", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := boolFromMap(tt.m, tt.key)
			if val != tt.wantVal || ok != tt.wantOk {
				t.Errorf("boolFromMap() = (%v, %v), want (%v, %v)", val, ok, tt.wantVal, tt.wantOk)
			}
		})
	}
}
