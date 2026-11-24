package groups

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateGroupRequest_toCommand_v1(t *testing.T) {
	r := CreateGroupRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchGroupRequest_toCommand_v1(t *testing.T) {
	r := PatchGroupRequest{}
	result := r.toCommand("test-id")
	_ = result
}
