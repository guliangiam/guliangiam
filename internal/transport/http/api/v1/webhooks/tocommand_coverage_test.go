package webhooks

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateWebhookRequest_toCommand_v1(t *testing.T) {
	r := CreateWebhookRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchWebhookRequest_toCommand_v1(t *testing.T) {
	r := PatchWebhookRequest{}
	result := r.toCommand("test-id")
	_ = result
}
