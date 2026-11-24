package agents

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateAgentRequest_toCommand_v1(t *testing.T) {
	r := CreateAgentRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchAgentRequest_toCommand_v1(t *testing.T) {
	r := PatchAgentRequest{}
	result := r.toCommand("test-id")
	_ = result
}
