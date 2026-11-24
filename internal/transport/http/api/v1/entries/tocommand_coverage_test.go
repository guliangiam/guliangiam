package entries

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateEntryRequest_toCommand_v1(t *testing.T) {
	r := CreateEntryRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchEntryRequest_toCommand_v1(t *testing.T) {
	r := PatchEntryRequest{}
	result := r.toCommand("test-id", "org-1")
	_ = result
}
