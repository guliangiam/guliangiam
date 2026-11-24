package radiusconfigs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateRadiusConfigRequest_toCommand_v1(t *testing.T) {
	r := CreateRadiusConfigRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchRadiusConfigRequest_toCommand_v1(t *testing.T) {
	r := PatchRadiusConfigRequest{}
	result := r.toCommand("test-id")
	_ = result
}
