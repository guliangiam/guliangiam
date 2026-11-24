package radiusaccountings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateRadiusAccountingRequest_toCommand_v1(t *testing.T) {
	r := CreateRadiusAccountingRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchRadiusAccountingRequest_toCommand_v1(t *testing.T) {
	r := PatchRadiusAccountingRequest{}
	result := r.toCommand("test-id")
	_ = result
}
