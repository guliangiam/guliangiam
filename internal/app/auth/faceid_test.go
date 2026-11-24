package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	applications "github.com/guliangiam/guliangiam/internal/domain/applications"
	faceidsessions "github.com/guliangiam/guliangiam/internal/domain/faceidsessions"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Face ID 登录的服务层用例：complete 必须签发会话、落服务端解析的会话元数据、执行
// 与密码登录同一套租户会话策略，并在带 responseType 却缺 applicationId 时拦成 422
// （此前它恒以 nil application 调 handleLoggedIn，OIDC 授权码流程会 panic 成 500）。

// loginTestFaceIDSessionRepository 是 Face ID 会话仓储的内存实现。
type loginTestFaceIDSessionRepository struct {
	sessions map[string]*faceidsessions.Session
	consumed []string
}

func (r *loginTestFaceIDSessionRepository) GetBySessionID(_ context.Context, sessionID string) (*faceidsessions.Session, error) {
	session, ok := r.sessions[sessionID]
	if !ok {
		return nil, apperrors.NewNotFound("face id session not found")
	}
	return session, nil
}

func (r *loginTestFaceIDSessionRepository) Create(_ context.Context, session *faceidsessions.Session) error {
	if r.sessions == nil {
		r.sessions = map[string]*faceidsessions.Session{}
	}
	r.sessions[session.SessionID] = session
	return nil
}

func (r *loginTestFaceIDSessionRepository) Update(ctx context.Context, session *faceidsessions.Session) error {
	return r.Create(ctx, session)
}

func (r *loginTestFaceIDSessionRepository) ConsumeVerified(_ context.Context, sessionID string) error {
	if _, ok := r.sessions[sessionID]; !ok {
		return apperrors.NewNotFound("face id session not found")
	}
	r.consumed = append(r.consumed, sessionID)
	return nil
}

func (r *loginTestFaceIDSessionRepository) DeleteBySessionID(_ context.Context, sessionID string) error {
	delete(r.sessions, sessionID)
	return nil
}

func (r *loginTestFaceIDSessionRepository) DeleteExpired(context.Context) (int64, error) {
	return 0, nil
}

// newVerifiedFaceIDSession 放入一条已验证的 Face ID 会话并返回 sessionId。
func newVerifiedFaceIDSession(repo *loginTestFaceIDSessionRepository, userID string) string {
	const sessionID = "faceid-session-001"
	repo.sessions = map[string]*faceidsessions.Session{
		sessionID: {
			ID:         "019fid-0000-0000-0000-000000000001",
			SessionID:  sessionID,
			UserID:     userID,
			Challenge:  "challenge",
			Verified:   true,
			LivenessOK: true,
			ExpiresAt:  time.Now().Add(5 * time.Minute),
		},
	}
	return sessionID
}

func newFaceIDTestService(
	user *userdomain.User,
	org *orgdomain.Organization,
	app *applications.Application,
	sessRepo *loginTestSessionRepository,
	faceidRepo *loginTestFaceIDSessionRepository,
) Service {
	uow := loginTestUnitOfWork{
		orgRepo:    newQRTestOrgRepository(org),
		userRepo:   newQRTestUserRepository(user),
		sessRepo:   sessRepo,
		faceidRepo: faceidRepo,
	}
	if app != nil {
		uow.appRepo = &loginTestAppRepository{apps: map[string]*applications.Application{app.ID: app}}
	}
	return newTestService(uow)
}

func TestCompleteFaceIDLogin_IssuesSessionWithApplicationContext(t *testing.T) {
	user := newTestUser(func(u *userdomain.User) { u.IsVerified = true })
	org := newTestOrg()
	app := newTestApp()
	faceidRepo := &loginTestFaceIDSessionRepository{}
	sessionID := newVerifiedFaceIDSession(faceidRepo, user.ID)
	sessRepo := &loginTestSessionRepository{}
	svc := newFaceIDTestService(user, org, app, sessRepo, faceidRepo)

	result, err := svc.CompleteFaceIDLogin(context.Background(), CompleteFaceIDLoginCommand{
		SessionID:     sessionID,
		ApplicationID: app.ID,
		IPAddress:     "203.0.113.7",
		UserAgent:     "test-agent",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "success", result.Response.Status)
	assert.Equal(t, []string{sessionID}, faceidRepo.consumed, "会话一次性消费")

	issued := sessRepo.sessions[result.SessionID]
	require.NotNil(t, issued)
	assert.Equal(t, "faceid", issued.DeviceInfo["loginMethod"])
	assert.Equal(t, app.ID, issued.ApplicationID, "协议上下文的目标应用要绑进会话")
	assert.Equal(t, "203.0.113.7", issued.IPAddress)
	assert.Equal(t, "test-agent", issued.UserAgent)

	require.NotNil(t, user.LastSigninAt, "服务端登录路径同样要记录 signin 统计")
	assert.Equal(t, "203.0.113.7", user.LastSigninIP)
}

func TestCompleteFaceIDLogin_RequiresApplicationForProtocolContext(t *testing.T) {
	user := newTestUser(func(u *userdomain.User) { u.IsVerified = true })
	org := newTestOrg()
	faceidRepo := &loginTestFaceIDSessionRepository{}
	sessionID := newVerifiedFaceIDSession(faceidRepo, user.ID)
	svc := newFaceIDTestService(user, org, nil, &loginTestSessionRepository{}, faceidRepo)

	// 带 responseType 却不带 applicationId：必须是 422 而不是 nil application panic。
	_, err := svc.CompleteFaceIDLogin(context.Background(), CompleteFaceIDLoginCommand{
		SessionID:    sessionID,
		ResponseType: "code",
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
}

func TestCompleteFaceIDLogin_AppliesOrganizationSessionPolicy(t *testing.T) {
	user := newTestUser(func(u *userdomain.User) { u.IsVerified = true })
	org := newConcurrentPolicyOrg(1, "reject_new")
	app := newTestApp()
	faceidRepo := &loginTestFaceIDSessionRepository{}
	sessionID := newVerifiedFaceIDSession(faceidRepo, user.ID)

	existing := newPolicySession("019ses-existing", user.ID, "203.0.113.9", time.Now().Add(-time.Hour))
	sessRepo := &loginTestSessionRepository{
		sessions: map[string]*sessiondomain.Session{existing.ID: existing},
	}
	svc := newFaceIDTestService(user, org, app, sessRepo, faceidRepo)

	_, err := svc.CompleteFaceIDLogin(context.Background(), CompleteFaceIDLoginCommand{
		SessionID:     sessionID,
		ApplicationID: app.ID,
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
}

// ──────────────────────────────────────────────
// InitiateFaceID 测试
// ──────────────────────────────────────────────

func TestInitiateFaceID_Success_WhenRealNameVerifiedAndTemplateRegistered(t *testing.T) {
	// Face ID 是主登录凭证：可用性 = 已实名 + 有效底图，不再依赖 MfaMethods[faceid]。
	// 这里不设置任何 MFA method，仅靠实名 + 底图（legacy JSONB 回退）即可发起。
	user := newTestUser(func(u *userdomain.User) {
		u.IsVerified = true
		u.MfaMethods = nil
		u.FaceIds = []userdomain.FaceId{
			{ImageUrl: "https://cdn.example.com/face-template.bin"},
		}
	})
	org := newTestOrg()
	faceidRepo := &loginTestFaceIDSessionRepository{}
	svc := newFaceIDTestService(user, org, nil, &loginTestSessionRepository{}, faceidRepo)

	result, err := svc.InitiateFaceID(context.Background(), InitiateFaceIDCommand{
		OrganizationSlug: org.Slug,
		Username:         user.Username,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.SessionID)
	assert.NotEmpty(t, result.Challenge)
	assert.Equal(t, 2*time.Minute, time.Duration(result.ExpiresIn)*time.Second)

	// repo 落库了新 session
	created, ok := faceidRepo.sessions[result.SessionID]
	require.True(t, ok, "session should be persisted")
	assert.Equal(t, user.ID, created.UserID)
	assert.Equal(t, 0, created.AttemptCount, "new session starts with 0 attempts")
}

func TestInitiateFaceID_Rejects_WhenUserHasNoFaceTemplate(t *testing.T) {
	user := newTestUser(func(u *userdomain.User) {
		u.IsVerified = true
		// 已实名但底图未注册 — 必须在 Initiate 阶段拒绝，而不是延迟到 Verify。
		u.FaceIds = nil
	})
	org := newTestOrg()
	faceidRepo := &loginTestFaceIDSessionRepository{}
	svc := newFaceIDTestService(user, org, nil, &loginTestSessionRepository{}, faceidRepo)

	_, err := svc.InitiateFaceID(context.Background(), InitiateFaceIDCommand{
		OrganizationSlug: org.Slug,
		Username:         user.Username,
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnauthorized, apperrors.FromError(err).Code,
		"无 template 必须返回 401 而不是让错误延迟到 Verify 阶段")
}

func TestInitiateFaceID_Rejects_WhenNotRealNameVerified(t *testing.T) {
	// 有底图但未实名：实名门槛先于底图门槛拒绝（设计 3.2 已实名前置）。
	user := newTestUser(func(u *userdomain.User) {
		u.IsVerified = false
		u.FaceIds = []userdomain.FaceId{
			{ImageUrl: "https://cdn.example.com/face-template.bin"},
		}
	})
	org := newTestOrg()
	faceidRepo := &loginTestFaceIDSessionRepository{}
	svc := newFaceIDTestService(user, org, nil, &loginTestSessionRepository{}, faceidRepo)

	_, err := svc.InitiateFaceID(context.Background(), InitiateFaceIDCommand{
		OrganizationSlug: org.Slug,
		Username:         user.Username,
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnauthorized, apperrors.FromError(err).Code)
	assert.Empty(t, faceidRepo.sessions, "未实名不得创建 faceid 会话")
}
