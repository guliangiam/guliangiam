package auth

import (
	"context"
	"testing"
	"time"

	lockoutattempts "github.com/guliangiam/guliangiam/internal/domain/lockoutattempts"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// withLoginVerificationConsumer swaps the wired login code consumer for the
// duration of one test and restores it afterwards.
func withLoginVerificationConsumer(t *testing.T, consume func(ctx context.Context, organizationID, verificationType, recipient, code, clientIP string) error) {
	t.Helper()
	original := consumeLoginVerificationCode
	consumeLoginVerificationCode = consume
	t.Cleanup(func() { consumeLoginVerificationCode = original })
}

func otpLoginCommand(username, verificationType, code string) LoginCommand {
	return LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         username,
		VerificationCode: code,
		VerificationType: verificationType,
		ApplicationID:    testAppID,
		IPAddress:        "203.0.113.9",
	}
}

func requireUnauthorized(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("Login() error = nil, want generic credential error")
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
}

func TestOtpLogin_EmailIdentifier_Success(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	var gotOrg, gotType, gotRecipient, gotCode, gotIP string
	withLoginVerificationConsumer(t, func(_ context.Context, organizationID, verificationType, recipient, code, clientIP string) error {
		gotOrg, gotType, gotRecipient, gotCode, gotIP = organizationID, verificationType, recipient, code, clientIP
		return nil
	})

	result, err := svc.Login(context.Background(), otpLoginCommand("TESTUSER@example.com", "email", "123456"))
	if err != nil {
		t.Fatalf("Login() error = %v, want nil", err)
	}
	if result == nil || result.Response.Status != LoginStatusSuccess {
		t.Fatalf("Login() result = %+v, want successful login", result)
	}
	if _, ok := uow.sessRepo.sessions[testSessID]; !ok {
		t.Fatalf("expected session %q to be created", testSessID)
	}
	if gotOrg != testOrgID || gotType != "email" || gotRecipient != "TESTUSER@example.com" || gotCode != "123456" || gotIP != "203.0.113.9" {
		t.Fatalf("consumer args = (%q,%q,%q,%q,%q), want tenant, table type, identifier, code, ip",
			gotOrg, gotType, gotRecipient, gotCode, gotIP)
	}
}

func TestOtpLogin_SmsIdentifier_Success(t *testing.T) {
	org := newTestOrg()
	user := newTestUser(func(u *userdomain.User) { u.Profile.Phone = "13800138000" })
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	var gotType string
	withLoginVerificationConsumer(t, func(_ context.Context, _, verificationType, _, _, _ string) error {
		gotType = verificationType
		return nil
	})

	result, err := svc.Login(context.Background(), otpLoginCommand("13800138000", "sms", "654321"))
	if err != nil {
		t.Fatalf("Login() error = %v, want nil", err)
	}
	if result == nil || result.Response.Status != LoginStatusSuccess {
		t.Fatalf("Login() result = %+v, want successful login", result)
	}
	// The request channel is "sms"; storage uses "phone".
	if gotType != "phone" {
		t.Fatalf("consumer type = %q, want %q", gotType, "phone")
	}
}

func TestOtpLogin_WrongCode_ReturnsGenericFailureAndCountsFailure(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	withLoginVerificationConsumer(t, func(_ context.Context, _, _, _, _, _ string) error {
		return apperrors.NewInvalidArgument("verification code is invalid or expired")
	})

	result, err := svc.Login(context.Background(), otpLoginCommand("testuser@example.com", "email", "000000"))
	if result != nil {
		t.Fatalf("Login() result = %+v, want nil", result)
	}
	requireUnauthorized(t, err)

	attempt := uow.lockoutAttempt.attempts[testUserID]
	if attempt == nil || attempt.Attempts < 1 {
		t.Fatalf("lockout attempt = %+v, want at least one recorded failure", attempt)
	}
}

func TestOtpLogin_ExpiredCode_ReturnsGenericFailure(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	withLoginVerificationConsumer(t, func(_ context.Context, _, _, _, _, _ string) error {
		return apperrors.NewNotFound("verification not found or expired")
	})

	_, err := svc.Login(context.Background(), otpLoginCommand("testuser@example.com", "email", "123456"))
	requireUnauthorized(t, err)
}

func TestOtpLogin_ReplayedCode_ReturnsGenericFailure(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	consumed := false
	withLoginVerificationConsumer(t, func(_ context.Context, _, _, _, _, _ string) error {
		if consumed {
			return apperrors.NewNotFound("verification already consumed")
		}
		consumed = true
		return nil
	})

	if _, err := svc.Login(context.Background(), otpLoginCommand("testuser@example.com", "email", "123456")); err != nil {
		t.Fatalf("first Login() error = %v, want nil", err)
	}
	_, err := svc.Login(context.Background(), otpLoginCommand("testuser@example.com", "email", "123456"))
	requireUnauthorized(t, err)
}

func TestOtpLogin_RateLimited_ReturnsTooManyRequests(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	withLoginVerificationConsumer(t, func(_ context.Context, _, _, _, _, _ string) error {
		return apperrors.NewTooManyRequests("too many failed attempts, please request a new code")
	})

	result, err := svc.Login(context.Background(), otpLoginCommand("testuser@example.com", "email", "123456"))
	if result != nil {
		t.Fatalf("Login() result = %+v, want nil", result)
	}
	typed, ok := err.(*apperrors.Error)
	if !ok || typed.Code != apperrors.CodeTooManyRequests {
		t.Fatalf("Login() error = %v, want code %q", err, apperrors.CodeTooManyRequests)
	}
}

func TestOtpLogin_ContactChannelMismatch_ReturnsGenericFailure(t *testing.T) {
	org := newTestOrg()
	user := newTestUser(func(u *userdomain.User) { u.Profile.Phone = "13800138000" })
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	called := false
	withLoginVerificationConsumer(t, func(_ context.Context, _, _, _, _, _ string) error {
		called = true
		return nil
	})

	// The identifier resolves to the user's phone, but the request claims the
	// email channel: the code was never sent to an email for this account.
	_, err := svc.Login(context.Background(), otpLoginCommand("13800138000", "email", "123456"))
	requireUnauthorized(t, err)
	if called {
		t.Fatalf("consumer was called for a mismatched contact channel")
	}
}

func TestOtpLogin_UnknownIdentifier_ReturnsGenericFailure(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	called := false
	withLoginVerificationConsumer(t, func(_ context.Context, _, _, _, _, _ string) error {
		called = true
		return nil
	})

	_, err := svc.Login(context.Background(), otpLoginCommand("unknown@example.com", "email", "123456"))
	requireUnauthorized(t, err)
	if called {
		t.Fatalf("consumer was called for an unknown identifier")
	}
}

func TestOtpLogin_LockedAccount_ReturnsGenericFailure(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newTestUow(org, user, app)
	lockedUntil := time.Date(2026, 5, 15, 11, 0, 0, 0, time.UTC)
	uow.lockoutAttempt.attempts = map[string]*lockoutattempts.Attempt{
		testUserID: {UserID: testUserID, Attempts: 5, LockedUntil: &lockedUntil},
	}
	svc := newTestService(uow)

	called := false
	withLoginVerificationConsumer(t, func(_ context.Context, _, _, _, _, _ string) error {
		called = true
		return nil
	})

	_, err := svc.Login(context.Background(), otpLoginCommand("testuser@example.com", "email", "123456"))
	requireUnauthorized(t, err)
	if called {
		t.Fatalf("consumer was called for a locked account")
	}
}

func TestOtpLogin_PasswordAndCode_ReturnsInvalidArgument(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	called := false
	withLoginVerificationConsumer(t, func(_ context.Context, _, _, _, _, _ string) error {
		called = true
		return nil
	})

	command := otpLoginCommand("testuser@example.com", "email", "123456")
	command.Password = testPassword
	_, err := svc.Login(context.Background(), command)
	typed, ok := err.(*apperrors.Error)
	if !ok || typed.Code != apperrors.CodeInvalidArgument {
		t.Fatalf("Login() error = %v, want code %q", err, apperrors.CodeInvalidArgument)
	}
	if called {
		t.Fatalf("consumer was called for an ambiguous password+code request")
	}
}

func TestOtpLogin_MfaStillRequired(t *testing.T) {
	org := newTestOrg(func(o *orgdomain.Organization) {
		o.Security = orgdomain.OrganizationSecurityPolicy{
			MfaPolicy:      map[string]any{"required": true},
			OrganizationID: testOrgID,
		}
	})
	user := newTestUser(func(u *userdomain.User) {
		u.MfaMethods = []userdomain.UserMfaMethod{
			{ID: "mfa-totp-1", MethodType: "totp", Enabled: true, Verified: true},
		}
	})
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	withLoginVerificationConsumer(t, func(_ context.Context, _, _, _, _, _ string) error {
		return nil
	})

	result, err := svc.Login(context.Background(), otpLoginCommand("testuser@example.com", "email", "123456"))
	if err != nil {
		t.Fatalf("Login() error = %v, want nil", err)
	}
	if result == nil || result.Response.Status != LoginStatusMfaRequired {
		t.Fatalf("Login() result = %+v, want %q", result, LoginStatusMfaRequired)
	}
	if result.Response.ChallengeID == "" {
		t.Fatalf("Login() ChallengeID = empty, want a pending MFA challenge")
	}
}
