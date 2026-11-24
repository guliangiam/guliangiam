package faceidproviders

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateFaceIdProviderRequest_toCommand_v1(t *testing.T) {
	r := CreateFaceIdProviderRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchFaceIdProviderRequest_toCommand_v1(t *testing.T) {
	r := PatchFaceIdProviderRequest{}
	result := r.toCommand("test-id")
	_ = result
}
