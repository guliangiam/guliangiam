package captchaproviders

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateCaptchaProviderRequest_toCommand_v1(t *testing.T) {
	r := CreateCaptchaProviderRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchCaptchaProviderRequest_toCommand_v1(t *testing.T) {
	r := PatchCaptchaProviderRequest{}
	result := r.toCommand("test-id")
	_ = result
}
