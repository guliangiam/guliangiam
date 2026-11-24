package roles

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateRoleRequest_ToCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		isEnabled := true
		req := CreateRoleRequest{
			OrganizationID: "org-001",
			Name:           "admin",
			DisplayName:    "Admin",
			Description:    "Administrator role",
			GroupIDs:       []string{"group-001"},
			Users:          []string{"user-001"},
			RoleIDs:        []string{"role-002"},
			Domains:        []string{"example.com"},
			IsEnabled:      &isEnabled,
		}

		cmd := req.toCommand()

		assert.Equal(t, "org-001", cmd.OrganizationID)
		assert.Equal(t, "admin", cmd.Name)
		assert.Equal(t, "Admin", cmd.DisplayName)
		assert.Equal(t, "Administrator role", cmd.Description)
		assert.Equal(t, []string{"group-001"}, cmd.GroupIDs)
		assert.Equal(t, []string{"user-001"}, cmd.Users)
		assert.True(t, *cmd.IsEnabled)
	})
}

func TestPatchRoleRequest_ToCommand(t *testing.T) {
	t.Run("with_nil_slices", func(t *testing.T) {
		displayName := "Updated Role"
		isEnabled := false
		req := PatchRoleRequest{
			DisplayName: &displayName,
			IsEnabled:   &isEnabled,
			GroupIDs:    nil,
			Users:       nil,
			Revision:    2,
		}

		cmd := req.toCommand("role-001")

		assert.Equal(t, "role-001", cmd.ID)
		assert.Equal(t, &displayName, cmd.DisplayName)
		assert.False(t, *cmd.IsEnabled)
		assert.Nil(t, cmd.GroupIDs)
		assert.Nil(t, cmd.Users)
	})

	t.Run("with_slices", func(t *testing.T) {
		req := PatchRoleRequest{
			GroupIDs: []string{"g1", "g2"},
			Users:    []string{"u1"},
			RoleIDs:  []string{"r1"},
			Domains:  []string{"d1"},
		}

		cmd := req.toCommand("role-002")

		assert.Equal(t, []string{"g1", "g2"}, cmd.GroupIDs)
		assert.Equal(t, []string{"u1"}, cmd.Users)
		assert.Equal(t, []string{"r1"}, cmd.RoleIDs)
		assert.Equal(t, []string{"d1"}, cmd.Domains)
	})
}
