package qrcodesessions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestSession_Validate(t *testing.T) {
	now := time.Now().Add(5 * time.Minute)

	t.Run("valid_session", func(t *testing.T) {
		s := Session{
			ID:             "qrs-001",
			CodeID:         "code-abc",
			OrganizationID: "org-1",
			Status:         "pending",
			ExpiresAt:      now,
		}
		assert.NoError(t, s.Validate())
	})

	t.Run("missing_id", func(t *testing.T) {
		s := Session{CodeID: "code", OrganizationID: "org", Status: "pending", ExpiresAt: now}
		assert.Error(t, s.Validate())
	})

	t.Run("missing_codeId", func(t *testing.T) {
		s := Session{ID: "qrs-001", OrganizationID: "org", Status: "pending", ExpiresAt: now}
		assert.Error(t, s.Validate())
	})

	t.Run("missing_organizationId", func(t *testing.T) {
		s := Session{ID: "qrs-001", CodeID: "code", Status: "pending", ExpiresAt: now}
		assert.Error(t, s.Validate())
	})

	t.Run("invalid_status", func(t *testing.T) {
		s := Session{ID: "qrs-001", CodeID: "code", OrganizationID: "org", Status: "invalid", ExpiresAt: now}
		err := s.Validate()
		assert.Error(t, err)
		appErr := apperrors.FromError(err)
		assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
		assert.Equal(t, "status", appErr.Meta["field"])
	})

	t.Run("missing_expiresAt", func(t *testing.T) {
		s := Session{ID: "qrs-001", CodeID: "code", OrganizationID: "org", Status: "pending"}
		assert.Error(t, s.Validate())
	})
}

func TestSession_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name       string
		fromStatus string
		toStatus   string
		want       bool
	}{
		// Happy path: strict one-way flow
		{"pending_to_scanned", "pending", "scanned", true},
		{"pending_to_expired", "pending", "expired", true},
		{"scanned_to_confirmed", "scanned", "confirmed", true},
		{"scanned_to_expired", "scanned", "expired", true},
		{"confirmed_to_exchanged", "confirmed", "exchanged", true},
		{"confirmed_to_expired", "confirmed", "expired", true},

		// Illegal transitions: backward or skip
		{"pending_to_confirmed_skip", "pending", "confirmed", false},
		{"pending_to_exchanged_skip", "pending", "exchanged", false},
		{"scanned_to_pending_backward", "scanned", "pending", false},
		{"confirmed_to_scanned_backward", "confirmed", "scanned", false},
		{"exchanged_is_terminal", "exchanged", "expired", false},
		{"exchanged_to_any", "exchanged", "pending", false},
		{"expired_is_terminal", "expired", "pending", false},
		{"expired_to_scanned", "expired", "scanned", false},

		// Same status is not a valid transition
		{"pending_to_pending", "pending", "pending", false},
		{"scanned_to_scanned", "scanned", "scanned", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Session{Status: tt.fromStatus}
			got := s.CanTransitionTo(tt.toStatus)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSession_IsExpired(t *testing.T) {
	now := time.Now()

	t.Run("not_expired", func(t *testing.T) {
		s := Session{ExpiresAt: now.Add(5 * time.Minute)}
		assert.False(t, s.IsExpired(now))
	})

	t.Run("expired", func(t *testing.T) {
		s := Session{ExpiresAt: now.Add(-1 * time.Minute)}
		assert.True(t, s.IsExpired(now))
	})
}

func TestSession_IsTerminal(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"pending_is_not_terminal", "pending", false},
		{"scanned_is_not_terminal", "scanned", false},
		{"confirmed_is_not_terminal", "confirmed", false},
		{"exchanged_is_terminal", "exchanged", true},
		{"expired_is_terminal", "expired", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Session{Status: tt.status}
			assert.Equal(t, tt.want, s.IsTerminal())
		})
	}
}
