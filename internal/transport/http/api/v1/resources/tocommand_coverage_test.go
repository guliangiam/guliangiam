package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateResourceRequest_toCommand_v1(t *testing.T) {
	r := CreateResourceRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchResourceRequest_toCommand_v1(t *testing.T) {
	r := PatchResourceRequest{}
	result := r.toCommand("test-id")
	_ = result
}
