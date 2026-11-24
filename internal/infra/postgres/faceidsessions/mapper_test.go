package faceidsessions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestToDomain(t *testing.T) {
	now := time.Now()

	t.Run("verified_session", func(t *testing.T) {
		row := sessionRow{
			ID:         "fs-001",
			SessionID:  "face-session-1",
			Challenge:  "blink",
			UserID:     "user-001",
			ExpiresAt:  now.Add(5 * time.Minute),
			Verified:   true,
			LivenessOK: true,
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		got := toDomain(row)

		assert.Equal(t, "fs-001", got.ID)
		assert.Equal(t, "face-session-1", got.SessionID)
		assert.Equal(t, "blink", got.Challenge)
		assert.Equal(t, "user-001", got.UserID)
		assert.True(t, got.Verified)
		assert.True(t, got.LivenessOK)
	})

	t.Run("unverified_session", func(t *testing.T) {
		row := sessionRow{
			ID:         "fs-002",
			Verified:   false,
			LivenessOK: false,
		}

		got := toDomain(row)

		assert.False(t, got.Verified)
		assert.False(t, got.LivenessOK)
	})
}
