package pricings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreatePricingRequest_toCommand_v1(t *testing.T) {
	r := CreatePricingRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchPricingRequest_toCommand_v1(t *testing.T) {
	r := PatchPricingRequest{}
	result := r.toCommand("test-id")
	_ = result
}
