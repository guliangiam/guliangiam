package auth

import (
	"testing"
	"unicode/utf8"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
)

func TestDefaultPasswordPolicy(t *testing.T) {
	policy := defaultPasswordPolicy()
	if policy.MinLength != orgdomain.DefaultPasswordMinLength {
		t.Errorf("MinLength = %d, want %d", policy.MinLength, orgdomain.DefaultPasswordMinLength)
	}
	if policy.MaxLength != 128 {
		t.Errorf("MaxLength = %d, want 128", policy.MaxLength)
	}
	if !policy.RequireUppercase {
		t.Error("RequireUppercase should be true")
	}
	if !policy.RequireLowercase {
		t.Error("RequireLowercase should be true")
	}
	if !policy.RequireDigit {
		t.Error("RequireDigit should be true")
	}
	if !policy.RequireSpecial {
		t.Error("RequireSpecial should be true")
	}
	if !policy.RejectCommonPasswords {
		t.Error("RejectCommonPasswords should be true")
	}
	if !policy.RejectUsername {
		t.Error("RejectUsername should be true")
	}
}

func TestResolvePasswordPolicy_NilOrg(t *testing.T) {
	policy := resolvePasswordPolicy(nil)
	if policy.MinLength != orgdomain.DefaultPasswordMinLength {
		t.Errorf("expected default MinLength=8, got %d", policy.MinLength)
	}
}

func TestResolvePasswordPolicy_EmptyPolicy(t *testing.T) {
	org := &orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{},
	}
	policy := resolvePasswordPolicy(org)
	if policy.MinLength != orgdomain.DefaultPasswordMinLength {
		t.Errorf("expected default MinLength=8, got %d", policy.MinLength)
	}
}

func TestResolvePasswordPolicy_CustomValues(t *testing.T) {
	org := &orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{
			PasswordPolicy: map[string]interface{}{
				"minLength":            8,
				"maxLength":            256,
				"requireUppercase":     false,
				"requireDigit":         false,
				"passwordHistoryCount": 5,
				"passwordMaxAgeDays":   90,
			},
		},
	}
	policy := resolvePasswordPolicy(org)
	if policy.MinLength != 8 {
		t.Errorf("MinLength = %d, want 8", policy.MinLength)
	}
	if policy.MaxLength != 256 {
		t.Errorf("MaxLength = %d, want 256", policy.MaxLength)
	}
	if policy.RequireUppercase {
		t.Error("RequireUppercase should be false")
	}
	if policy.RequireDigit {
		t.Error("RequireDigit should be false")
	}
	if policy.PasswordHistoryCount != 5 {
		t.Errorf("PasswordHistoryCount = %d, want 5", policy.PasswordHistoryCount)
	}
	if policy.PasswordMaxAgeDays != 90 {
		t.Errorf("PasswordMaxAgeDays = %d, want 90", policy.PasswordMaxAgeDays)
	}
}

func TestValidatePasswordAgainstPolicy_Valid(t *testing.T) {
	policy := defaultPasswordPolicy()
	err := validatePasswordAgainstPolicy("Str0ng!Passw0rd", "testuser", policy)
	if err != nil {
		t.Errorf("expected valid password, got error: %v", err)
	}
}

func TestValidatePasswordAgainstPolicy_TooShort(t *testing.T) {
	policy := defaultPasswordPolicy()
	err := validatePasswordAgainstPolicy("Ab1!", "testuser", policy)
	if err == nil {
		t.Error("expected error for short password")
	}
}

func TestValidatePasswordAgainstPolicy_TooLong(t *testing.T) {
	policy := PasswordPolicyConfig{MinLength: 8, MaxLength: 10}
	longPass := "abcdefghijklmnop"
	err := validatePasswordAgainstPolicy(longPass, "testuser", policy)
	if err == nil {
		t.Error("expected error for long password")
	}
}

func TestValidatePasswordAgainstPolicy_UnicodeLengthCountsRunes(t *testing.T) {
	policy := PasswordPolicyConfig{MinLength: 4, MaxLength: 4}
	password := "密码!🌍"
	if got := utf8.RuneCountInString(password); got != 4 {
		t.Fatalf("test password rune count = %d, want 4", got)
	}
	if err := validatePasswordAgainstPolicy(password, "", policy); err != nil {
		t.Fatalf("four-rune password rejected by four-character policy: %v", err)
	}

	policy.MinLength = 5
	if err := validatePasswordAgainstPolicy(password, "", policy); err == nil {
		t.Fatal("four-rune password accepted by five-character minimum")
	}

	policy.MinLength = 1
	policy.MaxLength = 3
	if err := validatePasswordAgainstPolicy(password, "", policy); err == nil {
		t.Fatal("four-rune password accepted by three-character maximum")
	}
}

func TestValidatePasswordAgainstPolicy_MissingUppercase(t *testing.T) {
	policy := PasswordPolicyConfig{MinLength: 8, RequireUppercase: true}
	err := validatePasswordAgainstPolicy("alllowercase1!", "testuser", policy)
	if err == nil {
		t.Error("expected error for missing uppercase")
	}
}

func TestValidatePasswordAgainstPolicy_MissingLowercase(t *testing.T) {
	policy := PasswordPolicyConfig{MinLength: 8, RequireLowercase: true}
	err := validatePasswordAgainstPolicy("ALLUPPERCASE1!", "testuser", policy)
	if err == nil {
		t.Error("expected error for missing lowercase")
	}
}

func TestValidatePasswordAgainstPolicy_MissingDigit(t *testing.T) {
	policy := PasswordPolicyConfig{MinLength: 8, RequireDigit: true}
	err := validatePasswordAgainstPolicy("NoDigitsHere!", "testuser", policy)
	if err == nil {
		t.Error("expected error for missing digit")
	}
}

func TestValidatePasswordAgainstPolicy_MissingSpecial(t *testing.T) {
	policy := PasswordPolicyConfig{MinLength: 8, RequireSpecial: true}
	err := validatePasswordAgainstPolicy("NoSpecial123", "testuser", policy)
	if err == nil {
		t.Error("expected error for missing special character")
	}
}

func TestValidatePasswordAgainstPolicy_ContainsUsername(t *testing.T) {
	policy := PasswordPolicyConfig{MinLength: 8, RejectUsername: true}
	err := validatePasswordAgainstPolicy("admin123456!", "admin", policy)
	if err == nil {
		t.Error("expected error for password containing username")
	}
}

func TestValidatePasswordAgainstPolicy_UsernameExactMatch(t *testing.T) {
	policy := PasswordPolicyConfig{MinLength: 4, RejectUsername: true, RequireUppercase: false, RequireDigit: false, RequireSpecial: false}
	err := validatePasswordAgainstPolicy("testuser", "testuser", policy)
	if err == nil {
		t.Error("expected error for password equal to username")
	}
}

func TestValidatePasswordAgainstPolicy_CommonPassword(t *testing.T) {
	policy := PasswordPolicyConfig{MinLength: 6, RejectCommonPasswords: true, RequireUppercase: false, RequireLowercase: false, RequireDigit: false, RequireSpecial: false}
	err := validatePasswordAgainstPolicy("password", "testuser", policy)
	if err == nil {
		t.Error("expected error for common password")
	}
}

func TestValidatePasswordAgainstPolicy_RelayedPolicy(t *testing.T) {
	policy := PasswordPolicyConfig{MinLength: 6}
	err := validatePasswordAgainstPolicy("simple123", "testuser", policy)
	if err != nil {
		t.Errorf("expected valid for relaxed policy, got error: %v", err)
	}
}

func TestIsCommonPassword(t *testing.T) {
	if !isCommonPassword("123456") {
		t.Error("123456 should be common")
	}
	if !isCommonPassword("password") {
		t.Error("password should be common")
	}
	if isCommonPassword("Xk9#mL2$pQ") {
		t.Error("strong password should not be common")
	}
}

func TestIntFromMap(t *testing.T) {
	tests := []struct {
		name    string
		m       map[string]any
		key     string
		wantVal int
		wantOk  bool
	}{
		{"int value", map[string]any{"k": 42}, "k", 42, true},
		{"float64 value", map[string]any{"k": float64(42)}, "k", 42, true},
		{"string value", map[string]any{"k": "42"}, "k", 42, true},
		{"missing key", map[string]any{}, "k", 0, false},
		{"nil map", nil, "k", 0, false},
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

func TestBoolFromMap(t *testing.T) {
	tests := []struct {
		name    string
		m       map[string]any
		key     string
		wantVal bool
		wantOk  bool
	}{
		{"true bool", map[string]any{"k": true}, "k", true, true},
		{"false bool", map[string]any{"k": false}, "k", false, true},
		{"true string", map[string]any{"k": "true"}, "k", true, true},
		{"missing key", map[string]any{}, "k", false, false},
		{"nil map", nil, "k", false, false},
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

func TestHasSequentialChars(t *testing.T) {
	tests := []struct {
		name     string
		password string
		length   int
		want     bool
	}{
		{"abcd sequential", "abcd1234", 4, true},
		{"dcba reverse sequential", "dcba1234", 4, true},
		{"no sequential", "a1b2c3d4", 4, false},
		{"too short", "abc", 4, false},
		{"abc sequential len 4", "abc123", 4, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasSequentialChars(tt.password, tt.length)
			if got != tt.want {
				t.Errorf("hasSequentialChars() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasRepeatedChars(t *testing.T) {
	tests := []struct {
		name     string
		password string
		length   int
		want     bool
	}{
		{"aaaa repeated", "aaaa1234", 4, true},
		{"no repeated", "abcd1234", 4, false},
		{"too short", "aaa", 4, false},
		{"aaa repeated len 4", "aaa1234", 4, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasRepeatedChars(tt.password, tt.length)
			if got != tt.want {
				t.Errorf("hasRepeatedChars() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculatePasswordComplexityScore(t *testing.T) {
	tests := []struct {
		name     string
		password string
		want     int
	}{
		{"all types", "Ab1!", 4},
		{"upper lower", "Ab", 2},
		{"digit only", "1", 1},
		{"special only", "!", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculatePasswordComplexityScore(tt.password)
			if got != tt.want {
				t.Errorf("calculatePasswordComplexityScore() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestValidatePasswordAgainstPolicy_SequentialChars(t *testing.T) {
	policy := PasswordPolicyConfig{MinLength: 8, RequireUppercase: false, RequireDigit: false, RequireSpecial: false, RequireLowercase: false}
	err := validatePasswordAgainstPolicy("abcdefgh1", "user", policy)
	if err == nil {
		t.Error("expected error for sequential characters")
	}
}

func TestValidatePasswordAgainstPolicy_RepeatedChars(t *testing.T) {
	policy := PasswordPolicyConfig{MinLength: 8, RequireUppercase: false, RequireDigit: false, RequireSpecial: false, RequireLowercase: false}
	err := validatePasswordAgainstPolicy("aaaa1111bc", "user", policy)
	if err == nil {
		t.Error("expected error for repeated characters")
	}
}

func TestValidatePasswordAgainstPolicy_ComplexityScore(t *testing.T) {
	policy := PasswordPolicyConfig{MinLength: 12, RequireUppercase: false, RequireDigit: false, RequireSpecial: false, RequireLowercase: false}
	err := validatePasswordAgainstPolicy("aaaaaaaaaaaa", "user", policy)
	if err == nil {
		t.Error("expected error for low complexity with minLength>=12")
	}
}
