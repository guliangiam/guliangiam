package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	invitationsdomain "github.com/guliangiam/guliangiam/internal/domain/invitations"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// historyInvitationRepo is a single-row invitations fake. reuseAfterFirstGet
// simulates a concurrent request accepting the same code between the pre-check
// and the in-transaction re-read.
type historyInvitationRepo struct {
	invitationsdomain.Repository

	invitation         *invitationsdomain.Invitation
	reuseAfterFirstGet bool
	gets               int
	updated            *invitationsdomain.Invitation
}

func (r *historyInvitationRepo) GetByCode(_ context.Context, _ string) (*invitationsdomain.Invitation, error) {
	if r.invitation == nil {
		return nil, apperrors.NewNotFound("invitation not found")
	}
	r.gets++
	if r.reuseAfterFirstGet && r.gets > 1 {
		acceptedAt := time.Date(2026, 11, 6, 9, 59, 0, 0, time.UTC)
		r.invitation.Status = invitationsdomain.StatusAccepted
		r.invitation.AcceptedAt = &acceptedAt
	}
	return r.invitation, nil
}

func (r *historyInvitationRepo) Update(_ context.Context, invitation *invitationsdomain.Invitation) error {
	r.updated = invitation
	return nil
}

func assertAppErrorCode(t *testing.T, err error, want apperrors.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %q, got nil", want)
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error type = %T, want *apperrors.Error", err)
	}
	if appErr.Code != want {
		t.Fatalf("error code = %q, want %q", appErr.Code, want)
	}
}

func newSignupInvitation(mutators ...func(*invitationsdomain.Invitation)) *invitationsdomain.Invitation {
	now := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	invitation := &invitationsdomain.Invitation{
		ID:             "invitation-1",
		OrganizationID: testOrgID,
		Email:          "invited@example.com",
		Code:           "invite-code",
		Status:         invitationsdomain.StatusPending,
		ExpiresAt:      time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		InvitedBy:      testUserID,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}
	for _, fn := range mutators {
		fn(invitation)
	}
	return invitation
}

// R2-A01: anonymous reset/signup must resolve org/user rows under the system
// tenant scope (mirrors Login, login_flow.go); without it FORCE RLS rejects the
// lookup for every non-built-in tenant.
func TestResetPasswordRunsUnderSystemTenantScope(t *testing.T) {
	service, uow := newHistoryStack(0)

	if _, _, err := service.ResetPassword(context.Background(), ResetPasswordCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		VerificationCode: "123456",
		NewPassword:      "Reset!A2026",
	}); err != nil {
		t.Fatalf("reset password: %v", err)
	}

	if uow.orgs.lastCtx == nil {
		t.Fatal("organization lookup never ran")
	}
	if scope := tenantctx.TenantScopeFromContext(uow.orgs.lastCtx); scope.Mode != tenantctx.TenantFilterSystem {
		t.Fatalf("tenant scope mode = %q, want %q", scope.Mode, tenantctx.TenantFilterSystem)
	}
}

func TestSignupRunsUnderSystemTenantScope(t *testing.T) {
	service, uow := newHistoryStack(0)
	uow.users.user = nil

	if _, err := service.Signup(context.Background(), SignupCommand{
		OrganizationSlug: testOrgSlug,
		Username:         "newuser",
		Password:         "Signup!A2026",
		Email:            "newuser@example.com",
	}); err != nil {
		t.Fatalf("signup: %v", err)
	}

	if uow.orgs.lastCtx == nil {
		t.Fatal("organization lookup never ran")
	}
	if scope := tenantctx.TenantScopeFromContext(uow.orgs.lastCtx); scope.Mode != tenantctx.TenantFilterSystem {
		t.Fatalf("tenant scope mode = %q, want %q", scope.Mode, tenantctx.TenantFilterSystem)
	}
}

// R2-A03: the policy 422 must be independent of user existence — before the
// reorder a weak password returned 422 only for existing users and a generic
// 200 for everyone else, which is a username-enumeration oracle.
func TestResetPasswordPolicyErrorDoesNotRevealUserExistence(t *testing.T) {
	service, _ := newHistoryStack(0)

	// Weak password + nonexistent user → 422 (previously generic 200).
	_, _, err := service.ResetPassword(context.Background(), ResetPasswordCommand{
		OrganizationSlug: testOrgSlug,
		Username:         "ghost",
		VerificationCode: "123456",
		NewPassword:      "short",
	})
	assertAppErrorCode(t, err, apperrors.CodeUnprocessableEntity)

	// Weak password + existing user → the same 422, no distinguishing signal.
	_, _, err = service.ResetPassword(context.Background(), ResetPasswordCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		VerificationCode: "123456",
		NewPassword:      "short",
	})
	assertAppErrorCode(t, err, apperrors.CodeUnprocessableEntity)

	// Policy-conforming password + nonexistent user → generic success.
	result, _, err := service.ResetPassword(context.Background(), ResetPasswordCommand{
		OrganizationSlug: testOrgSlug,
		Username:         "ghost",
		VerificationCode: "123456",
		NewPassword:      "Reset!A2026",
	})
	if err != nil {
		t.Fatalf("strong password for nonexistent user must return the generic result, got %v", err)
	}
	if result == nil || result.Status != verificationPurposeReset {
		t.Fatalf("result = %#v, want generic %q status", result, verificationPurposeReset)
	}
}

// R2-B03: a consumed or expired invitation must surface the dedicated codes
// registered in errors.yaml instead of a generic invalid_argument.
func TestSignupInvitationAlreadyUsed(t *testing.T) {
	service, uow := newHistoryStack(0)
	uow.users.user = nil
	uow.invitations.invitation = newSignupInvitation(func(invitation *invitationsdomain.Invitation) {
		invitation.Status = invitationsdomain.StatusAccepted
		acceptedAt := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
		invitation.AcceptedAt = &acceptedAt
	})

	_, err := service.Signup(context.Background(), SignupCommand{
		OrganizationSlug: testOrgSlug,
		Username:         "newuser",
		Password:         "Signup!A2026",
		InvitationCode:   "invite-code",
	})
	assertAppErrorCode(t, err, apperrors.CodeInvitationUsed)
}

func TestSignupInvitationExpired(t *testing.T) {
	cases := []struct {
		name    string
		mutator func(*invitationsdomain.Invitation)
	}{
		{"status expired", func(invitation *invitationsdomain.Invitation) {
			invitation.Status = invitationsdomain.StatusExpired
		}},
		{"pending past expiresAt", func(invitation *invitationsdomain.Invitation) {
			invitation.ExpiresAt = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, uow := newHistoryStack(0)
			uow.users.user = nil
			uow.invitations.invitation = newSignupInvitation(tc.mutator)

			_, err := service.Signup(context.Background(), SignupCommand{
				OrganizationSlug: testOrgSlug,
				Username:         "newuser",
				Password:         "Signup!A2026",
				InvitationCode:   "invite-code",
			})
			assertAppErrorCode(t, err, apperrors.CodeInvitationExpired)
		})
	}
}

// R2-B08: an email-targeted invitation binds the signup email — a different
// email is rejected so the binding cannot be bypassed.
func TestSignupInvitationEmailMismatchRejected(t *testing.T) {
	service, uow := newHistoryStack(0)
	uow.users.user = nil
	uow.invitations.invitation = newSignupInvitation()

	_, err := service.Signup(context.Background(), SignupCommand{
		OrganizationSlug: testOrgSlug,
		Username:         "newuser",
		Password:         "Signup!A2026",
		Email:            "someone-else@example.com",
		InvitationCode:   "invite-code",
	})
	assertAppErrorCode(t, err, apperrors.CodeUnprocessableEntity)
	if uow.users.user != nil {
		t.Fatal("user was created despite the email mismatch")
	}
}

func TestSignupInvitationEmailInherited(t *testing.T) {
	service, uow := newHistoryStack(0)
	uow.users.user = nil
	uow.invitations.invitation = newSignupInvitation()

	if _, err := service.Signup(context.Background(), SignupCommand{
		OrganizationSlug: testOrgSlug,
		Username:         "newuser",
		Password:         "Signup!A2026",
		InvitationCode:   "invite-code",
	}); err != nil {
		t.Fatalf("signup: %v", err)
	}

	if uow.users.user == nil {
		t.Fatal("user was not created")
	}
	if got := uow.users.user.Profile.Email; got != "invited@example.com" {
		t.Fatalf("profile email = %q, want the invitation email %q", got, "invited@example.com")
	}
	if uow.invitations.updated == nil {
		t.Fatal("invitation was not marked accepted")
	}
	if uow.invitations.updated.Status != invitationsdomain.StatusAccepted {
		t.Fatalf("invitation status = %q, want %q", uow.invitations.updated.Status, invitationsdomain.StatusAccepted)
	}
}

// Concurrent one-code-many-uses: the pre-check passed but the in-transaction
// re-read sees the code already accepted → the signup must fail with
// invitation_used so the whole transaction rolls back.
func TestSignupInvitationConcurrentReuseRejected(t *testing.T) {
	service, uow := newHistoryStack(0)
	uow.users.user = nil
	uow.invitations.invitation = newSignupInvitation()
	uow.invitations.reuseAfterFirstGet = true

	_, err := service.Signup(context.Background(), SignupCommand{
		OrganizationSlug: testOrgSlug,
		Username:         "newuser",
		Password:         "Signup!A2026",
		InvitationCode:   "invite-code",
	})
	assertAppErrorCode(t, err, apperrors.CodeInvitationUsed)
}
