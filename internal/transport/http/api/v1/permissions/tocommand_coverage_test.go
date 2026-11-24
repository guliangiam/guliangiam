package permissions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreatePermissionRequest_toCommand_v1(t *testing.T) {
	r := CreatePermissionRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchPermissionRequest_toCommand_v1(t *testing.T) {
	r := PatchPermissionRequest{}
	result := r.toCommand("test-id")
	_ = result
}
