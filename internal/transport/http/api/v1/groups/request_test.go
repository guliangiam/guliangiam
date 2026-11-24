package groups

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateGroupRequest_ToCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		req := CreateGroupRequest{
			OrganizationID: "org-001",
			Name:           "engineering",
			DisplayName:    "Engineering",
			Manager:        "user-001",
			ContactEmail:   "eng@example.com",
			Type:           "virtual",
			ParentID:       "group-parent",
			IsTopGroup:     false,
			IsEnabled:      true,
			Homepage:       "https://eng.example.com",
			Users:          []string{"user-001", "user-002"},
		}

		cmd := req.toCommand()

		assert.Equal(t, "org-001", cmd.OrganizationID)
		assert.Equal(t, "engineering", cmd.Name)
		assert.Equal(t, "Engineering", cmd.DisplayName)
		assert.Equal(t, "user-001", cmd.Manager)
		assert.True(t, cmd.IsEnabled)
		assert.Equal(t, []string{"user-001", "user-002"}, cmd.Users)
	})
}

func TestPatchGroupRequest_ToCommand(t *testing.T) {
	t.Run("with_nil_users", func(t *testing.T) {
		displayName := "Updated Group"
		req := PatchGroupRequest{
			DisplayName: &displayName,
			Users:       nil,
			Revision:    3,
		}

		cmd := req.toCommand("group-001")

		assert.Equal(t, "group-001", cmd.ID)
		assert.Equal(t, &displayName, cmd.DisplayName)
		assert.Nil(t, cmd.Users)
	})

	t.Run("with_users", func(t *testing.T) {
		req := PatchGroupRequest{
			Users: []string{"user-003"},
		}

		cmd := req.toCommand("group-002")

		assert.Equal(t, []string{"user-003"}, cmd.Users)
	})
}
