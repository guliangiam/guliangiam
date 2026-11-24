package permissions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToPermission(t *testing.T) {
	now := time.Now()

	t.Run("basic_permission", func(t *testing.T) {
		row := permissionRow{
			ID:             "perm-001",
			OrganizationID: "org-001",
			Name:           "users:list",
			DisplayName:    "List Users",
			Description:    "Allow listing users",
			ResourceType:   "users",
			Effect:         "allow",
			IsEnabled:      true,
			State:          "approved",
			Users:          []byte(`["user-001"]`),
			Roles:          []byte(`["role-admin"]`),
			Resources:      []byte(`["users"]`),
			Actions:        []byte(`["list","get"]`),
			Domains:        []byte(`["domain-1"]`),
			Groups:         []byte(`[]`),
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		perm, err := toPermission(row)
		require.NoError(t, err)

		assert.Equal(t, "perm-001", perm.ID)
		assert.Equal(t, "org-001", perm.OrganizationID)
		assert.Equal(t, "users:list", perm.Name)
		assert.Equal(t, "allow", perm.Effect)
		assert.True(t, perm.IsEnabled)
		assert.Equal(t, "approved", perm.State)
	})

	t.Run("nil_jsonb_defaults_to_empty", func(t *testing.T) {
		row := permissionRow{
			ID:             "perm-002",
			OrganizationID: "org-001",
			Name:           "users:get",
			Users:          nil,
			Groups:         nil,
			Roles:          nil,
			Domains:        nil,
			Resources:      nil,
			Actions:        nil,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		perm, err := toPermission(row)
		require.NoError(t, err)

		assert.Equal(t, []string{}, perm.Users)
		assert.Equal(t, []string{}, perm.Groups)
		assert.Equal(t, []string{}, perm.Roles)
		assert.Equal(t, []string{}, perm.Domains)
		assert.Equal(t, []string{}, perm.Resources)
		assert.Equal(t, []string{}, perm.Actions)
	})

	t.Run("deny_effect", func(t *testing.T) {
		row := permissionRow{
			ID:             "perm-003",
			OrganizationID: "org-001",
			Name:           "users:delete",
			Effect:         "deny",
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		perm, err := toPermission(row)
		require.NoError(t, err)
		assert.Equal(t, "deny", perm.Effect)
	})

	t.Run("disabled_permission", func(t *testing.T) {
		row := permissionRow{
			ID:             "perm-004",
			OrganizationID: "org-001",
			Name:           "users:create",
			IsEnabled:      false,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		perm, err := toPermission(row)
		require.NoError(t, err)
		assert.False(t, perm.IsEnabled)
	})
}
