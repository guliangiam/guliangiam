package applications

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateApplicationRequest_toCommand_v1(t *testing.T) {
	r := CreateApplicationRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchApplicationRequest_toCommand_v1(t *testing.T) {
	r := PatchApplicationRequest{}
	result := r.toCommand("test-id")
	_ = result
}

func TestReplaceApplicationProviderBindingsRequest_toCommands_v1(t *testing.T) {
	r := ReplaceApplicationProviderBindingsRequest{}
	result := r.toCommands()
	_ = result
}
