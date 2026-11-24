package invitations

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/invitations"
	verificationsdomain "github.com/guliangiam/guliangiam/internal/domain/verifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ---------------------------------------------------------------------------
// BE-A-01: invitation code entropy / format
// ---------------------------------------------------------------------------

var invitationCodePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{22}$`)

func TestGenerateInvitationCode_FormatAndUniqueness(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		code, err := generateInvitationCode()
		if err != nil {
			t.Fatalf("generateInvitationCode() error = %v", err)
		}
		if !invitationCodePattern.MatchString(code) {
			t.Fatalf("code %q is not 22 url-safe base64 characters (128-bit entropy)", code)
		}
		if _, duplicate := seen[code]; duplicate {
			t.Fatalf("duplicate code %q after %d draws", code, i)
		}
		seen[code] = struct{}{}
	}
}

func TestCreateInvitation_CodeIndependentFromID(t *testing.T) {
	now := time.Now()
	repo := &mockInvitationRepo{}
	// Both the id and (pre-fix) the code came from the same UUID generator;
	// pinning the generator proves the code no longer derives from the id.
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "01930000-0000-7000-8000-000000000000"},
		UOW:   testUnitOfWork{invitationRepo: repo},
	}

	first, err := svc.CreateInvitation(context.Background(), CreateInvitationCommand{
		OrganizationID: "org-1",
		Email:          "user@example.com",
	})
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}
	second, err := svc.CreateInvitation(context.Background(), CreateInvitationCommand{
		OrganizationID: "org-1",
		Email:          "other@example.com",
	})
	if err != nil {
		t.Fatalf("CreateInvitation() second error = %v", err)
	}

	if !invitationCodePattern.MatchString(first.Code) {
		t.Errorf("code %q does not carry 128-bit url-safe entropy", first.Code)
	}
	if strings.HasPrefix(first.ID, first.Code) || strings.Contains(first.ID, first.Code) {
		t.Errorf("code %q must not be derived from id %q", first.Code, first.ID)
	}
	if first.Code == second.Code {
		t.Errorf("two invitations created in the same millisecond share code %q", first.Code)
	}
}

// ---------------------------------------------------------------------------
// BE-A-02: Send/Resend real delivery
// ---------------------------------------------------------------------------

type capturedEmail struct {
	to      string
	subject string
	body    string
}

type fakeEmailSender struct {
	sent []capturedEmail
	err  error
}

func (f *fakeEmailSender) Send(ctx context.Context, to, subject, body string) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, capturedEmail{to: to, subject: subject, body: body})
	return nil
}

type fakeSenderResolver struct {
	email verificationsdomain.EmailSender
	err   error
}

func (f fakeSenderResolver) ResolveEmailSender(ctx context.Context, organizationID string) (verificationsdomain.EmailSender, error) {
	return f.email, f.err
}

func (f fakeSenderResolver) ResolveSmsSender(ctx context.Context, organizationID string) (verificationsdomain.SmsSender, error) {
	return nil, nil
}

func pendingInvitationRepo() *mockInvitationRepo {
	return &mockInvitationRepo{
		invitations: []*domain.Invitation{
			{
				ID: "inv-1", OrganizationID: "org-1", Email: "invitee@example.com",
				Code: "abcdefghijklmnopqrstuv", Status: domain.StatusPending,
				ExpiresAt: time.Now().Add(7 * 24 * time.Hour), Revision: 1,
			},
		},
	}
}

func TestSendInvitation_DeliversEmail(t *testing.T) {
	repo := pendingInvitationRepo()
	sender := &fakeEmailSender{}
	svc := Service{
		Clock:          fakeClock{fixedTime: time.Now()},
		UOW:            testUnitOfWork{invitationRepo: repo},
		SenderResolver: fakeSenderResolver{email: sender},
	}

	if err := svc.SendInvitation(context.Background(), "org-1", "inv-1"); err != nil {
		t.Fatalf("SendInvitation() error = %v", err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("expected exactly 1 delivered email, got %d", len(sender.sent))
	}
	mail := sender.sent[0]
	if mail.to != "invitee@example.com" {
		t.Errorf("recipient = %q, want invitee@example.com", mail.to)
	}
	if !strings.Contains(mail.body, "abcdefghijklmnopqrstuv") {
		t.Errorf("email body does not carry the invitation code: %q", mail.body)
	}
}

func TestSendInvitation_DeliveryFailureIsUnavailable(t *testing.T) {
	repo := pendingInvitationRepo()
	sender := &fakeEmailSender{err: errors.New("smtp dial failed")}
	svc := Service{
		Clock:          fakeClock{fixedTime: time.Now()},
		UOW:            testUnitOfWork{invitationRepo: repo},
		SenderResolver: fakeSenderResolver{email: sender},
	}

	err := svc.SendInvitation(context.Background(), "org-1", "inv-1")
	if err == nil {
		t.Fatal("expected delivery failure to surface, got nil")
	}
	if apperrors.FromError(err).Code != apperrors.CodeUnavailable {
		t.Errorf("error code = %v, want unavailable (503)", apperrors.FromError(err).Code)
	}
}

func TestSendInvitation_ResolverFailurePropagates(t *testing.T) {
	repo := pendingInvitationRepo()
	svc := Service{
		Clock:          fakeClock{fixedTime: time.Now()},
		UOW:            testUnitOfWork{invitationRepo: repo},
		SenderResolver: fakeSenderResolver{err: apperrors.NewUnavailable("email provider disabled")},
	}

	err := svc.SendInvitation(context.Background(), "org-1", "inv-1")
	if err == nil || apperrors.FromError(err).Code != apperrors.CodeUnavailable {
		t.Fatalf("expected resolver 503 to propagate, got %v", err)
	}
}

func TestResendInvitation_DeliversEmail(t *testing.T) {
	repo := pendingInvitationRepo()
	sender := &fakeEmailSender{}
	now := time.Now()
	svc := Service{
		Clock:          fakeClock{fixedTime: now},
		UOW:            testUnitOfWork{invitationRepo: repo},
		SenderResolver: fakeSenderResolver{email: sender},
	}

	if err := svc.ResendInvitation(context.Background(), ResendInvitationCommand{ID: "inv-1", OrganizationID: "org-1"}); err != nil {
		t.Fatalf("ResendInvitation() error = %v", err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("expected exactly 1 delivered email, got %d", len(sender.sent))
	}
	if sender.sent[0].to != "invitee@example.com" {
		t.Errorf("recipient = %q, want invitee@example.com", sender.sent[0].to)
	}
	if !repo.invitations[0].ExpiresAt.Equal(now.Add(7 * 24 * time.Hour)) {
		t.Errorf("ExpiresAt = %v, want now+7d", repo.invitations[0].ExpiresAt)
	}
}

func TestResendInvitation_DeliveryFailureSurfaces(t *testing.T) {
	repo := pendingInvitationRepo()
	sender := &fakeEmailSender{err: errors.New("smtp dial failed")}
	svc := Service{
		Clock:          fakeClock{fixedTime: time.Now()},
		UOW:            testUnitOfWork{invitationRepo: repo},
		SenderResolver: fakeSenderResolver{email: sender},
	}

	err := svc.ResendInvitation(context.Background(), ResendInvitationCommand{ID: "inv-1", OrganizationID: "org-1"})
	if err == nil || apperrors.FromError(err).Code != apperrors.CodeUnavailable {
		t.Fatalf("expected resend delivery failure (503), got %v", err)
	}
}

// ---------------------------------------------------------------------------
// BE-A-15: ValidateInvitationCode distinguishes NotFound from outage
// ---------------------------------------------------------------------------

func TestValidateInvitationCode_InternalErrorIsUnavailable(t *testing.T) {
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UOW:   testUnitOfWork{invitationRepo: &mockInvitationRepo{getByCodeErr: errors.New("connection refused")}},
	}

	result, err := svc.ValidateInvitationCode(context.Background(), "any-code")
	if err == nil {
		t.Fatal("expected infrastructure outage to surface, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result on outage, got %+v", result)
	}
	if apperrors.FromError(err).Code != apperrors.CodeUnavailable {
		t.Errorf("error code = %v, want unavailable (503)", apperrors.FromError(err).Code)
	}
}

func TestValidateInvitationCode_NotFoundStaysInvalid(t *testing.T) {
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UOW:   testUnitOfWork{invitationRepo: &mockInvitationRepo{}},
	}

	result, err := svc.ValidateInvitationCode(context.Background(), "missing-code")
	if err != nil {
		t.Fatalf("NotFound must stay a silent invalid result, got error %v", err)
	}
	if result == nil || result.Valid {
		t.Errorf("expected Valid=false, got %+v", result)
	}
}

// ---------------------------------------------------------------------------
// BE-A-19: runInTx guard — a value without TxRunner yields Internal, not panic.
// (Typed IdentityUoW fields always declare WithinTransaction, so the guard is
// defense-in-depth for future narrow interfaces.)
// ---------------------------------------------------------------------------

func TestRunInTx_NonTxRunnerReturnsInternal(t *testing.T) {
	err := runInTx(context.Background(), struct{}{}, func(ctx context.Context, tx appuow.UnitOfWork) error {
		t.Fatal("transaction callback must not run without TxRunner")
		return nil
	})
	if err == nil {
		t.Fatal("expected internal error for value without TxRunner, got nil")
	}
	if apperrors.FromError(err).Code != apperrors.CodeInternal {
		t.Errorf("error code = %v, want internal", apperrors.FromError(err).Code)
	}
}
