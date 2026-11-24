package records

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCreateRecordRequest_Fields(t *testing.T) {
	t.Run("full_fields", func(t *testing.T) {
		req := CreateRecordRequest{
			OrganizationID: "org-001",
			ActorID:        "user-001",
			Action:         "login",
			ResourceType:   "session",
			ResourceID:     "sess-001",
			IP:             "192.168.1.1",
			UserAgent:      "Mozilla/5.0",
			Metadata:       map[string]any{"browser": "Chrome"},
		}

		assert.Equal(t, "org-001", req.OrganizationID)
		assert.Equal(t, "login", req.Action)
		assert.Equal(t, "Chrome", req.Metadata["browser"])
	})
}

func TestSearchRecordsRequest_Fields(t *testing.T) {
	t.Run("with_time_range", func(t *testing.T) {
		start := time.Now().Add(-24 * time.Hour)
		end := time.Now()
		req := SearchRecordsRequest{
			Page:           1,
			PageSize:       50,
			OrganizationID: "org-001",
			Action:         "login",
			StartTime:      &start,
			EndTime:        &end,
		}

		assert.Equal(t, 1, req.Page)
		assert.NotNil(t, req.StartTime)
		assert.NotNil(t, req.EndTime)
	})

	t.Run("nil_time_range", func(t *testing.T) {
		req := SearchRecordsRequest{
			StartTime: nil,
			EndTime:   nil,
		}

		assert.Nil(t, req.StartTime)
		assert.Nil(t, req.EndTime)
	})
}
