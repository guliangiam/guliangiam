package lockoutattempts

import (
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Default lockout constants per spec §5.1.1 rule 4.
const (
	DefaultMaxAttempts     = 5
	DefaultLockoutDuration = 30 * time.Minute
	ObservationWindow      = 15 * time.Minute
	SecondLockoutDuration  = 6 * time.Hour
	ThirdLockoutDuration   = 24 * time.Hour
	MaxLockoutLevel        = 3
)

// Attempt represents a login lockout tracking entry stored in the database.
type Attempt struct {
	ID              string     // UUIDv7 primary key
	UserID          string     // User being tracked
	Attempts        int        // Number of failed attempts
	LockedUntil     *time.Time // When the lockout expires (nil if not locked)
	WindowStartedAt time.Time
	LockoutLevel    int
	Revision        int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// IsLocked returns true if the account is currently locked (LockedUntil is in the future).
func (a *Attempt) IsLocked(now time.Time) bool {
	return a.LockedUntil != nil && now.Before(*a.LockedUntil)
}

// ShouldLock returns true if the attempt count has reached the threshold.
func (a *Attempt) ShouldLock(maxAttempts int) bool {
	return a.Attempts >= maxAttempts
}

// Lock sets the LockedUntil to now + duration, persisting the lockout to PostgreSQL.
func (a *Attempt) Lock(now time.Time, duration time.Duration) {
	lockedUntil := now.Add(duration)
	a.LockedUntil = &lockedUntil
}

// Increment increments the failed attempt count.
func (a *Attempt) Increment() {
	a.Attempts++
}

// Reset clears the attempts and lockout state (called on successful login).
func (a *Attempt) Reset() {
	a.Attempts = 0
	a.LockedUntil = nil
	a.WindowStartedAt = time.Time{}
	a.LockoutLevel = 0
}

// RecordFailure applies one failed login attempt. Persistence adapters implement
// the same transition atomically in SQL.
func (a *Attempt) RecordFailure(now time.Time, maxAttempts int, firstLockoutDuration time.Duration) {
	if a.LockedUntil != nil && now.Before(*a.LockedUntil) {
		return
	}

	if a.LockedUntil != nil || a.WindowStartedAt.IsZero() || !now.Before(a.WindowStartedAt.Add(ObservationWindow)) {
		a.Attempts = 1
		a.WindowStartedAt = now
		a.LockedUntil = nil
	} else {
		a.Attempts++
	}

	if a.Attempts < maxAttempts {
		return
	}

	duration := firstLockoutDuration
	switch a.LockoutLevel {
	case 1:
		duration = SecondLockoutDuration
	case 2:
		duration = ThirdLockoutDuration
	case 3:
		duration = ThirdLockoutDuration
	}
	lockedUntil := now.Add(duration)
	a.LockedUntil = &lockedUntil
	if a.LockoutLevel < MaxLockoutLevel {
		a.LockoutLevel++
	}
}

// Validate checks the attempt fields for business rule violations.
func (a *Attempt) Validate() error {
	if a.UserID == "" {
		return apperrors.NewInvalidArgument("userId is required")
	}
	if a.Attempts < 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "attempts must be non-negative", map[string]any{"field": "attempts", "value": a.Attempts, "min": 0})
	}
	return nil
}
