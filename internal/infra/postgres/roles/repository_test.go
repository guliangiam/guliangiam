package roles

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMax_Roles(t *testing.T) {
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

func TestRoleRow_AllFieldsAccessible(t *testing.T) {
	t.Parallel()

	now := time.Now()
	row := roleRow{
		ID:             "role-full",
		OrganizationID: "org-001",
		Name:           "admin",
		DisplayName:    "Administrator",
		Description:    "Full administrative access",
		Users:          []byte(`["user-001","user-002"]`),
		GroupIDs:       []byte(`["group-001","group-002"]`),
		RoleIDs:        []byte(`["role-super"]`),
		Domains:        []byte(`["domain-001"]`),
		IsEnabled:      true,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       3,
	}

	assert.Equal(t, "role-full", row.ID)
	assert.Equal(t, "org-001", row.OrganizationID)
	assert.Equal(t, "admin", row.Name)
	assert.Equal(t, "Administrator", row.DisplayName)
	assert.Equal(t, "Full administrative access", row.Description)
	assert.True(t, row.IsEnabled)
	assert.Equal(t, int64(3), row.Revision)
}

func TestRoleRow_Boundary(t *testing.T) {
	t.Parallel()

	t.Run("empty_name", func(t *testing.T) {
		row := roleRow{
			ID:             "role-empty-name",
			OrganizationID: "org-001",
			Name:           "",
			DisplayName:    "Empty Name Role",
			IsEnabled:      true,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		assert.Equal(t, "", row.Name)
	})

	t.Run("very_long_description", func(t *testing.T) {
		longDesc := string(make([]byte, 2000))
		row := roleRow{
			ID:             "role-long-desc",
			OrganizationID: "org-001",
			Name:           "longdesc",
			Description:    longDesc,
			IsEnabled:      true,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		assert.Equal(t, longDesc, row.Description)
	})

	t.Run("disabled_role", func(t *testing.T) {
		row := roleRow{
			ID:             "role-disabled",
			OrganizationID: "org-001",
			Name:           "disabled",
			IsEnabled:      false,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		assert.False(t, row.IsEnabled)
	})

	t.Run("nil_jsonb_fields", func(t *testing.T) {
		row := roleRow{
			ID:             "role-nil-jsonb",
			OrganizationID: "org-001",
			Name:           "niljsonb",
			Users:          nil,
			GroupIDs:       nil,
			RoleIDs:        nil,
			Domains:        nil,
			IsEnabled:      true,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		assert.Nil(t, row.Users)
		assert.Nil(t, row.GroupIDs)
		assert.Nil(t, row.RoleIDs)
		assert.Nil(t, row.Domains)
	})

	t.Run("empty_jsonb_fields", func(t *testing.T) {
		row := roleRow{
			ID:             "role-empty-jsonb",
			OrganizationID: "org-001",
			Name:           "emptyjsonb",
			Users:          []byte(`[]`),
			GroupIDs:       []byte(`[]`),
			RoleIDs:        []byte(`[]`),
			Domains:        []byte(`[]`),
			IsEnabled:      true,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		assert.Equal(t, []byte(`[]`), row.Users)
	})
}

func TestToRole_Boundary(t *testing.T) {
	t.Parallel()

	now := time.Now()

	t.Run("role_with_many_users", func(t *testing.T) {
		users := make([]string, 100)
		for i := range users {
			users[i] = "user-id"
		}
		usersJSON, _ := json.Marshal(users)
		row := roleRow{
			ID:             "role-many-users",
			OrganizationID: "org-001",
			Name:           "manyusers",
			Users:          usersJSON,
			GroupIDs:       []byte(`[]`),
			RoleIDs:        []byte(`[]`),
			Domains:        []byte(`[]`),
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		role, err := toRole(row)
		require.NoError(t, err)
		assert.Len(t, role.Users, 100)
	})

	t.Run("invalid_jsonb_users", func(t *testing.T) {
		row := roleRow{
			ID:             "role-invalid",
			OrganizationID: "org-001",
			Name:           "invalid",
			Users:          []byte(`invalid json{{{`),
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		_, err := toRole(row)
		assert.Error(t, err)
	})

	t.Run("nested_role_ids", func(t *testing.T) {
		row := roleRow{
			ID:             "role-nested",
			OrganizationID: "org-001",
			Name:           "nested",
			Users:          []byte(`[]`),
			GroupIDs:       []byte(`[]`),
			RoleIDs:        []byte(`["role-sub1","role-sub2","role-sub3"]`),
			Domains:        []byte(`[]`),
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		role, err := toRole(row)
		require.NoError(t, err)
		assert.Len(t, role.RoleIDs, 3)
	})

	t.Run("role_with_domains", func(t *testing.T) {
		row := roleRow{
			ID:             "role-domains",
			OrganizationID: "org-001",
			Name:           "domains",
			Users:          []byte(`[]`),
			GroupIDs:       []byte(`[]`),
			RoleIDs:        []byte(`[]`),
			Domains:        []byte(`["domain-1","domain-2"]`),
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		role, err := toRole(row)
		require.NoError(t, err)
		assert.Len(t, role.Domains, 2)
	})
}
