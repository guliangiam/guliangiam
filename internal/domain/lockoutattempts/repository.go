package lockoutattempts

import (
	"context"
	"time"
)

// Repository defines the interface for login lockout attempt persistence.
type Repository interface {
	GetByUserID(ctx context.Context, userID string) (*Attempt, error)
	Create(ctx context.Context, attempt *Attempt) error
	Update(ctx context.Context, attempt *Attempt) error
	// RecordFailure applies one failed login atomically: it creates the counter,
	// increments it, resets it after an expired lock, and locks the account when
	// the threshold is reached — all in a single statement so concurrent failures
	// cannot lose increments. The returned Attempt is the post-write state.
	RecordFailure(ctx context.Context, userID string, maxAttempts int, lockDurationMinutes int, now time.Time) (*Attempt, error)
	DeleteByUserID(ctx context.Context, userID string) error
	DeleteExpired(ctx context.Context) (int64, error)
}
