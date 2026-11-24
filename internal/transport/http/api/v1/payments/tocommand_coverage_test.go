package payments

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreatePaymentRequest_toCommand_v1(t *testing.T) {
	r := CreatePaymentRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchPaymentRequest_toCommand_v1(t *testing.T) {
	r := PatchPaymentRequest{}
	result := r.toCommand("test-id")
	_ = result
}
