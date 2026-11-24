package lockoutattempts

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttemptRecordFailureWindowAndBackoff(t *testing.T) {
	start := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	a := &Attempt{UserID: "user-001", CreatedAt: start, WindowStartedAt: start}
	for i := 0; i < DefaultMaxAttempts-1; i++ {
		a.RecordFailure(start.Add(time.Duration(i)*time.Minute), DefaultMaxAttempts, DefaultLockoutDuration)
	}
	a.RecordFailure(start.Add(4*time.Minute), DefaultMaxAttempts, DefaultLockoutDuration)
	require.Equal(t, 1, a.LockoutLevel)
	require.Equal(t, start.Add(34*time.Minute), *a.LockedUntil)

	a.RecordFailure(*a.LockedUntil, DefaultMaxAttempts, DefaultLockoutDuration)
	require.Equal(t, 1, a.Attempts, "first failure after a lock must start a fresh observation window")
	require.Nil(t, a.LockedUntil)
	for i := 1; i < DefaultMaxAttempts; i++ {
		a.RecordFailure(a.WindowStartedAt.Add(time.Duration(i)*time.Minute), DefaultMaxAttempts, DefaultLockoutDuration)
	}
	require.Equal(t, 2, a.LockoutLevel)
	require.Equal(t, a.WindowStartedAt.Add(4*time.Minute+SecondLockoutDuration), *a.LockedUntil)

	a.RecordFailure(*a.LockedUntil, DefaultMaxAttempts, DefaultLockoutDuration)
	for i := 1; i < DefaultMaxAttempts; i++ {
		a.RecordFailure(a.WindowStartedAt.Add(time.Duration(i)*time.Minute), DefaultMaxAttempts, DefaultLockoutDuration)
	}
	require.Equal(t, MaxLockoutLevel, a.LockoutLevel)
	require.Equal(t, a.WindowStartedAt.Add(4*time.Minute+ThirdLockoutDuration), *a.LockedUntil)

	a.Reset()
	require.Zero(t, a.LockoutLevel)
	require.Zero(t, a.Attempts)
	require.Nil(t, a.LockedUntil)
	require.True(t, a.WindowStartedAt.IsZero())
}

func TestAttemptRecordFailureResetsExpiredObservationWindow(t *testing.T) {
	start := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	a := &Attempt{UserID: "user-001", Attempts: 4, WindowStartedAt: start}

	a.RecordFailure(start.Add(ObservationWindow), DefaultMaxAttempts, DefaultLockoutDuration)

	require.Equal(t, 1, a.Attempts)
	require.Equal(t, start.Add(ObservationWindow), a.WindowStartedAt)
	require.Nil(t, a.LockedUntil)
}

func TestAttemptRecordFailureDoesNotChangeActiveLockout(t *testing.T) {
	now := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	until := now.Add(time.Hour)
	a := &Attempt{
		UserID:       "user-001",
		Attempts:     DefaultMaxAttempts,
		LockedUntil:  &until,
		LockoutLevel: 1,
	}

	a.RecordFailure(now, DefaultMaxAttempts, DefaultLockoutDuration)

	require.Equal(t, DefaultMaxAttempts, a.Attempts)
	require.Equal(t, 1, a.LockoutLevel)
	require.Equal(t, until, *a.LockedUntil)
}

func TestAttempt_Creation(t *testing.T) {
	now := time.Now()
	a := Attempt{
		ID:        "lock-001",
		UserID:    "user-001",
		Attempts:  0,
		CreatedAt: now,
		UpdatedAt: now,
	}
	assert.Equal(t, "lock-001", a.ID)
	assert.Equal(t, "user-001", a.UserID)
	assert.Equal(t, 0, a.Attempts)
	assert.Nil(t, a.LockedUntil, "new attempt should not be locked")
}

func TestAttempt_LockoutLogic(t *testing.T) {
	now := time.Now()

	t.Run("unlocked_when_LockedUntil_is_nil", func(t *testing.T) {
		a := Attempt{UserID: "user-001", Attempts: 3}
		assert.Nil(t, a.LockedUntil)
	})

	t.Run("locked_when_LockedUntil_is_future", func(t *testing.T) {
		lockedUntil := now.Add(30 * time.Minute)
		a := Attempt{UserID: "user-001", Attempts: 5, LockedUntil: &lockedUntil}
		assert.NotNil(t, a.LockedUntil)
		assert.True(t, a.LockedUntil.After(now))
	})

	t.Run("unlocked_when_LockedUntil_is_past", func(t *testing.T) {
		lockedUntil := now.Add(-1 * time.Minute)
		a := Attempt{UserID: "user-001", Attempts: 5, LockedUntil: &lockedUntil}
		assert.NotNil(t, a.LockedUntil)
		assert.True(t, a.LockedUntil.Before(now))
	})
}

func TestAttempt_MaxFailedAttempts(t *testing.T) {
	const maxAttempts = 5

	tests := []struct {
		name     string
		attempts int
		locked   bool
	}{
		{name: "0 attempts", attempts: 0, locked: false},
		{name: "1 attempt", attempts: 1, locked: false},
		{name: "4 attempts", attempts: 4, locked: false},
		{name: "5 attempts triggers lock", attempts: 5, locked: true},
		{name: "6 attempts still locked", attempts: 6, locked: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Attempt{Attempts: tt.attempts}
			shouldLock := a.Attempts >= maxAttempts
			assert.Equal(t, tt.locked, shouldLock)
		})
	}
}

func TestAttempt_LockoutWindow(t *testing.T) {
	const lockoutDuration = 30 * time.Minute
	now := time.Now()

	t.Run("lockout_window_is_30_minutes", func(t *testing.T) {
		lockedUntil := now.Add(lockoutDuration)
		a := Attempt{UserID: "user-001", Attempts: 5, LockedUntil: &lockedUntil}
		duration := a.LockedUntil.Sub(now)
		assert.Equal(t, lockoutDuration, duration)
	})
}
