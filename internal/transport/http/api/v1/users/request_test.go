package users

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateUserRequest_ToCommand(t *testing.T) {
	t.Run("maps_all_fields", func(t *testing.T) {
		r := CreateUserRequest{
			OrganizationID: "org-001",
			Username:       "testuser",
			ExternalID:     "ext-001",
			UserType:       "normal",
			Status:         "active",
			IsAdmin:        false,
			Karma:          42,
			Ranking:        7,
			Profile: CreateUserProfileRequest{
				DisplayName: "Test User",
				FirstName:   "Test",
				LastName:    "User",
				AvatarURL:   "https://example.com/avatar.png",
				Email:       "test@example.com",
				Phone:       "+8613800138000",
				Homepage:    "https://example.com",
				Education:   "University",
				Language:    "en",
				Title:       "Engineer",
			},
			RoleBindingIDs: []string{"role-001"},
			GroupIDs:       []string{"group-001"},
		}

		cmd := r.toCommand()

		assert.Equal(t, "org-001", cmd.OrganizationID)
		assert.Equal(t, "testuser", cmd.Username)
		assert.Equal(t, "ext-001", cmd.ExternalID)
		assert.Equal(t, "normal", cmd.UserType)
		assert.Equal(t, "active", cmd.Status)
		assert.False(t, cmd.IsAdmin)
		assert.Equal(t, 42, cmd.Karma)
		assert.Equal(t, 7, cmd.Ranking)
		assert.Equal(t, "Test User", cmd.Profile.DisplayName)
		assert.Equal(t, "test@example.com", cmd.Profile.Email)
		assert.Equal(t, "https://example.com", cmd.Profile.Homepage)
		assert.Equal(t, "University", cmd.Profile.Education)
		assert.Equal(t, "en", cmd.Profile.Language)
		assert.Equal(t, "Engineer", cmd.Profile.Title)
		assert.Equal(t, []string{"role-001"}, cmd.RoleBindingIDs)
		assert.Equal(t, []string{"group-001"}, cmd.GroupIDs)
	})

	t.Run("empty_fields", func(t *testing.T) {
		r := CreateUserRequest{}
		cmd := r.toCommand()
		assert.Equal(t, "", cmd.OrganizationID)
		assert.Equal(t, "", cmd.Username)
	})
}

func TestPatchUserRequest_ToCommand(t *testing.T) {
	t.Run("maps_all_fields", func(t *testing.T) {
		extID := "ext-002"
		userType := "admin"
		status := "disabled"
		isAdmin := true
		isForbidden := false
		tag := "vip"
		score := 100
		balance := 50.0

		r := PatchUserRequest{
			ExternalID:  &extID,
			UserType:    &userType,
			Status:      &status,
			IsAdmin:     &isAdmin,
			IsForbidden: &isForbidden,
			Tag:         &tag,
			Score:       &score,
			Balance:     &balance,
			Revision:    2,
		}

		cmd := r.toCommand("user-001")

		assert.Equal(t, "user-001", cmd.ID)
		assert.Equal(t, &extID, cmd.ExternalID)
		assert.Equal(t, &userType, cmd.UserType)
		assert.Equal(t, &isAdmin, cmd.IsAdmin)
		assert.Equal(t, &tag, cmd.Tag)
		assert.Equal(t, int64(2), cmd.Revision)
	})
}

func TestUpdateUserProfileRequest_ToCommand(t *testing.T) {
	t.Run("maps_all_fields", func(t *testing.T) {
		r := UpdateUserProfileRequest{
			DisplayName: "Updated Name",
			FirstName:   "First",
			LastName:    "Last",
			Email:       "updated@example.com",
			Phone:       "+8613800138001",
			CountryCode: "CN",
			Region:      "Beijing",
			Gender:      "female",
			Bio:         "Updated bio",
			RealName:    "Zhang San",
			IDCardType:  "idcard",
			IDCard:      "110101199001011234",
			Properties:  map[string]any{"theme": "dark"},
		}

		cmd := r.toCommand("user-001")

		assert.Equal(t, "user-001", cmd.UserID)
		assert.Equal(t, "Updated Name", cmd.DisplayName)
		assert.Equal(t, "updated@example.com", cmd.Email)
		assert.Equal(t, "Zhang San", cmd.RealName)
		assert.Equal(t, "idcard", cmd.IDCardType)
		assert.Equal(t, map[string]any{"theme": "dark"}, cmd.Properties)
	})

	t.Run("certificate_fields_mapped_to_command", func(t *testing.T) {
		r := UpdateUserProfileRequest{
			PassportNo:            "E12345678",
			DriverLicenseNo:       "D1234567",
			IDCardExpiry:          "2030-12-31",
			IssueCountry:          "CN",
			IssuingAuthority:      "MPS Beijing",
			PassportPhotoURL:      "https://cdn.example.com/passport.png",
			DriverLicenseFrontURL: "https://cdn.example.com/dl-front.png",
			DriverLicenseBackURL:  "https://cdn.example.com/dl-back.png",
		}

		cmd := r.toCommand("user-001")

		assert.Equal(t, "E12345678", cmd.PassportNo)
		assert.Equal(t, "D1234567", cmd.DriverLicenseNo)
		assert.Equal(t, "2030-12-31", cmd.IDCardExpiry)
		assert.Equal(t, "CN", cmd.IssueCountry)
		assert.Equal(t, "MPS Beijing", cmd.IssuingAuthority)
		assert.Equal(t, "https://cdn.example.com/passport.png", cmd.PassportPhotoURL)
		assert.Equal(t, "https://cdn.example.com/dl-front.png", cmd.DriverLicenseFrontURL)
		assert.Equal(t, "https://cdn.example.com/dl-back.png", cmd.DriverLicenseBackURL)
	})

	t.Run("affiliation_merged_into_properties", func(t *testing.T) {
		r := UpdateUserProfileRequest{
			Affiliation: "Beijing University",
			Properties:  map[string]any{"theme": "dark"},
		}

		cmd := r.toCommand("user-001")

		assert.Equal(t, "Beijing University", cmd.Properties["affiliation"])
		assert.Equal(t, "dark", cmd.Properties["theme"])
	})

	t.Run("affiliation_without_properties", func(t *testing.T) {
		r := UpdateUserProfileRequest{
			Affiliation: "Shanghai Corp",
		}

		cmd := r.toCommand("user-001")

		assert.Equal(t, "Shanghai Corp", cmd.Properties["affiliation"])
	})
}

func TestPatchUserRequest_ExtendedFieldsToCommand(t *testing.T) {
	karma, ranking := 42, 7
	language, title := "en", "Engineer"
	cmd := (PatchUserRequest{
		Karma:    &karma,
		Ranking:  &ranking,
		Language: &language,
		Title:    &title,
	}).toCommand("user-001")

	assert.Equal(t, &karma, cmd.Karma)
	assert.Equal(t, &ranking, cmd.Ranking)
	assert.Equal(t, &language, cmd.Language)
	assert.Equal(t, &title, cmd.Title)
}

func TestReplaceUserRoleBindingsRequest_ToCommands(t *testing.T) {
	t.Run("maps_items", func(t *testing.T) {
		r := ReplaceUserRoleBindingsRequest{
			Items: []ReplaceUserRoleBindingItemRequest{
				{RoleID: "role-001"},
				{RoleID: "role-002"},
			},
		}

		cmds := r.toCommands()
		assert.Len(t, cmds, 2)
		assert.Equal(t, "role-001", cmds[0].RoleID)
		assert.Equal(t, "role-002", cmds[1].RoleID)
	})

	t.Run("empty_items", func(t *testing.T) {
		r := ReplaceUserRoleBindingsRequest{Items: []ReplaceUserRoleBindingItemRequest{}}
		cmds := r.toCommands()
		assert.Empty(t, cmds)
	})
}

func TestReplaceUserGroupMembershipsRequest_ToCommands(t *testing.T) {
	t.Run("maps_items", func(t *testing.T) {
		r := ReplaceUserGroupMembershipsRequest{
			Items: []ReplaceUserGroupMembershipItemRequest{
				{GroupID: "group-001"},
				{GroupID: "group-002"},
			},
		}

		cmds := r.toCommands()
		assert.Len(t, cmds, 2)
		assert.Equal(t, "group-001", cmds[0].GroupID)
	})
}

func TestUpdateUserCredentialsRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields", func(t *testing.T) {
		needUpdatePassword := false
		r := UpdateUserCredentialsRequest{
			Password:           "newpassword",
			RotateAccessSecret: true,
			NeedUpdatePassword: &needUpdatePassword,
		}

		cmd := r.toCommand("user-001")

		assert.Equal(t, "user-001", cmd.UserID)
		assert.Equal(t, "newpassword", cmd.Password)
		assert.True(t, cmd.RotateAccessSecret)
		assert.NotNil(t, cmd.NeedUpdatePassword)
		assert.False(t, *cmd.NeedUpdatePassword)
	})
}

func TestReplaceUserIdentitiesRequest_ToCommands(t *testing.T) {
	t.Run("maps_items", func(t *testing.T) {
		r := ReplaceUserIdentitiesRequest{
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

		cmds := r.toCommands()
		assert.Len(t, cmds, 1)
		assert.Equal(t, "github", cmds[0].ProviderID)
		assert.Equal(t, "oauth", cmds[0].IdentityType)
		assert.Equal(t, "12345", cmds[0].Subject)
	})
}

func TestUpdateUserPreferredMfaRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields", func(t *testing.T) {
		r := UpdateUserPreferredMfaRequest{
			MfaType: "totp",
		}

		cmd := r.toCommand("user-001")
		assert.Equal(t, "user-001", cmd.UserID)
		assert.Equal(t, "totp", cmd.MfaType)
	})
}

func TestInitiateUserMfaSetupRequest_ToCommand(t *testing.T) {
	t.Run("maps_fields", func(t *testing.T) {
		r := InitiateUserMfaSetupRequest{
			MfaType: "totp",
		}

		cmd := r.toCommand("user-001")
		assert.Equal(t, "user-001", cmd.UserID)
		assert.Equal(t, "totp", cmd.MfaType)
	})
}
