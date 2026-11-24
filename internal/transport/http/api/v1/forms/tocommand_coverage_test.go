package forms

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateFormRequest_toCommand_v1(t *testing.T) {
	r := CreateFormRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchFormRequest_toCommand_v1(t *testing.T) {
	r := PatchFormRequest{}
	result := r.toCommand("test-id")
	_ = result
}
