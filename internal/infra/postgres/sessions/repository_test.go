package sessions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSessionRepository_Max(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maxInt(tt.value, tt.fallback)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSessionRow_FieldAccess(t *testing.T) {
	t.Parallel()

	t.Run("all_fields_accessible", func(t *testing.T) {
		row := sessionRow{
			ID:               "sess-001",
			UserID:           "user-001",
			Username:         "testuser",
			OrganizationID:   "org-001",
			OrganizationSlug: "test-org",
			ApplicationID:    "app-001",
			ApplicationSlug:  "test-app",
			SessionState:     "active",
			IPAddress:        "192.168.1.1",
			UserAgent:        "Mozilla/5.0",
			Revision:         1,
		}

		assert.Equal(t, "sess-001", row.ID)
		assert.Equal(t, "user-001", row.UserID)
		assert.Equal(t, "testuser", row.Username)
		assert.Equal(t, "org-001", row.OrganizationID)
		assert.Equal(t, "test-org", row.OrganizationSlug)
		assert.Equal(t, "app-001", row.ApplicationID)
		assert.Equal(t, "test-app", row.ApplicationSlug)
		assert.Equal(t, "active", row.SessionState)
		assert.Equal(t, "192.168.1.1", row.IPAddress)
		assert.Equal(t, "Mozilla/5.0", row.UserAgent)
		assert.Equal(t, int64(1), row.Revision)
	})
}

// --- Enhanced boundary tests for Sessions Repository ---

func TestSessionRow_Boundary(t *testing.T) {
	t.Parallel()

	t.Run("revoked_session", func(t *testing.T) {
		revokedAt := time.Now().Add(-1 * time.Hour)
		row := sessionRow{
			ID:           "sess-revoked",
			UserID:       "user-001",
			SessionState: "revoked",
			RevokedAt:    &revokedAt,
			ExpiresAt:    time.Now().Add(24 * time.Hour),
			CreatedAt:    time.Now(),
			LastSeenAt:   time.Now(),
			UpdatedAt:    time.Now(),
			Revision:     2,
		}

		assert.NotNil(t, row.RevokedAt)
		assert.Equal(t, "revoked", row.SessionState)
		assert.Equal(t, int64(2), row.Revision)
	})

	t.Run("expired_session", func(t *testing.T) {
		row := sessionRow{
			ID:           "sess-expired",
			UserID:       "user-001",
			SessionState: "expired",
			ExpiresAt:    time.Now().Add(-1 * time.Hour),
			CreatedAt:    time.Now().Add(-2 * time.Hour),
			LastSeenAt:   time.Now().Add(-90 * time.Minute),
			UpdatedAt:    time.Now(),
		}

		assert.True(t, row.ExpiresAt.Before(time.Now()))
		assert.Equal(t, "expired", row.SessionState)
	})

	t.Run("long_user_agent", func(t *testing.T) {
		longUA := string(make([]byte, 500))
		row := sessionRow{
			ID:           "sess-long-ua",
			UserID:       "user-001",
			UserAgent:    longUA,
			SessionState: "active",
			ExpiresAt:    time.Now().Add(24 * time.Hour),
			CreatedAt:    time.Now(),
			LastSeenAt:   time.Now(),
			UpdatedAt:    time.Now(),
		}

		assert.Equal(t, longUA, row.UserAgent)
	})

	t.Run("ipv6_address", func(t *testing.T) {
		row := sessionRow{
			ID:           "sess-ipv6",
			UserID:       "user-001",
			IPAddress:    "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			SessionState: "active",
			ExpiresAt:    time.Now().Add(24 * time.Hour),
			CreatedAt:    time.Now(),
			LastSeenAt:   time.Now(),
			UpdatedAt:    time.Now(),
		}

		assert.Equal(t, "2001:0db8:85a3:0000:0000:8a2e:0370:7334", row.IPAddress)
	})

	t.Run("empty_device_info", func(t *testing.T) {
		row := sessionRow{
			ID:           "sess-empty-device",
			UserID:       "user-001",
			DeviceInfo:   []byte("{}"),
			SessionState: "active",
			ExpiresAt:    time.Now().Add(24 * time.Hour),
			CreatedAt:    time.Now(),
			LastSeenAt:   time.Now(),
			UpdatedAt:    time.Now(),
		}

		assert.Equal(t, []byte("{}"), row.DeviceInfo)
	})

	t.Run("nil_device_info", func(t *testing.T) {
		row := sessionRow{
			ID:           "sess-nil-device",
			UserID:       "user-001",
			DeviceInfo:   nil,
			SessionState: "active",
			ExpiresAt:    time.Now().Add(24 * time.Hour),
			CreatedAt:    time.Now(),
			LastSeenAt:   time.Now(),
			UpdatedAt:    time.Now(),
		}

		assert.Nil(t, row.DeviceInfo)
	})
}

func TestMax_Sessions_Boundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    int
		fallback int
		want     int
	}{
		{"large_page_size", 500, 20, 500},
		{"zero_page_size_uses_fallback", 0, 20, 20},
		{"negative_page_size_uses_fallback", -1, 20, 20},
		{"one_value", 1, 20, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maxInt(tt.value, tt.fallback)
			assert.Equal(t, tt.want, got)
		})
	}
}
