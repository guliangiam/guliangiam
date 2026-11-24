package invitations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateInvitationRequest_toCommand_v1(t *testing.T) {
	r := CreateInvitationRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}
