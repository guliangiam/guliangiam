package groups

import (
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/groups"
	"github.com/stretchr/testify/assert"
)

func TestToListItem(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		group := &domain.Group{
			ID:             "group-001",
			OrganizationID: "org-001",
			Name:           "engineering",
			DisplayName:    "Engineering",
			Type:           "virtual",
			ParentID:       "parent-001",
			IsTopGroup:     false,
			IsEnabled:      true,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		result := toListItem(group)

		assert.Equal(t, "group-001", result.ID)
		assert.Equal(t, "engineering", result.Name)
		assert.Equal(t, "virtual", result.Type)
		assert.Equal(t, "parent-001", result.ParentID)
	})
}

func TestToDetail(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		group := &domain.Group{
			ID:             "group-001",
			OrganizationID: "org-001",
			Name:           "engineering",
			DisplayName:    "Engineering",
			Manager:        "user-001",
			ContactEmail:   "eng@example.com",
			Homepage:       "https://eng.example.com",
			Users:          []string{"u1", "u2"},
			Revision:       3,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		result := toDetail(group)

		assert.Equal(t, "group-001", result.ID)
		assert.Equal(t, "user-001", result.Manager)
		assert.Equal(t, []string{"u1", "u2"}, result.Users)
		assert.Equal(t, int64(3), result.Revision)
	})
}
