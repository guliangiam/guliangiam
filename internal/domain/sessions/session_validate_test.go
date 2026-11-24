package sessions

import (
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

func TestSession_Validate(t *testing.T) {
	now := time.Now().Add(24 * time.Hour)

	t.Run("valid_session", func(t *testing.T) {
		s := Session{
			ID:             "sess-001",
			UserID:         "user-001",
			OrganizationID: "org-001",
			ExpiresAt:      now,
			SessionState:   StateActive,
		}
		assert.NoError(t, s.Validate())
	})

	t.Run("missing_id", func(t *testing.T) {
		s := Session{UserID: "user-001", OrganizationID: "org-001", ExpiresAt: now}
		assert.Error(t, s.Validate())
	})

	t.Run("missing_userId", func(t *testing.T) {
		s := Session{ID: "sess-001", OrganizationID: "org-001", ExpiresAt: now}
		assert.Error(t, s.Validate())
	})

	t.Run("missing_organizationId", func(t *testing.T) {
		s := Session{ID: "sess-001", UserID: "user-001", ExpiresAt: now}
		assert.Error(t, s.Validate())
	})

	t.Run("missing_expiresAt", func(t *testing.T) {
		s := Session{ID: "sess-001", UserID: "user-001", OrganizationID: "org-001"}
		assert.Error(t, s.Validate())
	})
}

func TestSession_IsExpired(t *testing.T) {
	now := time.Now()

	t.Run("not_expired", func(t *testing.T) {
		s := Session{ExpiresAt: now.Add(1 * time.Hour)}
		assert.False(t, s.IsExpired(now))
	})

	t.Run("expired", func(t *testing.T) {
		s := Session{ExpiresAt: now.Add(-1 * time.Hour)}
		assert.True(t, s.IsExpired(now))
	})

	t.Run("exactly_at_expiry", func(t *testing.T) {
		s := Session{ExpiresAt: now}
		assert.False(t, s.IsExpired(now)) // exactly at boundary is not "after"
	})
}

func TestSession_IsRevoked(t *testing.T) {
	t.Run("not_revoked", func(t *testing.T) {
		s := Session{ID: "sess-001"}
		assert.False(t, s.IsRevoked())
	})

	t.Run("revoked", func(t *testing.T) {
		now := time.Now()
		s := Session{ID: "sess-001", RevokedAt: &now}
		assert.True(t, s.IsRevoked())
	})
}

func TestSession_IsActive(t *testing.T) {
	now := time.Now()

	t.Run("active_session", func(t *testing.T) {
		s := Session{
			ExpiresAt: now.Add(1 * time.Hour),
		}
		assert.True(t, s.IsActive(now))
	})

	t.Run("expired_session", func(t *testing.T) {
		s := Session{
			ExpiresAt: now.Add(-1 * time.Hour),
		}
		assert.False(t, s.IsActive(now))
	})

	t.Run("revoked_session", func(t *testing.T) {
		revokedAt := now
		s := Session{
			ExpiresAt: now.Add(1 * time.Hour),
			RevokedAt: &revokedAt,
		}
		assert.False(t, s.IsActive(now))
	})

	t.Run("expired_and_revoked", func(t *testing.T) {
		revokedAt := now
		s := Session{
			ExpiresAt: now.Add(-1 * time.Hour),
			RevokedAt: &revokedAt,
		}
		assert.False(t, s.IsActive(now))
	})
}

func TestSession_Validate_StateAnswers422(t *testing.T) {
	s := &Session{
		ID:             "s-1",
		UserID:         "u-1",
		OrganizationID: "o-1",
		ExpiresAt:      time.Now().Add(time.Hour),
		SessionState:   State("bogus"),
	}
	appErr := apperrors.FromError(s.Validate())
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "sessionState", appErr.Meta["field"])
	assert.NotNil(t, appErr.Meta["allowed"])
}
