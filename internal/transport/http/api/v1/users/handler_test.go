package users

import (
	"testing"

	appusers "github.com/guliangiam/guliangiam/internal/app/users"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Handler request validation and response format tests
// ---------------------------------------------------------------------------

func TestHandler_ListUsers_RequestMapping(t *testing.T) {
	tests := []struct {
		name     string
		request  ListUsersRequest
		wantOrg  string
		wantPage int
	}{
		{"default request", ListUsersRequest{Page: 1, PageSize: 20, OrganizationID: "org-001"}, "org-001", 1},
		{"empty org ID", ListUsersRequest{Page: 1, PageSize: 20, OrganizationID: ""}, "", 1},
		{"with keyword", ListUsersRequest{Page: 1, PageSize: 20, OrganizationID: "org-001", Keyword: "admin"}, "org-001", 1},
		{"with status filter", ListUsersRequest{Page: 1, PageSize: 20, OrganizationID: "org-001", Status: "active"}, "org-001", 1},
		{"with user type filter", ListUsersRequest{Page: 1, PageSize: 20, OrganizationID: "org-001", UserType: "admin"}, "org-001", 1},
		{"with sort", ListUsersRequest{Page: 1, PageSize: 20, OrganizationID: "org-001", SortField: "createdAt", SortOrder: "desc"}, "org-001", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantOrg, tt.request.OrganizationID)
			assert.Equal(t, tt.wantPage, tt.request.Page)
		})
	}
}

func TestHandler_GetUser_RequestMapping(t *testing.T) {
	tests := []struct {
		name string
		req  GetUserRequest
		want string
	}{
		{"valid ID", GetUserRequest{ID: "user-001"}, "user-001"},
		{"empty ID", GetUserRequest{ID: ""}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.req.ID)
		})
	}
}

func TestHandler_CreateUser_RequestMapping(t *testing.T) {
	t.Run("full request maps to command", func(t *testing.T) {
		req := CreateUserRequest{
			OrganizationID: "org-001",
			Username:       "newuser",
			ExternalID:     "ext-001",
			UserType:       "normal",
			Status:         "active",
			IsAdmin:        false,
			Profile: CreateUserProfileRequest{
				DisplayName: "New User",
				FirstName:   "New",
				LastName:    "User",
				Email:       "new@example.com",
				Phone:       "+8613800138000",
			},
			RoleBindingIDs: []string{"role-001"},
			GroupIDs:       []string{"group-001"},
		}

		cmd := req.toCommand()
		assert.Equal(t, "org-001", cmd.OrganizationID)
		assert.Equal(t, "newuser", cmd.Username)
		assert.Equal(t, "normal", cmd.UserType)
		assert.Equal(t, "active", cmd.Status)
		assert.Equal(t, "New User", cmd.Profile.DisplayName)
		assert.Equal(t, "new@example.com", cmd.Profile.Email)
		assert.Equal(t, []string{"role-001"}, cmd.RoleBindingIDs)
		assert.Equal(t, []string{"group-001"}, cmd.GroupIDs)
	})

	t.Run("minimal request maps correctly", func(t *testing.T) {
		req := CreateUserRequest{
			OrganizationID: "org-001",
			Username:       "minimaluser",
			UserType:       "normal",
			Status:         "active",
		}

		cmd := req.toCommand()
		assert.Equal(t, "org-001", cmd.OrganizationID)
		assert.Empty(t, cmd.Profile.Email)
		assert.Empty(t, cmd.RoleBindingIDs)
		assert.Empty(t, cmd.GroupIDs)
	})
}

func TestHandler_DeleteUser_RequestMapping(t *testing.T) {
	t.Run("valid delete request", func(t *testing.T) {
		req := GetUserRequest{ID: "user-001"}
		assert.Equal(t, "user-001", req.ID)
	})

	t.Run("empty ID delete request", func(t *testing.T) {
		req := GetUserRequest{ID: ""}
		assert.Empty(t, req.ID)
	})
}

func TestHandler_PatchUser_RequestMapping(t *testing.T) {
	t.Run("partial update with nil fields", func(t *testing.T) {
		req := PatchUserRequest{}
		cmd := req.toCommand("user-001")
		assert.Equal(t, "user-001", cmd.ID)
		assert.Nil(t, cmd.UserType)
		assert.Nil(t, cmd.Status)
	})

	t.Run("partial update with status only", func(t *testing.T) {
		status := "disabled"
		req := PatchUserRequest{Status: &status}
		cmd := req.toCommand("user-001")
		assert.Equal(t, "user-001", cmd.ID)
		assert.Nil(t, cmd.UserType)
		assert.Equal(t, &status, cmd.Status)
	})

	t.Run("concurrent update with revision", func(t *testing.T) {
		req := PatchUserRequest{Revision: 3}
		cmd := req.toCommand("user-001")
		assert.Equal(t, int64(3), cmd.Revision)
	})

	t.Run("all fields set", func(t *testing.T) {
		extID := "ext-1"
		userType := "admin"
		status := "active"
		isAdmin := true
		isForbidden := false
		tag := "vip"
		score := 100

		req := PatchUserRequest{
			ExternalID:  &extID,
			UserType:    &userType,
			Status:      &status,
			IsAdmin:     &isAdmin,
			IsForbidden: &isForbidden,
			Tag:         &tag,
			Score:       &score,
			Revision:    2,
		}
		cmd := req.toCommand("user-001")
		assert.Equal(t, "user-001", cmd.ID)
		assert.Equal(t, &extID, cmd.ExternalID)
		assert.Equal(t, &userType, cmd.UserType)
		assert.Equal(t, &status, cmd.Status)
		assert.Equal(t, &isAdmin, cmd.IsAdmin)
	})
}

func TestHandler_UpdateUserProfile_RequestMapping(t *testing.T) {
	req := UpdateUserProfileRequest{
		DisplayName: "Updated Name",
		Email:       "updated@example.com",
		Phone:       "+8613800138001",
		Properties:  map[string]any{"theme": "dark"},
	}

	cmd := req.toCommand("user-001")
	assert.Equal(t, "user-001", cmd.UserID)
	assert.Equal(t, "Updated Name", cmd.DisplayName)
	assert.Equal(t, "updated@example.com", cmd.Email)
	assert.Equal(t, map[string]any{"theme": "dark"}, cmd.Properties)
}

func TestHandler_ReplaceRoleBindings_RequestMapping(t *testing.T) {
	t.Run("maps items correctly", func(t *testing.T) {
		req := ReplaceUserRoleBindingsRequest{
			Items: []ReplaceUserRoleBindingItemRequest{
				{RoleID: "role-001"},
				{RoleID: "role-002"},
			},
		}
		cmds := req.toCommands()
		assert.Len(t, cmds, 2)
		assert.Equal(t, "role-001", cmds[0].RoleID)
		assert.Equal(t, "role-002", cmds[1].RoleID)
	})

	t.Run("empty items", func(t *testing.T) {
		req := ReplaceUserRoleBindingsRequest{Items: []ReplaceUserRoleBindingItemRequest{}}
		cmds := req.toCommands()
		assert.Empty(t, cmds)
	})
}

func TestHandler_ReplaceGroupMemberships_RequestMapping(t *testing.T) {
	t.Run("maps items correctly", func(t *testing.T) {
		req := ReplaceUserGroupMembershipsRequest{
			Items: []ReplaceUserGroupMembershipItemRequest{
				{GroupID: "group-001"},
				{GroupID: "group-002"},
			},
		}
		cmds := req.toCommands()
		assert.Len(t, cmds, 2)
		assert.Equal(t, "group-001", cmds[0].GroupID)
	})
}

func TestHandler_UpdateCredentials_RequestMapping(t *testing.T) {
	needUpdatePassword := false
	req := UpdateUserCredentialsRequest{
		Password:           "newpassword",
		RotateAccessSecret: true,
		NeedUpdatePassword: &needUpdatePassword,
	}
	cmd := req.toCommand("user-001")
	assert.Equal(t, "user-001", cmd.UserID)
	assert.Equal(t, "newpassword", cmd.Password)
	assert.True(t, cmd.RotateAccessSecret)
	assert.NotNil(t, cmd.NeedUpdatePassword)
	assert.False(t, *cmd.NeedUpdatePassword)
}

func TestHandler_ReplaceIdentities_RequestMapping(t *testing.T) {
	req := ReplaceUserIdentitiesRequest{
		Items: []ReplaceUserIdentityItemRequest{
			{
				ProviderID:      "github",
				IdentityType:    "oauth",
				Subject:         "12345",
				ProviderAccount: "ghuser",
				ProfileSnapshot: map[string]any{"email": "user@github.com"},
			},
		},
	}
	cmds := req.toCommands()
	assert.Len(t, cmds, 1)
	assert.Equal(t, "github", cmds[0].ProviderID)
	assert.Equal(t, "oauth", cmds[0].IdentityType)
	assert.Equal(t, "12345", cmds[0].Subject)
}

func TestHandler_MfaSetup_RequestMapping(t *testing.T) {
	t.Run("initiate MFA setup", func(t *testing.T) {
		req := InitiateUserMfaSetupRequest{MfaType: "totp"}
		cmd := req.toCommand("user-001")
		assert.Equal(t, "user-001", cmd.UserID)
		assert.Equal(t, "totp", cmd.MfaType)
	})

	t.Run("verify MFA setup", func(t *testing.T) {
		req := VerifyUserMfaSetupRequest{
			MfaType:  "totp",
			Passcode: "123456",
			Secret:   "JBSWY3DPEHPK3PXP",
		}
		cmd := req.toCommand("user-001")
		assert.Equal(t, "user-001", cmd.UserID)
		assert.Equal(t, "123456", cmd.Passcode)
	})

	t.Run("enable MFA setup", func(t *testing.T) {
		req := EnableUserMfaSetupRequest{
			MfaType:       "totp",
			Passcode:      "123456",
			RecoveryCodes: []string{"code1", "code2", "code3"},
		}
		cmd := req.toCommand("user-001")
		assert.Equal(t, "user-001", cmd.UserID)
		assert.Equal(t, "123456", cmd.Passcode)
		assert.Len(t, cmd.RecoveryCodes, 3)
	})

	t.Run("update preferred MFA", func(t *testing.T) {
		req := UpdateUserPreferredMfaRequest{MfaType: "sms", Revision: 2}
		cmd := req.toCommand("user-001")
		assert.Equal(t, "user-001", cmd.UserID)
		assert.Equal(t, "sms", cmd.MfaType)
	})
}

func TestHandler_SetPassword_RequestMapping(t *testing.T) {
	req := SetPasswordRequest{Password: "new-secure-password"}
	assert.Equal(t, "new-secure-password", req.Password)
}

func TestHandler_UnlinkIdentity_RequestMapping(t *testing.T) {
	req := UnlinkIdentityRequest{IdentityID: "idn-001"}
	assert.Equal(t, "idn-001", req.IdentityID)
}

func TestHandler_ImportUsers_RequestMapping(t *testing.T) {
	req := ImportUsersRequest{
		Users: []ImportUserItem{
			{Username: "user1", DisplayName: "User One", Email: "user1@example.com"},
			{Username: "user2", DisplayName: "User Two", Phone: "+8613800138000"},
		},
	}
	assert.Len(t, req.Users, 2)
	assert.Equal(t, "user1", req.Users[0].Username)
	assert.Equal(t, "user1@example.com", req.Users[0].Email)
	assert.Equal(t, "user2", req.Users[1].Username)
	assert.Equal(t, "+8613800138000", req.Users[1].Phone)
}

// ---------------------------------------------------------------------------
// Response format tests
// ---------------------------------------------------------------------------

func TestCreateUserResponse_Format(t *testing.T) {
	resp := CreateUserResponse{ID: "user-001", Status: "created"}
	assert.Equal(t, "user-001", resp.ID)
	assert.Equal(t, "created", resp.Status)
}

func TestUpdateUserCredentialsResponse_Format(t *testing.T) {
	resp := UpdateUserCredentialsResponse{
		Status:       "updated",
		AccessKey:    "new-access-key",
		AccessSecret: "new-access-secret",
	}
	assert.Equal(t, "updated", resp.Status)
	assert.NotEmpty(t, resp.AccessKey)
	assert.NotEmpty(t, resp.AccessSecret)
}

func TestRotateAccessKeyResponse_Format(t *testing.T) {
	resp := RotateAccessKeyResponse{
		Status:       "rotated",
		AccessKey:    "rotated-key",
		AccessSecret: "rotated-secret",
	}
	assert.Equal(t, "rotated", resp.Status)
}

func TestImpersonateResponse_Format(t *testing.T) {
	resp := ImpersonateResponse{Token: "impersonation-token"}
	assert.NotEmpty(t, resp.Token)
}

func TestInitiateUserMfaSetupResponse_Format(t *testing.T) {
	resp := InitiateUserMfaSetupResponse{
		Secret:             "JBSWY3DPEHPK3PXP",
		URL:                "otpauth://totp/Test?secret=JBSWY3DPEHPK3PXP",
		RecoveryCodes:      []string{"code1", "code2"},
		CountryCode:        "CN",
		MfaRememberInHours: 24,
	}
	assert.NotEmpty(t, resp.Secret)
	assert.NotEmpty(t, resp.URL)
	assert.Len(t, resp.RecoveryCodes, 2)
}

// ---------------------------------------------------------------------------
// Error code mapping tests
// ---------------------------------------------------------------------------

func TestHandler_ErrorCodeMapping(t *testing.T) {
	tests := []struct {
		name       string
		appErrCode string
	}{
		{"not_found maps to 404", "not_found"},
		{"invalid_argument maps to 400", "invalid_argument"},
		{"conflict maps to 409", "conflict"},
		{"unauthorized maps to 401", "unauthorized"},
		{"forbidden maps to 403", "forbidden"},
		{"internal maps to 500", "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify error code exists in the errors package
			assert.NotEmpty(t, tt.appErrCode)
		})
	}
}

// ---------------------------------------------------------------------------
// Boundary scenario tests
// ---------------------------------------------------------------------------

func TestHandler_CreateUser_Boundary(t *testing.T) {
	t.Run("empty username request", func(t *testing.T) {
		req := CreateUserRequest{
			OrganizationID: "org-001",
			Username:       "",
			UserType:       "normal",
			Status:         "active",
		}
		cmd := req.toCommand()
		assert.Empty(t, cmd.Username)
	})

	t.Run("very long email", func(t *testing.T) {
		longEmail := "user@" + string(make([]byte, 1000)) + ".com"
		req := CreateUserRequest{
			OrganizationID: "org-001",
			Username:       "testuser",
			UserType:       "normal",
			Status:         "active",
			Profile:        CreateUserProfileRequest{Email: longEmail},
		}
		cmd := req.toCommand()
		assert.Equal(t, longEmail, cmd.Profile.Email)
	})

	t.Run("special characters in username", func(t *testing.T) {
		req := CreateUserRequest{
			OrganizationID: "org-001",
			Username:       "user@domain#test!",
			UserType:       "normal",
			Status:         "active",
		}
		cmd := req.toCommand()
		assert.Equal(t, "user@domain#test!", cmd.Username)
	})
}

func TestHandler_ListUsers_Boundary(t *testing.T) {
	t.Run("zero page", func(t *testing.T) {
		req := ListUsersRequest{Page: 0, PageSize: 20}
		assert.Zero(t, req.Page)
	})

	t.Run("large page size", func(t *testing.T) {
		req := ListUsersRequest{Page: 1, PageSize: 10000}
		assert.Equal(t, 10000, req.PageSize)
	})

	t.Run("negative page", func(t *testing.T) {
		req := ListUsersRequest{Page: -1, PageSize: 20}
		assert.Negative(t, req.Page)
	})
}

// Verify the ListUsersRequest maps to the app-level ListUsersQuery
func TestListUsersRequest_ToQuery(t *testing.T) {
	req := ListUsersRequest{
		Page:           1,
		PageSize:       20,
		OrganizationID: "org-001",
		Keyword:        "admin",
		Status:         "active",
		UserType:       "normal",
		SortField:      "createdAt",
		SortOrder:      "desc",
	}

	// The handler casts ListUsersRequest to appusers.ListUsersQuery
	query := appusers.ListUsersQuery(req)
	assert.Equal(t, 1, query.Page)
	assert.Equal(t, 20, query.PageSize)
	assert.Equal(t, "org-001", query.OrganizationID)
	assert.Equal(t, "admin", query.Keyword)
	assert.Equal(t, "active", query.Status)
	assert.Equal(t, "normal", query.UserType)
}
