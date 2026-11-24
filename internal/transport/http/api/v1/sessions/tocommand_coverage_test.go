package sessions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPatchSessionRequest_toCommand_v1(t *testing.T) {
	r := PatchSessionRequest{}
	result := r.toCommand("test-id")
	_ = result
}

func TestListSessionsRequest_toQuery_v1(t *testing.T) {
	r := ListSessionsRequest{
		OrganizationID: "org-1",
	}
	result := r.toQuery()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}
