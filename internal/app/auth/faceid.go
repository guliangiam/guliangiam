package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	"github.com/guliangiam/guliangiam/internal/domain/faceidproviders"
	faceiddomain "github.com/guliangiam/guliangiam/internal/domain/faceidsessions"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	// faceIDSessionTTL 覆盖移动端弱网场景的目标态 TTL。
	// 原始 10min → 目标态 2min（faceid.md §附录 A TTL 收敛）。
	// 安全原则：人脸比对是高频活体检测，长 TTL 允许更长的会话劫持窗口。
	faceIDSessionTTL = 2 * time.Minute

	// faceIDMaxAttempts 业务级人脸比对尝试上限。
	// 超过后删除 session，防止暴力穷举（faceid.md §附录 A attempt_count 收敛）。
	faceIDMaxAttempts = 3
)

// InitiateFaceIDCommand contains parameters for initiating a Face ID session.
type InitiateFaceIDCommand struct {
	OrganizationSlug string
	Username         string
	ApplicationID    string
	IPAddress        string
}

// InitiateFaceIDResult contains the session information for the frontend.
type InitiateFaceIDResult struct {
	SessionID string `json:"sessionId"`
	Challenge string `json:"challenge"`
	ExpiresIn int64  `json:"expiresIn"`
}

// VerifyFaceIDCommand contains parameters for verifying a Face ID capture.
type VerifyFaceIDCommand struct {
	SessionID  string
	ImageData  string
	DeviceInfo string
}

// VerifyFaceIDResult contains the verification result.
type VerifyFaceIDResult struct {
	Verified       bool `json:"verified"`
	LivenessPassed bool `json:"livenessPassed"`
}

// CompleteFaceIDLoginCommand contains parameters for completing a Face ID login.
type CompleteFaceIDLoginCommand struct {
	SessionID         string
	PreviousSessionID string
	// ApplicationID 是协议上下文的目标应用（授权页登录时由控制台给出）。
	ApplicationID string
	ResponseType  string
	RedirectURI   string
	State         string
	Nonce         string
	Scope         []string

	// 会话元数据：由传输层从连接与请求头解析，不是客户端可自报的字段
	// （LastSigninIP 与异常登录检测都依赖它）。
	IPAddress string
	UserAgent string
}

var faceIdVerifyLivenessFunc faceIdVerifyLivenessFuncType = defaultVerifyLiveness

type faceIdVerifyLivenessFuncType func(ctx context.Context, providerType, accessKeyID, accessKeySecret, endpoint string, imageData string, deviceInfo string) (isLive bool, score float64, sessionID string, err error)

func defaultVerifyLiveness(_ context.Context, _, _, _, _, _, _ string) (bool, float64, string, error) {
	return false, 0, "", apperrors.NewUnimplemented("face ID liveness verification not configured: faceIdVerifyLivenessFunc not wired")
}

var faceIdCompareFaceFunc faceIdCompareFaceFuncType = defaultCompareFace

type faceIdCompareFaceFuncType func(ctx context.Context, providerType, accessKeyID, accessKeySecret, endpoint string, imageData string, storedTemplate string) (isMatch bool, confidence float64, err error)

func defaultCompareFace(_ context.Context, _, _, _, _, _, _ string) (bool, float64, error) {
	return false, 0, apperrors.NewUnimplemented("face ID comparison not configured: faceIdCompareFaceFunc not wired")
}

// SetFaceIdAdapters wires the Face ID adapter functions.
func SetFaceIdAdapters(verifyLivenessFn faceIdVerifyLivenessFuncType, compareFaceFn faceIdCompareFaceFuncType) {
	if verifyLivenessFn != nil {
		faceIdVerifyLivenessFunc = verifyLivenessFn
	}
	if compareFaceFn != nil {
		faceIdCompareFaceFunc = compareFaceFn
	}
}

// InitiateFaceID creates a new Face ID verification session persisted to the database.
func (s *Service) InitiateFaceID(ctx context.Context, command InitiateFaceIDCommand) (*InitiateFaceIDResult, error) {
	if strings.TrimSpace(command.OrganizationSlug) == "" {
		return nil, apperrors.NewInvalidArgument("organizationSlug is required")
	}
	if strings.TrimSpace(command.Username) == "" {
		return nil, apperrors.NewInvalidArgument("username is required")
	}

	organization, err := s.UOW.Organizations().GetBySlug(ctx, command.OrganizationSlug)
	if err != nil {
		return nil, err
	}

	user, err := s.UOW.Users().GetByUsername(ctx, organization.ID, command.Username)
	if err != nil {
		return nil, err
	}

	// Face ID 是主登录凭证（与密码/WebAuthn 同级），不是 MFA 二次因子：可用性 =
	// 已实名 + 已注册有效底图。注册闭环（user_face_templates）不再写 MfaMethods[faceid]，
	// 旧的 MfaMethods 寄生开关会让新注册用户无法登录，这里改为实名门槛 + 底图门槛。
	if !user.IsVerified {
		return nil, apperrors.NewUnauthorized("face id login requires real-name verification")
	}

	if _, err := s.getFaceTemplate(ctx, user); err != nil {
		return nil, apperrors.NewUnauthorized("face template not registered for this user")
	}

	sessionID, err := s.UUIDGenerator.NewString()
	if err != nil {
		return nil, apperrors.NewInternal("generate session id")
	}

	challengeBytes := make([]byte, 32)
	if _, err := rand.Read(challengeBytes); err != nil {
		return nil, apperrors.NewInternal("generate challenge")
	}
	challenge := base64.RawURLEncoding.EncodeToString(challengeBytes)

	now := s.Clock.Now()
	dbSessionID, err := s.UUIDGenerator.NewString()
	if err != nil {
		return nil, apperrors.NewInternal("generate db session id")
	}

	session := &faceiddomain.Session{
		ID:        dbSessionID,
		SessionID: sessionID,
		Challenge: challenge,
		UserID:    user.ID,
		ExpiresAt: now.Add(faceIDSessionTTL),
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.UOW.FaceIDSessions().Create(ctx, session); err != nil {
		return nil, err
	}

	return &InitiateFaceIDResult{
		SessionID: sessionID,
		Challenge: challenge,
		ExpiresIn: int64(faceIDSessionTTL / time.Second),
	}, nil
}

// VerifyFaceID verifies a face capture against the user's registered template.
func (s *Service) VerifyFaceID(ctx context.Context, command VerifyFaceIDCommand) (*VerifyFaceIDResult, error) {
	if strings.TrimSpace(command.SessionID) == "" {
		return nil, apperrors.NewInvalidArgument("sessionId is required")
	}
	if strings.TrimSpace(command.ImageData) == "" {
		return nil, apperrors.NewInvalidArgument("imageData is required")
	}

	session, err := s.UOW.FaceIDSessions().GetBySessionID(ctx, command.SessionID)
	if err != nil {
		return nil, apperrors.NewInvalidArgument("invalid or expired session")
	}

	now := s.Clock.Now()
	if now.After(session.ExpiresAt) {
		_ = s.UOW.FaceIDSessions().DeleteBySessionID(ctx, command.SessionID)
		return nil, apperrors.NewInvalidArgument("session has expired")
	}

	if session.Verified {
		return nil, apperrors.NewConflict("session already verified")
	}

	// attempt_count 前置闸门：超过上限直接删除 session，防止暴力穷举。
	// 返回 CodeAccountLocked（423 Locked）让前端区分"会话过期(400)"vs"尝试超限(423)"，
	// 引导用户重新发起 Initiate 流程（与密码登录的 account_locked 体验一致）。
	if session.AttemptCount >= faceIDMaxAttempts {
		_ = s.UOW.FaceIDSessions().DeleteBySessionID(ctx, command.SessionID)
		return nil, apperrors.NewAccountLocked("too many failed face verification attempts, please re-initiate")
	}

	user, err := s.UOW.Users().GetByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}

	provider, err := resolveFaceIdProvider(ctx, s.UOW, user.OrganizationID)
	if err != nil {
		return nil, apperrors.NewInternalf("face id provider not available: %v", err)
	}

	livenessOK, _, _, err := faceIdVerifyLivenessFunc(
		ctx,
		provider.ProviderType,
		provider.AccessKeyID,
		provider.AccessKeySecret,
		provider.Endpoint,
		command.ImageData,
		command.DeviceInfo,
	)
	if err != nil {
		// 保留适配器的错误码（unimplemented 必须原样拒绝，不能被包装成 500 后
		// 让调用方误判为可重试的瞬时故障）。
		return nil, err
	}

	if !livenessOK {
		session.AttemptCount++
		session.LivenessOK = false
		session.Verified = false
		session.UpdatedAt = now
		if err := s.UOW.FaceIDSessions().Update(ctx, session); err != nil {
			return nil, err
		}
		return &VerifyFaceIDResult{
			Verified:       false,
			LivenessPassed: false,
		}, nil
	}

	faceTemplate, err := s.getFaceTemplate(ctx, user)
	if err != nil {
		return nil, apperrors.NewInternal(err.Error())
	}

	matched, _, err := faceIdCompareFaceFunc(
		ctx,
		provider.ProviderType,
		provider.AccessKeyID,
		provider.AccessKeySecret,
		provider.Endpoint,
		command.ImageData,
		faceTemplate,
	)
	if err != nil {
		return nil, apperrors.NewInternalf("face comparison failed: %v", err)
	}

	// Face comparison 失败也递增 attempt_count；系统错误（err != nil）不递增。
	if !matched {
		session.AttemptCount++
	}
	session.Verified = matched
	session.LivenessOK = livenessOK
	session.UpdatedAt = now
	if err := s.UOW.FaceIDSessions().Update(ctx, session); err != nil {
		return nil, err
	}

	return &VerifyFaceIDResult{
		Verified:       matched,
		LivenessPassed: livenessOK,
	}, nil
}

// CompleteFaceIDLogin completes a Face ID login by issuing tokens.
func (s *Service) CompleteFaceIDLogin(ctx context.Context, command CompleteFaceIDLoginCommand) (*LoginResult, error) {
	if strings.TrimSpace(command.SessionID) == "" {
		return nil, apperrors.NewInvalidArgument("sessionId is required")
	}

	session, err := s.UOW.FaceIDSessions().GetBySessionID(ctx, command.SessionID)
	if err != nil {
		return nil, apperrors.NewInvalidArgument("invalid or expired session")
	}

	if !session.Verified {
		return nil, apperrors.NewUnauthorized("face id verification not completed")
	}

	// Consume the session before issuing anything. The delete is the gate: two
	// concurrent completions for the same session would otherwise both see
	// Verified=true and both mint a token pair.
	if err := s.UOW.FaceIDSessions().ConsumeVerified(ctx, command.SessionID); err != nil {
		return nil, err
	}

	user, err := s.UOW.Users().GetByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}

	// 实名门槛在签发边界再校验一次：Initiate 之后管理员可能撤销实名，
	// 已验证的 faceid 会话也不得给未实名用户签发 Token/Session。
	if !user.IsVerified {
		return nil, apperrors.NewUnauthorized("face id login requires real-name verification")
	}

	organization, err := s.UOW.Organizations().GetByID(ctx, user.OrganizationID)
	if err != nil {
		return nil, err
	}

	// 协议上下文的目标应用（与 /qrcode/exchange、WebAuthn finish 同义）：控制台在
	// 授权页发起 Face ID 登录时会带上 applicationId。此前这里恒为 nil 却照样把
	// responseType 交给 handleLoggedIn，OIDC 授权码流程会在 nil application 上取字段
	// panic（HTTP 500），现在缺应用直接拦成 422。
	var application *applicationdomain.Application
	if strings.TrimSpace(command.ApplicationID) != "" {
		application, err = s.UOW.Applications().GetByID(ctx, command.ApplicationID)
		if err != nil {
			return nil, err
		}
	}
	if application == nil && strings.TrimSpace(command.ResponseType) != "" {
		return nil, apperrors.NewInvalidArgument("applicationId is required when responseType is set")
	}

	loginCmd := LoginCommand{
		OrganizationSlug: organization.Slug,
		Username:         user.Username,
		ApplicationID:    command.ApplicationID,
		ResponseType:     command.ResponseType,
		RedirectURI:      command.RedirectURI,
		State:            command.State,
		Nonce:            command.Nonce,
		Scope:            command.Scope,
	}

	var result *LoginResult
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		now := s.Clock.Now()

		// ── Risk engine evaluation (faceid channel) ────────────────────
		// Face ID itself is a strong second factor — challenge is treated
		// as deny because there's no step-up beyond Face ID in this flow.
		uid := user.ID
		if evalErr := s.evaluateLoginRisk(ctx, tx, "faceid", organization.ID, &uid,
			command.IPAddress, command.UserAgent, "",
			map[string]any{"faceid_session": command.SessionID}); evalErr != nil {
			switch evalErr.(type) {
			case *RiskDeniedError, *RiskChallengeError:
				return apperrors.NewUnauthorized("access denied by risk assessment")
			default:
				return evalErr
			}
		}

		// ADR-029 pre_login (faceid channel): fail-closed policy gate
		// before the session row is committed.
		applicationID := ""
		if application != nil {
			applicationID = application.ID
		}
		if err := s.enforceActionsPreLogin(ctx, "faceid", organization.ID, user.ID, user.Username,
			applicationID, command.IPAddress, command.UserAgent); err != nil {
			return err
		}

		sessionID, err := s.UUIDGenerator.NewString()
		if err != nil {
			return apperrors.NewInternal("generate session id")
		}

		sid, err := newSessionSid()
		if err != nil {
			return err
		}
		sess := &sessiondomain.Session{
			ID:                  sessionID,
			Sid:                 sid,
			UserID:              user.ID,
			Username:            user.Username,
			OrganizationID:      organization.ID,
			OrganizationSlug:    organization.Slug,
			SessionState:        statusActive,
			DeviceInfo:          map[string]any{paramLoginMethod: "faceid"},
			AuthenticationLevel: loginMethodFido2,
			IPAddress:           strings.TrimSpace(command.IPAddress),
			UserAgent:           strings.TrimSpace(command.UserAgent),
			ExpiresAt:           now.Add(sessionTTL),
			CreatedAt:           now,
			LastSeenAt:          now,
			UpdatedAt:           now,
			Revision:            1,
		}
		if application != nil {
			sess.ApplicationID = application.ID
			sess.ApplicationSlug = application.Slug
		}
		if err := tx.Sessions().Create(ctx, sess); err != nil {
			return err
		}

		if err := s.embedAuditOutboxPlain(
			tx,
			ctx,
			"",
			"",
			"",
			sess.OrganizationID,
			sess.UserID,
			"session.created",
			"session",
			sess.ID,
			"session",
			"session.created",
			map[string]any{},
		); err != nil {
			return err
		}
		// S9: revoke old session carried by request (session fixation prevention)
		if strings.TrimSpace(command.PreviousSessionID) != "" {
			if err := revokePreviousSessionForRotation(ctx, tx.Sessions(), command.PreviousSessionID, now); err != nil {
				return err
			}

			if err := s.embedAuditOutboxPlain(
				tx,
				ctx,
				"",
				"",
				"",
				"",
				"",
				"session.revoked",
				"session",
				command.PreviousSessionID,
				"session",
				"session.revoked",
				map[string]any{},
			); err != nil {
				return err
			}
		}

		// 登录方式无关的会话策略与 signin 统计（排他登录 / 并发上限 / LastSignin*）。
		if err := s.applyLoginSessionPolicies(ctx, tx, user, application, organization.ID, sess, now); err != nil {
			return err
		}
		// 异常登录检测只记录告警，不阻断登录。
		s.warnAnomalousLogin(ctx, tx, user.ID, sess.IPAddress)

		loginResponse, err := s.handleLoggedIn(ctx, tx, loginCmd, organization, application, user, sid, now)
		if err != nil {
			return err
		}

		// 登录成功收口：审计 Record + Outbox 事件同事务（设计 3.2）。事件名用注册表
		// 已登记的 user.login_succeeded，payload.method=faceid 区分通道；不新造未登记
		// 事件，也不写入任何人脸/身份明文。
		if err := s.embedAuditOutboxPlain(
			tx,
			ctx,
			strings.TrimSpace(command.IPAddress),
			strings.TrimSpace(command.UserAgent),
			"",
			organization.ID,
			user.ID,
			"user.login_succeeded",
			"user",
			user.ID,
			"user",
			"user.login_succeeded",
			map[string]any{"method": "faceid", "sessionId": sess.ID},
		); err != nil {
			return err
		}

		result = &LoginResult{SessionID: sessionID, Response: *loginResponse}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func resolveFaceIdProvider(ctx context.Context, uow appuow.AuthUoW, orgID string) (*faceidproviders.FaceIdProvider, error) {
	isEnabled := true
	providers, _, err := uow.FaceIdProviders().List(ctx, faceidproviders.ListQuery{
		OrganizationID: orgID,
		IsEnabled:      &isEnabled,
		Page:           1,
		PageSize:       1,
	})
	if err != nil {
		return nil, apperrors.WrapInternal(err, "query face id providers")
	}

	if len(providers) == 0 {
		return nil, apperrors.NewUnimplemented("no enabled face id provider for organization")
	}

	return providers[0], nil
}

// faceTemplateReader is a narrow capability interface: appuow.UnitOfWork does
// not declare FaceTemplates(), so the concrete postgres/mysql UoW is reached by
// type assertion without expanding the shared interface and every fake.
type faceTemplateReader interface {
	FaceTemplates() userdomain.FaceTemplateRepository
}

// getFaceTemplate resolves the stored底图 reference used for 1:1 comparison.
// It prefers the authoritative encrypted user_face_templates row (decrypted to
// the base64 reference the provider compares against) and falls back to the
// legacy face_ids JSONB image URL for pre-migration rows. A decryption failure
// is surfaced, never silently downgraded to a legacy plaintext value.
func (s *Service) getFaceTemplate(ctx context.Context, user *userdomain.User) (string, error) {
	if p, ok := s.UOW.(faceTemplateReader); ok {
		if repo := p.FaceTemplates(); repo != nil {
			template, err := repo.GetActiveByUser(ctx, user.OrganizationID, user.ID)
			if err == nil {
				if template.TemplateRef != "" {
					return template.TemplateRef, nil
				}
			} else if apperrors.FromError(err).Code != apperrors.CodeNotFound {
				return "", err
			}
		}
	}
	for _, faceID := range user.FaceIds {
		if faceID.ImageUrl != "" {
			return faceID.ImageUrl, nil
		}
	}
	return "", apperrors.NewNotFound("no face template registered for user")
}
