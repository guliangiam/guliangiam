package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateProviderRequest_toCommand_v1(t *testing.T) {
	r := CreateProviderRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchProviderRequest_toCommand_v1(t *testing.T) {
	r := PatchProviderRequest{}
	result := r.toCommand("test-id")
	_ = result
}

func TestUpdateProviderConfigRequest_toCommand_v1(t *testing.T) {
	r := UpdateProviderConfigRequest{}
	result := r.toCommand("test-id")
	_ = result
}
