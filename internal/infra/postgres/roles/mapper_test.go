package roles

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToRole(t *testing.T) {
	now := time.Now()

	t.Run("basic_role", func(t *testing.T) {
		row := roleRow{
			ID:             "role-001",
			OrganizationID: "org-001",
			Name:           "admin",
			DisplayName:    "Administrator",
			Description:    "Full access role",
			IsEnabled:      true,
			Users:          []byte(`["user-001","user-002"]`),
			GroupIDs:       []byte(`["group-001"]`),
			RoleIDs:        []byte(`[]`),
			Domains:        []byte(`["domain-001"]`),
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		role, err := toRole(row)
		require.NoError(t, err)

		assert.Equal(t, "role-001", role.ID)
		assert.Equal(t, "org-001", role.OrganizationID)
		assert.Equal(t, "admin", role.Name)
		assert.Equal(t, "Administrator", role.DisplayName)
		assert.True(t, role.IsEnabled)
		assert.Equal(t, int64(1), role.Revision)
	})

	t.Run("nil_jsonb_defaults_to_empty", func(t *testing.T) {
		row := roleRow{
			ID:             "role-002",
			OrganizationID: "org-001",
			Name:           "viewer",
			DisplayName:    "Viewer",
			Users:          nil,
			GroupIDs:       nil,
			RoleIDs:        nil,
			Domains:        nil,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		role, err := toRole(row)
		require.NoError(t, err)

		assert.Equal(t, []string{}, role.Users)
		assert.Equal(t, []string{}, role.GroupIDs)
		assert.Equal(t, []string{}, role.RoleIDs)
		assert.Equal(t, []string{}, role.Domains)
	})

	t.Run("disabled_role", func(t *testing.T) {
		row := roleRow{
			ID:             "role-003",
			OrganizationID: "org-001",
			Name:           "deprecated",
			DisplayName:    "Deprecated Role",
			IsEnabled:      false,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		role, err := toRole(row)
		require.NoError(t, err)
		assert.False(t, role.IsEnabled)
	})
}
