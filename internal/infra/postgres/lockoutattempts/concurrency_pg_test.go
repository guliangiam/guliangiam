// Real-PostgreSQL evidence for AC-AUTH-006: one atomic INSERT ... ON CONFLICT
// must count every concurrent failure once and perform exactly one lock transition.
package lockoutattempts

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/lockoutattempts"
)

const (
	lockoutConcurrencyN    = 16
	lockoutDurationMinutes = 30
)

func newLockoutPGPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is not set; skipping real PostgreSQL lockout concurrency test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SELECT set_config('app.current_role', 'super_admin', false)`)
		return err
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))
	t.Cleanup(pool.Close)

	return pool
}

// seedLockoutUser creates an isolated organization/user pair; the user row is
// required by the lockout_attempts foreign key.
func seedLockoutUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()

	orgID := uuid.NewString()
	userID := uuid.NewString()
	slug := "g38a-lockout-" + strings.ReplaceAll(orgID[:8], "-", "")
	username := "g38a-lockout-" + strings.ReplaceAll(userID[:8], "-", "")

	_, err := pool.Exec(ctx,
		`INSERT INTO guliangiam.organizations (id, slug, display_name) VALUES ($1::uuid, $2, $3)`,
		orgID, slug, "G3-8a lockout fixture",
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO guliangiam.users (id, organization_id, username) VALUES ($1::uuid, $2::uuid, $3)`,
		userID, orgID, username,
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM guliangiam.users WHERE id = $1::uuid`, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM guliangiam.organizations WHERE id = $1::uuid`, orgID)
	})

	return userID
}

func runConcurrentRecordFailure(t *testing.T, ctx context.Context, repo Repository, userID string, maxAttempts int, clocks []time.Time) []*domain.Attempt {
	t.Helper()

	start := make(chan struct{})
	errs := make([]error, len(clocks))
	results := make([]*domain.Attempt, len(clocks))

	var wg sync.WaitGroup
	for i := range clocks {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errs[i] = repo.RecordFailure(ctx, userID, maxAttempts, lockoutDurationMinutes, clocks[i])
		}(i)
	}

	close(start)
	wg.Wait()

	for i, err := range errs {
		require.NoErrorf(t, err, "RecordFailure goroutine %d", i)
	}
	return results
}

// TestLockoutAttemptsPostgres_ConcurrentRecordFailureCountsEveryAttempt pins
// the no-lost-update half: with a threshold above N, every concurrent failure
// must be counted exactly once.
func TestLockoutAttemptsPostgres_ConcurrentRecordFailureCountsEveryAttempt(t *testing.T) {
	pool := newLockoutPGPool(t)
	ctx := context.Background()
	userID := seedLockoutUser(t, ctx, pool)

	repo := NewRepository(pool)
	maxAttempts := lockoutConcurrencyN + 4 // no lock: counting, not saturation
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)

	clocks := make([]time.Time, lockoutConcurrencyN)
	for i := range clocks {
		clocks[i] = now
	}
	results := runConcurrentRecordFailure(t, ctx, repo, userID, maxAttempts, clocks)

	for i, attempt := range results {
		require.NotNilf(t, attempt, "goroutine %d returned no attempt", i)
		require.LessOrEqual(t, attempt.Attempts, maxAttempts)
	}

	attempt, err := repo.GetByUserID(ctx, userID)
	require.NoError(t, err)
	require.NotNil(t, attempt)
	require.Equal(t, lockoutConcurrencyN, attempt.Attempts,
		"%d concurrent failures must produce exactly %d increments", lockoutConcurrencyN, lockoutConcurrencyN)
	require.Nil(t, attempt.LockedUntil, "threshold is above N, so the account must stay unlocked")
}

// TestLockoutAttemptsPostgres_ConcurrentThresholdLocksOnce pins the
// exactly-one-lock-transition half. Each goroutine passes a distinct `now`, so
// the call that wrote the lock window is the only one whose returned
// locked_until equals its own now + lock duration.
func TestLockoutAttemptsPostgres_ConcurrentThresholdLocksOnce(t *testing.T) {
	pool := newLockoutPGPool(t)
	ctx := context.Background()
	userID := seedLockoutUser(t, ctx, pool)

	repo := NewRepository(pool)
	const maxAttempts = 5
	base := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)

	clocks := make([]time.Time, lockoutConcurrencyN)
	for i := range clocks {
		clocks[i] = base.Add(time.Duration(i) * time.Millisecond)
	}
	results := runConcurrentRecordFailure(t, ctx, repo, userID, maxAttempts, clocks)

	lockTransitions := 0
	var transitionAt time.Time
	for i, attempt := range results {
		require.NotNilf(t, attempt, "goroutine %d returned no attempt", i)
		require.LessOrEqual(t, attempt.Attempts, maxAttempts, "no call may over-count past the threshold")
		if attempt.LockedUntil != nil && attempt.LockedUntil.Equal(clocks[i].Add(lockoutDurationMinutes*time.Minute)) {
			lockTransitions++
			transitionAt = *attempt.LockedUntil
		}
	}
	require.Equal(t, 1, lockTransitions, "exactly one concurrent call may perform the lock transition")

	attempt, err := repo.GetByUserID(ctx, userID)
	require.NoError(t, err)
	require.NotNil(t, attempt)
	require.Equal(t, maxAttempts, attempt.Attempts, "attempts saturate at the threshold")
	require.NotNil(t, attempt.LockedUntil)
	require.Equal(t, transitionAt, *attempt.LockedUntil, "the stored lock window must be the one the transition wrote")

	// A later failure inside the lock window must not increment or re-lock.
	_, err = repo.RecordFailure(ctx, userID, maxAttempts, lockoutDurationMinutes, base.Add(20*time.Minute))
	require.NoError(t, err)

	locked, err := repo.GetByUserID(ctx, userID)
	require.NoError(t, err)
	require.NotNil(t, locked)
	require.Equal(t, maxAttempts, locked.Attempts)
	require.NotNil(t, locked.LockedUntil)
	require.Equal(t, transitionAt, *locked.LockedUntil, "the lock window is immutable while locked")
}

func TestLockoutAttemptsPostgres_ObservationWindowAndBackoff(t *testing.T) {
	pool := newLockoutPGPool(t)
	ctx := context.Background()
	userID := seedLockoutUser(t, ctx, pool)
	repo := NewRepository(pool)
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)

	for i := 0; i < domain.DefaultMaxAttempts; i++ {
		_, err := repo.RecordFailure(ctx, userID, domain.DefaultMaxAttempts, lockoutDurationMinutes, now.Add(time.Duration(i)*time.Minute))
		require.NoError(t, err)
	}
	first, err := repo.GetByUserID(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, 1, first.LockoutLevel)
	require.Equal(t, now, first.WindowStartedAt)
	require.Equal(t, now.Add(4*time.Minute+domain.DefaultLockoutDuration), *first.LockedUntil)

	nextWindow := *first.LockedUntil
	for i := 0; i < domain.DefaultMaxAttempts; i++ {
		_, err := repo.RecordFailure(ctx, userID, domain.DefaultMaxAttempts, lockoutDurationMinutes, nextWindow.Add(time.Duration(i)*time.Minute))
		require.NoError(t, err)
	}
	second, err := repo.GetByUserID(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, 2, second.LockoutLevel)
	require.Equal(t, nextWindow.Add(4*time.Minute+domain.SecondLockoutDuration), *second.LockedUntil)

	nextWindow = *second.LockedUntil
	for i := 0; i < domain.DefaultMaxAttempts; i++ {
		_, err := repo.RecordFailure(ctx, userID, domain.DefaultMaxAttempts, lockoutDurationMinutes, nextWindow.Add(time.Duration(i)*time.Minute))
		require.NoError(t, err)
	}
	third, err := repo.GetByUserID(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, domain.MaxLockoutLevel, third.LockoutLevel)
	require.Equal(t, nextWindow.Add(4*time.Minute+domain.ThirdLockoutDuration), *third.LockedUntil)
}

func TestLockoutAttemptsPostgres_ObservationWindowResetsAttempts(t *testing.T) {
	pool := newLockoutPGPool(t)
	ctx := context.Background()
	userID := seedLockoutUser(t, ctx, pool)
	repo := NewRepository(pool)
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)

	for i := 0; i < 4; i++ {
		_, err := repo.RecordFailure(ctx, userID, domain.DefaultMaxAttempts, lockoutDurationMinutes, now.Add(time.Duration(i)*time.Minute))
		require.NoError(t, err)
	}
	_, err := repo.RecordFailure(ctx, userID, domain.DefaultMaxAttempts, lockoutDurationMinutes, now.Add(domain.ObservationWindow))
	require.NoError(t, err)

	attempt, err := repo.GetByUserID(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, 1, attempt.Attempts)
	require.Equal(t, now.Add(domain.ObservationWindow), attempt.WindowStartedAt)
	require.Nil(t, attempt.LockedUntil)
}
