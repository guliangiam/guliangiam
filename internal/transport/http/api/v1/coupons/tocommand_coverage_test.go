package coupons

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateCouponRequest_toCommand_v1(t *testing.T) {
	r := CreateCouponRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchCouponRequest_toCommand_v1(t *testing.T) {
	r := PatchCouponRequest{}
	result := r.toCommand("test-id")
	_ = result
}
