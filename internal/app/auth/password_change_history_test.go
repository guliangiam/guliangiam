package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	captchaproviders "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	invitationsdomain "github.com/guliangiam/guliangiam/internal/domain/invitations"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	outboxdomain "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	sessionsdomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	verificationsdomain "github.com/guliangiam/guliangiam/internal/domain/verifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/passwordhistory"
)

// historyUserRepo is a stateful in-memory users repository that actually stores
// password history, so the reuse check can be exercised end to end.
type historyUserRepo struct {
	userdomain.Repository

	user       *userdomain.User
	history    []userdomain.PasswordHistoryEntry
	failCreate error
	failRead   error
	updated    *userdomain.UserCredentials
}

func (r *historyUserRepo) GetByUsername(_ context.Context, organizationID, username string) (*userdomain.User, error) {
	if r.user != nil && r.user.OrganizationID == organizationID && r.user.Username == username {
		return r.user, nil
	}
	return nil, apperrors.NewNotFound("user not found")
}

func (r *historyUserRepo) Create(_ context.Context, user *userdomain.User) error {
	r.user = user
	return nil
}

func (r *historyUserRepo) Update(_ context.Context, user *userdomain.User) error {
	r.user = user
	return nil
}

func (r *historyUserRepo) UpdateCredentials(_ context.Context, credentials *userdomain.UserCredentials) error {
	if r.user != nil {
		r.user.Credentials = *credentials
	}
	return nil
}

func (r *historyUserRepo) SetContactVerifiedForOrganization(context.Context, string, string, bool, bool) error {
	return nil
}

func (r *historyUserRepo) UpdateCollectionsForOrganization(context.Context, string, string, userdomain.UserCollections) error {
	return nil
}

func (r *historyUserRepo) UpdatePasswordCredential(_ context.Context, credentials *userdomain.UserCredentials) error {
	r.updated = credentials
	if r.user != nil {
		r.user.Credentials = *credentials
	}
	return nil
}

func (r *historyUserRepo) GetPasswordHistory(_ context.Context, userID string, limit int) ([]userdomain.PasswordHistoryEntry, error) {
	if r.failRead != nil {
		return nil, r.failRead
	}
	entries := make([]userdomain.PasswordHistoryEntry, 0, len(r.history))
	for _, entry := range r.history {
		if entry.UserID == userID {
			entries = append(entries, entry)
		}
	}
	if limit > 0 && len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	return entries, nil
}

func (r *historyUserRepo) CreatePasswordHistory(_ context.Context, entry *userdomain.PasswordHistoryEntry) error {
	if r.failCreate != nil {
		return r.failCreate
	}
	r.history = append(r.history, *entry)
	return nil
}

func (r *historyUserRepo) DeletePasswordHistoryBeyond(_ context.Context, _ string, _ int) error {
	return nil
}

type historyOrgRepo struct {
	orgdomain.Repository
	org     *orgdomain.Organization
	lastCtx context.Context
}

func (r *historyOrgRepo) GetBySlug(ctx context.Context, _ string) (*orgdomain.Organization, error) {
	r.lastCtx = ctx
	return r.org, nil
}

func (r *historyOrgRepo) GetByID(_ context.Context, _ string) (*orgdomain.Organization, error) {
	return r.org, nil
}

type historySessionRepo struct {
	sessionsdomain.Repository
}

func (r *historySessionRepo) RevokeByUserExcept(_ context.Context, _ string, _ string, _ time.Time) (int64, error) {
	return 0, nil
}

func (r *historySessionRepo) RevokeByUser(_ context.Context, _ string, _ time.Time) (int64, error) {
	return 0, nil
}

type historyTokenRepo struct {
	tokendomain.Repository
}

func (r *historyTokenRepo) RevokeRefreshByUser(_ context.Context, _ string, _ time.Time) (int64, error) {
	return 0, nil
}

func (r *historyTokenRepo) RevokeByQuery(_ context.Context, _ tokendomain.RevokeQuery, _ time.Time) (int64, error) {
	return 0, nil
}

type historyVerificationRepo struct {
	verificationsdomain.Repository
	verification *verificationsdomain.Verification
}

func (r *historyVerificationRepo) GetActiveByRecipientAndCode(_ context.Context, _ string, _ string, _ string) (*verificationsdomain.Verification, error) {
	return r.verification, nil
}

func (r *historyVerificationRepo) UpdateStatus(_ context.Context, _ string, _ string) error {
	return nil
}

type historyRecordRepo struct {
	recordsdomain.Repository
	created int
}

func (r *historyRecordRepo) Create(_ context.Context, _ *recordsdomain.Record) error {
	r.created++
	return nil
}

type historyOutboxRepo struct {
	outboxdomain.Repository
	enqueued int
	events   []*outboxdomain.OutboxEvent
}

func (r *historyOutboxRepo) Enqueue(_ context.Context, event *outboxdomain.OutboxEvent) error {
	r.enqueued++
	r.events = append(r.events, event)
	return nil
}

type historyUoW struct {
	appuow.UnitOfWork

	users         *historyUserRepo
	orgs          *historyOrgRepo
	sessions      *historySessionRepo
	tokens        *historyTokenRepo
	verifications *historyVerificationRepo
	records       *historyRecordRepo
	outbox        *historyOutboxRepo
	invitations   *historyInvitationRepo
	captcha       captchaproviders.Repository
}

func (u *historyUoW) CaptchaProviders() captchaproviders.Repository { return u.captcha }

func (u *historyUoW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func (u *historyUoW) Users() userdomain.Repository                  { return u.users }
func (u *historyUoW) Organizations() orgdomain.Repository           { return u.orgs }
func (u *historyUoW) Sessions() sessionsdomain.Repository           { return u.sessions }
func (u *historyUoW) Tokens() tokendomain.Repository                { return u.tokens }
func (u *historyUoW) Verifications() verificationsdomain.Repository { return u.verifications }
func (u *historyUoW) Records() recordsdomain.Repository             { return u.records }
func (u *historyUoW) Outbox() outboxdomain.Repository               { return u.outbox }
func (u *historyUoW) Invitations() invitationsdomain.Repository     { return u.invitations }

func newHistoryStack(historyCount int) (*Service, *historyUoW) {
	org := newTestOrg(func(o *orgdomain.Organization) {
		o.Security.PasswordPolicy = map[string]any{"passwordHistoryCount": historyCount, "minLength": 8}
	})
	uow := &historyUoW{
		users:         &historyUserRepo{user: newTestUser()},
		orgs:          &historyOrgRepo{org: org},
		sessions:      &historySessionRepo{},
		tokens:        &historyTokenRepo{},
		verifications: &historyVerificationRepo{verification: &verificationsdomain.Verification{ID: "verification-1", OrganizationID: testOrgID}},
		records:       &historyRecordRepo{},
		outbox:        &historyOutboxRepo{},
		invitations:   &historyInvitationRepo{},
	}
	service := &Service{
		UOW:           uow,
		Clock:         fakeClock{fixedTime: time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "generated-id"},
	}
	return service, uow
}

func mustChangePassword(t *testing.T, service *Service, oldPassword, newPassword string) {
	t.Helper()
	if _, _, err := service.ChangePassword(context.Background(), ChangePasswordCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		OldPassword:      oldPassword,
		NewPassword:      newPassword,
	}); err != nil {
		t.Fatalf("change password %q -> %q: %v", oldPassword, newPassword, err)
	}
}

func TestChangePasswordRejectsPasswordFromHistory(t *testing.T) {
	const (
		passwordB = "Hist0ry!B2026"
		passwordC = "Hist0ry!C2026"
	)
	service, uow := newHistoryStack(5)

	mustChangePassword(t, service, testPassword, passwordB)
	mustChangePassword(t, service, passwordB, passwordC)

	_, _, err := service.ChangePassword(context.Background(), ChangePasswordCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		OldPassword:      passwordC,
		NewPassword:      passwordB,
	})
	if err == nil {
		t.Fatal("reusing a password from history must be rejected")
	}

	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error type = %T, want *apperrors.Error", err)
	}
	if appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("code = %q, want %q", appErr.Code, apperrors.CodeUnprocessableEntity)
	}
	if appErr.Meta["rule"] != "password_history" {
		t.Fatalf("meta.rule = %v, want password_history", appErr.Meta["rule"])
	}
	if got := appErr.Meta["historyCount"]; got != 5 {
		t.Fatalf("meta.historyCount = %v, want 5", got)
	}
	failures, ok := appErr.Meta["failures"].([]map[string]any)
	if !ok || len(failures) != 1 {
		t.Fatalf("meta.failures = %#v, want one item", appErr.Meta["failures"])
	}
	if failures[0]["rule"] != "password_history" {
		t.Fatalf("failures[0].rule = %v, want password_history", failures[0]["rule"])
	}

	// Every archived row must be verifiable with the history algorithm.
	if len(uow.users.history) != 2 {
		t.Fatalf("history entries = %d, want 2", len(uow.users.history))
	}
	for i, plaintext := range []string{testPassword, passwordB} {
		matched, verifyErr := passwordhistory.Verify(plaintext, uow.users.history[i].PasswordHash)
		if verifyErr != nil || !matched {
			t.Fatalf("history[%d] is not a history-pepper hash: matched=%v err=%v", i, matched, verifyErr)
		}
	}
}

func TestChangePasswordAllowsReuseWhenHistoryDisabled(t *testing.T) {
	service, _ := newHistoryStack(0)

	mustChangePassword(t, service, testPassword, "Hist0ry!B2026")
	mustChangePassword(t, service, "Hist0ry!B2026", testPassword)
}

func TestPasswordHistoryLookupFailureFailsOpenAndCounts(t *testing.T) {
	const newPassword = "Hist0ry!B2026"
	service, uow := newHistoryStack(5)
	uow.users.failRead = errors.New("history store unavailable")

	before := counterValue(t, "password_history_fail_open_total")

	if _, _, err := service.ChangePassword(context.Background(), ChangePasswordCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		OldPassword:      testPassword,
		NewPassword:      newPassword,
	}); err != nil {
		t.Fatalf("change must succeed when the history store is unavailable: %v", err)
	}

	if delta := counterValue(t, "password_history_fail_open_total") - before; delta != 1 {
		t.Fatalf("password_history_fail_open_total delta = %v, want 1", delta)
	}
	if uow.users.updated == nil || uow.users.updated.PasswordHash == "" {
		t.Fatal("password credential was not updated")
	}
}

func counterValue(t *testing.T, familyName string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() != familyName {
			continue
		}
		for _, metric := range family.GetMetric() {
			if metric.Counter != nil {
				return metric.Counter.GetValue()
			}
		}
	}
	return 0
}

func TestChangePasswordHistoryWriteFailureRollsBack(t *testing.T) {
	service, uow := newHistoryStack(5)
	uow.users.user.Credentials.NeedUpdatePassword = true
	uow.users.failCreate = errors.New("history store unavailable")

	_, _, err := service.ChangePassword(context.Background(), ChangePasswordCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		OldPassword:      testPassword,
		NewPassword:      "Hist0ry!B2026",
	})
	if err == nil {
		t.Fatal("history write failure must surface")
	}
	if uow.users.updated != nil {
		t.Fatal("password credential was updated despite history failure")
	}
	if uow.users.user.Credentials.PasswordHash != testPasswordHash {
		t.Fatal("password_hash changed despite history failure")
	}
	if !uow.users.user.Credentials.NeedUpdatePassword {
		t.Fatal("need_update_password changed despite history failure")
	}
	if uow.records.created != 0 {
		t.Fatalf("audit records written = %d, want 0", uow.records.created)
	}
	if uow.outbox.enqueued != 0 {
		t.Fatalf("outbox events written = %d, want 0", uow.outbox.enqueued)
	}
}

func TestSignupWritesPepperedHistory(t *testing.T) {
	const password = "Signup!A2026"
	service, uow := newHistoryStack(5)
	uow.users.user = nil

	if _, err := service.Signup(context.Background(), SignupCommand{
		OrganizationSlug: testOrgSlug,
		Username:         "newuser",
		Password:         password,
		Email:            "newuser@example.com",
	}); err != nil {
		t.Fatalf("signup: %v", err)
	}

	if len(uow.users.history) != 1 {
		t.Fatalf("history entries = %d, want 1", len(uow.users.history))
	}
	matched, err := passwordhistory.Verify(password, uow.users.history[0].PasswordHash)
	if err != nil || !matched {
		t.Fatalf("signup history is not a history-pepper hash: matched=%v err=%v", matched, err)
	}
}

func TestResetPasswordWritesPepperedHistory(t *testing.T) {
	const password = "Reset!A2026"
	service, uow := newHistoryStack(5)

	if _, _, err := service.ResetPassword(context.Background(), ResetPasswordCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		VerificationCode: "123456",
		NewPassword:      password,
	}); err != nil {
		t.Fatalf("reset password: %v", err)
	}

	if len(uow.users.history) != 1 {
		t.Fatalf("history entries = %d, want 1", len(uow.users.history))
	}
	matched, err := passwordhistory.Verify(password, uow.users.history[0].PasswordHash)
	if err != nil || !matched {
		t.Fatalf("reset history is not a history-pepper hash: matched=%v err=%v", matched, err)
	}
}
