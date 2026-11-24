package groups

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateCreateGroupCommand(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		cmd := CreateGroupCommand{
			OrganizationID: "org-001",
			Name:           "engineering",
			DisplayName:    "Engineering",
		}
		assert.NoError(t, validateCreateGroupCommand(cmd))
	})

	t.Run("missing_organization_id", func(t *testing.T) {
		cmd := CreateGroupCommand{Name: "eng", DisplayName: "Eng"}
		assert.Error(t, validateCreateGroupCommand(cmd))
	})

	t.Run("missing_name", func(t *testing.T) {
		cmd := CreateGroupCommand{OrganizationID: "org-001", DisplayName: "Eng"}
		assert.Error(t, validateCreateGroupCommand(cmd))
	})

	t.Run("name_contains_slash", func(t *testing.T) {
		cmd := CreateGroupCommand{
			OrganizationID: "org-001",
			Name:           "eng/ops",
			DisplayName:    "Eng/Ops",
		}
		assert.Error(t, validateCreateGroupCommand(cmd))
	})

	t.Run("missing_display_name", func(t *testing.T) {
		cmd := CreateGroupCommand{OrganizationID: "org-001", Name: "eng"}
		assert.Error(t, validateCreateGroupCommand(cmd))
	})

	t.Run("empty_type_allowed_defaults_later", func(t *testing.T) {
		cmd := CreateGroupCommand{OrganizationID: "org-001", Name: "eng", DisplayName: "Eng"}
		assert.NoError(t, validateCreateGroupCommand(cmd))
	})

	t.Run("closed_set_types", func(t *testing.T) {
		for _, groupType := range []string{"virtual", "physical", "department"} {
			cmd := CreateGroupCommand{OrganizationID: "org-001", Name: "eng", DisplayName: "Eng", Type: groupType}
			assert.NoError(t, validateCreateGroupCommand(cmd), groupType)
		}
	})

	t.Run("type_outside_closed_set", func(t *testing.T) {
		for _, groupType := range []string{"admin", "operator", "VIRTUAL", "team"} {
			cmd := CreateGroupCommand{OrganizationID: "org-001", Name: "eng", DisplayName: "Eng", Type: groupType}
			assert.Error(t, validateCreateGroupCommand(cmd), groupType)
		}
	})
}

func TestValidatePatchGroupCommand(t *testing.T) {
	t.Run("valid_all_nil", func(t *testing.T) {
		cmd := PatchGroupCommand{}
		assert.NoError(t, validatePatchGroupCommand(cmd))
	})

	t.Run("blank_name", func(t *testing.T) {
		blank := ""
		cmd := PatchGroupCommand{Name: &blank}
		assert.Error(t, validatePatchGroupCommand(cmd))
	})

	t.Run("name_contains_slash", func(t *testing.T) {
		slash := "new/name"
		cmd := PatchGroupCommand{Name: &slash}
		assert.Error(t, validatePatchGroupCommand(cmd))
	})

	t.Run("blank_display_name", func(t *testing.T) {
		blank := ""
		cmd := PatchGroupCommand{DisplayName: &blank}
		assert.Error(t, validatePatchGroupCommand(cmd))
	})

	t.Run("valid_name", func(t *testing.T) {
		name := "updated-name"
		displayName := "Updated Name"
		cmd := PatchGroupCommand{Name: &name, DisplayName: &displayName}
		assert.NoError(t, validatePatchGroupCommand(cmd))
	})

	t.Run("nil_type_keeps_current", func(t *testing.T) {
		assert.NoError(t, validatePatchGroupCommand(PatchGroupCommand{}))
	})

	t.Run("blank_type_rejected", func(t *testing.T) {
		for _, blank := range []string{"", "   "} {
			value := blank
			assert.Error(t, validatePatchGroupCommand(PatchGroupCommand{Type: &value}))
		}
	})

	t.Run("closed_set_types", func(t *testing.T) {
		for _, groupType := range []string{"virtual", "physical", "department"} {
			value := groupType
			assert.NoError(t, validatePatchGroupCommand(PatchGroupCommand{Type: &value}), groupType)
		}
	})

	t.Run("type_outside_closed_set", func(t *testing.T) {
		for _, groupType := range []string{"admin", "operator", "team"} {
			value := groupType
			assert.Error(t, validatePatchGroupCommand(PatchGroupCommand{Type: &value}), groupType)
		}
	})
}
