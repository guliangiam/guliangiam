package orders

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateOrderRequest_toCommand_v1(t *testing.T) {
	r := CreateOrderRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchOrderRequest_toCommand_v1(t *testing.T) {
	r := PatchOrderRequest{}
	result := r.toCommand("test-id")
	_ = result
}
