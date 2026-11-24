package groups

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMax_Groups(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    int
		fallback int
		want     int
	}{
		{"positive_value", 5, 10, 5},
		{"zero_uses_fallback", 0, 10, 10},
		{"negative_uses_fallback", -1, 20, 20},
		{"both_positive", 15, 10, 15},
		{"same_values", 10, 10, 10},
		{"large_page_size", 500, 20, 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maxInt(tt.value, tt.fallback)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGroupRow_AllFieldsAccessible(t *testing.T) {
	t.Parallel()

	now := time.Now()
	row := groupRow{
		ID:             "group-full",
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
		Revision:       3,
	}

	assert.Equal(t, "group-full", row.ID)
	assert.Equal(t, "org-001", row.OrganizationID)
	assert.Equal(t, "engineering", row.Name)
	assert.Equal(t, "Engineering Team", row.DisplayName)
	assert.Equal(t, "manager-001", row.Manager)
	assert.Equal(t, "eng@example.com", row.ContactEmail)
	assert.Equal(t, "virtual", row.Type)
	assert.Equal(t, "parent-001", row.ParentID)
	assert.False(t, row.IsTopGroup)
	assert.True(t, row.IsEnabled)
	assert.Equal(t, "https://eng.example.com", row.Homepage)
	assert.Equal(t, int64(3), row.Revision)
}

func TestGroupRow_Boundary(t *testing.T) {
	t.Parallel()

	t.Run("empty_name", func(t *testing.T) {
		row := groupRow{
			ID:             "group-empty-name",
			OrganizationID: "org-001",
			Name:           "",
			DisplayName:    "Empty Name Group",
			IsEnabled:      true,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		assert.Equal(t, "", row.Name)
	})

	t.Run("disabled_group", func(t *testing.T) {
		row := groupRow{
			ID:             "group-disabled",
			OrganizationID: "org-001",
			Name:           "disabled",
			IsEnabled:      false,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		assert.False(t, row.IsEnabled)
	})

	t.Run("nil_users", func(t *testing.T) {
		row := groupRow{
			ID:             "group-nil-users",
			OrganizationID: "org-001",
			Name:           "nilusers",
			Users:          nil,
			IsEnabled:      true,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		assert.Nil(t, row.Users)
	})

	t.Run("empty_users", func(t *testing.T) {
		row := groupRow{
			ID:             "group-empty-users",
			OrganizationID: "org-001",
			Name:           "emptyusers",
			Users:          []byte(`[]`),
			IsEnabled:      true,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		assert.Equal(t, []byte(`[]`), row.Users)
	})

	t.Run("long_contact_email", func(t *testing.T) {
		longEmail := "very.long.email.address.that.exceeds.normal.length@very-long-domain-name.example.com"
		row := groupRow{
			ID:             "group-long-email",
			OrganizationID: "org-001",
			Name:           "longemail",
			ContactEmail:   longEmail,
			IsEnabled:      true,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		assert.Equal(t, longEmail, row.ContactEmail)
	})
}

func TestGroupRow_TreeStructure(t *testing.T) {
	t.Parallel()

	t.Run("top_level_group", func(t *testing.T) {
		row := groupRow{
			ID:             "group-root",
			OrganizationID: "org-001",
			Name:           "root",
			DisplayName:    "Root Group",
			ParentID:       "",
			IsTopGroup:     true,
			IsEnabled:      true,
			Users:          []byte(`[]`),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		assert.True(t, row.IsTopGroup)
		assert.Equal(t, "", row.ParentID)
	})

	t.Run("child_group", func(t *testing.T) {
		row := groupRow{
			ID:             "group-child",
			OrganizationID: "org-001",
			Name:           "child",
			DisplayName:    "Child Group",
			ParentID:       "group-root",
			IsTopGroup:     false,
			IsEnabled:      true,
			Users:          []byte(`[]`),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		assert.False(t, row.IsTopGroup)
		assert.Equal(t, "group-root", row.ParentID)
	})

	t.Run("deeply_nested_group", func(t *testing.T) {
		row := groupRow{
			ID:             "group-deep",
			OrganizationID: "org-001",
			Name:           "deep",
			DisplayName:    "Deeply Nested Group",
			ParentID:       "group-child",
			IsTopGroup:     false,
			IsEnabled:      true,
			Users:          []byte(`[]`),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		assert.False(t, row.IsTopGroup)
		assert.Equal(t, "group-child", row.ParentID)
	})

	t.Run("group_types", func(t *testing.T) {
		tests := []struct {
			name      string
			groupType string
		}{
			{"virtual_type", "virtual"},
			{"physical_type", "physical"},
			{"department_type", "department"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				row := groupRow{
					ID:             "group-type-" + tt.groupType,
					OrganizationID: "org-001",
					Name:           tt.groupType,
					Type:           tt.groupType,
					IsEnabled:      true,
					CreatedAt:      time.Now(),
					UpdatedAt:      time.Now(),
				}
				assert.Equal(t, tt.groupType, row.Type)
			})
		}
	})
}

func TestToGroup_Boundary(t *testing.T) {
	t.Parallel()

	now := time.Now()

	t.Run("group_with_many_users", func(t *testing.T) {
		users := make([]string, 200)
		for i := range users {
			users[i] = "user-id"
		}
		usersJSON, _ := json.Marshal(users)
		row := groupRow{
			ID:             "group-many-users",
			OrganizationID: "org-001",
			Name:           "manyusers",
			Users:          usersJSON,
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		group, err := toGroup(row)
		require.NoError(t, err)
		assert.Len(t, group.Users, 200)
	})

	t.Run("invalid_jsonb_users", func(t *testing.T) {
		row := groupRow{
			ID:             "group-invalid",
			OrganizationID: "org-001",
			Name:           "invalid",
			Users:          []byte(`invalid json{{{`),
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		_, err := toGroup(row)
		assert.Error(t, err)
	})

	t.Run("top_group_with_no_parent", func(t *testing.T) {
		row := groupRow{
			ID:             "group-top-no-parent",
			OrganizationID: "org-001",
			Name:           "topnoparent",
			ParentID:       "",
			IsTopGroup:     true,
			Users:          []byte(`[]`),
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		group, err := toGroup(row)
		require.NoError(t, err)
		assert.True(t, group.IsTopGroup)
		assert.Equal(t, "", group.ParentID)
	})

	t.Run("group_with_manager_and_contact", func(t *testing.T) {
		row := groupRow{
			ID:             "group-with-contact",
			OrganizationID: "org-001",
			Name:           "withcontact",
			Manager:        "manager-001",
			ContactEmail:   "team@example.com",
			Homepage:       "https://team.example.com",
			Users:          []byte(`[]`),
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		group, err := toGroup(row)
		require.NoError(t, err)
		assert.Equal(t, "manager-001", group.Manager)
		assert.Equal(t, "team@example.com", group.ContactEmail)
		assert.Equal(t, "https://team.example.com", group.Homepage)
	})
}
