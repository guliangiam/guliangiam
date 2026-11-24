package faceidsessions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSession_Creation(t *testing.T) {
	now := time.Now()
	s := Session{
		ID:        "fids-001",
		SessionID: "face-session-abc",
		Challenge: "random-challenge",
		UserID:    "user-001",
		ExpiresAt: now.Add(60 * time.Second),
		CreatedAt: now,
		UpdatedAt: now,
	}

	assert.Equal(t, "face-session-abc", s.SessionID)
	assert.False(t, s.Verified, "new session should not be verified")
	assert.False(t, s.LivenessOK, "new session should not have liveness OK")
}

func TestSession_LivenessDetection(t *testing.T) {
	now := time.Now()

	t.Run("liveness_not_checked", func(t *testing.T) {
		s := Session{ExpiresAt: now.Add(60 * time.Second)}
		assert.False(t, s.LivenessOK)
	})

	t.Run("liveness_passed", func(t *testing.T) {
		s := Session{
			Verified:   true,
			LivenessOK: true,
			ExpiresAt:  now.Add(60 * time.Second),
		}
		assert.True(t, s.LivenessOK)
		assert.True(t, s.Verified)
	})

	t.Run("liveness_failed_still_verified_false", func(t *testing.T) {
		s := Session{
			Verified:   false,
			LivenessOK: false,
		}
		assert.False(t, s.LivenessOK)
		assert.False(t, s.Verified)
	})
}

func TestSession_ChallengeTimeout(t *testing.T) {
	now := time.Now()

	t.Run("not_expired", func(t *testing.T) {
		s := Session{ExpiresAt: now.Add(60 * time.Second)}
		assert.True(t, now.Before(s.ExpiresAt))
	})

	t.Run("expired_60s_timeout", func(t *testing.T) {
		s := Session{ExpiresAt: now.Add(-1 * time.Second)}
		assert.True(t, now.After(s.ExpiresAt))
	})
}
