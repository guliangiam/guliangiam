package users

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateUserRequest_toCommand_v1(t *testing.T) {
	r := CreateUserRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchUserRequest_toCommand_v1(t *testing.T) {
	r := PatchUserRequest{}
	result := r.toCommand("test-id")
	_ = result
}

func TestUpdateUserProfileRequest_toCommand_v1(t *testing.T) {
	r := UpdateUserProfileRequest{}
	result := r.toCommand("test-id")
	_ = result
}

func TestReplaceUserRoleBindingsRequest_toCommands_v1(t *testing.T) {
	r := ReplaceUserRoleBindingsRequest{}
	result := r.toCommands()
	_ = result
}

func TestReplaceUserGroupMembershipsRequest_toCommands_v1(t *testing.T) {
	r := ReplaceUserGroupMembershipsRequest{}
	result := r.toCommands()
	_ = result
}

func TestUpdateUserCredentialsRequest_toCommand_v1(t *testing.T) {
	r := UpdateUserCredentialsRequest{}
	result := r.toCommand("test-id")
	_ = result
}

func TestReplaceUserIdentitiesRequest_toCommands_v1(t *testing.T) {
	r := ReplaceUserIdentitiesRequest{}
	result := r.toCommands()
	_ = result
}

func TestReplaceUserMfaMethodsRequest_toCommands_v1(t *testing.T) {
	r := ReplaceUserMfaMethodsRequest{}
	result := r.toCommands()
	_ = result
}

func TestUpdateUserPreferredMfaRequest_toCommand_v1(t *testing.T) {
	r := UpdateUserPreferredMfaRequest{}
	result := r.toCommand("test-id")
	_ = result
}

func TestInitiateUserMfaSetupRequest_toCommand_v1(t *testing.T) {
	r := InitiateUserMfaSetupRequest{}
	result := r.toCommand("test-id")
	_ = result
}

func TestVerifyUserMfaSetupRequest_toCommand_v1(t *testing.T) {
	r := VerifyUserMfaSetupRequest{}
	result := r.toCommand("test-id")
	_ = result
}

func TestEnableUserMfaSetupRequest_toCommand_v1(t *testing.T) {
	r := EnableUserMfaSetupRequest{}
	result := r.toCommand("test-id")
	_ = result
}
