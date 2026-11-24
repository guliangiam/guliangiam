package permissions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCreatePermissionRequest_ToCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		isEnabled := true
		req := CreatePermissionRequest{
			OrganizationID: "org-001",
			Name:           "read-users",
			DisplayName:    "Read Users",
			Description:    "Allow reading users",
			Users:          []string{"user-001"},
			Groups:         []string{"group-001"},
			Roles:          []string{"role-001"},
			Domains:        []string{"example.com"},
			ResourceType:   "application",
			Resources:      []string{"app-001"},
			Actions:        []string{"read"},
			Effect:         "allow",
			IsEnabled:      &isEnabled,
			ApproveTime:    "2025-01-01T00:00:00Z",
		}

		cmd := req.toCommand()

		assert.Equal(t, "org-001", cmd.OrganizationID)
		assert.Equal(t, "read-users", cmd.Name)
		assert.Equal(t, "application", cmd.ResourceType)
		assert.Equal(t, "allow", cmd.Effect)
		assert.True(t, *cmd.IsEnabled)
		assert.NotNil(t, cmd.ApproveTime)
	})

	t.Run("empty_approve_time", func(t *testing.T) {
		req := CreatePermissionRequest{
			ApproveTime: "",
		}

		cmd := req.toCommand()

		assert.Nil(t, cmd.ApproveTime)
	})

	t.Run("invalid_approve_time", func(t *testing.T) {
		req := CreatePermissionRequest{
			ApproveTime: "not-a-time",
		}

		cmd := req.toCommand()

		assert.Nil(t, cmd.ApproveTime)
	})
}

func TestPatchPermissionRequest_ToCommand(t *testing.T) {
	t.Run("with_nil_slices", func(t *testing.T) {
		displayName := "Updated Perm"
		req := PatchPermissionRequest{
			DisplayName: &displayName,
			Users:       nil,
			Groups:      nil,
			Revision:    2,
		}

		cmd := req.toCommand("perm-001")

		assert.Equal(t, "perm-001", cmd.ID)
		assert.Nil(t, cmd.Users)
		assert.Nil(t, cmd.Groups)
	})

	t.Run("with_slices", func(t *testing.T) {
		req := PatchPermissionRequest{
			Users:     []string{"u1"},
			Groups:    []string{"g1"},
			Roles:     []string{"r1"},
			Domains:   []string{"d1"},
			Resources: []string{"res1"},
			Actions:   []string{"write"},
		}

		cmd := req.toCommand("perm-002")

		assert.Equal(t, []string{"u1"}, cmd.Users)
		assert.Equal(t, []string{"g1"}, cmd.Groups)
		assert.Equal(t, []string{"r1"}, cmd.Roles)
		assert.Equal(t, []string{"d1"}, cmd.Domains)
		assert.Equal(t, []string{"res1"}, cmd.Resources)
		assert.Equal(t, []string{"write"}, cmd.Actions)
	})

	t.Run("with_approve_time", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Second)
		req := PatchPermissionRequest{
			ApproveTime: now.Format(time.RFC3339),
		}

		cmd := req.toCommand("perm-003")

		assert.NotNil(t, cmd.ApproveTime)
	})
}
