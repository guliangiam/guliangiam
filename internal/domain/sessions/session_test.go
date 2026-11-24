package sessions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestSession_Creation(t *testing.T) {
	now := time.Now()
	s := Session{
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
		ExpiresAt:        now.Add(24 * time.Hour),
		CreatedAt:        now,
		LastSeenAt:       now,
		UpdatedAt:        now,
		Revision:         1,
	}

	assert.Equal(t, "sess-001", s.ID)
	assert.Equal(t, StateActive, s.SessionState)
	assert.Nil(t, s.RevokedAt, "new session should not be revoked")
}

func TestSession_SetStateAnswers422(t *testing.T) {
	s := Session{SessionState: StateActive}

	err := s.SetState(State("bogus"))
	if err == nil {
		t.Fatal("unknown target state must fail")
	}
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "sessionState", appErr.Meta["field"])
	assert.Equal(t, "bogus", appErr.Meta["value"])
	assert.Equal(t, []string{"active", "idle", "expired", "revoked"}, appErr.Meta["allowed"])

	terminal := Session{SessionState: StateRevoked}
	err = terminal.SetState(StateActive)
	if err == nil {
		t.Fatal("revoked is terminal; revoked → active must fail")
	}
	appErr = apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "revoked", appErr.Meta["value"])
	assert.Equal(t, []string{}, appErr.Meta["allowed"])
}

func TestSession_Revocation(t *testing.T) {
	now := time.Now()

	t.Run("active_session_not_revoked", func(t *testing.T) {
		s := Session{
			ID:           "sess-001",
			SessionState: "active",
			ExpiresAt:    now.Add(24 * time.Hour),
		}
		assert.Nil(t, s.RevokedAt)
	})

	t.Run("revoked_session", func(t *testing.T) {
		revokedAt := now
		s := Session{
			ID:           "sess-001",
			SessionState: "revoked",
			RevokedAt:    &revokedAt,
			ExpiresAt:    now.Add(24 * time.Hour),
		}
		assert.NotNil(t, s.RevokedAt)
	})
}

func TestSession_Expiration(t *testing.T) {
	now := time.Now()

	t.Run("not_expired", func(t *testing.T) {
		s := Session{ExpiresAt: now.Add(1 * time.Hour)}
		assert.True(t, now.Before(s.ExpiresAt))
	})

	t.Run("expired", func(t *testing.T) {
		s := Session{ExpiresAt: now.Add(-1 * time.Hour)}
		assert.True(t, now.After(s.ExpiresAt))
	})
}

func TestSession_RevokedAtSemantics(t *testing.T) {
	now := time.Now()
	revokedAt := now

	s := Session{
		ID:        "sess-001",
		RevokedAt: &revokedAt,
	}
	assert.NotNil(t, s.RevokedAt, "RevokedAt non-nil means session is revoked")

	s2 := Session{ID: "sess-002"}
	assert.Nil(t, s2.RevokedAt, "RevokedAt nil means session is active")
}
