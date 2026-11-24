package lockoutattempts

import (
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

func TestAttempt_Validate(t *testing.T) {
	t.Run("valid_attempt", func(t *testing.T) {
		a := Attempt{UserID: "user-001", Attempts: 0}
		assert.NoError(t, a.Validate())
	})

	t.Run("missing_userId", func(t *testing.T) {
		a := Attempt{Attempts: 0}
		assert.Error(t, a.Validate())
	})

	t.Run("negative_attempts", func(t *testing.T) {
		a := Attempt{UserID: "user-001", Attempts: -1}
		assert.Error(t, a.Validate())
	})
}

func TestAttempt_IsLocked(t *testing.T) {
	now := time.Now()

	t.Run("not_locked_nil", func(t *testing.T) {
		a := Attempt{UserID: "user-001", Attempts: 3}
		assert.False(t, a.IsLocked(now))
	})

	t.Run("locked_future", func(t *testing.T) {
		lockedUntil := now.Add(30 * time.Minute)
		a := Attempt{UserID: "user-001", Attempts: 5, LockedUntil: &lockedUntil}
		assert.True(t, a.IsLocked(now))
	})

	t.Run("lockout_expired", func(t *testing.T) {
		lockedUntil := now.Add(-1 * time.Minute)
		a := Attempt{UserID: "user-001", Attempts: 5, LockedUntil: &lockedUntil}
		assert.False(t, a.IsLocked(now))
	})
}

func TestAttempt_ShouldLock(t *testing.T) {
	tests := []struct {
		name        string
		attempts    int
		maxAttempts int
		want        bool
	}{
		{"0_attempts", 0, 5, false},
		{"4_attempts", 4, 5, false},
		{"5_attempts_triggers_lock", 5, 5, true},
		{"6_attempts_still_locked", 6, 5, true},
		{"custom_threshold_3", 3, 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Attempt{Attempts: tt.attempts}
			assert.Equal(t, tt.want, a.ShouldLock(tt.maxAttempts))
		})
	}
}

func TestAttempt_Lock(t *testing.T) {
	now := time.Now()
	a := Attempt{UserID: "user-001", Attempts: 5}

	a.Lock(now, DefaultLockoutDuration)

	assert.NotNil(t, a.LockedUntil)
	assert.Equal(t, now.Add(DefaultLockoutDuration), *a.LockedUntil)
	assert.True(t, a.IsLocked(now))
}

func TestAttempt_Increment(t *testing.T) {
	a := Attempt{UserID: "user-001", Attempts: 3}
	a.Increment()
	assert.Equal(t, 4, a.Attempts)

	a.Increment()
	assert.Equal(t, 5, a.Attempts)
}

func TestAttempt_Reset(t *testing.T) {
	now := time.Now()
	lockedUntil := now.Add(30 * time.Minute)
	a := Attempt{UserID: "user-001", Attempts: 5, LockedUntil: &lockedUntil}

	a.Reset()

	assert.Equal(t, 0, a.Attempts)
	assert.Nil(t, a.LockedUntil)
}

func TestAttempt_FullLockoutCycle(t *testing.T) {
	now := time.Now()
	a := Attempt{UserID: "user-001", Attempts: 0}

	// Simulate 5 failed login attempts
	for i := 0; i < 5; i++ {
		a.Increment()
	}
	assert.True(t, a.ShouldLock(DefaultMaxAttempts))

	// Lock the account
	a.Lock(now, DefaultLockoutDuration)
	assert.True(t, a.IsLocked(now))

	// After lockout duration expires, successful login resets
	future := now.Add(31 * time.Minute)
	assert.False(t, a.IsLocked(future))
	a.Reset()
	assert.Equal(t, 0, a.Attempts)
	assert.Nil(t, a.LockedUntil)
}

func TestAttempt_Validate_Answers422(t *testing.T) {
	err := (&Attempt{UserID: "user-1", Attempts: -1}).Validate()
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "attempts", appErr.Meta["field"])
}
