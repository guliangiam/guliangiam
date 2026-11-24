package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateRuleRequest_toCommand_v1(t *testing.T) {
	r := CreateRuleRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchRuleRequest_toCommand_v1(t *testing.T) {
	r := PatchRuleRequest{}
	result := r.toCommand("test-id")
	_ = result
}
