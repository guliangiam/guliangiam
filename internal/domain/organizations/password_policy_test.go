package organizations

import "testing"

func TestEffectivePasswordPolicy_DefaultWhenUnset(t *testing.T) {
	policy := EffectivePasswordPolicy(nil)
	if policy.MinLength != DefaultPasswordMinLength {
		t.Fatalf("MinLength = %d, want %d", policy.MinLength, DefaultPasswordMinLength)
	}
	if policy.MaxLength != DefaultPasswordMaxLength {
		t.Fatalf("MaxLength = %d, want %d", policy.MaxLength, DefaultPasswordMaxLength)
	}
	if policy.PasswordHistoryCount != 5 {
		t.Fatalf("PasswordHistoryCount = %d, want 5", policy.PasswordHistoryCount)
	}
	if policy.PasswordMaxAgeDays != 90 {
		t.Fatalf("PasswordMaxAgeDays = %d, want 90", policy.PasswordMaxAgeDays)
	}

	org := &Organization{}
	policy = EffectivePasswordPolicy(org)
	if policy != DefaultPasswordPolicy() {
		t.Fatalf("policy = %+v, want default %+v", policy, DefaultPasswordPolicy())
	}
}

func TestEffectivePasswordPolicy_OrganizationOverrides(t *testing.T) {
	org := &Organization{}
	org.Security.PasswordPolicy = map[string]any{
		"minLength":            12,
		"maxLength":            64,
		"requireSpecial":       false,
		"passwordHistoryCount": 5,
	}

	policy := EffectivePasswordPolicy(org)
	if policy.MinLength != 12 {
		t.Fatalf("MinLength = %d, want 12", policy.MinLength)
	}
	if policy.MaxLength != 64 {
		t.Fatalf("MaxLength = %d, want 64", policy.MaxLength)
	}
	if policy.RequireSpecial {
		t.Fatal("RequireSpecial = true, want false")
	}
	if policy.PasswordHistoryCount != 5 {
		t.Fatalf("PasswordHistoryCount = %d, want 5", policy.PasswordHistoryCount)
	}
	// 未配置的字段仍沿用默认值。
	if !policy.RequireUppercase || !policy.RequireDigit {
		t.Fatal("unspecified character-class requirements should keep defaults")
	}
	if policy.PasswordMaxAgeDays != 90 {
		t.Fatalf("unspecified password max age = %d, want default 90", policy.PasswordMaxAgeDays)
	}
}
