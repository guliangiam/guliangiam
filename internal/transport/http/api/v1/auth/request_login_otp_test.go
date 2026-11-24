package auth

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoginRequest_Validate_VerificationFields(t *testing.T) {
	cases := []struct {
		name    string
		request LoginRequest
		wantErr bool
	}{
		{
			name:    "optional fields absent",
			request: LoginRequest{Username: "user", Password: "secret", ApplicationID: "app"},
		},
		{
			name:    "valid email channel",
			request: LoginRequest{Username: "user@example.com", VerificationCode: "123456", VerificationType: "email"},
		},
		{
			name:    "valid sms channel",
			request: LoginRequest{Username: "13800138000", VerificationCode: "123456", VerificationType: "sms"},
		},
		{
			name:    "verification code too long",
			request: LoginRequest{Username: "user", VerificationCode: strings.Repeat("1", maxStringLength+1), VerificationType: "email"},
			wantErr: true,
		},
		{
			name:    "verification type too long",
			request: LoginRequest{Username: "user", VerificationType: strings.Repeat("a", maxStringLength+1)},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.request.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("Validate() error = nil, want invalid argument")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestLoginRequest_ToCommand_MapsVerificationFields(t *testing.T) {
	request := LoginRequest{Username: "user@example.com", VerificationCode: "123456", VerificationType: "email"}
	command := request.toCommand(httptest.NewRequest("POST", "/api/v1/auth/login", nil))
	if command.VerificationCode != "123456" || command.VerificationType != "email" {
		t.Fatalf("toCommand() verification fields = (%q,%q), want (123456,email)",
			command.VerificationCode, command.VerificationType)
	}
}

func TestLoginRequest_ToCommand_PasswordOnlyLeavesVerificationFieldsEmpty(t *testing.T) {
	request := LoginRequest{Username: "user", Password: "secret"}
	command := request.toCommand(httptest.NewRequest("POST", "/api/v1/auth/login", nil))
	if command.VerificationCode != "" || command.VerificationType != "" {
		t.Fatalf("toCommand() verification fields = (%q,%q), want empty",
			command.VerificationCode, command.VerificationType)
	}
}
