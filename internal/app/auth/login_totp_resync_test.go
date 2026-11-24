package auth

import (
	"context"
	"testing"
	"time"

	mfachallengedomain "github.com/guliangiam/guliangiam/internal/domain/mfachallenges"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/pquerna/otp/totp"
)

const totpTestSecret = "JBSWY3DPEHPK3PXP"

func totpTestUser() *userdomain.User {
	return &userdomain.User{
		TotpSecret: totpTestSecret,
		MfaMethods: []userdomain.UserMfaMethod{{
			ID:         "mfa-totp-1",
			MethodType: mfaTypeTotp,
			Enabled:    true,
			Verified:   true,
		}},
	}
}

func totpCodeAt(t *testing.T, now time.Time) string {
	t.Helper()
	code, err := totp.GenerateCode(totpTestSecret, now.UTC())
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}
	return code
}

func TestVerifyLoginMfaTOTPAcceptsValidCode(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	user := totpTestUser()
	result, err := verifyLoginMfa(context.Background(), nil, LoginCommand{MfaType: mfaTypeTotp, Passcode: totpCodeAt(t, now)}, nil, user, nil, enabledVerifiedMfaMethods(user), now, nil)
	if err != nil {
		t.Fatalf("verifyLoginMfa() error = %v", err)
	}
	if !result.Completed {
		t.Fatalf("expected TOTP verification to complete login")
	}
	if user.MfaMethods[0].LastUsedAt == nil || !user.MfaMethods[0].LastUsedAt.Equal(now) {
		t.Fatalf("expected TOTP method last used timestamp, got %+v", user.MfaMethods[0].LastUsedAt)
	}
}

func TestVerifyLoginMfaTOTPRejectsReplayedCode(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	user := totpTestUser()
	usedAt := now
	user.MfaMethods[0].LastUsedAt = &usedAt

	_, err := verifyLoginMfa(context.Background(), nil, LoginCommand{MfaType: mfaTypeTotp, Passcode: totpCodeAt(t, now)}, nil, user, nil, enabledVerifiedMfaMethods(user), now, nil)
	if err == nil {
		t.Fatalf("expected replayed TOTP code to be rejected")
	}
	if appErr := apperrors.FromError(err); appErr.Code != apperrors.CodeUnauthorized {
		t.Fatalf("error code = %q, want %q", appErr.Code, apperrors.CodeUnauthorized)
	}
}

func TestVerifyLoginMfaTOTPControlledResyncRequiresLaterWindow(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	user := totpTestUser()
	challenge := &mfachallengedomain.Challenge{
		ID:             "challenge-totp-resync",
		OrganizationID: "org-1",
		UserID:         "user-1",
		State:          mfachallengedomain.StatePending,
		MethodType:     mfaTypeTotp,
		FailedAttempts: totpResyncAfterFailures,
		ExpiresAt:      now.Add(10 * time.Minute),
		CreatedAt:      now.Add(-time.Minute),
		UpdatedAt:      now,
		Revision:       1,
	}
	tx := loginMfaTestUnitOfWork{challengeRepo: &loginMfaTestChallengeRepository{
		items: map[string]*mfachallengedomain.Challenge{challenge.ID: challenge},
	}}

	// Same window as the last failure: even a mathematically valid code is refused.
	_, err := verifyLoginMfa(context.Background(), tx, LoginCommand{MfaType: mfaTypeTotp, Passcode: totpCodeAt(t, now)}, nil, user, challenge, enabledVerifiedMfaMethods(user), now, nil)
	if err == nil {
		t.Fatalf("expected same-window code to be rejected during controlled resync")
	}
	if challenge.FailedAttempts != totpResyncAfterFailures+1 {
		t.Fatalf("FailedAttempts = %d, want %d", challenge.FailedAttempts, totpResyncAfterFailures+1)
	}

	// A code from the next window proves the client moved forward and is accepted.
	next := now.Add(totpPeriod)
	result, err := verifyLoginMfa(context.Background(), tx, LoginCommand{MfaType: mfaTypeTotp, Passcode: totpCodeAt(t, next)}, nil, user, challenge, enabledVerifiedMfaMethods(user), next, nil)
	if err != nil {
		t.Fatalf("verifyLoginMfa() next window error = %v", err)
	}
	if !result.Completed {
		t.Fatalf("expected next-window TOTP code to complete login")
	}
}

func TestVerifyLoginMfaTOTPLocksOutAfterMaxAttemptsAcrossReloads(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	user := totpTestUser()
	challenge := &mfachallengedomain.Challenge{
		ID:             "challenge-totp-lockout",
		OrganizationID: "org-1",
		UserID:         "user-1",
		State:          mfachallengedomain.StatePending,
		MethodType:     mfaTypeTotp,
		// The count arrives from persistence (login_challenges.metadata), not
		// from the in-process attempt counter.
		FailedAttempts: maxMfaAttempts - 1,
		ExpiresAt:      now.Add(10 * time.Minute),
		CreatedAt:      now.Add(-time.Minute),
		UpdatedAt:      now,
		Revision:       1,
	}
	repo := &loginMfaTestChallengeRepository{items: map[string]*mfachallengedomain.Challenge{challenge.ID: challenge}}
	tx := loginMfaTestUnitOfWork{challengeRepo: repo}

	_, err := verifyLoginMfa(context.Background(), tx, LoginCommand{MfaType: mfaTypeTotp, Passcode: "000000"}, nil, user, challenge, enabledVerifiedMfaMethods(user), now, nil)
	if err == nil || apperrors.FromError(err).Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected the last allowed attempt to fail with unauthorized, got %v", err)
	}

	result, err := verifyLoginMfa(context.Background(), tx, LoginCommand{MfaType: mfaTypeTotp, Passcode: "000000"}, nil, user, challenge, enabledVerifiedMfaMethods(user), now, nil)
	if err == nil || apperrors.FromError(err).Code != apperrors.CodeForbidden {
		t.Fatalf("expected exhausted challenge to be forbidden, got %v", err)
	}
	if result.Failure == nil || !result.Failure.Exhausted {
		t.Fatalf("expected the forbidden result to carry an exhausted challenge marker, got %+v", result.Failure)
	}
	// The row is removed by persistMfaChallengeFailure after the login
	// transaction rolls back, not by the transaction itself.
	if _, ok := repo.items[challenge.ID]; !ok {
		t.Fatalf("challenge must still be visible to the persistence transaction")
	}
	svc := &Service{Clock: fakeClock{fixedTime: now}, UOW: tx}
	if err := svc.persistMfaChallengeFailure(context.Background(), *result.Failure); err != nil {
		t.Fatalf("persistMfaChallengeFailure() error = %v", err)
	}
	if _, ok := repo.items[challenge.ID]; ok {
		t.Fatalf("expected exhausted challenge to be deleted after persistence")
	}
}

func TestPersistMfaChallengeFailureSurvivesRollback(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	challenge := &mfachallengedomain.Challenge{
		ID:             "challenge-persist",
		OrganizationID: "org-1",
		UserID:         "user-1",
		State:          mfachallengedomain.StatePending,
		ExpiresAt:      now.Add(5 * time.Minute),
		CreatedAt:      now.Add(-time.Minute),
		UpdatedAt:      now,
		Revision:       1,
	}
	repo := &loginMfaTestChallengeRepository{items: map[string]*mfachallengedomain.Challenge{challenge.ID: challenge}}
	svc := &Service{Clock: fakeClock{fixedTime: now}, UOW: loginMfaTestUnitOfWork{challengeRepo: repo}}

	if err := svc.persistMfaChallengeFailure(context.Background(), mfaChallengeFailure{
		ChallengeID: challenge.ID,
		MethodType:  mfaTypeEmail,
		Now:         now,
	}); err != nil {
		t.Fatalf("persistMfaChallengeFailure() error = %v", err)
	}

	persisted := repo.items[challenge.ID]
	if persisted.FailedAttempts != 1 {
		t.Fatalf("FailedAttempts = %d, want 1", persisted.FailedAttempts)
	}
	if persisted.Metadata[mfachallengedomain.MetadataKeyFailedAttempts] != 1 {
		t.Fatalf("metadata mirror = %v, want 1", persisted.Metadata[mfachallengedomain.MetadataKeyFailedAttempts])
	}
	if persisted.Revision != 2 {
		t.Fatalf("Revision = %d, want 2", persisted.Revision)
	}
}

func TestPersistMfaChallengeFailureDeletesExpiredChallenge(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	challenge := &mfachallengedomain.Challenge{
		ID:             "challenge-expired",
		OrganizationID: "org-1",
		UserID:         "user-1",
		State:          mfachallengedomain.StatePending,
		ExpiresAt:      now.Add(-time.Second),
		CreatedAt:      now.Add(-time.Minute),
		UpdatedAt:      now,
		Revision:       1,
	}
	repo := &loginMfaTestChallengeRepository{items: map[string]*mfachallengedomain.Challenge{challenge.ID: challenge}}
	svc := &Service{Clock: fakeClock{fixedTime: now}, UOW: loginMfaTestUnitOfWork{challengeRepo: repo}}

	if err := svc.persistMfaChallengeFailure(context.Background(), mfaChallengeFailure{
		ChallengeID: challenge.ID,
		MethodType:  mfaTypeTotp,
		Now:         now,
	}); err != nil {
		t.Fatalf("persistMfaChallengeFailure() error = %v", err)
	}
	if _, ok := repo.items[challenge.ID]; ok {
		t.Fatalf("expected expired challenge to be deleted")
	}
}

func TestChallengeRecordFailureMirrorsMetadata(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	challenge := &mfachallengedomain.Challenge{ID: "challenge-meta", UpdatedAt: now}
	challenge.RecordFailure(now)
	if challenge.FailedAttempts != 1 {
		t.Fatalf("FailedAttempts = %d, want 1", challenge.FailedAttempts)
	}
	if got, ok := challenge.Metadata[mfachallengedomain.MetadataKeyFailedAttempts]; !ok || got != 1 {
		t.Fatalf("metadata failed attempts = %v (present=%v), want 1", got, ok)
	}
}
