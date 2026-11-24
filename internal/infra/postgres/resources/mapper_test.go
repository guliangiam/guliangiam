package resources

import (
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/resources"
	"github.com/stretchr/testify/assert"
)

func TestToResource(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		row := resourceRow{
			ID:             "res-001",
			OrganizationID: "org-001",
			ApplicationID:  "app-001",
			Name:           "my-resource",
			DisplayName:    "My Resource",
			Type:           "api",
			Tag:            "v1",
			FilePath:       "/path/to/file",
			FileSize:       1024,
			MimeType:       "application/json",
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		result := toResource(row)

		expected := &domain.Resource{
			ID:             "res-001",
			OrganizationID: "org-001",
			ApplicationID:  "app-001",
			Name:           "my-resource",
			DisplayName:    "My Resource",
			Type:           "api",
			Tag:            "v1",
			FilePath:       "/path/to/file",
			FileSize:       1024,
			MimeType:       "application/json",
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}
		assert.Equal(t, expected, result)
	})

	t.Run("minimal_fields", func(t *testing.T) {
		now := time.Now()
		row := resourceRow{
			ID:             "res-002",
			OrganizationID: "org-002",
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		result := toResource(row)

		assert.Equal(t, "res-002", result.ID)
		assert.Equal(t, "org-002", result.OrganizationID)
		assert.Equal(t, "", result.Name)
		assert.Equal(t, int64(0), result.FileSize)
	})
}
