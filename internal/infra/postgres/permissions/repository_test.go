package permissions

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMax_Permissions(t *testing.T) {
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

func TestPermissionRow_AllFieldsAccessible(t *testing.T) {
	t.Parallel()

	now := time.Now()
	approveTime := now.Add(-1 * time.Hour)
	row := permissionRow{
		ID:             "perm-full",
		OrganizationID: "org-001",
		Name:           "users:list",
		DisplayName:    "List Users",
		Description:    "Allow listing all users",
		Users:          []byte(`["user-001"]`),
		Groups:         []byte(`["group-001"]`),
		Roles:          []byte(`["role-admin"]`),
		Domains:        []byte(`["domain-1"]`),
		ResourceType:   "users",
		Resources:      []byte(`["users"]`),
		Actions:        []byte(`["list","get"]`),
		Effect:         "allow",
		IsEnabled:      true,
		Submitter:      "admin-001",
		Approver:       "superadmin-001",
		State:          "approved",
		Model:          "RBAC",
		Adapter:        "casbin",
		Sorter:         "priority",
		Application:    "app-001",
		ApproveTime:    &approveTime,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       2,
	}

	assert.Equal(t, "perm-full", row.ID)
	assert.Equal(t, "org-001", row.OrganizationID)
	assert.Equal(t, "users:list", row.Name)
	assert.Equal(t, "List Users", row.DisplayName)
	assert.Equal(t, "users", row.ResourceType)
	assert.Equal(t, "allow", row.Effect)
	assert.True(t, row.IsEnabled)
	assert.Equal(t, "approved", row.State)
	assert.Equal(t, "RBAC", row.Model)
	assert.Equal(t, "casbin", row.Adapter)
	assert.NotNil(t, row.ApproveTime)
	assert.Equal(t, int64(2), row.Revision)
}

func TestPermissionRow_Boundary(t *testing.T) {
	t.Parallel()

	t.Run("deny_effect", func(t *testing.T) {
		row := permissionRow{
			ID:             "perm-deny",
			OrganizationID: "org-001",
			Name:           "users:delete",
			Effect:         "deny",
			IsEnabled:      true,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		assert.Equal(t, "deny", row.Effect)
	})

	t.Run("disabled_permission", func(t *testing.T) {
		row := permissionRow{
			ID:             "perm-disabled",
			OrganizationID: "org-001",
			Name:           "users:create",
			IsEnabled:      false,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		assert.False(t, row.IsEnabled)
	})

	t.Run("pending_state", func(t *testing.T) {
		row := permissionRow{
			ID:             "perm-pending",
			OrganizationID: "org-001",
			Name:           "users:update",
			State:          "pending",
			IsEnabled:      true,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		assert.Equal(t, "pending", row.State)
	})

	t.Run("nil_approve_time", func(t *testing.T) {
		row := permissionRow{
			ID:             "perm-no-approve",
			OrganizationID: "org-001",
			Name:           "users:read",
			ApproveTime:    nil,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		assert.Nil(t, row.ApproveTime)
	})

	t.Run("nil_jsonb_fields", func(t *testing.T) {
		row := permissionRow{
			ID:             "perm-nil-jsonb",
			OrganizationID: "org-001",
			Name:           "niljsonb",
			Users:          nil,
			Groups:         nil,
			Roles:          nil,
			Domains:        nil,
			Resources:      nil,
			Actions:        nil,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		assert.Nil(t, row.Users)
		assert.Nil(t, row.Groups)
		assert.Nil(t, row.Roles)
		assert.Nil(t, row.Domains)
		assert.Nil(t, row.Resources)
		assert.Nil(t, row.Actions)
	})

	t.Run("multiple_resource_types", func(t *testing.T) {
		row := permissionRow{
			ID:             "perm-multi-resource",
			OrganizationID: "org-001",
			Name:           "resources:all",
			ResourceType:   "applications",
			Resources:      []byte(`["applications","users","groups"]`),
			Actions:        []byte(`["*"]`),
			Effect:         "allow",
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		assert.Equal(t, "applications", row.ResourceType)
	})
}

func TestToPermission_Boundary(t *testing.T) {
	t.Parallel()

	now := time.Now()

	t.Run("permission_with_many_resources", func(t *testing.T) {
		resources := make([]string, 50)
		for i := range resources {
			resources[i] = "resource"
		}
		resourcesJSON, _ := json.Marshal(resources)
		row := permissionRow{
			ID:             "perm-many-resources",
			OrganizationID: "org-001",
			Name:           "resources:many",
			Resources:      resourcesJSON,
			Actions:        []byte(`["read"]`),
			Users:          []byte(`[]`),
			Groups:         []byte(`[]`),
			Roles:          []byte(`[]`),
			Domains:        []byte(`[]`),
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		perm, err := toPermission(row)
		require.NoError(t, err)
		assert.Len(t, perm.Resources, 50)
	})

	t.Run("invalid_jsonb_resources", func(t *testing.T) {
		row := permissionRow{
			ID:             "perm-invalid",
			OrganizationID: "org-001",
			Name:           "invalid",
			Resources:      []byte(`invalid json{{{`),
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		_, err := toPermission(row)
		assert.Error(t, err)
	})

	t.Run("casbin_policy_structure", func(t *testing.T) {
		row := permissionRow{
			ID:             "perm-casbin",
			OrganizationID: "org-001",
			Name:           "casbin:policy",
			Users:          []byte(`["user-001"]`),
			Roles:          []byte(`["role-admin"]`),
			Domains:        []byte(`["domain-1"]`),
			Resources:      []byte(`["users","groups"]`),
			Actions:        []byte(`["read","write"]`),
			Effect:         "allow",
			Model:          "RBAC",
			Adapter:        "casbin",
			IsEnabled:      true,
			State:          "approved",
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		perm, err := toPermission(row)
		require.NoError(t, err)
		assert.Equal(t, "RBAC", perm.Model)
		assert.Equal(t, "casbin", perm.Adapter)
		assert.Equal(t, "approved", perm.State)
		assert.Len(t, perm.Resources, 2)
		assert.Len(t, perm.Actions, 2)
	})
}
