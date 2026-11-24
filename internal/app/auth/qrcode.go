package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applications "github.com/guliangiam/guliangiam/internal/domain/applications"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	qrcodedomain "github.com/guliangiam/guliangiam/internal/domain/qrcodesessions"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

const (
	qrCodeLength       = 32
	qrCodeTTL          = 5 * time.Minute
	qrCodePollInterval = 3 * time.Second
)

// CreateQRCodeCommand contains parameters for creating a QR code login session.
type CreateQRCodeCommand struct {
	OrganizationID string
	ApplicationID  string
}

// CreateQRCodeResult contains the generated QR code data.
type CreateQRCodeResult struct {
	CodeID    string `json:"codeId"`
	Code      string `json:"code"`
	ExpiresIn int64  `json:"expiresIn"`
	Interval  int64  `json:"interval"`
}

// ScanQRCodeCommand contains parameters for scanning a QR code.
//
// 扫码者身份有两条互斥来源（OPEN-API-15 的修法）：
//   - 已认证调用方：身份只取自会话，ScannerUsername/ScannerOrganizationID 由传输层
//     从 SessionContext 注入，请求体里的身份字段一概不读；
//   - 匿名调用方：只允许自动注册一个尚不存在的账号（Username/DisplayName），
//     命中既有账号一律拒绝——「自报身份 + 绑定既有账号」就是冒充。
type ScanQRCodeCommand struct {
	// Code is the QR payload (the opaque code the scanning device read).
	Code string
	// Username/DisplayName are the anonymous registration input.
	Username    string
	DisplayName string
	// ScannerUsername/ScannerOrganizationID come from the caller's session.
	ScannerUsername       string
	ScannerOrganizationID string
	IPAddress             string
	UserAgent             string
}

// ScanQRCodeResult reports the outcome of a scan.
type ScanQRCodeResult struct {
	// IsNewUser is true when an anonymous scanner was auto-registered.
	IsNewUser bool `json:"isNewUser"`
}

// ConfirmQRCodeCommand contains parameters for confirming a QR code login.
type ConfirmQRCodeCommand struct {
	// Code is the QR payload the scanning device read.
	Code string
	// ScannerUsername/ScannerOrganizationID come from the confirming session;
	// 确认者必须就是扫码绑定的那个人。
	ScannerUsername       string
	ScannerOrganizationID string
	Approved              bool
}

// PollQRCodeResult contains the QR code status for polling.
type PollQRCodeResult struct {
	Status    string `json:"status"` // "pending", "scanned", "confirmed", "expired"
	CodeID    string `json:"codeId"`
	ExpiresIn int64  `json:"expiresIn,omitempty"`
}

// CreateQRCode generates a new QR code for login and persists it to the database.
func (s *Service) CreateQRCode(ctx context.Context, command CreateQRCodeCommand) (*CreateQRCodeResult, error) {
	organizationID := strings.TrimSpace(command.OrganizationID)
	applicationID := strings.TrimSpace(command.ApplicationID)
	if organizationID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	if applicationID == "" {
		return nil, apperrors.NewInvalidArgument("applicationId is required")
	}
	// 创建是匿名端点：把不存在的应用与跨组织的应用挡在会话创建之前，
	// 否则只会落到外键错误（500），或建出一个永远无法完成登录的会话。
	application, err := s.UOW.Applications().GetByID(ctx, applicationID)
	if err != nil {
		return nil, apperrors.NewInvalidArgument("application not found")
	}
	if application.OrganizationID != organizationID {
		return nil, apperrors.NewInvalidArgument("application does not belong to organization")
	}
	// 匿名创建端点没有会话租户上下文；qr_code_sessions 受 FORCE RLS 保护，
	// 必须显式把已校验的组织钉进 TenantScope（与 saml_acs_audit 同一模式），
	// 否则 INSERT 被 WITH CHECK 拒绝成 500。
	ctx = tenantctx.WithTenantScope(ctx, organizationID, "", tenantctx.TenantFilterOptional)

	codeBytes := make([]byte, qrCodeLength)
	if _, err := rand.Read(codeBytes); err != nil {
		return nil, apperrors.NewInternal("generate qr code")
	}
	code := base64.RawURLEncoding.EncodeToString(codeBytes)

	codeID, err := s.UUIDGenerator.NewString()
	if err != nil {
		return nil, apperrors.NewInternal("generate qr code id")
	}

	now := s.Clock.Now()

	session := &qrcodedomain.Session{
		ID:             codeID,
		CodeID:         codeID,
		Code:           code,
		OrganizationID: organizationID,
		ApplicationID:  applicationID,
		ExpiresAt:      now.Add(qrCodeTTL),
		Status:         "pending",
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	// RLS 的租户 GUC 只在 UOW 事务内注入（uow.setTenantContext），
	// 仓储直连池不会应用 TenantScope，因此 INSERT 必须走事务。
	if err := s.UOW.WithinTransaction(ctx, func(txCtx context.Context, tx appuow.UnitOfWork) error {
		return tx.QRCodeSessions().Create(txCtx, session)
	}); err != nil {
		return nil, err
	}

	return &CreateQRCodeResult{
		CodeID:    codeID,
		Code:      code,
		ExpiresIn: int64(qrCodeTTL / time.Second),
		Interval:  int64(qrCodePollInterval / time.Second),
	}, nil
}

// ScanQRCode binds the scanning subject to a pending QR code session.
//
// 返回的 sessionID 只在「匿名扫码 → 自动注册」路径非空：扫码端此时本来没有会话，
// 不签发它就永远走不到 confirm（confirm 需要会话），传输层据此设置会话 cookie。
func (s *Service) ScanQRCode(ctx context.Context, command ScanQRCodeCommand) (*ScanQRCodeResult, string, error) {
	code := strings.TrimSpace(command.Code)
	if code == "" {
		return nil, "", apperrors.NewInvalidArgument("code is required")
	}

	var result *ScanQRCodeResult
	var issuedSessionID string
	// ADR-029 post_registration 在事务提交后触发（fail-open），先记住
	// 匿名扫码注册出的账号事实。
	var registeredUser *userdomain.User
	var registeredAppID string

	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		session, err := tx.QRCodeSessions().GetByCode(ctx, code)
		if err != nil {
			return apperrors.NewInvalidArgument("invalid qr code")
		}

		now := s.Clock.Now()
		if now.After(session.ExpiresAt) {
			_ = tx.QRCodeSessions().DeleteByCodeID(ctx, session.CodeID)
			return apperrors.NewInvalidArgument("qr code has expired")
		}

		if session.Status != "pending" {
			return apperrors.NewConflict("qr code already scanned")
		}

		user, isNewUser, scannerSessionID, err := s.resolveQrScanner(ctx, tx, session, command, now)
		if err != nil {
			return err
		}

		session.Status = "scanned"
		session.UserID = user.ID
		session.Username = user.Username
		session.OrganizationID = user.OrganizationID
		session.IPAddress = strings.TrimSpace(command.IPAddress)
		session.UserAgent = strings.TrimSpace(command.UserAgent)
		session.UpdatedAt = now

		if err := tx.QRCodeSessions().UpdateStatus(ctx, session); err != nil {
			return err
		}

		result = &ScanQRCodeResult{IsNewUser: isNewUser}
		issuedSessionID = scannerSessionID
		if isNewUser {
			registeredUser = user
			registeredAppID = session.ApplicationID
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}

	if registeredUser != nil {
		s.dispatchPostRegistration(ctx, registeredUser.OrganizationID, registeredUser.ID, registeredUser.Username,
			registeredAppID, "QR Code Scan")
	}

	return result, issuedSessionID, nil
}

// resolveQrScanner decides whose identity the scan binds.
//
// 已认证调用方：身份取自会话（GetByUsername + 组织一致性 + 可登录性校验），
// 请求体里的身份字段被完全忽略。匿名调用方：按组织开关自动注册新账号，
// 若同名账号已存在则拒绝——这正是 OPEN-API-15 关闭的位置。
func (s *Service) resolveQrScanner(ctx context.Context, tx appuow.UnitOfWork, qrSession *qrcodedomain.Session, command ScanQRCodeCommand, now time.Time) (scanner *userdomain.User, created bool, message string, err error) {
	scannerUsername := strings.TrimSpace(command.ScannerUsername)
	if scannerUsername != "" {
		organizationID := strings.TrimSpace(command.ScannerOrganizationID)
		if organizationID == "" {
			return nil, false, "", apperrors.NewUnauthorized("authentication required")
		}

		user, err := tx.Users().GetByUsername(ctx, organizationID, scannerUsername)
		if err != nil || user == nil {
			return nil, false, "", apperrors.NewUnauthorized("scanning account not found")
		}
		if !isUserAllowedToLogin(user.Status, user.IsForbidden) {
			return nil, false, "", apperrors.NewForbidden("user cannot sign in")
		}
		// 二维码会话的组织即被登录应用的组织：不允许跨组织把会话换给别的组织的账号。
		if qrSession.OrganizationID != "" && user.OrganizationID != qrSession.OrganizationID {
			return nil, false, "", apperrors.NewForbidden("account does not belong to the application organization")
		}
		return user, false, "", nil
	}

	username := strings.TrimSpace(command.Username)
	if username == "" {
		return nil, false, "", apperrors.NewInvalidArgument("username is required for anonymous scan")
	}
	if qrSession.OrganizationID == "" {
		return nil, false, "", apperrors.NewForbidden("qr session has no organization; anonymous registration is unavailable")
	}

	organization, err := tx.Organizations().GetByID(ctx, qrSession.OrganizationID)
	if err != nil {
		return nil, false, "", apperrors.NewInvalidArgument("organization not found")
	}
	if organization.DisableSignup {
		return nil, false, "", apperrors.NewForbidden("organization does not allow sign-up")
	}
	if organization.DisableSignin {
		return nil, false, "", apperrors.NewForbidden("organization has sign-in disabled")
	}

	// 关键：匿名扫码只能创建新账号，不能绑定既有账号。
	if existing, err := tx.Users().GetByUsername(ctx, organization.ID, username); err == nil && existing != nil {
		return nil, false, "", apperrors.NewForbidden("account already exists; sign in before scanning")
	}

	user, err := s.autoRegisterQrScanUser(ctx, tx, organization, qrSession, username, command.DisplayName, now)
	if err != nil {
		return nil, false, "", err
	}

	scannerSessionID, err := s.issueQrScannerSession(ctx, tx, organization, qrSession, user, command, now)
	if err != nil {
		return nil, false, "", err
	}

	return user, true, scannerSessionID, nil
}

// autoRegisterQrScanUser creates the account an anonymous scan registers.
// 门控（DisableSignup/DisableSignin）由调用方先行校验，字段口径对齐 Signup 与
// autoRegisterSocialUser：无密码凭据，register_type/register_source 记录来源。
func (s *Service) autoRegisterQrScanUser(ctx context.Context, tx appuow.UnitOfWork, organization *orgdomain.Organization, qrSession *qrcodedomain.Session, username, displayName string, now time.Time) (*userdomain.User, error) {
	userID, err := s.UUIDGenerator.NewString()
	if err != nil {
		return nil, apperrors.NewInternal("generate user id")
	}

	user := &userdomain.User{
		ID:             userID,
		OrganizationID: organization.ID,
		Username:       username,
		// 访客账号：匿名扫码只能自动注册轻量账号，联系方式与密码由本人后续在账号页
		// 绑定/设置（绑定时要求验证码），因此这里显式落 user_type=guest 便于治理与筛选。
		UserType:            "guest",
		Status:              statusActive,
		SignupApplicationID: qrSession.ApplicationID,
		RegisterType:        "QR Code Scan",
		RegisterSource:      organization.Slug + "/" + qrSession.ApplicationID,
		Score:               organization.InitScore,
		Profile: userdomain.UserProfile{
			UserID:      userID,
			DisplayName: firstNonEmpty(strings.TrimSpace(displayName), username),
			AvatarURL:   organization.DefaultAvatar,
		},
		CreatedAt: now,
		UpdatedAt: now,
		Revision:  1,
	}
	if err := tx.Users().Create(ctx, user); err != nil {
		return nil, err
	}

	if err := s.embedAuditOutboxPlain(
		tx,
		ctx,
		"",
		"",
		"",
		user.OrganizationID,
		user.ID,
		"user.created",
		"user",
		user.ID,
		"user",
		"user.created",
		map[string]any{"registerType": "QR Code Scan"},
	); err != nil {
		return nil, err
	}

	return user, nil
}

// issueQrScannerSession logs the freshly registered scanner in.
//
// 没有这一步，匿名扫码注册出来的账号无法 confirm（confirm 要求会话）。这个会话同时
// 就是「设备记忆」：sessionTTL 为 30 天，同一设备下次扫码时 scan 走已认证路径、
// 身份取自会话，直接复用同一账号，不会重复建号（回归见 qrcode_test.go 的复用用例）。
func (s *Service) issueQrScannerSession(ctx context.Context, tx appuow.UnitOfWork, organization *orgdomain.Organization, qrSession *qrcodedomain.Session, user *userdomain.User, command ScanQRCodeCommand, now time.Time) (string, error) {
	// ── Risk engine evaluation (qrcode channel) ───────────────────────
	// Anonymous registration + first session — gate before we commit the
	// session row. No step-up path, so challenge is deny.
	uid := user.ID
	if evalErr := s.evaluateLoginRisk(ctx, tx, "qrcode", organization.ID, &uid,
		strings.TrimSpace(command.IPAddress), strings.TrimSpace(command.UserAgent), "",
		map[string]any{fieldCode: strings.TrimSpace(command.Code)}); evalErr != nil {
		switch evalErr.(type) {
		case *RiskDeniedError, *RiskChallengeError:
			return "", apperrors.NewUnauthorized("access denied by risk assessment")
		default:
			return "", evalErr
		}
	}

	// ADR-029 pre_login (qrcode channel): fail-closed policy gate before
	// the scanner session is committed.
	if err := s.enforceActionsPreLogin(ctx, "qrcode", organization.ID, user.ID, user.Username,
		qrSession.ApplicationID, command.IPAddress, command.UserAgent); err != nil {
		return "", err
	}

	sessionID, err := s.UUIDGenerator.NewString()
	if err != nil {
		return "", apperrors.NewInternal("generate session id")
	}

	sid, err := newSessionSid()
	if err != nil {
		return "", err
	}

	sess := &sessiondomain.Session{
		ID:               sessionID,
		Sid:              sid,
		UserID:           user.ID,
		Username:         user.Username,
		OrganizationID:   organization.ID,
		OrganizationSlug: organization.Slug,
		ApplicationID:    qrSession.ApplicationID,
		SessionState:     statusActive,
		DeviceInfo:       map[string]any{paramLoginMethod: "qrcode", "registration": "anonymous-scan"},
		IPAddress:        strings.TrimSpace(command.IPAddress),
		UserAgent:        strings.TrimSpace(command.UserAgent),
		ExpiresAt:        now.Add(sessionTTL),
		CreatedAt:        now,
		LastSeenAt:       now,
		UpdatedAt:        now,
		Revision:         1,
	}
	if err := tx.Sessions().Create(ctx, sess); err != nil {
		return "", err
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
		return "", err
	}

	return sessionID, nil
}

// ConfirmQRCode confirms or rejects a QR code login from the scanning device.
func (s *Service) ConfirmQRCode(ctx context.Context, command ConfirmQRCodeCommand) error {
	code := strings.TrimSpace(command.Code)
	if code == "" {
		return apperrors.NewInvalidArgument("code is required")
	}

	session, err := s.UOW.QRCodeSessions().GetByCode(ctx, code)
	if err != nil {
		return apperrors.NewInvalidArgument("invalid qr code")
	}

	now := s.Clock.Now()
	if now.After(session.ExpiresAt) {
		_ = s.UOW.QRCodeSessions().DeleteByCodeID(ctx, session.CodeID)
		return apperrors.NewInvalidArgument("qr code has expired")
	}

	if session.Status != "scanned" {
		return apperrors.NewConflict("qr code has not been scanned yet")
	}

	// 只有扫码绑定的人能确认：否则任何拿到二维码载荷的人都能把别人的会话推进到
	// confirmed，进而让 PC 端换到那个人的令牌。
	if strings.TrimSpace(command.ScannerUsername) != session.Username ||
		(strings.TrimSpace(command.ScannerOrganizationID) != "" &&
			session.OrganizationID != "" &&
			strings.TrimSpace(command.ScannerOrganizationID) != session.OrganizationID) {
		return apperrors.NewForbidden("only the scanning account can confirm")
	}

	if command.Approved {
		session.Status = "confirmed"
		session.UpdatedAt = now
		return s.UOW.QRCodeSessions().UpdateStatus(ctx, session)
	}

	// Rejected - remove the entry.
	_ = s.UOW.QRCodeSessions().DeleteByCodeID(ctx, session.CodeID)
	return nil
}

// PollQRCodeStatus returns the current status of a QR code login session.
func (s *Service) PollQRCodeStatus(ctx context.Context, codeID string) (*PollQRCodeResult, error) {
	if strings.TrimSpace(codeID) == "" {
		return nil, apperrors.NewInvalidArgument("codeId is required")
	}

	session, err := s.UOW.QRCodeSessions().GetByCodeID(ctx, codeID)
	if err != nil {
		// Unknown or already-consumed code is reported as expired; log the miss
		// so genuine storage failures remain diagnosable.
		slog.DebugContext(ctx, "qr code session lookup failed; reporting expired", "error", err)
		return &PollQRCodeResult{Status: "expired", CodeID: codeID}, nil
	}

	now := s.Clock.Now()
	if now.After(session.ExpiresAt) {
		_ = s.UOW.QRCodeSessions().DeleteByCodeID(ctx, codeID)
		return &PollQRCodeResult{Status: "expired", CodeID: codeID}, nil
	}

	remaining := int64(session.ExpiresAt.Sub(now) / time.Second)
	return &PollQRCodeResult{
		Status:    session.Status,
		CodeID:    codeID,
		ExpiresIn: remaining,
	}, nil
}

// ExchangeQRCodeCommand contains parameters for exchanging a confirmed QR code for
// the initiating device's session and login result.
type ExchangeQRCodeCommand struct {
	CodeID string
	// PreviousSessionID 是发起端自带的旧会话（请求 cookie），成功后在事务内撤销
	// （S9 会话固定防护）；匿名发起端通常为空。
	PreviousSessionID string

	// 协议上下文（可选）：与 /faceid/complete 同义。授权页上的扫码登录靠它拿回授权码
	// 或回调地址；普通控制台登录留空即可（OPEN-API-17）。
	ResponseType string
	RedirectURI  string
	State        string
	Nonce        string
	Scope        []string

	// 会话元数据：由传输层从连接与请求头解析，不是客户端可自报的字段
	// （LastSigninIP 与异常登录检测都依赖它）。
	IPAddress string
	UserAgent string
}

// ExchangeQRCode exchanges a confirmed QR code for the initiating device's session
// and — like /faceid/complete — returns a login result carrying the protocol outcome.
//
// 会话在 exchange 签发：发起端是匿名 PC，没有既有会话可复用。协议上下文（responseType/
// redirectUri/state/nonce/scope）透传给 handleLoggedIn，因此授权页上的扫码登录也能拿到
// 授权码/回调地址，而不是只落一个会话（OPEN-API-17）。返回的 sessionID 由传输层写成 cookie。
func (s *Service) ExchangeQRCode(ctx context.Context, command ExchangeQRCodeCommand) (*LoginResult, string, error) {
	if strings.TrimSpace(command.CodeID) == "" {
		return nil, "", apperrors.NewInvalidArgument("codeId is required")
	}

	session, err := s.UOW.QRCodeSessions().GetByCodeID(ctx, command.CodeID)
	if err != nil {
		return nil, "", apperrors.NewInvalidArgument("invalid qr code")
	}

	now := s.Clock.Now()
	if now.After(session.ExpiresAt) {
		_ = s.UOW.QRCodeSessions().DeleteByCodeID(ctx, command.CodeID)
		return nil, "", apperrors.NewInvalidArgument("qr code has expired")
	}

	if session.Status != "confirmed" {
		return nil, "", apperrors.NewUnauthorized("qr code not confirmed")
	}

	if session.UserID == "" {
		return nil, "", apperrors.NewUnauthorized("qr code has no associated user")
	}

	// Consume the session immediately to prevent replay.
	_ = s.UOW.QRCodeSessions().DeleteByCodeID(ctx, command.CodeID)

	var result *LoginResult
	var issuedSessionID string
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		user, err := tx.Users().GetByID(ctx, session.UserID)
		if err != nil {
			return err
		}

		organizationID := session.OrganizationID
		if organizationID == "" {
			organizationID = user.OrganizationID
		}
		organization, err := tx.Organizations().GetByID(ctx, organizationID)
		if err != nil {
			return err
		}

		var application *applications.Application
		if session.ApplicationID != "" {
			application, err = tx.Applications().GetByID(ctx, session.ApplicationID)
			if err != nil {
				return err
			}
		}

		if err := s.enforceActionsPreLogin(
			ctx,
			"qrcode_exchange",
			organization.ID,
			user.ID,
			user.Username,
			session.ApplicationID,
			command.IPAddress,
			command.UserAgent,
		); err != nil {
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
		loginSession := &sessiondomain.Session{
			ID:               sessionID,
			Sid:              sid,
			UserID:           user.ID,
			Username:         user.Username,
			OrganizationID:   organization.ID,
			OrganizationSlug: organization.Slug,
			ApplicationID:    session.ApplicationID,
			SessionState:     statusActive,
			DeviceInfo:       map[string]any{paramLoginMethod: "qrcode"},
			IPAddress:        strings.TrimSpace(command.IPAddress),
			UserAgent:        strings.TrimSpace(command.UserAgent),
			ExpiresAt:        now.Add(sessionTTL),
			CreatedAt:        now,
			LastSeenAt:       now,
			UpdatedAt:        now,
			Revision:         1,
		}
		if application != nil {
			loginSession.ApplicationSlug = application.Slug
		}
		if err := tx.Sessions().Create(ctx, loginSession); err != nil {
			return err
		}
		if err := s.embedAuditOutboxPlain(
			tx,
			ctx,
			"",
			"",
			"",
			loginSession.OrganizationID,
			loginSession.UserID,
			"session.created",
			"session",
			loginSession.ID,
			"session",
			"session.created",
			map[string]any{},
		); err != nil {
			return err
		}
		// S9: revoke old session carried by request (session fixation prevention)
		if strings.TrimSpace(command.PreviousSessionID) != "" {
			// Not best-effort: a failed revoke rolls the whole login back instead of
			// leaving the old session alive next to the freshly issued one.
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

		// 登录方式无关的会话策略与 signin 统计（排他登录 / 并发上限 / LastSignin*）：
		// 匿名 PC 的扫码登录与密码登录受同一套租户策略约束。
		if err := s.applyLoginSessionPolicies(ctx, tx, user, application, organization.ID, loginSession, now); err != nil {
			return err
		}
		// 异常登录检测只记录告警，不阻断登录。
		s.warnAnomalousLogin(ctx, tx, user.ID, loginSession.IPAddress)

		loginResponse, err := s.handleLoggedIn(ctx, tx, LoginCommand{
			OrganizationSlug: organization.Slug,
			Username:         user.Username,
			ApplicationID:    session.ApplicationID,
			ResponseType:     command.ResponseType,
			RedirectURI:      command.RedirectURI,
			State:            command.State,
			Nonce:            command.Nonce,
			Scope:            command.Scope,
		}, organization, application, user, sid, now)
		if err != nil {
			return err
		}

		result = &LoginResult{SessionID: sessionID, Response: *loginResponse}
		issuedSessionID = sessionID
		return nil
	})
	if err != nil {
		return nil, "", err
	}

	return result, issuedSessionID, nil
}
