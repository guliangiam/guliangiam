package tickets

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateTicketRequest_toCommand_v1(t *testing.T) {
	r := CreateTicketRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchTicketRequest_toCommand_v1(t *testing.T) {
	r := PatchTicketRequest{}
	result := r.toCommand("test-id", "org-1")
	_ = result
}
