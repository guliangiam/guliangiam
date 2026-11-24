package sessions

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The sid column is nullable: a row written before it existed reads back as
// NULL, and the domain must see "" so the ID token omits the claim instead of
// publishing the string "<nil>" (设计报告 14.12.3).
func TestToSession_Sid(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		sid := "gl_sid_value"
		session, err := toSession(sessionRow{ID: "sess-1", Sid: &sid})
		require.NoError(t, err)
		assert.Equal(t, "gl_sid_value", session.Sid)
	})

	t.Run("null", func(t *testing.T) {
		session, err := toSession(sessionRow{ID: "sess-1"})
		require.NoError(t, err)
		assert.Empty(t, session.Sid)
	})
}

func TestToSession(t *testing.T) {
	now := time.Now()
	deviceInfo, _ := json.Marshal(map[string]any{"browser": "Chrome", "os": "Linux"})

	t.Run("basic_mapping", func(t *testing.T) {
		row := sessionRow{
			ID:               "sess-001",
			UserID:           "user-001",
			Username:         "testuser",
			OrganizationID:   "org-001",
			OrganizationSlug: "test-org",
			ApplicationID:    "app-001",
			ApplicationSlug:  "test-app",
			SessionState:     "active",
			DeviceInfo:       deviceInfo,
			IPAddress:        "192.168.1.1",
			UserAgent:        "Mozilla/5.0",
			ExpiresAt:        now.Add(24 * time.Hour),
			CreatedAt:        now,
			LastSeenAt:       now,
			UpdatedAt:        now,
			Revision:         1,
		}

		session, err := toSession(row)
		require.NoError(t, err)

		assert.Equal(t, "sess-001", session.ID)
		assert.Equal(t, "user-001", session.UserID)
		assert.Equal(t, "testuser", session.Username)
		assert.Equal(t, "org-001", session.OrganizationID)
		assert.Equal(t, "active", string(session.SessionState))
		assert.Equal(t, "192.168.1.1", session.IPAddress)
		assert.Equal(t, int64(1), session.Revision)
		assert.Equal(t, "Chrome", session.DeviceInfo["browser"])
	})

	t.Run("nil_device_info", func(t *testing.T) {
		row := sessionRow{
			ID:           "sess-002",
			UserID:       "user-002",
			DeviceInfo:   nil,
			SessionState: "active",
			ExpiresAt:    now.Add(24 * time.Hour),
			CreatedAt:    now,
			LastSeenAt:   now,
			UpdatedAt:    now,
		}

		session, err := toSession(row)
		require.NoError(t, err)
		assert.NotNil(t, session.DeviceInfo, "DeviceInfo should be empty map, not nil")
		assert.Empty(t, session.DeviceInfo)
	})

	t.Run("empty_device_info", func(t *testing.T) {
		row := sessionRow{
			ID:           "sess-003",
			UserID:       "user-003",
			DeviceInfo:   []byte("{}"),
			SessionState: "active",
			ExpiresAt:    now.Add(24 * time.Hour),
			CreatedAt:    now,
			LastSeenAt:   now,
			UpdatedAt:    now,
		}

		session, err := toSession(row)
		require.NoError(t, err)
		assert.NotNil(t, session.DeviceInfo)
	})

	t.Run("revoked_session", func(t *testing.T) {
		revokedAt := now.Add(-1 * time.Hour)
		row := sessionRow{
			ID:           "sess-004",
			UserID:       "user-004",
			RevokedAt:    &revokedAt,
			SessionState: "revoked",
			ExpiresAt:    now.Add(24 * time.Hour),
			CreatedAt:    now,
			LastSeenAt:   now,
			UpdatedAt:    now,
		}

		session, err := toSession(row)
		require.NoError(t, err)
		assert.NotNil(t, session.RevokedAt)
	})
}

func TestToSession_InvalidDeviceInfo(t *testing.T) {
	now := time.Now()
	row := sessionRow{
		ID:           "sess-005",
		UserID:       "user-005",
		DeviceInfo:   []byte("invalid json{{{"),
		SessionState: "active",
		ExpiresAt:    now.Add(24 * time.Hour),
		CreatedAt:    now,
		LastSeenAt:   now,
		UpdatedAt:    now,
	}

	_, err := toSession(row)
	assert.Error(t, err, "invalid JSON should return error")
}
