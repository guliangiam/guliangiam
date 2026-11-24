package plans

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreatePlanRequest_toCommand_v1(t *testing.T) {
	r := CreatePlanRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchPlanRequest_toCommand_v1(t *testing.T) {
	r := PatchPlanRequest{}
	result := r.toCommand("test-id")
	_ = result
}
