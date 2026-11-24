package users

import (
	"testing"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestValidatePasswordComplexity_Valid(t *testing.T) {
	err := validatePasswordComplexity("Str0ng!Passw0rd", "testuser", orgdomain.DefaultPasswordPolicy())
	if err != nil {
		t.Errorf("expected valid password, got error: %v", err)
	}
}

// 组织策略 min=12 时，8 位密码必须被拒且 meta 指向 12。
func TestValidatePasswordComplexity_OrganizationMinLength(t *testing.T) {
	policy := orgdomain.DefaultPasswordPolicy()
	policy.MinLength = 12

	err := validatePasswordComplexity("Str0ng!8", "testuser", policy)
	appErr := assertUsersAppError(t, err, apperrors.CodeUnprocessableEntity)
	if appErr.Meta["min"] != 12 {
		t.Fatalf("meta = %v, want min=12", appErr.Meta)
	}

	err = validatePasswordComplexity("Str0ng!Pass1", "testuser", policy)
	if err != nil {
		t.Errorf("expected 12-character password to pass policy, got error: %v", err)
	}
}

func TestValidatePasswordComplexity_TooLong(t *testing.T) {
	longPass := ""
	for i := 0; i < 129; i++ {
		longPass += "a"
	}
	err := validatePasswordComplexity("A1!"+longPass, "testuser", orgdomain.DefaultPasswordPolicy())
	appErr := assertUsersAppError(t, err, apperrors.CodeUnprocessableEntity)
	if appErr.Meta["max"] != 128 {
		t.Fatalf("meta = %v, want max=128", appErr.Meta)
	}
}

func TestValidatePasswordComplexity_MissingUppercase(t *testing.T) {
	err := validatePasswordComplexity("alllowercase1!", "testuser", orgdomain.DefaultPasswordPolicy())
	appErr := assertUsersAppError(t, err, apperrors.CodeUnprocessableEntity)
	if appErr.Meta["field"] != "password" {
		t.Fatalf("meta = %v, want field=password", appErr.Meta)
	}
}

func TestValidatePasswordComplexity_MissingLowercase(t *testing.T) {
	err := validatePasswordComplexity("ALLUPPERCASE1!", "testuser", orgdomain.DefaultPasswordPolicy())
	assertUsersAppError(t, err, apperrors.CodeUnprocessableEntity)
}

func TestValidatePasswordComplexity_MissingDigit(t *testing.T) {
	err := validatePasswordComplexity("NoDigitsHere!", "testuser", orgdomain.DefaultPasswordPolicy())
	assertUsersAppError(t, err, apperrors.CodeUnprocessableEntity)
}

func TestValidatePasswordComplexity_MissingSpecial(t *testing.T) {
	err := validatePasswordComplexity("NoSpecial123", "testuser", orgdomain.DefaultPasswordPolicy())
	assertUsersAppError(t, err, apperrors.CodeUnprocessableEntity)
}

func TestValidatePasswordComplexity_ContainsUsername(t *testing.T) {
	err := validatePasswordComplexity("Testuser123!@", "testuser", orgdomain.DefaultPasswordPolicy())
	appErr := assertUsersAppError(t, err, apperrors.CodeUnprocessableEntity)
	if _, leaked := appErr.Meta["value"]; leaked {
		t.Fatalf("password value leaked into meta: %v", appErr.Meta)
	}
}

func TestValidatePasswordComplexity_EmptyUsername(t *testing.T) {
	err := validatePasswordComplexity("Str0ng!Passw0rd", "", orgdomain.DefaultPasswordPolicy())
	if err != nil {
		t.Errorf("expected valid when username is empty, got error: %v", err)
	}
}
