package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applications "github.com/guliangiam/guliangiam/internal/domain/applications"
	ldapdomain "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	providerdomain "github.com/guliangiam/guliangiam/internal/domain/providers"
	qrcodedomain "github.com/guliangiam/guliangiam/internal/domain/qrcodesessions"
	qrmock "github.com/guliangiam/guliangiam/internal/domain/qrcodesessions/mock"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"go.uber.org/mock/gomock"
)

// previousSessionID 是请求携带的旧会话（S9 轮换目标）。
const previousSessionID = "019prev-0000-7000-8000-000000000001"

// rotationTestLdapID 是 LDAP 登录用例使用的配置 ID。
const rotationTestLdapID = "ldap-provider-1"

// rotationTestPushProviderID 是推送 MFA 用例使用的通知提供商 ID。
const rotationTestPushProviderID = "push-provider-1"

// rotationTestSocialProviderID 是社交登录用例使用的提供商 ID。
const rotationTestSocialProviderID = "social-provider-1"

// recordingSessionRepository 记录 Revoke 调用，并允许注入撤销错误。
type recordingSessionRepository struct {
	*loginTestSessionRepository
	revokeErr      error
	revokeCalls    []string
	getByIDSession *sessiondomain.Session
}

func (r *recordingSessionRepository) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	r.revokeCalls = append(r.revokeCalls, id)
	if r.revokeErr != nil {
		return r.revokeErr
	}
	return r.loginTestSessionRepository.Revoke(ctx, id, revokedAt)
}

// getByIDSession 供需要先读取旧会话的入口（推送 MFA）使用。
func (r *recordingSessionRepository) GetByID(ctx context.Context, id string) (*sessiondomain.Session, error) {
	if r.getByIDSession != nil && r.getByIDSession.ID == id {
		return r.getByIDSession, nil
	}
	return r.loginTestSessionRepository.GetByID(ctx, id)
}

func (r *recordingSessionRepository) snapshot() map[string]*sessiondomain.Session {
	out := make(map[string]*sessiondomain.Session, len(r.loginTestSessionRepository.sessions))
	for id, sess := range r.loginTestSessionRepository.sessions {
		out[id] = sess
	}
	return out
}

func (r *recordingSessionRepository) restore(snapshot map[string]*sessiondomain.Session) {
	r.loginTestSessionRepository.sessions = snapshot
}

// staticLdapRepository 只服务 LDAP 登录路径的配置解析。
type staticLdapRepository struct {
	ldap *ldapdomain.Ldap
}

func (r *staticLdapRepository) GetByID(ctx context.Context, id string) (*ldapdomain.Ldap, error) {
	if r.ldap != nil && r.ldap.ID == id {
		return r.ldap, nil
	}
	return nil, apperrors.NewNotFound("ldap config not found")
}

func (r *staticLdapRepository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*ldapdomain.Ldap, error) {
	ldap, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ldap.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("ldap config not found")
	}
	return ldap, nil
}

func (r *staticLdapRepository) List(ctx context.Context, query ldapdomain.ListQuery) ([]*ldapdomain.Ldap, int64, error) {
	if r.ldap == nil {
		return nil, 0, nil
	}
	return []*ldapdomain.Ldap{r.ldap}, 1, nil
}

func (r *staticLdapRepository) Create(ctx context.Context, ldap *ldapdomain.Ldap) error { return nil }
func (r *staticLdapRepository) Update(ctx context.Context, ldap *ldapdomain.Ldap) error { return nil }
func (r *staticLdapRepository) Delete(ctx context.Context, id, organizationID string) error {
	return nil
}

// rotationTestProviderRepository 只服务需要读取提供商配置的入口。
type rotationTestProviderRepository struct {
	providerdomain.Repository
	provider *providerdomain.Provider
}

func (r rotationTestProviderRepository) GetByID(ctx context.Context, id string) (*providerdomain.Provider, error) {
	if r.provider == nil {
		return nil, apperrors.NewNotFound("provider not found")
	}
	return r.provider, nil
}

// rotationTestUnitOfWork 复刻真实事务的回滚语义：闭包返回错误时会话表回滚到
// 调用前的快照。这样测试才能证伪「失败仍留下新会话」。
type rotationTestUnitOfWork struct {
	loginTestUnitOfWork
	sessRepo     *recordingSessionRepository
	ldapRepo     ldapdomain.Repository
	providerRepo providerdomain.Repository
}

func (u rotationTestUnitOfWork) Sessions() sessiondomain.Repository { return u.sessRepo }

func (u rotationTestUnitOfWork) Ldaps() ldapdomain.Repository { return u.ldapRepo }

func (u rotationTestUnitOfWork) Providers() providerdomain.Repository { return u.providerRepo }

func (u rotationTestUnitOfWork) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	before := u.sessRepo.snapshot()
	if err := fn(ctx, u); err != nil {
		u.sessRepo.restore(before)
		return err
	}
	return nil
}

func newRotationTestService(uow rotationTestUnitOfWork) Service {
	return Service{
		Clock:                  fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator:          fakeUUIDGenerator{fixedID: testSessID},
		UOW:                    uow,
		WebAuthnAllowedOrigins: []string{"https://login.example.com"},
	}
}

func newRotationTestUow(t *testing.T, revokeErr error) (rotationTestUnitOfWork, *recordingSessionRepository) {
	t.Helper()

	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	base := newTestUow(org, user, app)

	repo := &recordingSessionRepository{
		loginTestSessionRepository: base.sessRepo,
		revokeErr:                  revokeErr,
	}

	base.sessRepo = nil
	return rotationTestUnitOfWork{loginTestUnitOfWork: base, sessRepo: repo}, repo
}

// TestLogin_RevokePreviousSessionFails_RollsBackRotation 钉住主登录入口：
// 旧会话撤销失败时整笔事务失败、不签发新会话。
func TestLogin_RevokePreviousSessionFails_RollsBackRotation(t *testing.T) {
	uow, repo := newRotationTestUow(t, errors.New("connection reset by peer"))
	svc := newRotationTestService(uow)

	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug:  testOrgSlug,
		Username:          testUsername,
		Password:          testPassword,
		ApplicationID:     testAppID,
		PreviousSessionID: previousSessionID,
	})

	if err == nil {
		t.Fatalf("Login() error = nil, want failure when revoking the previous session fails")
	}
	if result != nil {
		t.Fatalf("Login() result = %+v, want nil", result)
	}
	if len(repo.revokeCalls) != 1 || repo.revokeCalls[0] != previousSessionID {
		t.Fatalf("Revoke calls = %v, want [%s]", repo.revokeCalls, previousSessionID)
	}
	if _, ok := repo.sessions[testSessID]; ok {
		t.Fatalf("new session %q persisted despite rollback", testSessID)
	}
	if len(repo.sessions) != 0 {
		t.Fatalf("sessions = %d, want 0 after rollback", len(repo.sessions))
	}
}

// TestLoginWithLDAP_RevokePreviousSessionFails_RollsBackRotation 钉住 LDAP 入口。
func TestLoginWithLDAP_RevokePreviousSessionFails_RollsBackRotation(t *testing.T) {
	originalBind := ldapBindAndSearchFunc
	ldapBindAndSearchFunc = func(ctx context.Context, host string, port int, bindDN, bindPassword, baseDN string, useTLS bool, username, password, filter, attributeName string) (*LdapUserInfo, error) {
		return &LdapUserInfo{UID: testUsername, CommonName: testUsername}, nil
	}
	t.Cleanup(func() { ldapBindAndSearchFunc = originalBind })

	uow, repo := newRotationTestUow(t, errors.New("connection reset by peer"))
	uow.ldapRepo = &staticLdapRepository{ldap: &ldapdomain.Ldap{
		ID:             rotationTestLdapID,
		OrganizationID: testOrgID,
		Host:           "ldap.example.com",
		Port:           389,
		BaseDN:         "dc=example,dc=com",
		IsEnabled:      true,
	}}
	svc := newRotationTestService(uow)

	result, err := svc.LoginWithLDAP(context.Background(), LdapLoginCommand{
		OrganizationSlug:  testOrgSlug,
		Username:          testUsername,
		Password:          "ldap-secret",
		LdapID:            rotationTestLdapID,
		PreviousSessionID: previousSessionID,
		ApplicationID:     testAppID,
	})

	if err == nil {
		t.Fatalf("LoginWithLDAP() error = nil, want failure when revoking the previous session fails")
	}
	if result != nil {
		t.Fatalf("LoginWithLDAP() result = %+v, want nil", result)
	}
	if len(repo.revokeCalls) != 1 || repo.revokeCalls[0] != previousSessionID {
		t.Fatalf("Revoke calls = %v, want [%s]", repo.revokeCalls, previousSessionID)
	}
	if len(repo.sessions) != 0 {
		t.Fatalf("sessions = %d, want 0 after rollback", len(repo.sessions))
	}
}

// TestKerberosLogin_RevokePreviousSessionFails_RollsBackRotation 钉住 Kerberos 入口。
func TestKerberosLogin_RevokePreviousSessionFails_RollsBackRotation(t *testing.T) {
	originalValidate := validateSPNEGOToken
	validateSPNEGOToken = func(authHeader string) (string, string, error) {
		return testUsername, "EXAMPLE.COM", nil
	}
	t.Cleanup(func() { validateSPNEGOToken = originalValidate })

	uow, repo := newRotationTestUow(t, errors.New("connection reset by peer"))
	svc := newRotationTestService(uow)

	result, err := svc.KerberosLogin(context.Background(), KerberosLoginCommand{
		OrganizationSlug:  testOrgSlug,
		SPNEGOToken:       "spnego-token",
		PreviousSessionID: previousSessionID,
	})

	if err == nil {
		t.Fatalf("KerberosLogin() error = nil, want failure when revoking the previous session fails")
	}
	if result != nil {
		t.Fatalf("KerberosLogin() result = %+v, want nil", result)
	}
	if len(repo.revokeCalls) != 1 || repo.revokeCalls[0] != previousSessionID {
		t.Fatalf("Revoke calls = %v, want [%s]", repo.revokeCalls, previousSessionID)
	}
	if len(repo.sessions) != 0 {
		t.Fatalf("sessions = %d, want 0 after rollback", len(repo.sessions))
	}
}

// TestLogin_RevokePreviousSessionSucceeds_KeepsRotationBehaviour 成功路径不回归：
// 撤销成功时登录照常完成，新会话落库，旧会话被撤销。
func TestLogin_RevokePreviousSessionSucceeds_KeepsRotationBehaviour(t *testing.T) {
	uow, repo := newRotationTestUow(t, nil)
	svc := newRotationTestService(uow)

	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug:  testOrgSlug,
		Username:          testUsername,
		Password:          testPassword,
		ApplicationID:     testAppID,
		PreviousSessionID: previousSessionID,
	})

	if err != nil {
		t.Fatalf("Login() error = %v, want nil", err)
	}
	if result == nil || result.SessionID != testSessID {
		t.Fatalf("Login() result = %+v, want session %q", result, testSessID)
	}
	if _, ok := repo.sessions[testSessID]; !ok {
		t.Fatalf("new session %q not persisted", testSessID)
	}
	if len(repo.revoked) != 1 || repo.revoked[0] != previousSessionID {
		t.Fatalf("revoked = %v, want [%s]", repo.revoked, previousSessionID)
	}
}

// TestLogin_RevokePreviousSessionIgnorableErrors_Tolerated 钉住边界：旧会话
// 不存在（NotFound）或已不可用（Unauthorized）时登录照常成功。
func TestLogin_RevokePreviousSessionIgnorableErrors_Tolerated(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{name: "not found", err: apperrors.NewNotFound("session not found")},
		{name: "unauthorized", err: apperrors.NewUnauthorized("session already invalid")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uow, repo := newRotationTestUow(t, tc.err)
			svc := newRotationTestService(uow)

			result, err := svc.Login(context.Background(), LoginCommand{
				OrganizationSlug:  testOrgSlug,
				Username:          testUsername,
				Password:          testPassword,
				ApplicationID:     testAppID,
				PreviousSessionID: previousSessionID,
			})

			if err != nil {
				t.Fatalf("Login() error = %v, want nil for ignorable revoke error", err)
			}
			if result == nil || result.SessionID != testSessID {
				t.Fatalf("Login() result = %+v, want session %q", result, testSessID)
			}
			if _, ok := repo.sessions[testSessID]; !ok {
				t.Fatalf("new session %q not persisted", testSessID)
			}
			if len(repo.revokeCalls) != 1 {
				t.Fatalf("Revoke calls = %v, want exactly one attempt", repo.revokeCalls)
			}
		})
	}
}

// TestRevokePreviousSessionForRotation_EmptyID_SkipsRepo 空 ID 不应触碰仓储。
func TestRevokePreviousSessionForRotation_EmptyID_SkipsRepo(t *testing.T) {
	repo := &recordingSessionRepository{
		loginTestSessionRepository: &loginTestSessionRepository{},
		revokeErr:                  errors.New("must not be called"),
	}

	if err := revokePreviousSessionForRotation(context.Background(), repo, "   ", time.Now()); err != nil {
		t.Fatalf("revokePreviousSessionForRotation() error = %v, want nil", err)
	}
	if len(repo.revokeCalls) != 0 {
		t.Fatalf("Revoke calls = %v, want none", repo.revokeCalls)
	}
}

// TestExchangeQRCode_RevokePreviousSessionFails_RollsBackRotation 钉住扫码入口。
func TestExchangeQRCode_RevokePreviousSessionFails_RollsBackRotation(t *testing.T) {
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

	repo := &recordingSessionRepository{
		loginTestSessionRepository: &loginTestSessionRepository{},
		revokeErr:                  errors.New("connection reset by peer"),
	}
	uow := rotationTestUnitOfWork{
		loginTestUnitOfWork: loginTestUnitOfWork{
			orgRepo:  newQRTestOrgRepository(org),
			userRepo: newQRTestUserRepository(user),
			appRepo:  &loginTestAppRepository{apps: map[string]*applications.Application{app.ID: app}},
			qrRepo:   qrRepo,
		},
		sessRepo: repo,
	}
	svc := newRotationTestService(uow)

	result, sessionID, err := svc.ExchangeQRCode(context.Background(), ExchangeQRCodeCommand{
		CodeID:            confirmed.CodeID,
		PreviousSessionID: previousSessionID,
	})

	if err == nil {
		t.Fatalf("ExchangeQRCode() error = nil, want failure when revoking the previous session fails")
	}
	if result != nil {
		t.Fatalf("ExchangeQRCode() result = %+v, want nil", result)
	}
	if sessionID != "" {
		t.Fatalf("ExchangeQRCode() sessionID = %q, want empty after rollback", sessionID)
	}
	if len(repo.revokeCalls) != 1 || repo.revokeCalls[0] != previousSessionID {
		t.Fatalf("Revoke calls = %v, want [%s]", repo.revokeCalls, previousSessionID)
	}
	if len(repo.sessions) != 0 {
		t.Fatalf("sessions = %d, want 0 after rollback", len(repo.sessions))
	}
}

// TestLoginWithProvider_RevokePreviousSessionFails_RollsBackRotation 钉住社交登录入口。
func TestLoginWithProvider_RevokePreviousSessionFails_RollsBackRotation(t *testing.T) {
	originalExchange := exchangeSocialCode
	exchangeSocialCode = func(context.Context, SocialProviderConfig, string) (*SocialUserInfo, error) {
		return &SocialUserInfo{ID: "subject-1", Username: testUsername, Email: "testuser@example.com"}, nil
	}
	t.Cleanup(func() { exchangeSocialCode = originalExchange })

	uow, repo := newRotationTestUow(t, errors.New("connection reset by peer"))
	uow.providerRepo = rotationTestProviderRepository{provider: &providerdomain.Provider{
		ID:             rotationTestSocialProviderID,
		OrganizationID: testOrgID,
		Name:           "corp-github",
		Category:       providerdomain.CategoryOAuth,
		ProviderType:   providerdomain.ProviderGitHub,
		Enabled:        true,
	}}
	svc := newRotationTestService(uow)

	result, err := svc.LoginWithProvider(context.Background(), SocialLoginCommand{
		OrganizationSlug:  testOrgSlug,
		ProviderID:        rotationTestSocialProviderID,
		Code:              "authorization-code",
		ApplicationID:     testAppID,
		PreviousSessionID: previousSessionID,
	})

	if err == nil {
		t.Fatalf("LoginWithProvider() error = nil, want failure when revoking the previous session fails")
	}
	if result != nil {
		t.Fatalf("LoginWithProvider() result = %+v, want nil", result)
	}
	if len(repo.revokeCalls) != 1 || repo.revokeCalls[0] != previousSessionID {
		t.Fatalf("Revoke calls = %v, want [%s]", repo.revokeCalls, previousSessionID)
	}
	if len(repo.sessions) != 0 {
		t.Fatalf("sessions = %d, want 0 after rollback", len(repo.sessions))
	}
}

// TestEnablePushMfa_RevokePreviousSessionFails_RollsBackRotation 钉住推送 MFA 入口。
func TestEnablePushMfa_RevokePreviousSessionFails_RollsBackRotation(t *testing.T) {
	uow, repo := newRotationTestUow(t, errors.New("connection reset by peer"))
	repo.getByIDSession = &sessiondomain.Session{
		ID:               previousSessionID,
		UserID:           testUserID,
		Username:         testUsername,
		OrganizationID:   testOrgID,
		OrganizationSlug: testOrgSlug,
		ApplicationID:    testAppID,
	}
	uow.providerRepo = rotationTestProviderRepository{provider: &providerdomain.Provider{
		ID:             rotationTestPushProviderID,
		OrganizationID: testOrgID,
		Name:           "corp-push",
		Category:       providerdomain.ProviderCategory("Notification"),
		Enabled:        true,
	}}
	svc := newRotationTestService(uow)

	newSessionID, err := svc.EnablePushMfa(context.Background(), EnablePushMfaCommand{
		UserID:            testUserID,
		MfaPushReceiver:   "push@example.com",
		MfaPushProvider:   rotationTestPushProviderID,
		PreviousSessionID: previousSessionID,
	})

	if err == nil {
		t.Fatalf("EnablePushMfa() error = nil, want failure when revoking the previous session fails")
	}
	if newSessionID != "" {
		t.Fatalf("EnablePushMfa() newSessionID = %q, want empty after rollback", newSessionID)
	}
	if len(repo.revokeCalls) != 1 || repo.revokeCalls[0] != previousSessionID {
		t.Fatalf("Revoke calls = %v, want [%s]", repo.revokeCalls, previousSessionID)
	}
	if _, ok := repo.sessions[testSessID]; ok {
		t.Fatalf("rotated session %q persisted despite rollback", testSessID)
	}
	if len(repo.sessions) != 0 {
		t.Fatalf("sessions = %d, want 0 after rollback", len(repo.sessions))
	}
}
