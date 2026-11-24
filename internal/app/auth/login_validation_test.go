package auth

import (
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestValidateLoginCommand_Valid(t *testing.T) {
	tests := []struct {
		name    string
		command LoginCommand
	}{
		{
			name: "basic login",
			command: LoginCommand{
				OrganizationSlug: "acme",
				Username:         "alice",
				Password:         "P@ssw0rd",
				ApplicationID:    "app-1",
			},
		},
		{
			name: "login with challenge ID",
			command: LoginCommand{
				ChallengeID: "challenge-1",
				Passcode:    "123456",
			},
		},
		{
			name: "login with MFA push",
			command: LoginCommand{
				ChallengeID: "challenge-1",
				MfaType:     "push",
			},
		},
		{
			name: "login with recovery code",
			command: LoginCommand{
				ChallengeID:  "challenge-1",
				RecoveryCode: "ABCD-EFGH",
			},
		},
		{
			name: "login with MFA remember",
			command: LoginCommand{
				OrganizationSlug:  "acme",
				Username:          "alice",
				Password:          "P@ssw0rd",
				ApplicationID:     "app-1",
				Passcode:          "123456",
				EnableMfaRemember: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLoginCommand(tt.command)
			if err != nil {
				t.Errorf("validateLoginCommand() error = %v, want nil", err)
			}
		})
	}
}

func TestValidateLoginCommand_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		command LoginCommand
		wantErr bool
		errCode apperrors.Code
	}{
		{
			name: "missing organizationSlug",
			command: LoginCommand{
				Username:      "alice",
				Password:      "P@ssw0rd",
				ApplicationID: "app-1",
			},
			wantErr: true,
			errCode: apperrors.CodeInvalidArgument,
		},
		{
			name: "missing username",
			command: LoginCommand{
				OrganizationSlug: "acme",
				Password:         "P@ssw0rd",
				ApplicationID:    "app-1",
			},
			wantErr: true,
			errCode: apperrors.CodeInvalidArgument,
		},
		{
			name: "missing password",
			command: LoginCommand{
				OrganizationSlug: "acme",
				Username:         "alice",
				ApplicationID:    "app-1",
			},
			wantErr: true,
			errCode: apperrors.CodeInvalidArgument,
		},
		{
			name: "console login without applicationId resolves the default",
			command: LoginCommand{
				OrganizationSlug: "acme",
				Username:         "alice",
				Password:         "P@ssw0rd",
			},
			wantErr: false,
		},
		{
			name: "protocol login without applicationId",
			command: LoginCommand{
				OrganizationSlug: "acme",
				Username:         "alice",
				Password:         "P@ssw0rd",
				ResponseType:     "code",
			},
			wantErr: true,
			errCode: apperrors.CodeInvalidArgument,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLoginCommand(tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateLoginCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				typed, ok := err.(*apperrors.Error)
				if !ok {
					t.Fatalf("error type = %T, want *apperrors.Error", err)
				}
				if typed.Code != tt.errCode {
					t.Errorf("error code = %q, want %q", typed.Code, tt.errCode)
				}
			}
		})
	}
}

func TestValidateLoginCommand_PasscodeAndRecoveryCodeConflict(t *testing.T) {
	err := validateLoginCommand(LoginCommand{
		OrganizationSlug: "acme",
		Username:         "alice",
		Password:         "P@ssw0rd",
		ApplicationID:    "app-1",
		Passcode:         "123456",
		RecoveryCode:     "ABCD-EFGH",
	})
	if err == nil {
		t.Fatal("expected error when both passcode and recoveryCode are provided")
	}
	typed, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("error type = %T, want *apperrors.Error", err)
	}
	if typed.Code != apperrors.CodeInvalidArgument {
		t.Errorf("error code = %q, want %q", typed.Code, apperrors.CodeInvalidArgument)
	}
}

func TestValidateLoginCommand_ChallengeRequiresPasscodeOrRecovery(t *testing.T) {
	err := validateLoginCommand(LoginCommand{
		ChallengeID: "challenge-1",
	})
	if err == nil {
		t.Fatal("expected error when challengeId is provided without passcode or recoveryCode")
	}
	typed, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("error type = %T, want *apperrors.Error", err)
	}
	if typed.Code != apperrors.CodeInvalidArgument {
		t.Errorf("error code = %q, want %q", typed.Code, apperrors.CodeInvalidArgument)
	}
}

func TestValidateLoginCommand_MfaTypeRequiresPasscode(t *testing.T) {
	err := validateLoginCommand(LoginCommand{
		OrganizationSlug: "acme",
		Username:         "alice",
		Password:         "P@ssw0rd",
		ApplicationID:    "app-1",
		MfaType:          "totp",
	})
	if err == nil {
		t.Fatal("expected error when mfaType is provided without passcode")
	}
	typed, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("error type = %T, want *apperrors.Error", err)
	}
	if typed.Code != apperrors.CodeInvalidArgument {
		t.Errorf("error code = %q, want %q", typed.Code, apperrors.CodeInvalidArgument)
	}
}

func TestValidateLoginCommand_EnableMfaRememberRequiresPasscode(t *testing.T) {
	err := validateLoginCommand(LoginCommand{
		OrganizationSlug:  "acme",
		Username:          "alice",
		Password:          "P@ssw0rd",
		ApplicationID:     "app-1",
		EnableMfaRemember: true,
	})
	if err == nil {
		t.Fatal("expected error when enableMfaRemember is set without passcode or recoveryCode")
	}
	typed, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("error type = %T, want *apperrors.Error", err)
	}
	if typed.Code != apperrors.CodeInvalidArgument {
		t.Errorf("error code = %q, want %q", typed.Code, apperrors.CodeInvalidArgument)
	}
}

func TestValidateLoginCommand_ChallengeWithPasscode(t *testing.T) {
	err := validateLoginCommand(LoginCommand{
		ChallengeID: "challenge-1",
		Passcode:    "123456",
	})
	if err != nil {
		t.Errorf("validateLoginCommand() error = %v, want nil", err)
	}
}

func TestValidateLoginCommand_PushMfaWithoutPasscode(t *testing.T) {
	err := validateLoginCommand(LoginCommand{
		OrganizationSlug: "acme",
		Username:         "alice",
		Password:         "P@ssw0rd",
		ApplicationID:    "app-1",
		MfaType:          "push",
	})
	if err != nil {
		t.Errorf("validateLoginCommand() error = %v, want nil for push MFA without passcode", err)
	}
}
