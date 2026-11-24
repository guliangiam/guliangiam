package users

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	outboxdomain "github.com/guliangiam/guliangiam/internal/domain/outbox"
	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	trusteddevicesdomain "github.com/guliangiam/guliangiam/internal/domain/trusteddevices"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/passwordhistory"
)

// ---------------------------------------------------------------------------
// Fakes for the password-reset defence paths
// ---------------------------------------------------------------------------

type stubPlatformAdminRepo struct {
	platformadmins.PlatformAdminRepository
	admin *platformadmins.PlatformAdmin
	err   error
}

func (s stubPlatformAdminRepo) GetByUserID(context.Context, string) (*platformadmins.PlatformAdmin, error) {
	return s.admin, s.err
}

type stubPlatformAdminScopeRepo struct {
	platformadmins.PlatformAdminScopeRepository
	scopes []*platformadmins.PlatformAdminScope
	err    error
}

func (s stubPlatformAdminScopeRepo) ListByPlatformAdmin(context.Context, string, platformadmins.ScopeListOptions) ([]*platformadmins.PlatformAdminScope, error) {
	return s.scopes, s.err
}

type stubTrustedDevices struct {
	trusteddevicesdomain.Repository
	revoked int64
	orgIDs  []string
	userIDs []string
}

func (s *stubTrustedDevices) RevokeByUser(_ context.Context, organizationID, userID string, _ time.Time) (int64, error) {
	s.orgIDs = append(s.orgIDs, organizationID)
	s.userIDs = append(s.userIDs, userID)
	return s.revoked, nil
}

type capturingOutbox struct {
	outboxdomain.Repository
	events []*outboxdomain.OutboxEvent
}

func (o *capturingOutbox) Enqueue(_ context.Context, event *outboxdomain.OutboxEvent) error {
	o.events = append(o.events, event)
	return nil
}

type capturingNotifier struct {
	commands []NotificationCommand
	err      error
}

func (n *capturingNotifier) Notify(_ context.Context, command NotificationCommand) error {
	n.commands = append(n.commands, command)
	return n.err
}

func platformAdminNotFound() stubPlatformAdminRepo {
	return stubPlatformAdminRepo{err: apperrors.NewNotFound("platform admin not found")}
}

func resetService(uow testUnitOfWork) *Service {
	return &Service{
		Clock: fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUID:  fakeUUIDGenerator{fixedID: "019hist-f000-7000-8000-000000000001"},
		UOW:   uow,
	}
}

func requireCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T (%v)", err, err)
	}
	if appErr.Code != code {
		t.Fatalf("code = %q, want %q (%v)", appErr.Code, code, err)
	}
}

func newResetTarget() *userdomain.User {
	user := newTestUser()
	user.Credentials = userdomain.UserCredentials{
		UserID:       user.ID,
		HasPassword:  true,
		PasswordHash: "old-argon2id-hash",
		PasswordAlgo: userdomain.PasswordAlgorithmArgon2ID,
	}
	return user
}

func baseResetCommand() ResetUserPasswordCommand {
	return ResetUserPasswordCommand{
		OrganizationID:      testOrganizationID,
		UserID:              testUserID,
		Password:            "NewPassw0rd!2026",
		ActorID:             "019operator-0000-7000-8000-000000000002",
		ActorOrganizationID: testOrganizationID,
	}
}

// ---------------------------------------------------------------------------
// Defence paths
// ---------------------------------------------------------------------------

func TestResetUserPassword_CrossOrganizationReturnsNotFound(t *testing.T) {
	userRepo := &mockUserRepo{
		getByIDForOrganizationFn: func(context.Context, string, string) (*userdomain.User, error) {
			return nil, apperrors.NewNotFound("user not found")
		},
	}
	uow := testUnitOfWork{users: userRepo, platformAdmins: platformAdminNotFound()}

	_, err := resetService(uow).ResetUserPassword(context.Background(), baseResetCommand())
	requireCode(t, err, apperrors.CodeNotFound)
}

func TestResetUserPassword_SelfResetRejected(t *testing.T) {
	user := newResetTarget()
	userRepo := &mockUserRepo{
		getByIDForOrganizationFn: func(context.Context, string, string) (*userdomain.User, error) {
			return user, nil
		},
	}
	command := baseResetCommand()
	command.ActorID = testUserID

	uow := testUnitOfWork{users: userRepo, platformAdmins: platformAdminNotFound()}
	_, err := resetService(uow).ResetUserPassword(context.Background(), command)
	requireCode(t, err, apperrors.CodeUnprocessableEntity)
}

func TestResetUserPassword_PlatformAdminTargetHiddenFromNonPlatformActor(t *testing.T) {
	user := newResetTarget()
	userRepo := &mockUserRepo{
		getByIDForOrganizationFn: func(context.Context, string, string) (*userdomain.User, error) {
			return user, nil
		},
	}
	adminRepo := stubPlatformAdminRepo{admin: &platformadmins.PlatformAdmin{
		ID: "019pa-0000-7000-8000-000000000001", UserID: testUserID, IsActive: true,
	}}

	command := baseResetCommand()
	command.ActorIsPlatformAdmin = false

	uow := testUnitOfWork{users: userRepo, platformAdmins: adminRepo}
	_, err := resetService(uow).ResetUserPassword(context.Background(), command)
	requireCode(t, err, apperrors.CodeNotFound)
}

func TestResetUserPassword_BootstrapPlatformAdminAlwaysHidden(t *testing.T) {
	user := newResetTarget()
	userRepo := &mockUserRepo{
		getByIDForOrganizationFn: func(context.Context, string, string) (*userdomain.User, error) {
			return user, nil
		},
	}
	adminRepo := stubPlatformAdminRepo{admin: &platformadmins.PlatformAdmin{
		ID: "019pa-0000-7000-8000-000000000001", UserID: testUserID, IsActive: true,
	}}
	scopeRepo := stubPlatformAdminScopeRepo{scopes: []*platformadmins.PlatformAdminScope{
		{ID: "019scope-0000-7000-8000-000000000001", PlatformAdminID: "019pa-0000-7000-8000-000000000001", IsBootstrap: true},
	}}

	command := baseResetCommand()
	command.ActorIsPlatformAdmin = true

	uow := testUnitOfWork{users: userRepo, platformAdmins: adminRepo, platformAdminScope: scopeRepo}
	_, err := resetService(uow).ResetUserPassword(context.Background(), command)
	requireCode(t, err, apperrors.CodeNotFound)
}

func TestResetUserPassword_PolicyViolationRejected(t *testing.T) {
	userRepo := &mockUserRepo{
		getByIDForOrganizationFn: func(context.Context, string, string) (*userdomain.User, error) {
			return newResetTarget(), nil
		},
	}
	orgRepo := &mockOrgRepo{getByIDFn: func(context.Context, string) (*orgdomain.Organization, error) {
		return newTestOrganization(), nil
	}}

	command := baseResetCommand()
	command.Password = "short"

	uow := testUnitOfWork{users: userRepo, organizations: orgRepo, platformAdmins: platformAdminNotFound()}
	_, err := resetService(uow).ResetUserPassword(context.Background(), command)
	requireCode(t, err, apperrors.CodeUnprocessableEntity)
}

func TestResetUserPassword_ReasonTooLongRejected(t *testing.T) {
	command := baseResetCommand()
	command.Reason = strings.Repeat("长", 129)

	_, err := resetService(testUnitOfWork{}).ResetUserPassword(context.Background(), command)
	requireCode(t, err, apperrors.CodeUnprocessableEntity)
}

// TestResetUserPassword_SuccessRevokesCredentialsAndAudits 覆盖：
//   - 口令列定向写入（need_update_password=true + 新哈希）
//   - 旧哈希进入历史
//   - 会话 / refresh / access / 受信设备全部吊销，并回传计数
//   - 审计 payload 只含计数与租户，不含任何口令或哈希
func TestResetUserPassword_SuccessRevokesCredentialsAndAudits(t *testing.T) {
	user := newResetTarget()
	userRepo := &mockUserRepo{
		getByIDForOrganizationFn: func(context.Context, string, string) (*userdomain.User, error) {
			return user, nil
		},
	}
	var savedCreds *userdomain.UserCredentials
	userRepo.updatePasswordCredentialFn = func(_ context.Context, credentials *userdomain.UserCredentials) error {
		savedCreds = credentials
		return nil
	}
	var historyEntry *userdomain.PasswordHistoryEntry
	userRepo.createPasswordHistoryFn = func(_ context.Context, entry *userdomain.PasswordHistoryEntry) error {
		historyEntry = entry
		return nil
	}
	orgRepo := &mockOrgRepo{getByIDFn: func(context.Context, string) (*orgdomain.Organization, error) {
		return newTestOrganization(), nil
	}}

	sessions := &mockSessionRepo{revokeByUserFn: func(context.Context, string, time.Time) (int64, error) {
		return 2, nil
	}}
	tokens := &mockTokenRepo{
		revokeRefreshByUserFn: func(context.Context, string, time.Time) (int64, error) { return 1, nil },
		revokeByQueryFn:       func(context.Context, tokendomain.RevokeQuery, time.Time) (int64, error) { return 3, nil },
	}
	devices := &stubTrustedDevices{revoked: 4}
	records := &fakeRecordsRepo{}
	outbox := &capturingOutbox{}

	uow := testUnitOfWork{
		users:          userRepo,
		organizations:  orgRepo,
		sessions:       sessions,
		tokens:         tokens,
		records:        records,
		outbox:         outbox,
		trustedDevices: devices,
		platformAdmins: platformAdminNotFound(),
	}

	command := baseResetCommand()
	command.Reason = "employee reported a compromised laptop"

	service := resetService(uow)
	notifier := &capturingNotifier{}
	service.Notifier = notifier

	result, err := service.ResetUserPassword(context.Background(), command)
	if err != nil {
		t.Fatalf("ResetUserPassword() error = %v", err)
	}

	if savedCreds == nil {
		t.Fatal("password credentials were not written")
	}
	if savedCreds.UserID != testUserID || !savedCreds.HasPassword || !savedCreds.NeedUpdatePassword {
		t.Fatalf("unexpected credential write: %+v", savedCreds)
	}
	if savedCreds.PasswordAlgo != userdomain.PasswordAlgorithmArgon2ID || savedCreds.PasswordUpdatedAt == nil {
		t.Fatalf("unexpected credential algo/timestamp: %+v", savedCreds)
	}
	if savedCreds.PasswordHash == "" || savedCreds.PasswordHash == user.Credentials.PasswordHash {
		t.Fatalf("password hash was not rotated")
	}

	if historyEntry == nil || historyEntry.PasswordAlgo != userdomain.PasswordAlgorithmArgon2ID {
		t.Fatalf("password history entry missing or wrong algo: %+v", historyEntry)
	}
	if historyEntry.PasswordHash == "old-argon2id-hash" {
		t.Fatal("history stored the credential hash instead of the peppered history hash")
	}
	matched, err := passwordhistory.Verify("NewPassw0rd!2026", historyEntry.PasswordHash)
	if err != nil || !matched {
		t.Fatalf("history hash is not verifiable with the history algorithm: matched=%v err=%v", matched, err)
	}

	if result.SessionsRevoked != 2 || result.RefreshRevoked != 1 || result.AccessRevoked != 3 || result.TrustedDevicesRevoked != 4 {
		t.Fatalf("unexpected revocation counts: %+v", result)
	}
	if len(devices.orgIDs) != 1 || devices.orgIDs[0] != testOrganizationID || devices.userIDs[0] != testUserID {
		t.Fatalf("trusted devices revoked with wrong scope: org=%v user=%v", devices.orgIDs, devices.userIDs)
	}

	if len(records.created) != 1 {
		t.Fatalf("audit records = %d, want 1", len(records.created))
	}
	record := records.created[0]
	if record.Action != "user.password.reset" || record.ActorID != command.ActorID || record.ResourceID != testUserID {
		t.Fatalf("unexpected audit record: %+v", record)
	}
	if record.OrganizationID != testOrganizationID {
		t.Fatalf("audit organization = %q, want %q", record.OrganizationID, testOrganizationID)
	}

	if len(outbox.events) != 1 {
		t.Fatalf("outbox events = %d, want 1", len(outbox.events))
	}
	var payload map[string]any
	if err := json.Unmarshal(outbox.events[0].Payload, &payload); err != nil {
		t.Fatalf("decode outbox payload: %v", err)
	}
	if payload["targetUserId"] != testUserID {
		t.Fatalf("payload targetUserId = %v", payload["targetUserId"])
	}
	if payload["method"] != "admin" {
		t.Fatalf("payload method = %v", payload["method"])
	}
	if payload["sessionRevokedCount"] != float64(2) || payload["trustedDevicesRevokedCount"] != float64(4) {
		t.Fatalf("payload counts = %v / %v", payload["sessionRevokedCount"], payload["trustedDevicesRevokedCount"])
	}
	if payload["reason"] != command.Reason {
		t.Fatalf("payload reason = %v", payload["reason"])
	}
	for _, forbidden := range []string{"password", "passwordHash", "newPassword"} {
		if _, exists := payload[forbidden]; exists {
			t.Fatalf("audit payload must not contain %q", forbidden)
		}
	}

	if len(notifier.commands) != 1 {
		t.Fatalf("notifications = %d, want 1", len(notifier.commands))
	}
	notification := notifier.commands[0]
	if notification.OrganizationID != testOrganizationID || notification.UserID != testUserID {
		t.Fatalf("notification target = %s/%s, want %s/%s",
			notification.OrganizationID, notification.UserID, testOrganizationID, testUserID)
	}
	if notification.Category != "security" || notification.EventType != "user.password.reset" {
		t.Fatalf("unexpected notification category/event: %+v", notification)
	}
	if notification.Link != "/account" || notification.SourceID != testUserID {
		t.Fatalf("unexpected notification link/source: %+v", notification)
	}
	if notification.Title == "" || notification.Content == "" {
		t.Fatalf("notification title/content must not be empty: %+v", notification)
	}
	if strings.Contains(notification.Content, command.Password) {
		t.Fatal("notification content must not contain the new password")
	}
}

// TestResetUserPassword_NotificationFailureDoesNotFailReset 覆盖：通知投递失败
// 只记录日志，不影响已完成的口令重置与审计。
func TestResetUserPassword_NotificationFailureDoesNotFailReset(t *testing.T) {
	user := newResetTarget()
	userRepo := &mockUserRepo{
		getByIDForOrganizationFn: func(context.Context, string, string) (*userdomain.User, error) {
			return user, nil
		},
	}
	orgRepo := &mockOrgRepo{getByIDFn: func(context.Context, string) (*orgdomain.Organization, error) {
		return newTestOrganization(), nil
	}}
	uow := testUnitOfWork{
		users:          userRepo,
		organizations:  orgRepo,
		sessions:       &mockSessionRepo{},
		tokens:         &mockTokenRepo{},
		records:        &fakeRecordsRepo{},
		outbox:         &capturingOutbox{},
		trustedDevices: &stubTrustedDevices{},
		platformAdmins: platformAdminNotFound(),
	}

	service := resetService(uow)
	notifier := &capturingNotifier{err: errors.New("inbox unavailable")}
	service.Notifier = notifier

	if _, err := service.ResetUserPassword(context.Background(), baseResetCommand()); err != nil {
		t.Fatalf("ResetUserPassword() must tolerate notification failure, got %v", err)
	}
	if len(notifier.commands) != 1 {
		t.Fatalf("notifications = %d, want 1 attempt", len(notifier.commands))
	}
}
