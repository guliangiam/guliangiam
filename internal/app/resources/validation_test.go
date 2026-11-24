package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateCreateResourceCommand(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		cmd := CreateResourceCommand{
			OrganizationID: "org-001",
			Name:           "logo",
			DisplayName:    "Logo",
		}
		assert.NoError(t, validateCreateResourceCommand(cmd))
	})

	t.Run("valid_with_type", func(t *testing.T) {
		cmd := CreateResourceCommand{
			OrganizationID: "org-001",
			Name:           "logo",
			DisplayName:    "Logo",
			Type:           "image",
		}
		assert.NoError(t, validateCreateResourceCommand(cmd))
	})

	t.Run("missing_organization_id", func(t *testing.T) {
		cmd := CreateResourceCommand{Name: "logo", DisplayName: "Logo"}
		assert.Error(t, validateCreateResourceCommand(cmd))
	})

	t.Run("missing_name", func(t *testing.T) {
		cmd := CreateResourceCommand{OrganizationID: "org-001", DisplayName: "Logo"}
		assert.Error(t, validateCreateResourceCommand(cmd))
	})

	t.Run("missing_display_name", func(t *testing.T) {
		cmd := CreateResourceCommand{OrganizationID: "org-001", Name: "logo"}
		assert.Error(t, validateCreateResourceCommand(cmd))
	})

	t.Run("invalid_type", func(t *testing.T) {
		cmd := CreateResourceCommand{
			OrganizationID: "org-001",
			Name:           "logo",
			DisplayName:    "Logo",
			Type:           "invalid",
		}
		assert.Error(t, validateCreateResourceCommand(cmd))
	})

	t.Run("all_valid_types", func(t *testing.T) {
		validTypes := []string{"image", "css", "js", "file", "video", "audio"}
		for _, typ := range validTypes {
			cmd := CreateResourceCommand{
				OrganizationID: "org-001",
				Name:           "res",
				DisplayName:    "Res",
				Type:           typ,
			}
			assert.NoError(t, validateCreateResourceCommand(cmd), "type: %s", typ)
		}
	})
}

func TestValidatePatchResourceCommand(t *testing.T) {
	t.Run("valid_all_nil", func(t *testing.T) {
		cmd := PatchResourceCommand{}
		assert.NoError(t, validatePatchResourceCommand(cmd))
	})

	t.Run("blank_display_name", func(t *testing.T) {
		blank := ""
		cmd := PatchResourceCommand{DisplayName: &blank}
		assert.Error(t, validatePatchResourceCommand(cmd))
	})

	t.Run("invalid_type", func(t *testing.T) {
		invalidType := "invalid"
		cmd := PatchResourceCommand{Type: &invalidType}
		assert.Error(t, validatePatchResourceCommand(cmd))
	})

	t.Run("valid_type", func(t *testing.T) {
		validType := "image"
		displayName := "Updated"
		cmd := PatchResourceCommand{Type: &validType, DisplayName: &displayName}
		assert.NoError(t, validatePatchResourceCommand(cmd))
	})
}
