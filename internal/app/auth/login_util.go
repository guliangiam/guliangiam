package auth

import (
	"context"
	"strings"
	"time"

	"github.com/guliangiam/guliangiam/cred"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	mfachallengedomain "github.com/guliangiam/guliangiam/internal/domain/mfachallenges"
	subscriptiondomain "github.com/guliangiam/guliangiam/internal/domain/subscriptions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

func invalidChallengeError() error {
	return apperrors.NewUnauthorized("mfa challenge is invalid or expired")
}

// genericLoginFailureError is the single external shape of every credential
// level login rejection. A locked account, an unknown username and a wrong
// password must be indistinguishable to the caller: same status code, same
// code and same message. Operational detail (lockout remaining time) stays in
// server-side logs and audit payloads.
func genericLoginFailureError() error {
	return apperrors.NewUnauthorized("invalid username or password")
}

func checkApplicationTagAccess(application *applicationdomain.Application, user *userdomain.User) error {
	if len(application.Tags) == 0 {
		return nil
	}
	userTag := strings.TrimSpace(user.Tag)
	if userTag == "" {
		return apperrors.NewUnauthorized("user does not have a required tag for this application")
	}
	for _, tag := range application.Tags {
		if strings.TrimSpace(tag) == userTag {
			return nil
		}
	}
	return apperrors.NewUnauthorized("user tag does not match any application tag")
}

func checkPaidUserSubscription(ctx context.Context, tx appuow.UnitOfWork, user *userdomain.User, now time.Time) error {
	if user.UserType != "paid-user" {
		return nil
	}

	// Filter on the two states that can still grant entitlement (§1.3);
	// `expired`/`cancelled` are terminal and can never do so.
	for _, state := range []string{subscriptiondomain.StateActive, subscriptiondomain.StatePastDue} {
		matched, _, err := tx.Subscriptions().List(ctx, subscriptiondomain.ListQuery{
			User:     user.ID,
			State:    &state,
			PageSize: 1,
		})
		if err != nil {
			return apperrors.WrapInternal(err, "check subscription")
		}
		if len(matched) > 0 && matched[0].IsEntitled(now) {
			return nil
		}
	}

	return apperrors.NewForbidden("paid user has no active subscription")
}

func isCredentialError(err error) bool {
	if err == nil {
		return false
	}
	typed, ok := err.(*apperrors.Error)
	if !ok {
		return false
	}
	return typed.Code == apperrors.CodeUnauthorized
}

// mfaChallengeFailure is the sidecar of a rejected MFA attempt. Every MFA
// rejection makes the login transaction roll back, so the failure counter
// cannot be written in place: it is collected here and committed by
// persistMfaChallengeFailure in its own transaction after the rollback.
type mfaChallengeFailure struct {
	ChallengeID string
	MethodType  string
	Now         time.Time
	// Exhausted marks a request that found the challenge already at the
	// attempt limit; the challenge row is removed instead of incremented.
	Exhausted bool
}

// recordMfaFailure mutates the in-memory challenge so the current request can
// keep using the counter (resync gate, attempt limit); persistence is deferred.
func recordMfaFailure(challenge *mfachallengedomain.Challenge, now time.Time) {
	if challenge == nil {
		return
	}
	challenge.RecordFailure(now)
}

func mfaFailureOf(challenge *mfachallengedomain.Challenge, methodType string, now time.Time) *mfaChallengeFailure {
	if challenge == nil {
		return nil
	}
	return &mfaChallengeFailure{ChallengeID: challenge.ID, MethodType: methodType, Now: now}
}

// persistMfaChallengeFailure applies a rejected MFA attempt after the login
// transaction has rolled back. Reloading the challenge keeps the optimistic
// revision honest; the row is deleted when the attempt limit is reached or the
// challenge has expired meanwhile. Failures only warn at the call site and never
// change the response returned to the caller.
func (s *Service) persistMfaChallengeFailure(ctx context.Context, failure mfaChallengeFailure) error {
	return s.UOW.WithinTransaction(tenantctx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		challenge, err := tx.MfaChallenges().GetByID(ctx, failure.ChallengeID)
		if err != nil {
			if apperrors.FromError(err).Code == apperrors.CodeNotFound {
				return nil
			}
			return err
		}
		if challenge == nil {
			return nil
		}
		if failure.Exhausted || !challenge.ExpiresAt.After(failure.Now) {
			return tx.MfaChallenges().Delete(ctx, challenge.ID)
		}

		prior := challenge.FailedAttempts
		challenge.RecordFailure(failure.Now)
		challenge.Revision++
		if err := tx.MfaChallenges().Update(ctx, challenge); err != nil {
			return err
		}
		if failure.MethodType == mfaTypeTotp &&
			prior < totpResyncAfterFailures && challenge.FailedAttempts >= totpResyncAfterFailures {
			emitTotpResyncAudit(ctx, tx, challenge)
		}
		return nil
	})
}

// deleteExpiredLoginChallenge drops a challenge that the rolled back login
// transaction tried to clean up. Anything still valid is left untouched.
func (s *Service) deleteExpiredLoginChallenge(ctx context.Context, challengeID string) error {
	return s.UOW.WithinTransaction(tenantctx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		challenge, err := tx.MfaChallenges().GetByID(ctx, challengeID)
		if err != nil {
			if apperrors.FromError(err).Code == apperrors.CodeNotFound {
				return nil
			}
			return err
		}
		if challenge == nil || challenge.ExpiresAt.After(s.Clock.Now()) {
			return nil
		}
		return tx.MfaChallenges().Delete(ctx, challenge.ID)
	})
}

func upgradePasswordAlgorithm(password string, credentials *userdomain.UserCredentials) (bool, error) {
	if credentials == nil {
		return false, nil
	}

	manager := cred.GetCredManager(hashArgon2ID)
	if manager == nil {
		return false, apperrors.NewInternal("argon2id credential manager unavailable")
	}

	newHash := manager.GetHashedPassword(password, "")
	if strings.TrimSpace(newHash) == "" {
		return false, apperrors.NewInternal("argon2id hash produced empty result")
	}

	now := time.Now()
	credentials.PasswordHash = newHash
	credentials.PasswordAlgo = hashArgon2ID
	credentials.PasswordUpdatedAt = &now

	return true, nil
}
