package subscriptions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateSubscriptionRequest_toCommand_v1(t *testing.T) {
	r := CreateSubscriptionRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchSubscriptionRequest_toCommand_v1(t *testing.T) {
	r := PatchSubscriptionRequest{}
	result := r.toCommand("test-id")
	_ = result
}
