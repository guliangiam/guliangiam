package mcpservers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateMcpServerRequest_toCommand_v1(t *testing.T) {
	r := CreateMcpServerRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchMcpServerRequest_toCommand_v1(t *testing.T) {
	r := PatchMcpServerRequest{}
	result := r.toCommand("test-id")
	_ = result
}
