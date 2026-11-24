package records

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func strPtr(s string) *string { return &s }

func TestToRecord(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		row := recordRow{
			ID:             "rec-001",
			OrganizationID: "org-001",
			ActorID:        strPtr("user-001"),
			Action:         "login",
			ResourceType:   "session",
			ResourceID:     strPtr("sess-001"),
			IP:             strPtr("192.168.1.1"),
			UserAgent:      "Mozilla/5.0",
			Metadata:       map[string]any{"browser": "Chrome", "os": "Linux"},
			CreatedAt:      now,
		}

		result := toRecord(row)

		assert.Equal(t, "rec-001", result.ID)
		assert.Equal(t, "org-001", result.OrganizationID)
		assert.Equal(t, "user-001", result.ActorID)
		assert.Equal(t, "login", result.Action)
		assert.Equal(t, "session", result.ResourceType)
		assert.Equal(t, "sess-001", result.ResourceID)
		assert.Equal(t, "192.168.1.1", result.IP)
		assert.Equal(t, "Mozilla/5.0", result.UserAgent)
		assert.Equal(t, "Chrome", result.Metadata["browser"])
	})

	t.Run("minimal_fields", func(t *testing.T) {
		now := time.Now()
		row := recordRow{
			ID:             "rec-002",
			OrganizationID: "org-002",
			CreatedAt:      now,
		}

		result := toRecord(row)

		assert.Equal(t, "rec-002", result.ID)
		assert.Equal(t, "", result.ActorID)
		assert.Equal(t, "", result.Action)
		assert.Nil(t, result.Metadata)
	})
}
