package roles

import (
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/roles"
	"github.com/stretchr/testify/assert"
)

func TestToListItem(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		role := &domain.Role{
			ID:             "role-001",
			OrganizationID: "org-001",
			Name:           "admin",
			DisplayName:    "Admin",
			Description:    "Administrator role",
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		result := toListItem(role)

		assert.Equal(t, "role-001", result.ID)
		assert.Equal(t, "org-001", result.OrganizationID)
		assert.Equal(t, "admin", result.Name)
		assert.Equal(t, "Admin", result.DisplayName)
		assert.True(t, result.IsEnabled)
	})
}

func TestToDetail(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		role := &domain.Role{
			ID:             "role-001",
			OrganizationID: "org-001",
			Name:           "admin",
			DisplayName:    "Admin",
			GroupIDs:       []string{"g1", "g2"},
			Users:          []string{"u1"},
			RoleIDs:        []string{"r1"},
			Domains:        []string{"d1"},
			Revision:       2,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		result := toDetail(role)

		assert.Equal(t, "role-001", result.ID)
		assert.Equal(t, []string{"g1", "g2"}, result.GroupIDs)
		assert.Equal(t, []string{"u1"}, result.Users)
		assert.Equal(t, int64(2), result.Revision)
	})
}
