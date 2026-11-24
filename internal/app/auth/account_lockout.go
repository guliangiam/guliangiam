package auth

import (
	"context"
	"log/slog"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/domain/lockoutattempts"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// LockoutPolicyConfig represents the lockout policy configuration for an organization.
type LockoutPolicyConfig struct {
	MaxAttempts            int `json:"maxAttempts"`
	LockoutDurationMinutes int `json:"lockoutDurationMinutes"`
}

func defaultLockoutPolicy() LockoutPolicyConfig {
	return LockoutPolicyConfig{
		MaxAttempts:            5,
		LockoutDurationMinutes: 30,
	}
}

func resolveLockoutPolicy(org *orgdomain.Organization) LockoutPolicyConfig {
	policy := defaultLockoutPolicy()

	if org == nil || len(org.Security.LockoutPolicy) == 0 {
		return policy
	}

	lp := org.Security.LockoutPolicy

	if v, ok := intFromMap(lp, "maxAttempts"); ok && v > 0 {
		policy.MaxAttempts = v
	}
	if v, ok := intFromMap(lp, "lockoutDurationMinutes"); ok && v > 0 {
		policy.LockoutDurationMinutes = v
	}

	return policy
}

// CheckAccountLockout checks if the account is currently locked due to too many
// failed login attempts. Returns an error if the account is locked.
func (s *Service) CheckAccountLockout(ctx context.Context, userID string) error {
	now := s.Clock.Now()
	return s.checkAccountLockoutAtTime(ctx, userID, now)
}

func (s *Service) checkAccountLockoutAtTime(ctx context.Context, userID string, now time.Time) error {
	return checkAccountLockoutAtTimeWithUOW(ctx, s.UOW, userID, now)
}

func checkAccountLockoutAtTimeWithUOW(ctx context.Context, uow appuow.AuthUoW, userID string, now time.Time) error {
	locked, _, err := accountLockoutStatus(ctx, uow, userID, now)
	if err != nil {
		return err
	}
	if locked {
		return genericLoginFailureError()
	}
	return nil
}

// accountLockoutStatus resolves the lockout state without deciding how it is
// presented to the caller. A locked account is an internal security fact: the
// login flow must answer with the same generic credential error as a wrong
// password, while the remaining time is logged (and audited) server-side only.
func accountLockoutStatus(ctx context.Context, uow appuow.AuthUoW, userID string, now time.Time) (locked bool, remaining time.Duration, err error) {
	if userID == "" {
		return false, 0, apperrors.NewInvalidArgument("user id is required")
	}
	attempt, err := uow.LockoutAttempts().GetByUserID(ctx, userID)
	if err != nil {
		return false, 0, apperrors.WrapInternalf(err, "get lockout attempt for user %s", userID)
	}
	if attempt == nil || attempt.LockedUntil == nil {
		return false, 0, nil
	}

	// If the lockout has expired, clear the entry.
	if !now.Before(*attempt.LockedUntil) {
		return false, 0, nil
	}

	remaining = attempt.LockedUntil.Sub(now)
	slog.InfoContext(ctx, "login blocked by account lockout",
		"user_id", userID,
		"remaining_seconds", int(remaining.Round(time.Second)/time.Second),
	)
	return true, remaining, nil
}

// RecordLoginFailure records a failed login attempt and locks the account if the
// maximum number of attempts is exceeded.
func (s *Service) RecordLoginFailure(ctx context.Context, userID, orgID string) error {
	if userID == "" {
		return apperrors.NewInvalidArgument("user id is required")
	}
	if orgID == "" {
		return apperrors.NewInvalidArgument("organization id is required")
	}
	org, err := s.UOW.Organizations().GetByID(ctx, orgID)
	if err != nil {
		return apperrors.WrapInternalf(err, "get organization %s for lockout policy", orgID)
	}

	policy := resolveLockoutPolicy(org)
	now := s.Clock.Now()
	_, err = s.recordFailureAtTime(ctx, userID, policy, now)
	return err
}

func recordLoginFailureWithTx(ctx context.Context, tx appuow.UnitOfWork, userID, orgID string, now time.Time) (*lockoutattempts.Attempt, error) {
	org, err := tx.Organizations().GetByID(ctx, orgID)
	if err != nil {
		return nil, err
	}

	policy := resolveLockoutPolicy(org)
	return recordFailureAtTimeWithUOW(ctx, tx, userID, policy, now)
}

func (s *Service) recordFailureAtTime(ctx context.Context, userID string, policy LockoutPolicyConfig, now time.Time) (*lockoutattempts.Attempt, error) {
	return recordFailureAtTimeWithUOW(ctx, s.UOW, userID, policy, now)
}

func recordFailureAtTimeWithUOW(ctx context.Context, uow appuow.AuthUoW, userID string, policy LockoutPolicyConfig, now time.Time) (*lockoutattempts.Attempt, error) {
	// Single atomic statement: the previous Get + mutate + Update flow lost
	// increments under concurrency, which let parallel login attempts stay below
	// MaxAttempts forever.
	return uow.LockoutAttempts().RecordFailure(ctx, userID, policy.MaxAttempts, policy.LockoutDurationMinutes, now)
}

// ResetLoginFailures resets the failed login attempt counter for a user.
func (s *Service) ResetLoginFailures(ctx context.Context, userID string) error {
	return s.UOW.LockoutAttempts().DeleteByUserID(ctx, userID)
}

func resetLoginFailuresWithTx(ctx context.Context, tx appuow.UnitOfWork, userID string) error {
	return tx.LockoutAttempts().DeleteByUserID(ctx, userID)
}
