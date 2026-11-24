package mfachallenges

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChallenge_Creation(t *testing.T) {
	now := time.Now()
	c := Challenge{
		ID:             "mfac-001",
		OrganizationID: "org-001",
		UserID:         "user-001",
		ApplicationID:  "app-001",
		State:          StatePending,
		MethodType:     "totp",
		ExpiresAt:      now.Add(5 * time.Minute),
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	assert.Equal(t, StatePending, c.State)
	assert.Equal(t, "totp", c.MethodType)
	assert.Equal(t, 0, c.FailedAttempts)
}

func TestChallenge_StateConstants(t *testing.T) {
	assert.Equal(t, "pending", StatePending)
	assert.Equal(t, "push_pending", StatePushPending)
	assert.Equal(t, "approved", StateApproved)
	assert.Equal(t, "rejected", StateRejected)
}

func TestChallenge_StateTransitions(t *testing.T) {
	validTransitions := map[string][]string{
		StatePending:     {StatePushPending, StateApproved, StateRejected},
		StatePushPending: {StateApproved, StateRejected},
		StateApproved:    {}, // terminal
		StateRejected:    {}, // terminal
	}

	tests := []struct {
		from  string
		to    string
		valid bool
	}{
		{from: StatePending, to: StateApproved, valid: true},
		{from: StatePending, to: StateRejected, valid: true},
		{from: StatePending, to: StatePushPending, valid: true},
		{from: StatePushPending, to: StateApproved, valid: true},
		{from: StatePushPending, to: StateRejected, valid: true},
		{from: StateApproved, to: StatePending, valid: false}, // terminal state
		{from: StateRejected, to: StatePending, valid: false}, // terminal state
	}

	for _, tt := range tests {
		t.Run(tt.from+"->"+tt.to, func(t *testing.T) {
			transitions, ok := validTransitions[tt.from]
			assert.True(t, ok)
			isValid := false
			for _, allowed := range transitions {
				if allowed == tt.to {
					isValid = true
					break
				}
			}
			assert.Equal(t, tt.valid, isValid)
		})
	}
}

func TestChallenge_FailedAttemptsIncrement(t *testing.T) {
	now := time.Now()
	c := Challenge{
		ID:        "mfac-001",
		State:     StatePending,
		ExpiresAt: now.Add(5 * time.Minute),
	}

	assert.Equal(t, 0, c.FailedAttempts)
	c.FailedAttempts++
	assert.Equal(t, 1, c.FailedAttempts)
}

func TestChallenge_Expiration(t *testing.T) {
	now := time.Now()

	t.Run("not_expired", func(t *testing.T) {
		c := Challenge{ExpiresAt: now.Add(5 * time.Minute)}
		assert.True(t, now.Before(c.ExpiresAt))
	})

	t.Run("expired", func(t *testing.T) {
		c := Challenge{ExpiresAt: now.Add(-1 * time.Minute)}
		assert.True(t, now.After(c.ExpiresAt))
	})
}
