package roles

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateRoleRequest_toCommand_v1(t *testing.T) {
	r := CreateRoleRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchRoleRequest_toCommand_v1(t *testing.T) {
	r := PatchRoleRequest{}
	result := r.toCommand("test-id")
	_ = result
}
