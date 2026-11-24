package users

import (
	"testing"

	"github.com/stretchr/testify/assert"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ---------------------------------------------------------------------------
// Boundary scenario tests for user CRUD operations
// ---------------------------------------------------------------------------

func TestCreateUserCommand_Boundary_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		cmd     CreateUserWithProfileCommand
		wantErr bool
	}{
		{
			name: "valid minimal user",
			cmd: CreateUserWithProfileCommand{
				CreateUserCommand: CreateUserCommand{
					OrganizationID: "org-001",
					Username:       "testuser",
					UserType:       "normal",
					Status:         "active",
				},
				Profile: CreateUserProfileCommand{
					DisplayName: "Test User",
				},
			},
			wantErr: false,
		},
		{
			name: "empty username",
			cmd: CreateUserWithProfileCommand{
				CreateUserCommand: CreateUserCommand{
					OrganizationID: "org-001",
					Username:       "",
					UserType:       "normal",
					Status:         "active",
				},
			},
			wantErr: true,
		},
		{
			name: "whitespace-only username",
			cmd: CreateUserWithProfileCommand{
				CreateUserCommand: CreateUserCommand{
					OrganizationID: "org-001",
					Username:       "   ",
					UserType:       "normal",
					Status:         "active",
				},
			},
			wantErr: true,
		},
		{
			name: "very long username",
			cmd: CreateUserWithProfileCommand{
				CreateUserCommand: CreateUserCommand{
					OrganizationID: "org-001",
					Username:       "a_very_long_username_that_exceeds_normal_limits_for_testing_boundary_conditions",
					UserType:       "normal",
					Status:         "active",
				},
				Profile: CreateUserProfileCommand{
					DisplayName: "Long Username User",
				},
			},
			wantErr: false, // length validation is at transport layer
		},
		{
			name: "special characters in username",
			cmd: CreateUserWithProfileCommand{
				CreateUserCommand: CreateUserCommand{
					OrganizationID: "org-001",
					Username:       "user@domain#test!",
					UserType:       "normal",
					Status:         "active",
				},
				Profile: CreateUserProfileCommand{
					DisplayName: "Special User",
				},
			},
			wantErr: false, // format validation is at transport layer
		},
		{
			name: "unicode username",
			cmd: CreateUserWithProfileCommand{
				CreateUserCommand: CreateUserCommand{
					OrganizationID: "org-001",
					Username:       "用户名",
					UserType:       "normal",
					Status:         "active",
				},
				Profile: CreateUserProfileCommand{
					DisplayName: "Unicode User",
				},
			},
			wantErr: false,
		},
		{
			name: "missing organization ID",
			cmd: CreateUserWithProfileCommand{
				CreateUserCommand: CreateUserCommand{
					Username: "testuser",
					UserType: "normal",
					Status:   "active",
				},
			},
			wantErr: true,
		},
		{
			name: "missing user type",
			cmd: CreateUserWithProfileCommand{
				CreateUserCommand: CreateUserCommand{
					OrganizationID: "org-001",
					Username:       "testuser",
					Status:         "active",
				},
			},
			wantErr: true,
		},
		{
			name: "missing status",
			cmd: CreateUserWithProfileCommand{
				CreateUserCommand: CreateUserCommand{
					OrganizationID: "org-001",
					Username:       "testuser",
					UserType:       "normal",
				},
			},
			wantErr: true,
		},
		{
			name: "admin user",
			cmd: CreateUserWithProfileCommand{
				CreateUserCommand: CreateUserCommand{
					OrganizationID: "org-001",
					Username:       "admin",
					UserType:       "admin",
					Status:         "active",
					IsAdmin:        true,
				},
				Profile: CreateUserProfileCommand{
					DisplayName: "Admin User",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCreateUserCommand(tt.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCreateUserCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateUserProfileCommand_EmailBoundary(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid email", "user@example.com", false},
		{"email with subdomain", "user@mail.example.com", false},
		{"email with plus", "user+tag@example.com", false},
		{"empty email", "", false}, // optional field
		{"missing @ sign", "userexample.com", true},
		{"missing domain", "user@", true},
		{"missing local part", "@example.com", true},
		{"double @", "user@@example.com", true},
		{"spaces in email", "user @example.com", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := CreateUserWithProfileCommand{
				CreateUserCommand: CreateUserCommand{
					OrganizationID: "org-001",
					Username:       "testuser",
					UserType:       "normal",
					Status:         "active",
				},
				Profile: CreateUserProfileCommand{
					DisplayName: "Test User",
					Email:       tt.email,
				},
			}
			err := validateCreateUserCommand(cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("email validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPatchUserCommand_Boundary(t *testing.T) {
	t.Run("all nil fields is valid", func(t *testing.T) {
		cmd := PatchUserCommand{ID: "user-001"}
		assert.NoError(t, validatePatchUserCommand(cmd))
	})

	t.Run("blank user type fails", func(t *testing.T) {
		blank := "  "
		cmd := PatchUserCommand{ID: "user-001", UserType: &blank}
		assert.Error(t, validatePatchUserCommand(cmd))
	})

	t.Run("valid user type passes", func(t *testing.T) {
		userType := "admin"
		cmd := PatchUserCommand{ID: "user-001", UserType: &userType}
		assert.NoError(t, validatePatchUserCommand(cmd))
	})

	t.Run("blank status fails", func(t *testing.T) {
		blank := "  "
		cmd := PatchUserCommand{ID: "user-001", Status: &blank}
		assert.Error(t, validatePatchUserCommand(cmd))
	})

	t.Run("valid status passes", func(t *testing.T) {
		status := "disabled"
		cmd := PatchUserCommand{ID: "user-001", Status: &status}
		assert.NoError(t, validatePatchUserCommand(cmd))
	})

	t.Run("status outside the users_status enum fails", func(t *testing.T) {
		// 枚举事实源：state-machines.yaml §users_status；旧词表 inactive/suspended
		// 必须被 app 层拒绝为 422，而不是直达 ck_users_status 违约成 500。
		for _, invalid := range []string{"inactive", "suspended", "forbidden", " Active"} {
			status := invalid
			cmd := PatchUserCommand{ID: "user-001", Status: &status}
			err := validatePatchUserCommand(cmd)
			assert.Error(t, err, "status %q must be rejected", invalid)
			var appErr *apperrors.Error
			if assert.ErrorAs(t, err, &appErr) {
				assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
			}
		}
	})

	t.Run("concurrent update with revision", func(t *testing.T) {
		cmd := PatchUserCommand{ID: "user-001", Revision: 5}
		assert.NoError(t, validatePatchUserCommand(cmd))
	})
}

func TestUpdateUserProfileCommand_Boundary(t *testing.T) {
	t.Run("all empty fields valid", func(t *testing.T) {
		cmd := UpdateUserProfileCommand{UserID: "user-001"}
		assert.NoError(t, validateUpdateUserProfileCommand(cmd))
	})

	t.Run("valid email", func(t *testing.T) {
		cmd := UpdateUserProfileCommand{UserID: "user-001", Email: "user@example.com"}
		assert.NoError(t, validateUpdateUserProfileCommand(cmd))
	})

	t.Run("invalid email", func(t *testing.T) {
		cmd := UpdateUserProfileCommand{UserID: "user-001", Email: "not-an-email"}
		assert.Error(t, validateUpdateUserProfileCommand(cmd))
	})

	t.Run("valid phone", func(t *testing.T) {
		cmd := UpdateUserProfileCommand{UserID: "user-001", Phone: "+8613800138000"}
		assert.NoError(t, validateUpdateUserProfileCommand(cmd))
	})

	t.Run("valid avatar URL", func(t *testing.T) {
		cmd := UpdateUserProfileCommand{UserID: "user-001", AvatarURL: "https://cdn.example.com/avatar.png"}
		assert.NoError(t, validateUpdateUserProfileCommand(cmd))
	})

	t.Run("invalid avatar URL", func(t *testing.T) {
		cmd := UpdateUserProfileCommand{UserID: "user-001", AvatarURL: "not-a-url"}
		assert.Error(t, validateUpdateUserProfileCommand(cmd))
	})

	t.Run("properties map", func(t *testing.T) {
		cmd := UpdateUserProfileCommand{
			UserID:     "user-001",
			Properties: map[string]any{"theme": "dark", "language": "zh"},
		}
		assert.NoError(t, validateUpdateUserProfileCommand(cmd))
	})
}

func TestUpdateUserCredentialsCommand_Boundary(t *testing.T) {
	t.Run("valid password", func(t *testing.T) {
		cmd := UpdateUserCredentialsCommand{UserID: "user-001", Password: "securepassword123"}
		assert.NoError(t, validateUpdateUserCredentialsCommand(cmd))
	})

	// 口令长度/复杂度由组织策略在 Service 内校验，命令层只校验字段组合。
	t.Run("password set requires policy validation at service layer", func(t *testing.T) {
		cmd := UpdateUserCredentialsCommand{UserID: "user-001", Password: "abc"}
		assert.NoError(t, validateUpdateUserCredentialsCommand(cmd))
	})

	t.Run("empty command fails", func(t *testing.T) {
		cmd := UpdateUserCredentialsCommand{UserID: "user-001"}
		assert.Error(t, validateUpdateUserCredentialsCommand(cmd))
	})

	t.Run("rotate access secret only", func(t *testing.T) {
		cmd := UpdateUserCredentialsCommand{UserID: "user-001", RotateAccessSecret: true}
		assert.NoError(t, validateUpdateUserCredentialsCommand(cmd))
	})

	t.Run("need update password only", func(t *testing.T) {
		needUpdatePassword := true
		cmd := UpdateUserCredentialsCommand{UserID: "user-001", NeedUpdatePassword: &needUpdatePassword}
		assert.NoError(t, validateUpdateUserCredentialsCommand(cmd))
	})

	t.Run("clear need update password only", func(t *testing.T) {
		needUpdatePassword := false
		cmd := UpdateUserCredentialsCommand{UserID: "user-001", NeedUpdatePassword: &needUpdatePassword}
		assert.NoError(t, validateUpdateUserCredentialsCommand(cmd))
	})
}

func TestReplaceUserIdentitiesCommand_Boundary(t *testing.T) {
	t.Run("valid single identity", func(t *testing.T) {
		cmds := []ReplaceUserIdentityCommand{
			{ProviderID: "github", IdentityType: "oauth", Subject: "12345"},
		}
		assert.NoError(t, validateReplaceUserIdentitiesCommands(cmds))
	})

	t.Run("missing provider ID", func(t *testing.T) {
		cmds := []ReplaceUserIdentityCommand{
			{IdentityType: "oauth", Subject: "12345"},
		}
		assert.Error(t, validateReplaceUserIdentitiesCommands(cmds))
	})

	t.Run("missing identity type", func(t *testing.T) {
		cmds := []ReplaceUserIdentityCommand{
			{ProviderID: "github", Subject: "12345"},
		}
		assert.Error(t, validateReplaceUserIdentitiesCommands(cmds))
	})

	t.Run("missing subject", func(t *testing.T) {
		cmds := []ReplaceUserIdentityCommand{
			{ProviderID: "github", IdentityType: "oauth"},
		}
		assert.Error(t, validateReplaceUserIdentitiesCommands(cmds))
	})

	t.Run("duplicate provider+subject", func(t *testing.T) {
		cmds := []ReplaceUserIdentityCommand{
			{ProviderID: "github", IdentityType: "oauth", Subject: "12345"},
			{ProviderID: "github", IdentityType: "oauth", Subject: "12345"},
		}
		assert.Error(t, validateReplaceUserIdentitiesCommands(cmds))
	})

	t.Run("duplicate ID", func(t *testing.T) {
		cmds := []ReplaceUserIdentityCommand{
			{ID: "idn-001", ProviderID: "github", IdentityType: "oauth", Subject: "12345"},
			{ID: "idn-001", ProviderID: "google", IdentityType: "oauth", Subject: "67890"},
		}
		assert.Error(t, validateReplaceUserIdentitiesCommands(cmds))
	})

	t.Run("multiple valid identities", func(t *testing.T) {
		cmds := []ReplaceUserIdentityCommand{
			{ProviderID: "github", IdentityType: "oauth", Subject: "12345"},
			{ProviderID: "google", IdentityType: "oauth", Subject: "67890"},
		}
		assert.NoError(t, validateReplaceUserIdentitiesCommands(cmds))
	})
}

func TestReplaceUserRoleBindingCommand_Boundary(t *testing.T) {
	t.Run("valid bindings", func(t *testing.T) {
		cmds := []ReplaceUserRoleBindingCommand{
			{RoleID: "role-001"}, {RoleID: "role-002"},
		}
		assert.NoError(t, validateReplaceUserRoleBindingCommands(cmds))
	})

	t.Run("empty role ID", func(t *testing.T) {
		cmds := []ReplaceUserRoleBindingCommand{{RoleID: ""}}
		assert.Error(t, validateReplaceUserRoleBindingCommands(cmds))
	})

	t.Run("duplicate role IDs", func(t *testing.T) {
		cmds := []ReplaceUserRoleBindingCommand{
			{RoleID: "role-001"}, {RoleID: "role-001"},
		}
		assert.Error(t, validateReplaceUserRoleBindingCommands(cmds))
	})
}

func TestReplaceUserGroupMembershipCommand_Boundary(t *testing.T) {
	t.Run("valid memberships", func(t *testing.T) {
		cmds := []ReplaceUserGroupMembershipCommand{
			{GroupID: "group-001"}, {GroupID: "group-002"},
		}
		assert.NoError(t, validateReplaceUserGroupMembershipCommands(cmds))
	})

	t.Run("empty group ID", func(t *testing.T) {
		cmds := []ReplaceUserGroupMembershipCommand{{GroupID: ""}}
		assert.Error(t, validateReplaceUserGroupMembershipCommands(cmds))
	})

	t.Run("duplicate group IDs", func(t *testing.T) {
		cmds := []ReplaceUserGroupMembershipCommand{
			{GroupID: "group-001"}, {GroupID: "group-001"},
		}
		assert.Error(t, validateReplaceUserGroupMembershipCommands(cmds))
	})
}

func TestReplaceUserMfaMethodCommand_Boundary(t *testing.T) {
	t.Run("valid methods", func(t *testing.T) {
		cmds := []ReplaceUserMfaMethodCommand{
			{MethodType: "totp"}, {MethodType: "sms"},
		}
		assert.NoError(t, validateReplaceUserMfaMethodsCommands(cmds))
	})

	t.Run("empty method type", func(t *testing.T) {
		cmds := []ReplaceUserMfaMethodCommand{{MethodType: ""}}
		assert.Error(t, validateReplaceUserMfaMethodsCommands(cmds))
	})

	t.Run("duplicate method types", func(t *testing.T) {
		cmds := []ReplaceUserMfaMethodCommand{
			{MethodType: "totp"}, {MethodType: "totp"},
		}
		assert.Error(t, validateReplaceUserMfaMethodsCommands(cmds))
	})
}
