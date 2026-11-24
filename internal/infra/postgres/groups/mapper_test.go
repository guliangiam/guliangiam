package groups

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToGroup(t *testing.T) {
	now := time.Now()

	t.Run("basic_group", func(t *testing.T) {
		row := groupRow{
			ID:             "group-001",
			OrganizationID: "org-001",
			Name:           "engineering",
			DisplayName:    "Engineering Team",
			Manager:        "manager-001",
			ContactEmail:   "eng@example.com",
			Type:           "virtual",
			ParentID:       "parent-001",
			IsTopGroup:     false,
			IsEnabled:      true,
			Homepage:       "https://eng.example.com",
			Users:          []byte(`["user-001","user-002","user-003"]`),
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		group, err := toGroup(row)
		require.NoError(t, err)

		assert.Equal(t, "group-001", group.ID)
		assert.Equal(t, "org-001", group.OrganizationID)
		assert.Equal(t, "engineering", group.Name)
		assert.Equal(t, "Engineering Team", group.DisplayName)
		assert.Equal(t, "manager-001", group.Manager)
		assert.Equal(t, "virtual", group.Type)
		assert.Equal(t, "parent-001", group.ParentID)
		assert.False(t, group.IsTopGroup)
		assert.True(t, group.IsEnabled)
		assert.Equal(t, int64(1), group.Revision)
	})

	t.Run("top_level_group", func(t *testing.T) {
		row := groupRow{
			ID:             "group-002",
			OrganizationID: "org-001",
			Name:           "root",
			DisplayName:    "Root Group",
			IsTopGroup:     true,
			IsEnabled:      true,
			ParentID:       "",
			Users:          []byte(`[]`),
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		group, err := toGroup(row)
		require.NoError(t, err)
		assert.True(t, group.IsTopGroup)
		assert.Equal(t, "", group.ParentID)
	})

	t.Run("nil_users_defaults_to_empty", func(t *testing.T) {
		row := groupRow{
			ID:             "group-003",
			OrganizationID: "org-001",
			Name:           "empty",
			DisplayName:    "Empty Group",
			Users:          nil,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		group, err := toGroup(row)
		require.NoError(t, err)
		assert.Equal(t, []string{}, group.Users)
	})

	t.Run("disabled_group", func(t *testing.T) {
		row := groupRow{
			ID:             "group-004",
			OrganizationID: "org-001",
			Name:           "disabled",
			DisplayName:    "Disabled Group",
			IsEnabled:      false,
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		group, err := toGroup(row)
		require.NoError(t, err)
		assert.False(t, group.IsEnabled)
	})
}
