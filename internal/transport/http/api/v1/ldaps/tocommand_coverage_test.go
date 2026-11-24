package ldaps

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateLdapRequest_toCommand_v1(t *testing.T) {
	r := CreateLdapRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchLdapRequest_toCommand_v1(t *testing.T) {
	r := PatchLdapRequest{}
	result := r.toCommand("test-id")
	_ = result
}
