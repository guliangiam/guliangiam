package users

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// assertUsersAppError pins the code and the HTTP status for the 400/422 split.
func assertUsersAppError(t *testing.T, err error, code apperrors.Code) *apperrors.Error {
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

func TestValidateCreateUserCommand(t *testing.T) {
	t.Run("valid_command", func(t *testing.T) {
		cmd := CreateUserWithProfileCommand{
			CreateUserCommand: CreateUserCommand{
				OrganizationID: "org-001",
				Username:       "testuser",
				UserType:       "normal",
				Status:         "active",
			},
			Profile: CreateUserProfileCommand{
				DisplayName: "Test User",
			},
		}
		assert.NoError(t, validateCreateUserCommand(cmd))
	})

	t.Run("missing_organization_id", func(t *testing.T) {
		cmd := CreateUserWithProfileCommand{
			CreateUserCommand: CreateUserCommand{
				Username: "testuser",
				UserType: "normal",
				Status:   "active",
			},
		}
		assert.Error(t, validateCreateUserCommand(cmd))
	})

	t.Run("missing_username", func(t *testing.T) {
		cmd := CreateUserWithProfileCommand{
			CreateUserCommand: CreateUserCommand{
				OrganizationID: "org-001",
				UserType:       "normal",
				Status:         "active",
			},
		}
		assert.Error(t, validateCreateUserCommand(cmd))
	})

	t.Run("missing_user_type", func(t *testing.T) {
		cmd := CreateUserWithProfileCommand{
			CreateUserCommand: CreateUserCommand{
				OrganizationID: "org-001",
				Username:       "testuser",
				Status:         "active",
			},
		}
		assert.Error(t, validateCreateUserCommand(cmd))
	})

	t.Run("missing_status", func(t *testing.T) {
		cmd := CreateUserWithProfileCommand{
			CreateUserCommand: CreateUserCommand{
				OrganizationID: "org-001",
				Username:       "testuser",
				UserType:       "normal",
			},
		}
		assert.Error(t, validateCreateUserCommand(cmd))
	})

	t.Run("missing_display_name", func(t *testing.T) {
		cmd := CreateUserWithProfileCommand{
			CreateUserCommand: CreateUserCommand{
				OrganizationID: "org-001",
				Username:       "testuser",
				UserType:       "normal",
				Status:         "active",
			},
			Profile: CreateUserProfileCommand{
				DisplayName: "",
			},
		}
		assert.Error(t, validateCreateUserCommand(cmd))
	})

	t.Run("valid_display_name", func(t *testing.T) {
		cmd := CreateUserWithProfileCommand{
			CreateUserCommand: CreateUserCommand{
				OrganizationID: "org-001",
				Username:       "testuser",
				UserType:       "normal",
				Status:         "active",
			},
			Profile: CreateUserProfileCommand{
				DisplayName: "Test User",
			},
		}
		assert.NoError(t, validateCreateUserCommand(cmd))
	})

	t.Run("invalid_country_code", func(t *testing.T) {
		cmd := CreateUserWithProfileCommand{
			CreateUserCommand: CreateUserCommand{
				OrganizationID: "org-001",
				Username:       "testuser",
				UserType:       "normal",
				Status:         "active",
			},
			Profile: CreateUserProfileCommand{
				DisplayName: "Test User",
				CountryCode: "+86",
			},
		}
		assertUsersAppError(t, validateCreateUserCommand(cmd), apperrors.CodeUnprocessableEntity)
	})

	t.Run("valid_initial_password", func(t *testing.T) {
		cmd := CreateUserWithProfileCommand{
			CreateUserCommand: CreateUserCommand{
				OrganizationID: "org-001",
				Username:       "testuser",
				UserType:       "normal",
				Status:         "active",
			},
			Profile:         CreateUserProfileCommand{DisplayName: "Test User"},
			InitialPassword: "Str0ng!Pass1",
		}
		assert.NoError(t, validateCreateUserCommand(cmd))
	})

	t.Run("invalid_email", func(t *testing.T) {
		cmd := CreateUserWithProfileCommand{
			CreateUserCommand: CreateUserCommand{
				OrganizationID: "org-001",
				Username:       "testuser",
				UserType:       "normal",
				Status:         "active",
			},
			Profile: CreateUserProfileCommand{
				DisplayName: "Test User",
				Email:       "invalid-email",
			},
		}
		assertUsersAppError(t, validateCreateUserCommand(cmd), apperrors.CodeUnprocessableEntity)
	})

	t.Run("invalid_homepage", func(t *testing.T) {
		cmd := CreateUserWithProfileCommand{
			CreateUserCommand: CreateUserCommand{
				OrganizationID: "org-001",
				Username:       "testuser",
				UserType:       "normal",
				Status:         "active",
			},
			Profile: CreateUserProfileCommand{
				DisplayName: "Test User",
				Homepage:    "not-a-url",
			},
		}
		assertUsersAppError(t, validateCreateUserCommand(cmd), apperrors.CodeUnprocessableEntity)
	})

	t.Run("with_role_bindings", func(t *testing.T) {
		cmd := CreateUserWithProfileCommand{
			CreateUserCommand: CreateUserCommand{
				OrganizationID: "org-001",
				Username:       "testuser",
				UserType:       "normal",
				Status:         "active",
			},
			Profile: CreateUserProfileCommand{
				DisplayName: "Test User",
			},
			RoleBindingIDs: []string{"role-001", "role-002"},
		}
		assert.NoError(t, validateCreateUserCommand(cmd))
	})
}

func TestValidatePatchUserCommand(t *testing.T) {
	t.Run("nil_fields_valid", func(t *testing.T) {
		cmd := PatchUserCommand{}
		assert.NoError(t, validatePatchUserCommand(cmd))
	})

	t.Run("blank_user_type_fails", func(t *testing.T) {
		blank := "  "
		cmd := PatchUserCommand{UserType: &blank}
		assert.Error(t, validatePatchUserCommand(cmd))
	})

	t.Run("valid_user_type", func(t *testing.T) {
		userType := "admin"
		cmd := PatchUserCommand{UserType: &userType}
		assert.NoError(t, validatePatchUserCommand(cmd))
	})

	t.Run("blank_status_fails", func(t *testing.T) {
		blank := "  "
		cmd := PatchUserCommand{Status: &blank}
		assert.Error(t, validatePatchUserCommand(cmd))
	})
}

func TestValidateUpdateUserProfileCommand(t *testing.T) {
	t.Run("empty_fields_valid", func(t *testing.T) {
		cmd := UpdateUserProfileCommand{}
		assert.NoError(t, validateUpdateUserProfileCommand(cmd))
	})

	t.Run("valid_email", func(t *testing.T) {
		cmd := UpdateUserProfileCommand{Email: "user@example.com"}
		assert.NoError(t, validateUpdateUserProfileCommand(cmd))
	})

	t.Run("invalid_email", func(t *testing.T) {
		cmd := UpdateUserProfileCommand{Email: "not-an-email"}
		assertUsersAppError(t, validateUpdateUserProfileCommand(cmd), apperrors.CodeUnprocessableEntity)
	})

	t.Run("valid_avatar_url", func(t *testing.T) {
		cmd := UpdateUserProfileCommand{AvatarURL: "https://example.com/avatar.png"}
		assert.NoError(t, validateUpdateUserProfileCommand(cmd))
	})

	t.Run("invalid_avatar_url", func(t *testing.T) {
		cmd := UpdateUserProfileCommand{AvatarURL: "not-a-url"}
		assertUsersAppError(t, validateUpdateUserProfileCommand(cmd), apperrors.CodeUnprocessableEntity)
	})

	t.Run("invalid_homepage_property", func(t *testing.T) {
		cmd := UpdateUserProfileCommand{
			Properties: map[string]any{"homepage": "not-a-url"},
		}
		assertUsersAppError(t, validateUpdateUserProfileCommand(cmd), apperrors.CodeUnprocessableEntity)
	})

	t.Run("homepage_not_a_string", func(t *testing.T) {
		cmd := UpdateUserProfileCommand{
			Properties: map[string]any{"homepage": 42},
		}
		appErr := assertUsersAppError(t, validateUpdateUserProfileCommand(cmd), apperrors.CodeUnprocessableEntity)
		if appErr.Meta["expected"] != "string" {
			t.Fatalf("meta = %v, want expected=string", appErr.Meta)
		}
	})

	t.Run("valid_iso_country_and_e164_phone", func(t *testing.T) {
		cmd := UpdateUserProfileCommand{CountryCode: "CN", Phone: "+8613800138000"}
		assert.NoError(t, validateUpdateUserProfileCommand(cmd))
	})

	t.Run("dial_code_is_not_country_code", func(t *testing.T) {
		cmd := UpdateUserProfileCommand{CountryCode: "+86"}
		appErr := assertUsersAppError(t, validateUpdateUserProfileCommand(cmd), apperrors.CodeUnprocessableEntity)
		if appErr.Meta["format"] != "iso3166-alpha2" {
			t.Fatalf("meta = %v, want iso3166-alpha2", appErr.Meta)
		}
	})

	t.Run("invalid_phone", func(t *testing.T) {
		cmd := UpdateUserProfileCommand{Phone: "CN13800138000"}
		appErr := assertUsersAppError(t, validateUpdateUserProfileCommand(cmd), apperrors.CodeUnprocessableEntity)
		if appErr.Meta["format"] != "e164" {
			t.Fatalf("meta = %v, want e164", appErr.Meta)
		}
	})
}

func TestValidateUpdateUserProfileCommand_DocumentFields(t *testing.T) {
	long := func(n int) string { return strings.Repeat("x", n) }

	t.Run("valid_document_fields", func(t *testing.T) {
		cmd := UpdateUserProfileCommand{
			IDCardExpiry:     "2030-12-31",
			IssueCountry:     "CN",
			PassportNo:       "E12345678",
			DriverLicenseNo:  "D1234567",
			IssuingAuthority: "上海市公安局浦东分局",
		}
		assert.NoError(t, validateUpdateUserProfileCommand(cmd))
	})

	t.Run("empty_document_fields_pass", func(t *testing.T) {
		cmd := UpdateUserProfileCommand{}
		assert.NoError(t, validateUpdateUserProfileCommand(cmd))
	})

	dateCases := []struct {
		name  string
		value string
		valid bool
	}{
		{"iso_date", "2030-12-31", true},
		{"compact_date_rejected", "20301231", false},
		{"slash_date_rejected", "2030/12/31", false},
		{"unpadded_rejected", "2030-1-31", false},
		{"impossible_day_rejected", "2030-02-30", false},
		{"month_overflow_rejected", "2030-13-01", false},
		{"datetime_rejected", "2030-12-31T00:00:00", false},
	}
	for _, tc := range dateCases {
		tc := tc
		t.Run("expiry_"+tc.name, func(t *testing.T) {
			err := validateUpdateUserProfileCommand(UpdateUserProfileCommand{IDCardExpiry: tc.value})
			if tc.valid {
				assert.NoError(t, err)
				return
			}
			appErr := assertUsersAppError(t, err, apperrors.CodeUnprocessableEntity)
			assert.Equal(t, "idCardExpiry", appErr.Meta["field"])
			assert.Equal(t, "date", appErr.Meta["format"])
		})
	}

	lengthCases := []struct {
		field string
		max   int
	}{
		{"issueCountry", 8},
		{"passportNo", 64},
		{"driverLicenseNo", 64},
		{"issuingAuthority", 255},
	}
	for _, tc := range lengthCases {
		tc := tc
		t.Run(tc.field+"_at_limit", func(t *testing.T) {
			cmd := UpdateUserProfileCommand{}
			switch tc.field {
			case "issueCountry":
				cmd.IssueCountry = long(tc.max)
			case "passportNo":
				cmd.PassportNo = long(tc.max)
			case "driverLicenseNo":
				cmd.DriverLicenseNo = long(tc.max)
			case "issuingAuthority":
				cmd.IssuingAuthority = long(tc.max)
			}
			assert.NoError(t, validateUpdateUserProfileCommand(cmd))
		})
		t.Run(tc.field+"_over_limit", func(t *testing.T) {
			cmd := UpdateUserProfileCommand{}
			switch tc.field {
			case "issueCountry":
				cmd.IssueCountry = long(tc.max + 1)
			case "passportNo":
				cmd.PassportNo = long(tc.max + 1)
			case "driverLicenseNo":
				cmd.DriverLicenseNo = long(tc.max + 1)
			case "issuingAuthority":
				cmd.IssuingAuthority = long(tc.max + 1)
			}
			appErr := assertUsersAppError(t, validateUpdateUserProfileCommand(cmd), apperrors.CodeUnprocessableEntity)
			assert.Equal(t, tc.max, appErr.Meta["max"])
		})
	}

	t.Run("issuingAuthority_rune_counted", func(t *testing.T) {
		// 255 个汉字（765 字节）按字符数计应通过 VARCHAR(255) 限制。
		cmd := UpdateUserProfileCommand{IssuingAuthority: strings.Repeat("局", 255)}
		assert.NoError(t, validateUpdateUserProfileCommand(cmd))
		cmd.IssuingAuthority = strings.Repeat("局", 256)
		assertUsersAppError(t, validateUpdateUserProfileCommand(cmd), apperrors.CodeUnprocessableEntity)
	})
}

func TestValidateUpdateUserCredentialsCommand(t *testing.T) {
	t.Run("valid_password", func(t *testing.T) {
		cmd := UpdateUserCredentialsCommand{Password: "securepassword123"}
		assert.NoError(t, validateUpdateUserCredentialsCommand(cmd))
	})

	// 口令长度由组织策略在 Service 内校验；命令层不再持有 12 位常量。
	t.Run("short_password_deferred_to_policy", func(t *testing.T) {
		cmd := UpdateUserCredentialsCommand{Password: "short"}
		assert.NoError(t, validateUpdateUserCredentialsCommand(cmd))
	})

	t.Run("rotate_access_secret", func(t *testing.T) {
		cmd := UpdateUserCredentialsCommand{RotateAccessSecret: true}
		assert.NoError(t, validateUpdateUserCredentialsCommand(cmd))
	})

	t.Run("need_update_password", func(t *testing.T) {
		needUpdatePassword := true
		cmd := UpdateUserCredentialsCommand{NeedUpdatePassword: &needUpdatePassword}
		assert.NoError(t, validateUpdateUserCredentialsCommand(cmd))
	})

	t.Run("empty_command_fails", func(t *testing.T) {
		cmd := UpdateUserCredentialsCommand{}
		assertUsersAppError(t, validateUpdateUserCredentialsCommand(cmd), apperrors.CodeInvalidArgument)
	})
}

func TestValidateOptionalEmail(t *testing.T) {
	t.Run("empty_string_passes", func(t *testing.T) {
		assert.NoError(t, validateOptionalEmail("", "email"))
	})

	t.Run("valid_email_passes", func(t *testing.T) {
		assert.NoError(t, validateOptionalEmail("user@example.com", "email"))
	})

	t.Run("invalid_email_fails", func(t *testing.T) {
		appErr := assertUsersAppError(t, validateOptionalEmail("invalid", "email"), apperrors.CodeUnprocessableEntity)
		if appErr.Meta["format"] != "email" {
			t.Fatalf("meta = %v, want format=email", appErr.Meta)
		}
	})
}

func TestValidateOptionalURL(t *testing.T) {
	t.Run("empty_string_passes", func(t *testing.T) {
		assert.NoError(t, validateOptionalURL("", "url"))
	})

	t.Run("valid_url_passes", func(t *testing.T) {
		assert.NoError(t, validateOptionalURL("https://example.com", "url"))
	})

	t.Run("relative_url_fails", func(t *testing.T) {
		assertUsersAppError(t, validateOptionalURL("/path", "url"), apperrors.CodeUnprocessableEntity)
	})

	t.Run("invalid_url_fails", func(t *testing.T) {
		assertUsersAppError(t, validateOptionalURL("://bad", "url"), apperrors.CodeUnprocessableEntity)
	})
}

func TestValidateReplaceUserIdentitiesCommands(t *testing.T) {
	t.Run("valid_single_identity", func(t *testing.T) {
		cmds := []ReplaceUserIdentityCommand{
			{ProviderID: "github", IdentityType: "oauth", Subject: "12345"},
		}
		assert.NoError(t, validateReplaceUserIdentitiesCommands(cmds))
	})

	t.Run("missing_provider_id", func(t *testing.T) {
		cmds := []ReplaceUserIdentityCommand{
			{IdentityType: "oauth", Subject: "12345"},
		}
		assertUsersAppError(t, validateReplaceUserIdentitiesCommands(cmds), apperrors.CodeInvalidArgument)
	})

	t.Run("duplicate_provider_subject", func(t *testing.T) {
		cmds := []ReplaceUserIdentityCommand{
			{ProviderID: "github", IdentityType: "oauth", Subject: "12345"},
			{ProviderID: "github", IdentityType: "oauth", Subject: "12345"},
		}
		assertUsersAppError(t, validateReplaceUserIdentitiesCommands(cmds), apperrors.CodeUnprocessableEntity)
	})

	t.Run("duplicate_id", func(t *testing.T) {
		cmds := []ReplaceUserIdentityCommand{
			{ID: "idn-001", ProviderID: "github", IdentityType: "oauth", Subject: "12345"},
			{ID: "idn-001", ProviderID: "google", IdentityType: "oauth", Subject: "67890"},
		}
		assertUsersAppError(t, validateReplaceUserIdentitiesCommands(cmds), apperrors.CodeUnprocessableEntity)
	})
}

func TestValidateReplaceUserMfaMethodsCommands(t *testing.T) {
	t.Run("valid_methods", func(t *testing.T) {
		cmds := []ReplaceUserMfaMethodCommand{
			{MethodType: "totp"},
			{MethodType: "sms"},
		}
		assert.NoError(t, validateReplaceUserMfaMethodsCommands(cmds))
	})

	t.Run("duplicate_method_type", func(t *testing.T) {
		cmds := []ReplaceUserMfaMethodCommand{
			{MethodType: "totp"},
			{MethodType: "totp"},
		}
		assertUsersAppError(t, validateReplaceUserMfaMethodsCommands(cmds), apperrors.CodeUnprocessableEntity)
	})

	t.Run("empty_method_type", func(t *testing.T) {
		cmds := []ReplaceUserMfaMethodCommand{
			{MethodType: ""},
		}
		assertUsersAppError(t, validateReplaceUserMfaMethodsCommands(cmds), apperrors.CodeInvalidArgument)
	})
}

func TestValidateReplaceUserRoleBindingCommands(t *testing.T) {
	t.Run("valid_bindings", func(t *testing.T) {
		cmds := []ReplaceUserRoleBindingCommand{
			{RoleID: "role-001"},
			{RoleID: "role-002"},
		}
		assert.NoError(t, validateReplaceUserRoleBindingCommands(cmds))
	})

	t.Run("duplicate_role_id", func(t *testing.T) {
		cmds := []ReplaceUserRoleBindingCommand{
			{RoleID: "role-001"},
			{RoleID: "role-001"},
		}
		assertUsersAppError(t, validateReplaceUserRoleBindingCommands(cmds), apperrors.CodeUnprocessableEntity)
	})

	t.Run("empty_role_id", func(t *testing.T) {
		cmds := []ReplaceUserRoleBindingCommand{
			{RoleID: ""},
		}
		assertUsersAppError(t, validateReplaceUserRoleBindingCommands(cmds), apperrors.CodeInvalidArgument)
	})
}

func TestValidateReplaceUserGroupMembershipCommands(t *testing.T) {
	t.Run("valid_memberships", func(t *testing.T) {
		cmds := []ReplaceUserGroupMembershipCommand{
			{GroupID: "group-001"},
			{GroupID: "group-002"},
		}
		assert.NoError(t, validateReplaceUserGroupMembershipCommands(cmds))
	})

	t.Run("duplicate_group_id", func(t *testing.T) {
		cmds := []ReplaceUserGroupMembershipCommand{
			{GroupID: "group-001"},
			{GroupID: "group-001"},
		}
		assertUsersAppError(t, validateReplaceUserGroupMembershipCommands(cmds), apperrors.CodeUnprocessableEntity)
	})
}

func TestValidateUpdateUserPreferredMfaCommand(t *testing.T) {
	t.Run("valid_command", func(t *testing.T) {
		cmd := UpdateUserPreferredMfaCommand{UserID: "user-001", MfaType: "totp"}
		assert.NoError(t, validateUpdateUserPreferredMfaCommand(cmd))
	})

	t.Run("missing_user_id", func(t *testing.T) {
		cmd := UpdateUserPreferredMfaCommand{MfaType: "totp"}
		assert.Error(t, validateUpdateUserPreferredMfaCommand(cmd))
	})

	t.Run("missing_mfa_type", func(t *testing.T) {
		cmd := UpdateUserPreferredMfaCommand{UserID: "user-001"}
		assert.Error(t, validateUpdateUserPreferredMfaCommand(cmd))
	})
}

func TestValidateEnableUserMfaSetupCommand(t *testing.T) {
	t.Run("valid_with_recovery_codes", func(t *testing.T) {
		cmd := EnableUserMfaSetupCommand{
			UserID:        "user-001",
			MfaType:       "totp",
			RecoveryCodes: []string{"code1", "code2"},
		}
		assert.NoError(t, validateEnableUserMfaSetupCommand(cmd))
	})

	t.Run("missing_recovery_codes", func(t *testing.T) {
		cmd := EnableUserMfaSetupCommand{
			UserID:  "user-001",
			MfaType: "totp",
		}
		assert.Error(t, validateEnableUserMfaSetupCommand(cmd))
	})
}
