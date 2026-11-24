package auth

import (
	"testing"
)

func TestResolveVerificationType(t *testing.T) {
	tests := []struct {
		name      string
		email     string
		phone     string
		wantRecip string
		wantVType string
	}{
		{"email_only", "alice@example.com", "", "alice@example.com", "email"},
		{"phone_only", "", "123456", "123456", "phone"},
		{"both", "alice@example.com", "123456", "alice@example.com", "email"},
		{"neither", "", "", "", ""},
		{"whitespace_email", "  ", "123456", "123456", "phone"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recip, vType := resolveVerificationType(tt.email, tt.phone)
			if recip != tt.wantRecip || vType != tt.wantVType {
				t.Fatalf("got (%s, %s), want (%s, %s)", recip, vType, tt.wantRecip, tt.wantVType)
			}
		})
	}
}

func TestValidateResetPasswordCommand(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		err := validateResetPasswordCommand(ResetPasswordCommand{
			OrganizationSlug: "org1", Username: "alice", VerificationCode: "123456", NewPassword: "P@ssw0rd",
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("missing_org", func(t *testing.T) {
		err := validateResetPasswordCommand(ResetPasswordCommand{
			Username: "alice", VerificationCode: "123456", NewPassword: "P@ssw0rd",
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("missing_username", func(t *testing.T) {
		err := validateResetPasswordCommand(ResetPasswordCommand{
			OrganizationSlug: "org1", VerificationCode: "123456", NewPassword: "P@ssw0rd",
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("missing_code", func(t *testing.T) {
		err := validateResetPasswordCommand(ResetPasswordCommand{
			OrganizationSlug: "org1", Username: "alice", NewPassword: "P@ssw0rd",
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("missing_password", func(t *testing.T) {
		err := validateResetPasswordCommand(ResetPasswordCommand{
			OrganizationSlug: "org1", Username: "alice", VerificationCode: "123456",
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestValidateSignupCommand(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		err := validateSignupCommand(SignupCommand{
			OrganizationSlug: "org1", Username: "alice", Password: "P@ssw0rd",
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("missing_org", func(t *testing.T) {
		err := validateSignupCommand(SignupCommand{Username: "alice", Password: "P@ssw0rd"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("missing_username", func(t *testing.T) {
		err := validateSignupCommand(SignupCommand{OrganizationSlug: "org1", Password: "P@ssw0rd"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("missing_password", func(t *testing.T) {
		err := validateSignupCommand(SignupCommand{OrganizationSlug: "org1", Username: "alice"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
