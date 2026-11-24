package syncers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateSyncerRequest_toCommand_v1(t *testing.T) {
	r := CreateSyncerRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchSyncerRequest_toCommand_v1(t *testing.T) {
	r := PatchSyncerRequest{}
	result := r.toCommand("test-id")
	_ = result
}
