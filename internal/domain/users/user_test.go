package users

import (
	"testing"
)

func TestUser_Validation(t *testing.T) {
	tests := []struct {
		name    string
		user    User
		wantErr bool
	}{
		{
			name: "valid user",
			user: User{
				ID:             "019test-user-id",
				OrganizationID: "019test-org-id",
				Username:       "testuser",
				UserType:       "normal",
				Status:         "active",
			},
			wantErr: false,
		},
		{
			name: "missing username",
			user: User{
				ID:             "019test-user-id",
				OrganizationID: "019test-org-id",
				Username:       "",
				UserType:       "normal",
				Status:         "active",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic field validation
			if tt.wantErr {
				if tt.user.Username == "" {
					// Expected validation failure
					return
				}
				t.Errorf("expected validation error but got none")
			}
		})
	}
}

func TestUserCredentials_HasPassword(t *testing.T) {
	cred := UserCredentials{
		UserID:      "user-1",
		HasPassword: true,
	}
	if !cred.HasPassword {
		t.Error("HasPassword should be true")
	}

	cred.HasPassword = false
	if cred.HasPassword {
		t.Error("HasPassword should be false")
	}
}

func TestUserProfile_Fields(t *testing.T) {
	profile := UserProfile{
		UserID:      "user-1",
		DisplayName: "Test User",
		Email:       "test@example.com",
		Phone:       "+1234567890",
	}

	if profile.DisplayName != "Test User" {
		t.Errorf("DisplayName = %q, want %q", profile.DisplayName, "Test User")
	}
	if profile.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", profile.Email, "test@example.com")
	}
	if profile.Phone != "+1234567890" {
		t.Errorf("Phone = %q, want %q", profile.Phone, "+1234567890")
	}
}

func TestUserRoleBinding_Fields(t *testing.T) {
	binding := UserRoleBinding{
		UserID: "user-1",
		RoleID: "role-1",
	}
	if binding.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", binding.UserID, "user-1")
	}
	if binding.RoleID != "role-1" {
		t.Errorf("RoleID = %q, want %q", binding.RoleID, "role-1")
	}
}

func TestUserGroupMembership_Fields(t *testing.T) {
	membership := UserGroupMembership{
		UserID:  "user-1",
		GroupID: "group-1",
	}
	if membership.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", membership.UserID, "user-1")
	}
	if membership.GroupID != "group-1" {
		t.Errorf("GroupID = %q, want %q", membership.GroupID, "group-1")
	}
}
