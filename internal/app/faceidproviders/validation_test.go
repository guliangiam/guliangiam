package faceidproviders

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateCreateFaceIdProviderCommand(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		cmd := CreateFaceIdProviderCommand{OrganizationID: "org-001", Name: "aliyun-face", ProviderType: "aliyun"}
		assert.NoError(t, validateCreateFaceIdProviderCommand(cmd))
	})
	t.Run("missing_org", func(t *testing.T) {
		cmd := CreateFaceIdProviderCommand{Name: "face", ProviderType: "aliyun"}
		assert.Error(t, validateCreateFaceIdProviderCommand(cmd))
	})
	t.Run("missing_name", func(t *testing.T) {
		cmd := CreateFaceIdProviderCommand{OrganizationID: "org-001", ProviderType: "aliyun"}
		assert.Error(t, validateCreateFaceIdProviderCommand(cmd))
	})
}

func TestValidatePatchFaceIdProviderCommand(t *testing.T) {
	t.Run("valid_nil", func(t *testing.T) {
		assert.NoError(t, validatePatchFaceIdProviderCommand(PatchFaceIdProviderCommand{}))
	})
	t.Run("blank_name", func(t *testing.T) {
		blank := ""
		assert.Error(t, validatePatchFaceIdProviderCommand(PatchFaceIdProviderCommand{Name: &blank}))
	})
}
