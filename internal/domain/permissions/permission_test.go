package permissions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPermission_Fields(t *testing.T) {
	now := time.Now()
	approveTime := now.Add(-1 * time.Hour)

	p := Permission{
		ID:             "perm-001",
		OrganizationID: "org-001",
		Name:           "read-users",
		DisplayName:    "Read Users",
		Description:    "Allows reading user data",
		Users:          []string{"user-1", "user-2"},
		Groups:         []string{"group-1"},
		Roles:          []string{"admin"},
		Domains:        []string{"domain-1"},
		ResourceType:   "application",
		Resources:      []string{"resource-1"},
		Actions:        []string{"read"},
		Effect:         "allow",
		Model:          "rbac",
		Adapter:        "pg-adapter",
		Sorter:         "sort-1",
		Application:    "app-1",
		IsEnabled:      true,
		Submitter:      "user-1",
		Approver:       "admin-1",
		State:          "active",
		ApproveTime:    &approveTime,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	assert.Equal(t, "perm-001", p.ID)
	assert.Equal(t, "org-001", p.OrganizationID)
	assert.Equal(t, "read-users", p.Name)
	assert.Equal(t, "Read Users", p.DisplayName)
	assert.Equal(t, "Allows reading user data", p.Description)
	assert.Equal(t, []string{"user-1", "user-2"}, p.Users)
	assert.Equal(t, []string{"group-1"}, p.Groups)
	assert.Equal(t, []string{"admin"}, p.Roles)
	assert.Equal(t, []string{"domain-1"}, p.Domains)
	assert.Equal(t, "application", p.ResourceType)
	assert.Equal(t, []string{"resource-1"}, p.Resources)
	assert.Equal(t, []string{"read"}, p.Actions)
	assert.Equal(t, "allow", p.Effect)
	assert.Equal(t, "rbac", p.Model)
	assert.Equal(t, "pg-adapter", p.Adapter)
	assert.Equal(t, "sort-1", p.Sorter)
	assert.Equal(t, "app-1", p.Application)
	assert.True(t, p.IsEnabled)
	assert.Equal(t, "user-1", p.Submitter)
	assert.Equal(t, "admin-1", p.Approver)
	assert.Equal(t, "active", p.State)
	assert.NotNil(t, p.ApproveTime)
	assert.Equal(t, approveTime, *p.ApproveTime)
	assert.Equal(t, now, p.CreatedAt)
	assert.Equal(t, now, p.UpdatedAt)
	assert.Equal(t, int64(1), p.Revision)
}

func TestPermission_ApproveTimeNil(t *testing.T) {
	p := Permission{
		ID:          "perm-002",
		ApproveTime: nil,
	}
	assert.Nil(t, p.ApproveTime)
}

func TestPermission_EmptySlices(t *testing.T) {
	p := Permission{
		Users:     []string{},
		Groups:    nil,
		Roles:     []string{},
		Domains:   nil,
		Resources: []string{},
		Actions:   nil,
	}
	assert.Empty(t, p.Users)
	assert.Nil(t, p.Groups)
	assert.Empty(t, p.Roles)
	assert.Nil(t, p.Domains)
	assert.Empty(t, p.Resources)
	assert.Nil(t, p.Actions)
}

func TestPermission_ZeroValue(t *testing.T) {
	var p Permission
	assert.Equal(t, "", p.ID)
	assert.False(t, p.IsEnabled)
	assert.Equal(t, int64(0), p.Revision)
	assert.Nil(t, p.ApproveTime)
}
