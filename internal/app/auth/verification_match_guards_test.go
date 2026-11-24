package auth

import (
	"context"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationsdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	sessionsdomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	verificationsdomain "github.com/guliangiam/guliangiam/internal/domain/verifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// stubAttemptLimiter 记录失败预算的三类调用，供 B06 断言。
type stubAttemptLimiter struct {
	allowed  bool
	recorded []string
	resets   int
}

func (s *stubAttemptLimiter) CheckAttempt(verificationType, recipient string) bool {
	return s.allowed
}

func (s *stubAttemptLimiter) RecordFailedAttempt(verificationType, recipient string) {
	s.recorded = append(s.recorded, verificationType+":"+recipient)
}

func (s *stubAttemptLimiter) ResetAttempts(verificationType, recipient string) {
	s.resets++
}

func useStubLimiter(t *testing.T, limiter *stubAttemptLimiter) {
	t.Helper()
	WireVerificationAttemptLimiter(limiter)
	t.Cleanup(func() { WireVerificationAttemptLimiter(nil) })
}

// B06：他组织签发的码与本组织无效码同形拒绝（NotFound），并计入失败预算。
func TestMatchVerificationCode_OrgMismatchRejected(t *testing.T) {
	limiter := &stubAttemptLimiter{allowed: true}
	useStubLimiter(t, limiter)

	repo := &historyVerificationRepo{verification: &verificationsdomain.Verification{
		ID:             "verification-1",
		OrganizationID: "org-other",
	}}

	_, err := matchVerificationCode(context.Background(), repo, testOrgID, "user@example.com", verificationTypeEmail, "123456")
	if err == nil {
		t.Fatal("expected rejection for a cross-organization code")
	}
	if code := apperrors.FromError(err).Code; code != apperrors.CodeNotFound {
		t.Fatalf("error code = %q, want %q", code, apperrors.CodeNotFound)
	}
	if len(limiter.recorded) != 1 || limiter.recorded[0] != "email:user@example.com" {
		t.Fatalf("recorded failures = %v, want one email:user@example.com entry", limiter.recorded)
	}
}

// B06：失败预算耗尽时在触库前拒绝（repo 为 nil，若被触达会 panic），返回 429。
func TestMatchVerificationCode_ExhaustedBudgetRefusesBeforeRepo(t *testing.T) {
	useStubLimiter(t, &stubAttemptLimiter{allowed: false})

	_, err := matchVerificationCode(context.Background(), nil, testOrgID, "user@example.com", verificationTypeEmail, "123456")
	if err == nil {
		t.Fatal("expected rejection once the failure budget is exhausted")
	}
	if code := apperrors.FromError(err).Code; code != apperrors.CodeTooManyRequests {
		t.Fatalf("error code = %q, want %q", code, apperrors.CodeTooManyRequests)
	}
}

// B06：同组织匹配成功即清零预算；绑定比较大小写不敏感（与 consumeVerificationCode 同口径）。
func TestMatchVerificationCode_SuccessResetsBudget(t *testing.T) {
	limiter := &stubAttemptLimiter{allowed: true}
	useStubLimiter(t, limiter)

	repo := &historyVerificationRepo{verification: &verificationsdomain.Verification{
		ID:             "verification-1",
		OrganizationID: "ORG-OTHER-CASE",
	}}

	verification, err := matchVerificationCode(context.Background(), repo, "org-other-case", "user@example.com", verificationTypeEmail, "123456")
	if err != nil {
		t.Fatalf("matchVerificationCode() error = %v, want nil", err)
	}
	if verification.ID != "verification-1" {
		t.Fatalf("verification = %+v, want verification-1", verification)
	}
	if limiter.resets != 1 {
		t.Fatalf("resets = %d, want 1", limiter.resets)
	}
	if len(limiter.recorded) != 0 {
		t.Fatalf("recorded failures = %v, want none", limiter.recorded)
	}
}

func TestVerificationMatchError_PreservesExhaustedBudget(t *testing.T) {
	if err := verificationMatchError(apperrors.NewTooManyRequests("budget"), "generic"); apperrors.FromError(err).Code != apperrors.CodeTooManyRequests {
		t.Fatalf("429 must propagate, got %v", err)
	}
	if err := verificationMatchError(apperrors.NewNotFound("gone"), "generic"); apperrors.FromError(err).Code != apperrors.CodeInvalidArgument {
		t.Fatalf("other failures must collapse to invalid argument, got %v", err)
	}
}

// ── B07：重置后轮换会话的 TTL 跟随应用的 EnableSigninSession ──────────

type resetRotationSessionRepo struct {
	sessionsdomain.Repository
	old     *sessionsdomain.Session
	created []*sessionsdomain.Session
}

func (r *resetRotationSessionRepo) GetByID(_ context.Context, id string) (*sessionsdomain.Session, error) {
	if r.old != nil && id == r.old.ID {
		return r.old, nil
	}
	return nil, apperrors.NewNotFound("session not found")
}

func (r *resetRotationSessionRepo) Create(_ context.Context, session *sessionsdomain.Session) error {
	r.created = append(r.created, session)
	return nil
}

func (r *resetRotationSessionRepo) RevokeByUser(_ context.Context, _ string, _ time.Time) (int64, error) {
	return 1, nil
}

type resetRotationAppRepo struct {
	applicationsdomain.Repository
	app *applicationsdomain.Application
}

func (r *resetRotationAppRepo) GetByID(_ context.Context, _ string) (*applicationsdomain.Application, error) {
	if r.app == nil {
		return nil, apperrors.NewNotFound("application not found")
	}
	return r.app, nil
}

type resetRotationUoW struct {
	*historyUoW
	sessions *resetRotationSessionRepo
	apps     *resetRotationAppRepo
}

func (u *resetRotationUoW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func (u *resetRotationUoW) Sessions() sessionsdomain.Repository         { return u.sessions }
func (u *resetRotationUoW) Applications() applicationsdomain.Repository { return u.apps }

func newResetRotationStack(enableSigninSession bool) (*Service, *resetRotationUoW) {
	service, base := newHistoryStack(0)
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	uow := &resetRotationUoW{
		historyUoW: base,
		sessions: &resetRotationSessionRepo{old: &sessionsdomain.Session{
			ID:             "sess-old-1",
			UserID:         testUserID,
			Username:       testUsername,
			OrganizationID: testOrgID,
			ApplicationID:  testAppID,
			SessionState:   statusActive,
			ExpiresAt:      now.Add(time.Hour),
			CreatedAt:      now,
		}},
		apps: &resetRotationAppRepo{app: &applicationsdomain.Application{
			ID:                  testAppID,
			OrganizationID:      testOrgID,
			EnableSigninSession: enableSigninSession,
		}},
	}
	service.UOW = uow
	return service, uow
}

func resetWithRotation(t *testing.T, service *Service) {
	t.Helper()
	if _, _, err := service.ResetPassword(context.Background(), ResetPasswordCommand{
		OrganizationSlug:  testOrgSlug,
		Username:          testUsername,
		VerificationCode:  "123456",
		NewPassword:       "Reset!A2026",
		PreviousSessionID: "sess-old-1",
	}); err != nil {
		t.Fatalf("reset password: %v", err)
	}
}

func TestResetPassword_RotatedSessionHonorsSigninSessionPolicy(t *testing.T) {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

	t.Run("short TTL when EnableSigninSession is false", func(t *testing.T) {
		service, uow := newResetRotationStack(false)
		resetWithRotation(t, service)
		if len(uow.sessions.created) != 1 {
			t.Fatalf("created sessions = %d, want 1", len(uow.sessions.created))
		}
		if got, want := uow.sessions.created[0].ExpiresAt, now.Add(shortSessionTTL); !got.Equal(want) {
			t.Fatalf("rotated session ExpiresAt = %v, want %v", got, want)
		}
	})

	t.Run("full TTL when EnableSigninSession is true", func(t *testing.T) {
		service, uow := newResetRotationStack(true)
		resetWithRotation(t, service)
		if len(uow.sessions.created) != 1 {
			t.Fatalf("created sessions = %d, want 1", len(uow.sessions.created))
		}
		if got, want := uow.sessions.created[0].ExpiresAt, now.Add(sessionTTL); !got.Equal(want) {
			t.Fatalf("rotated session ExpiresAt = %v, want %v", got, want)
		}
	})

	t.Run("full TTL when the old session carries no application", func(t *testing.T) {
		service, uow := newResetRotationStack(false)
		uow.sessions.old.ApplicationID = ""
		resetWithRotation(t, service)
		if len(uow.sessions.created) != 1 {
			t.Fatalf("created sessions = %d, want 1", len(uow.sessions.created))
		}
		if got, want := uow.sessions.created[0].ExpiresAt, now.Add(sessionTTL); !got.Equal(want) {
			t.Fatalf("rotated session ExpiresAt = %v, want %v", got, want)
		}
	})
}
