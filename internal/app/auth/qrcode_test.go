package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	applications "github.com/guliangiam/guliangiam/internal/domain/applications"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	qrcodedomain "github.com/guliangiam/guliangiam/internal/domain/qrcodesessions"
	qrmock "github.com/guliangiam/guliangiam/internal/domain/qrcodesessions/mock"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// QR Code 登录的服务层用例：身份绑定（OPEN-API-15）、自动注册、以及 exchange 的
// 会话签发（OPEN-API-16）。字段名与状态取值对齐 internal/app/auth/qrcode.go。
const testQRCode = "qr-payload-001"

// newQRLoginSession builds a pending QR session owned by the test org/app.
func newQRLoginSession(overrides ...func(*qrcodedomain.Session)) *qrcodedomain.Session {
	session := &qrcodedomain.Session{
		ID:             testSessID,
		CodeID:         "qr-session-001",
		Code:           testQRCode,
		OrganizationID: testOrgID,
		ApplicationID:  testAppID,
		ExpiresAt:      time.Date(2026, 5, 15, 10, 5, 0, 0, time.UTC),
		Status:         "pending",
		CreatedAt:      time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC),
		Revision:       1,
	}
	for _, fn := range overrides {
		fn(session)
	}
	return session
}

func newQRTestUserRepository(users ...*userdomain.User) *loginTestUserRepository {
	repo := &loginTestUserRepository{
		users:      map[string]*userdomain.User{},
		usersByOrg: map[string]map[string]*userdomain.User{},
	}
	for _, user := range users {
		repo.users[user.ID] = user
		if repo.usersByOrg[user.OrganizationID] == nil {
			repo.usersByOrg[user.OrganizationID] = map[string]*userdomain.User{}
		}
		repo.usersByOrg[user.OrganizationID][user.Username] = user
	}
	return repo
}

func newQRTestOrgRepository(orgs ...*orgdomain.Organization) *loginTestOrgRepository {
	bySlug := map[string]*orgdomain.Organization{}
	for _, org := range orgs {
		bySlug[org.Slug] = org
	}
	return &loginTestOrgRepository{orgs: bySlug}
}

func TestScanQRCode_AuthenticatedScannerBindsSessionSubject(t *testing.T) {
	ctrl := gomock.NewController(t)
	user := newTestUser()
	qrRepo := qrmock.NewMockRepository(ctrl)
	qrRepo.EXPECT().GetByCode(gomock.Any(), testQRCode).Return(newQRLoginSession(), nil)
	qrRepo.EXPECT().UpdateStatus(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, session *qrcodedomain.Session) error {
			assert.Equal(t, "scanned", session.Status)
			assert.Equal(t, user.ID, session.UserID)
			assert.Equal(t, testUsername, session.Username)
			return nil
		})

	svc := newTestService(loginTestUnitOfWork{
		orgRepo:  newQRTestOrgRepository(newTestOrg()),
		userRepo: newQRTestUserRepository(user),
		qrRepo:   qrRepo,
		sessRepo: &loginTestSessionRepository{},
	})

	result, sessionID, err := svc.ScanQRCode(context.Background(), ScanQRCodeCommand{
		Code:                  testQRCode,
		Username:              "body-supplied-identity",
		ScannerUsername:       testUsername,
		ScannerOrganizationID: testOrgID,
	})
	require.NoError(t, err)
	assert.False(t, result.IsNewUser)
	assert.Empty(t, sessionID, "已认证扫码端已有会话，不应再签发一个")
}

func TestScanQRCode_AnonymousCannotBindExistingAccount(t *testing.T) {
	ctrl := gomock.NewController(t)
	user := newTestUser()
	qrRepo := qrmock.NewMockRepository(ctrl)
	qrRepo.EXPECT().GetByCode(gomock.Any(), testQRCode).Return(newQRLoginSession(), nil)
	// 不得发生状态迁移：拒绝必须发生在绑定之前。

	svc := newTestService(loginTestUnitOfWork{
		orgRepo:  newQRTestOrgRepository(newTestOrg()),
		userRepo: newQRTestUserRepository(user),
		qrRepo:   qrRepo,
		sessRepo: &loginTestSessionRepository{},
	})

	_, sessionID, err := svc.ScanQRCode(context.Background(), ScanQRCodeCommand{
		Code:     testQRCode,
		Username: testUsername,
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeForbidden, apperrors.FromError(err).Code)
	assert.Empty(t, sessionID)
}

func TestScanQRCode_AnonymousAutoRegistersScannerAndIssuesSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := newTestOrg(func(o *orgdomain.Organization) {
		o.InitScore = 42
		o.DefaultAvatar = "https://cdn.example.com/avatar.png"
	})
	users := newQRTestUserRepository()
	sessRepo := &loginTestSessionRepository{}
	qrRepo := qrmock.NewMockRepository(ctrl)
	qrRepo.EXPECT().GetByCode(gomock.Any(), testQRCode).Return(newQRLoginSession(), nil)
	qrRepo.EXPECT().UpdateStatus(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, session *qrcodedomain.Session) error {
			assert.Equal(t, "scanned", session.Status)
			assert.Equal(t, testSessID, session.UserID)
			return nil
		})

	svc := newTestService(loginTestUnitOfWork{
		orgRepo:  newQRTestOrgRepository(org),
		userRepo: users,
		qrRepo:   qrRepo,
		sessRepo: sessRepo,
	})

	result, sessionID, err := svc.ScanQRCode(context.Background(), ScanQRCodeCommand{
		Code:        testQRCode,
		Username:    "newcomer",
		DisplayName: "New Comer",
	})
	require.NoError(t, err)
	assert.True(t, result.IsNewUser)
	require.NotEmpty(t, sessionID, "匿名注册后必须签发会话，否则走不到 confirm")

	created, ok := users.users[testSessID]
	require.True(t, ok, "匿名扫码必须自动注册账号")
	assert.Equal(t, "newcomer", created.Username)
	assert.Equal(t, testOrgID, created.OrganizationID)
	assert.Equal(t, "QR Code Scan", created.RegisterType)
	assert.Equal(t, "New Comer", created.Profile.DisplayName)
	assert.Equal(t, 42, created.Score)
	assert.Equal(t, "guest", created.UserType, "匿名扫码只能注册访客账号，联系方式由本人后续绑定")
	assert.False(t, created.Credentials.HasPassword, "扫码注册的账号不带密码凭据")

	issued := sessRepo.sessions[sessionID]
	require.NotNil(t, issued)
	assert.Equal(t, created.ID, issued.UserID)
	assert.Equal(t, "qrcode", issued.DeviceInfo["loginMethod"])
	// 设备记忆就是这张会话：sessionTTL = 30 天（fakeClock 固定 2026-05-15 10:00 UTC）。
	assert.Equal(t, time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC), issued.ExpiresAt)
}

func TestScanQRCode_SameDeviceReusesTheGuestAccount(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := newTestOrg()
	users := newQRTestUserRepository()
	sessRepo := &loginTestSessionRepository{}
	qrRepo := qrmock.NewMockRepository(ctrl)

	// 第一次：匿名扫码 → 自动注册访客账号 + 签发扫码端会话（设备记忆）。
	first := newQRLoginSession()
	qrRepo.EXPECT().GetByCode(gomock.Any(), testQRCode).Return(first, nil)
	qrRepo.EXPECT().UpdateStatus(gomock.Any(), gomock.Any()).Return(nil)
	svc := newTestService(loginTestUnitOfWork{
		orgRepo:  newQRTestOrgRepository(org),
		userRepo: users,
		qrRepo:   qrRepo,
		sessRepo: sessRepo,
	})

	registered, scannerSessionID, err := svc.ScanQRCode(context.Background(), ScanQRCodeCommand{
		Code:     testQRCode,
		Username: "newcomer",
	})
	require.NoError(t, err)
	assert.True(t, registered.IsNewUser)
	require.NotEmpty(t, scannerSessionID)
	require.Len(t, users.created, 1)

	// 第二次：同一设备带着会话再扫另一张二维码 → 身份取自会话，复用同一账号。
	second := newQRLoginSession(func(session *qrcodedomain.Session) {
		session.CodeID = "qr-session-002"
		session.Code = "qr-payload-002"
	})
	qrRepo.EXPECT().GetByCode(gomock.Any(), "qr-payload-002").Return(second, nil)
	qrRepo.EXPECT().UpdateStatus(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, session *qrcodedomain.Session) error {
			assert.Equal(t, testSessID, session.UserID, "第二次扫码必须绑定同一个访客账号")
			assert.Equal(t, "newcomer", session.Username)
			return nil
		})

	reused, secondScannerSessionID, err := svc.ScanQRCode(context.Background(), ScanQRCodeCommand{
		Code:                  "qr-payload-002",
		ScannerUsername:       "newcomer",
		ScannerOrganizationID: testOrgID,
	})
	require.NoError(t, err)
	assert.False(t, reused.IsNewUser, "设备已认领账号，不再自动注册")
	assert.Empty(t, secondScannerSessionID, "已认证扫码端已有会话，不重复签发")
	assert.Len(t, users.created, 1, "同一设备重复扫码不得再次建号")
}

func TestScanQRCode_AnonymousRegistrationRespectsOrganizationSwitch(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := newTestOrg(func(o *orgdomain.Organization) { o.DisableSignup = true })
	qrRepo := qrmock.NewMockRepository(ctrl)
	qrRepo.EXPECT().GetByCode(gomock.Any(), testQRCode).Return(newQRLoginSession(), nil)

	svc := newTestService(loginTestUnitOfWork{
		orgRepo:  newQRTestOrgRepository(org),
		userRepo: newQRTestUserRepository(),
		qrRepo:   qrRepo,
		sessRepo: &loginTestSessionRepository{},
	})

	_, _, err := svc.ScanQRCode(context.Background(), ScanQRCodeCommand{
		Code:     testQRCode,
		Username: "newcomer",
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeForbidden, apperrors.FromError(err).Code)
}

func TestConfirmQRCode_OnlyScanningSubjectCanConfirm(t *testing.T) {
	ctrl := gomock.NewController(t)
	scanned := newQRLoginSession(func(session *qrcodedomain.Session) {
		session.Status = "scanned"
		session.UserID = testUserID
		session.Username = testUsername
	})
	qrRepo := qrmock.NewMockRepository(ctrl)
	qrRepo.EXPECT().GetByCode(gomock.Any(), testQRCode).Return(scanned, nil).Times(2)
	qrRepo.EXPECT().UpdateStatus(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, session *qrcodedomain.Session) error {
			assert.Equal(t, "confirmed", session.Status)
			return nil
		})

	svc := newTestService(loginTestUnitOfWork{qrRepo: qrRepo})

	err := svc.ConfirmQRCode(context.Background(), ConfirmQRCodeCommand{
		Code:                  testQRCode,
		ScannerUsername:       "someone-else",
		ScannerOrganizationID: testOrgID,
		Approved:              true,
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeForbidden, apperrors.FromError(err).Code)

	err = svc.ConfirmQRCode(context.Background(), ConfirmQRCodeCommand{
		Code:                  testQRCode,
		ScannerUsername:       testUsername,
		ScannerOrganizationID: testOrgID,
		Approved:              true,
	})
	require.NoError(t, err)
}

func TestExchangeQRCode_IssuesInitiatorSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	confirmed := newQRLoginSession(func(session *qrcodedomain.Session) {
		session.Status = "confirmed"
		session.UserID = user.ID
		session.Username = user.Username
	})
	qrRepo := qrmock.NewMockRepository(ctrl)
	qrRepo.EXPECT().GetByCodeID(gomock.Any(), confirmed.CodeID).Return(confirmed, nil)
	qrRepo.EXPECT().DeleteByCodeID(gomock.Any(), confirmed.CodeID).Return(nil)
	// exchange 不再签发一次性 access token：会话创建后走 handleLoggedIn（与
	// /faceid/complete 同一条路径），控制台路径的产物是 LoginResult。
	sessRepo := &loginTestSessionRepository{}

	svc := newTestService(loginTestUnitOfWork{
		orgRepo:  newQRTestOrgRepository(org),
		userRepo: newQRTestUserRepository(user),
		appRepo:  &loginTestAppRepository{apps: map[string]*applications.Application{app.ID: app}},
		qrRepo:   qrRepo,
		sessRepo: sessRepo,
	})

	result, sessionID, err := svc.ExchangeQRCode(context.Background(), ExchangeQRCodeCommand{
		CodeID:            confirmed.CodeID,
		PreviousSessionID: "old-session",
		IPAddress:         "203.0.113.7",
		UserAgent:         "test-agent",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, sessionID, "匿名 PC 的登录态必须在 exchange 签发")
	assert.Equal(t, sessionID, result.SessionID)
	assert.Equal(t, "success", result.Response.Status)

	issued := sessRepo.sessions[sessionID]
	require.NotNil(t, issued)
	assert.Equal(t, user.ID, issued.UserID)
	assert.Equal(t, "qrcode", issued.DeviceInfo["loginMethod"])
	assert.Equal(t, []string{"old-session"}, sessRepo.revoked)
	assert.Equal(t, "203.0.113.7", issued.IPAddress, "会话元数据由服务端解析后落库")
	assert.Equal(t, "test-agent", issued.UserAgent)
	require.NotNil(t, user.LastSigninAt, "扫码登录同样要记录 signin 统计")
	assert.Equal(t, "203.0.113.7", user.LastSigninIP)
}

func TestExchangeQRCode_AppliesOrganizationSessionPolicy(t *testing.T) {
	ctrl := gomock.NewController(t)
	// 组织策略为 reject_new 且已有一个活跃会话：扫码 exchange 必须失败
	// （此前只有密码登录看这条策略，扫码登录可以把并发上限顶穿）。
	org := newConcurrentPolicyOrg(1, "reject_new")
	user := newTestUser()
	app := newTestApp()
	confirmed := newQRLoginSession(func(session *qrcodedomain.Session) {
		session.Status = "confirmed"
		session.UserID = user.ID
		session.Username = user.Username
	})
	qrRepo := qrmock.NewMockRepository(ctrl)
	qrRepo.EXPECT().GetByCodeID(gomock.Any(), confirmed.CodeID).Return(confirmed, nil)
	qrRepo.EXPECT().DeleteByCodeID(gomock.Any(), confirmed.CodeID).Return(nil)

	existing := newPolicySession("019ses-existing", user.ID, "203.0.113.9", time.Now().Add(-time.Hour))
	sessRepo := &loginTestSessionRepository{
		sessions: map[string]*sessiondomain.Session{existing.ID: existing},
	}

	svc := newTestService(loginTestUnitOfWork{
		orgRepo:  newQRTestOrgRepository(org),
		userRepo: newQRTestUserRepository(user),
		appRepo:  &loginTestAppRepository{apps: map[string]*applications.Application{app.ID: app}},
		qrRepo:   qrRepo,
		sessRepo: sessRepo,
	})

	result, sessionID, err := svc.ExchangeQRCode(context.Background(), ExchangeQRCodeCommand{CodeID: confirmed.CodeID})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
	assert.Nil(t, result)
	assert.Empty(t, sessionID)
}
