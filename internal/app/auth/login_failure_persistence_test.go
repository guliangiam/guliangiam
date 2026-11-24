package auth

import (
	"context"
	"errors"
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/domain/lockoutattempts"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// rollbackAwareLoginUOW models the production transaction boundary: writes
// made inside WithinTransaction are committed only when the callback returns
// nil. A credential error makes Login return an error, so anything written by
// the callback is discarded — exactly what postgres uow does.
type rollbackAwareLoginUOW struct {
	loginTestUnitOfWork
	committed  *loginTestLockoutAttemptRepository
	begins     int
	commits    int
	failSecond bool
}

func cloneLockoutAttempts(src map[string]*lockoutattempts.Attempt) map[string]*lockoutattempts.Attempt {
	cloned := make(map[string]*lockoutattempts.Attempt, len(src))
	for userID, attempt := range src {
		if attempt == nil {
			cloned[userID] = nil
			continue
		}
		copied := *attempt
		cloned[userID] = &copied
	}
	return cloned
}

func (u *rollbackAwareLoginUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	u.begins++
	if u.failSecond && u.begins > 1 {
		return errors.New("failure counter store unavailable")
	}

	working := &loginTestLockoutAttemptRepository{
		attempts: cloneLockoutAttempts(u.committed.attempts),
	}
	tx := u.loginTestUnitOfWork
	tx.lockoutAttempt = working

	if err := fn(ctx, &tx); err != nil {
		// rollback: the working copy is discarded.
		return err
	}

	u.committed = working
	u.commits++
	return nil
}

func (u *rollbackAwareLoginUOW) LockoutAttempts() lockoutattempts.Repository {
	return u.committed
}

func newRollbackTestService(uow appuow.AuthUoW) Service {
	svc := newTestService(loginTestUnitOfWork{})
	svc.UOW = uow
	return svc
}

// TestPasswordLogin_CredentialFailurePersistsAfterRollback locks the P0-1
// invariant: a wrong password must return the same generic Unauthorized error
// while the lockout counter is committed by a separate transaction.
func TestPasswordLogin_CredentialFailurePersistsAfterRollback(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	base := newTestUow(org, user, app)
	uow := &rollbackAwareLoginUOW{
		loginTestUnitOfWork: base,
		committed:           &loginTestLockoutAttemptRepository{},
	}
	svc := newRollbackTestService(uow)

	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         "WrongPassword!",
		ApplicationID:    testAppID,
	})
	if err == nil {
		t.Fatal("Login() error = nil, want credential error")
	}
	if result != nil {
		t.Fatalf("Login() result = %+v, want nil", result)
	}

	typed, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("Login() error type = %T, want *apperrors.Error", err)
	}
	if typed.Code != apperrors.CodeUnauthorized {
		t.Fatalf("Login() error code = %q, want %q", typed.Code, apperrors.CodeUnauthorized)
	}
	if typed.Message != "invalid username or password" {
		t.Fatalf("Login() error message = %q, want the generic credential error", typed.Message)
	}

	if uow.begins != 3 {
		t.Fatalf("transactions begun = %d, want 3 (login transaction + failure counter transaction + failure audit transaction)", uow.begins)
	}
	if uow.commits != 2 {
		t.Fatalf("committed transactions = %d, want 2 (failure counter + independent failure audit)", uow.commits)
	}

	attempt, getErr := uow.committed.GetByUserID(context.Background(), testUserID)
	if getErr != nil {
		t.Fatalf("GetByUserID() error = %v", getErr)
	}
	if attempt == nil {
		t.Fatal("failure counter was not committed after the login transaction rolled back")
	}
	if attempt.Attempts != 1 {
		t.Fatalf("committed attempts = %d, want 1", attempt.Attempts)
	}
}

// TestPasswordLogin_SuccessDoesNotRecordFailure guards the success path: no
// extra transaction and no lockout row may be produced by a successful login.
func TestPasswordLogin_SuccessDoesNotRecordFailure(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	base := newTestUow(org, user, app)
	uow := &rollbackAwareLoginUOW{
		loginTestUnitOfWork: base,
		committed:           &loginTestLockoutAttemptRepository{},
	}
	svc := newRollbackTestService(uow)

	if _, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
	}); err != nil {
		t.Fatalf("Login() error = %v, want nil", err)
	}

	if uow.begins != 1 {
		t.Fatalf("transactions begun = %d, want 1 (success must not open a failure-counter transaction)", uow.begins)
	}
	if attempt, _ := uow.committed.GetByUserID(context.Background(), testUserID); attempt != nil {
		t.Fatalf("successful login wrote a lockout attempt: %+v", attempt)
	}
}

// TestPasswordLogin_FailureRecordingErrorKeepsResponse verifies fail-open on
// availability: when the counter transaction itself fails, the client still
// receives the unchanged generic credential error.
func TestPasswordLogin_FailureRecordingErrorKeepsResponse(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	base := newTestUow(org, user, app)
	uow := &rollbackAwareLoginUOW{
		loginTestUnitOfWork: base,
		committed:           &loginTestLockoutAttemptRepository{},
		failSecond:          true,
	}
	svc := newRollbackTestService(uow)

	_, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         "WrongPassword!",
		ApplicationID:    testAppID,
	})
	if err == nil {
		t.Fatal("Login() error = nil, want credential error even when recording fails")
	}
	typed, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("Login() error type = %T, want *apperrors.Error", err)
	}
	if typed.Code != apperrors.CodeUnauthorized || typed.Message != "invalid username or password" {
		t.Fatalf("Login() error = %s/%q, want unchanged generic credential error", typed.Code, typed.Message)
	}
	if uow.commits != 0 {
		t.Fatalf("committed transactions = %d, want 0 when the counter store fails", uow.commits)
	}
}
