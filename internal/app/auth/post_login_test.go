package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applications "github.com/guliangiam/guliangiam/internal/domain/applications"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// post_login.go 收口点的用例：排他登录、并发上限的两种动作、signin 统计。
// 这套策略此前只作用于密码登录（LDAP/社交/Kerberos 各抄一半、三条服务端登录路径完全没有），
// 现在七条登录路径共用同一函数，因此行为边界在这一层锁死。

func newPolicySession(id, userID, ip string, createdAt time.Time) *sessiondomain.Session {
	return &sessiondomain.Session{
		ID:             id,
		UserID:         userID,
		Username:       testUsername,
		OrganizationID: testOrgID,
		SessionState:   "active",
		IPAddress:      ip,
		ExpiresAt:      createdAt.Add(30 * 24 * time.Hour),
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
		Revision:       1,
	}
}

func newPolicyTestService(user *userdomain.User, org *orgdomain.Organization, sessRepo *loginTestSessionRepository) Service {
	return newTestService(loginTestUnitOfWork{
		orgRepo:  newQRTestOrgRepository(org),
		userRepo: newQRTestUserRepository(user),
		sessRepo: sessRepo,
	})
}

func newConcurrentPolicyOrg(maxSessions int, action string) *orgdomain.Organization {
	return newTestOrg(func(org *orgdomain.Organization) {
		org.Security.ConcurrentSessionPolicy = map[string]any{
			"maxSessions": maxSessions,
			"action":      action,
		}
	})
}

func TestApplyLoginSessionPolicies_ExclusiveSigninRevokesSiblings(t *testing.T) {
	user := newTestUser()
	org := newTestOrg()
	app := newTestApp(func(application *applications.Application) {
		application.EnableExclusiveSignin = true
	})
	sessRepo := &loginTestSessionRepository{}
	svc := newPolicyTestService(user, org, sessRepo)
	now := svc.Clock.Now()
	fresh := newPolicySession(testSessID, user.ID, "203.0.113.7", now)

	require.NoError(t, svc.applyLoginSessionPolicies(context.Background(), svc.UOW.(appuow.UnitOfWork), user, app, org.ID, fresh, now))

	require.Len(t, sessRepo.revokedByUserAndApp, 1, "排他登录必须撤销同一 user+app 的其它会话")
	assert.Equal(t, [3]string{user.ID, app.ID, fresh.ID}, sessRepo.revokedByUserAndApp[0])
}

func TestApplyLoginSessionPolicies_SkipsExclusiveSigninWithoutApplication(t *testing.T) {
	user := newTestUser()
	org := newTestOrg()
	sessRepo := &loginTestSessionRepository{}
	svc := newPolicyTestService(user, org, sessRepo)
	now := svc.Clock.Now()
	fresh := newPolicySession(testSessID, user.ID, "203.0.113.7", now)

	// 没有应用维度的登录路径（如 Face ID 未带 applicationId）不应触发应用级排他登录。
	require.NoError(t, svc.applyLoginSessionPolicies(context.Background(), svc.UOW.(appuow.UnitOfWork), user, nil, org.ID, fresh, now))
	assert.Empty(t, sessRepo.revokedByUserAndApp)
}

func TestApplyLoginSessionPolicies_RejectNewRefusesLogin(t *testing.T) {
	user := newTestUser()
	org := newConcurrentPolicyOrg(1, "reject_new")
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	existing := newPolicySession("019ses-existing", user.ID, "203.0.113.9", now.Add(-time.Hour))
	sessRepo := &loginTestSessionRepository{
		sessions: map[string]*sessiondomain.Session{existing.ID: existing},
	}
	svc := newPolicyTestService(user, org, sessRepo)
	fresh := newPolicySession(testSessID, user.ID, "203.0.113.7", now)

	err := svc.applyLoginSessionPolicies(context.Background(), svc.UOW.(appuow.UnitOfWork), user, nil, org.ID, fresh, now)
	require.Error(t, err, "策略为 reject_new 时新登录必须失败")
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
	assert.Empty(t, sessRepo.revoked, "拒绝新会话时不得顺手撤销旧会话")
}

func TestApplyLoginSessionPolicies_RevokeOldestDropsTheOldestSession(t *testing.T) {
	user := newTestUser()
	org := newConcurrentPolicyOrg(2, "revoke_oldest")
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	older := newPolicySession("019ses-old", user.ID, "203.0.113.9", now.Add(-2*time.Hour))
	newer := newPolicySession("019ses-new", user.ID, "203.0.113.9", now.Add(-time.Hour))
	sessRepo := &loginTestSessionRepository{
		sessions: map[string]*sessiondomain.Session{older.ID: older, newer.ID: newer},
	}
	svc := newPolicyTestService(user, org, sessRepo)
	fresh := newPolicySession(testSessID, user.ID, "203.0.113.7", now)

	require.NoError(t, svc.applyLoginSessionPolicies(context.Background(), svc.UOW.(appuow.UnitOfWork), user, nil, org.ID, fresh, now))
	assert.Equal(t, []string{older.ID}, sessRepo.revoked, "revoke_oldest 只淘汰最旧的会话")
}

func TestApplyLoginSessionPolicies_LocksTheUserBeforeCounting(t *testing.T) {
	user := newTestUser()
	org := newConcurrentPolicyOrg(2, "revoke_oldest")
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	log := &loginTestCallLog{}
	userRepo := newQRTestUserRepository(user)
	userRepo.callLog = log
	sessRepo := &loginTestSessionRepository{callLog: log}
	svc := newTestService(loginTestUnitOfWork{
		orgRepo:  newQRTestOrgRepository(org),
		userRepo: userRepo,
		sessRepo: sessRepo,
	})
	fresh := newPolicySession(testSessID, user.ID, "203.0.113.7", now)

	require.NoError(t, svc.applyLoginSessionPolicies(context.Background(), svc.UOW.(appuow.UnitOfWork), user, nil, org.ID, fresh, now))

	// 顺序即语义：先锁用户行（并发登录的互斥锚点），再读会话数并淘汰。
	// 反过来（先读后锁）两个并发登录都会读到「未超限」而同时放行。
	assert.Equal(t, []string{"users.LockByID", "sessions.List"}, log.calls)
	assert.Equal(t, []string{user.ID}, userRepo.locked)
}

func TestApplyLoginSessionPolicies_DoesNotLockWhenUnlimited(t *testing.T) {
	user := newTestUser()
	org := newTestOrg() // 缺省策略：不限
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	log := &loginTestCallLog{}
	userRepo := newQRTestUserRepository(user)
	userRepo.callLog = log
	sessRepo := &loginTestSessionRepository{callLog: log}
	svc := newTestService(loginTestUnitOfWork{
		orgRepo:  newQRTestOrgRepository(org),
		userRepo: userRepo,
		sessRepo: sessRepo,
	})
	fresh := newPolicySession(testSessID, user.ID, "203.0.113.7", now)

	require.NoError(t, svc.applyLoginSessionPolicies(context.Background(), svc.UOW.(appuow.UnitOfWork), user, nil, org.ID, fresh, now))

	// 不限策略是绝大多数租户的配置：不该为此给每次登录多打一把行锁、多读一次会话数。
	assert.Empty(t, userRepo.locked)
	assert.Empty(t, log.calls)
}

func TestApplyLoginSessionPolicies_SinglePolicyKeepsOnlyNewest(t *testing.T) {
	user := newTestUser()
	org := newTestOrg(func(org *orgdomain.Organization) {
		org.Security.ConcurrentSessionPolicy = map[string]any{"concurrentSessionPolicy": "single"}
	})
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	older := newPolicySession("019ses-old", user.ID, "203.0.113.9", now.Add(-2*time.Hour))
	newer := newPolicySession("019ses-new", user.ID, "203.0.113.9", now.Add(-time.Hour))
	sessRepo := &loginTestSessionRepository{
		sessions: map[string]*sessiondomain.Session{older.ID: older, newer.ID: newer},
	}
	svc := newPolicyTestService(user, org, sessRepo)
	fresh := newPolicySession(testSessID, user.ID, "203.0.113.7", now)

	// single = 只保留最新一个会话：既有的两个都让位给本次登录。
	require.NoError(t, svc.applyLoginSessionPolicies(context.Background(), svc.UOW.(appuow.UnitOfWork), user, nil, org.ID, fresh, now))
	assert.Equal(t, []string{older.ID, newer.ID}, sessRepo.revoked)
}

func TestApplyLoginSessionPolicies_RequireMFAFailsClosed(t *testing.T) {
	user := newTestUser()
	org := newTestOrg(func(org *orgdomain.Organization) {
		org.Security.ConcurrentSessionPolicy = map[string]any{
			"concurrentSessionPolicy": "limit",
			"maxActiveSessions":       1,
			"action":                  "require_mfa",
		}
	})
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	existing := newPolicySession("019ses-existing", user.ID, "203.0.113.9", now.Add(-time.Hour))
	sessRepo := &loginTestSessionRepository{
		sessions: map[string]*sessiondomain.Session{existing.ID: existing},
	}
	svc := newPolicyTestService(user, org, sessRepo)
	fresh := newPolicySession(testSessID, user.ID, "203.0.113.7", now)

	// require_mfa 尚未实现：必须 fail-closed 拒绝新会话，而不是静默按 revoke_oldest 放行。
	err := svc.applyLoginSessionPolicies(context.Background(), svc.UOW.(appuow.UnitOfWork), user, nil, org.ID, fresh, now)
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
	assert.Contains(t, apperrors.FromError(err).Message, "require_mfa")
	assert.Empty(t, sessRepo.revoked, "fail-closed 时不得顺手淘汰旧会话")
}

func TestApplyLoginSessionPolicies_StampsSigninStats(t *testing.T) {
	user := newTestUser(func(user *userdomain.User) {
		user.SigninWrongTimes = 3
		user.Revision = 7
	})
	org := newTestOrg()
	sessRepo := &loginTestSessionRepository{}
	svc := newPolicyTestService(user, org, sessRepo)
	now := svc.Clock.Now()
	fresh := newPolicySession(testSessID, user.ID, "203.0.113.7", now)

	require.NoError(t, svc.applyLoginSessionPolicies(context.Background(), svc.UOW.(appuow.UnitOfWork), user, nil, org.ID, fresh, now))

	assert.Equal(t, 0, user.SigninWrongTimes, "成功登录必须清零失败次数")
	require.NotNil(t, user.LastSigninAt)
	assert.True(t, user.LastSigninAt.Equal(now))
	assert.Equal(t, "203.0.113.7", user.LastSigninIP, "LastSigninIP 取自刚落库的会话")
	assert.Equal(t, int64(8), user.Revision)
}
