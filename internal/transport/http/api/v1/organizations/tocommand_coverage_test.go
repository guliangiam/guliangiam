package organizations

import (
	"testing"
)

func TestCreateOrganizationRequest_toCommand_v1(t *testing.T) {
	r := CreateOrganizationRequest{}
	result := r.toCommand()
	_ = result
}

func TestPatchOrganizationRequest_toCommand_v1(t *testing.T) {
	r := PatchOrganizationRequest{}
	result := r.toCommand("test-id")
	_ = result
}

func TestUpdateOrganizationSettingsRequest_toCommand_v1(t *testing.T) {
	r := UpdateOrganizationSettingsRequest{}
	result := r.toCommand("test-id")
	_ = result
}

func TestUpdateOrganizationThemeRequest_toCommand_v1(t *testing.T) {
	r := UpdateOrganizationThemeRequest{}
	result := r.toCommand("test-id")
	_ = result
}

func TestUpdateOrganizationSecurityPolicyRequest_toCommand_v1(t *testing.T) {
	r := UpdateOrganizationSecurityPolicyRequest{}
	result := r.toCommand("test-id")
	_ = result
}
