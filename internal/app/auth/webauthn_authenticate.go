package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	sessionsdomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	web3identitydomain "github.com/guliangiam/guliangiam/internal/domain/web3identity"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	platformwebauthn "github.com/guliangiam/guliangiam/internal/platform/webauthn"
)

func (s *Service) WebAuthnAuthenticationBegin(ctx context.Context, command WebAuthnAuthenticationBeginCommand) (*WebAuthnAuthenticationBeginResult, error) {
	if strings.TrimSpace(command.Username) == "" {
		return nil, apperrors.NewInvalidArgument("username is required")
	}
	if strings.TrimSpace(command.OrganizationSlug) == "" {
		return nil, apperrors.NewInvalidArgument("organizationSlug is required")
	}

	// 匿名登录没有可信租户作用域，只有两处需要越权读取：
	// 1) 组织 slug → 组织实体（组织目录本身是登录前就必须可见的公共信息）；
	// 2) 之后的用户/凭证/挑战全部收窄到该组织，不再使用全库可见的 System 作用域。
	organization, err := s.UOW.Organizations().GetBySlug(tenantctx.System(ctx), command.OrganizationSlug)
	if err != nil {
		return nil, err
	}
	ctx = tenantctx.WithTenantScope(ctx, organization.ID, organization.Slug, tenantctx.TenantFilterCross)
	if !passkeyEnabled(organization) {
		return nil, apperrors.NewForbidden("passkey is disabled for this organization")
	}
	if organization.DisableSignin {
		return nil, apperrors.NewForbidden("organization has sign-in disabled")
	}

	user, err := s.UOW.Users().GetByUsername(ctx, organization.ID, command.Username)
	if err != nil {
		return nil, mapWebAuthnUserLookupError(err)
	}
	if !isUserAllowedToLogin(user.Status, user.IsForbidden) {
		return nil, apperrors.NewForbidden("user cannot sign in")
	}

	// Look up existing credentials for this user.
	credentials, err := s.UOW.WebAuthnCredentials().ListByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	if len(credentials) == 0 {
		return nil, apperrors.NewWithMeta(apperrors.CodeInvalidArgument,
			"no WebAuthn credentials registered for this user",
			map[string]any{"reason": "credential_not_found"})
	}

	// 已撤销的凭证不进 allowCredentials：否则浏览器会把它当作可用项让用户确认，
	// 直到 finish 才被 S3 守卫拒绝，表现为「弹窗成功但登录失败」。
	allowedIDs := make([]string, 0, len(credentials))
	for _, cred := range credentials {
		if cred.Status != webauthndomain.CredentialStatusActive {
			continue
		}
		allowedIDs = append(allowedIDs, base64.RawURLEncoding.EncodeToString(cred.CredentialID))
	}

	if len(allowedIDs) == 0 {
		return nil, apperrors.NewWithMeta(apperrors.CodeInvalidArgument,
			"no active WebAuthn credentials registered for this user",
			map[string]any{"reason": "credential_not_found"})
	}

	challenge, err := generateWebAuthnChallenge()
	if err != nil {
		return nil, apperrors.NewInternal("generate webauthn challenge")
	}

	// 挑战落到服务端表里（只存哈希），finish 才能在任何副本上原子消费它。
	// RP ID 与注册路径同一来源（服务端推导），finish 以行里固化的值为准。
	rpID, err := s.webAuthnRelyingPartyID()
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now()
	challengeID, err := s.UUIDGenerator.NewString()
	if err != nil {
		return nil, apperrors.NewInternal("generate webauthn challenge id")
	}
	if err := s.UOW.WebAuthnCredentials().CreateChallenge(ctx, &webauthndomain.Challenge{
		ID:             challengeID,
		ChallengeHash:  webauthndomain.HashChallenge(challenge),
		CeremonyType:   webauthndomain.CeremonyTypeAuthentication,
		UserID:         user.ID,
		OrganizationID: organization.ID,
		RelyingPartyID: rpID,
		ExpiresAt:      now.Add(webAuthnChallengeTimeout),
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}); err != nil {
		return nil, apperrors.NewInternal("store webauthn challenge")
	}

	// 与 finish 同一策略来源：浏览器拿到的 UV 要求必须与服务端校验口径一致。
	userVerification := "preferred"
	if s.webAuthnUVRequired(organization) {
		userVerification = "required"
	}

	return &WebAuthnAuthenticationBeginResult{
		ChallengeID:          challengeID,
		Challenge:            challenge,
		RelyingPartyID:       rpID,
		AllowedCredentialIDs: allowedIDs,
		Timeout:              int64(webAuthnChallengeTimeout / time.Millisecond),
		UserVerification:     userVerification,
	}, nil
}

// WebAuthnAuthenticationFinish completes a WebAuthn authentication ceremony.
//
// 登录态在此签发：断言校验通过后，在同一事务内创建会话、推进签名计数器、撤销请求
// 携带的旧会话（S9），再调用 handleLoggedIn 产出协议上下文——与 /faceid/complete、
// 扫码 exchange 同形。返回的 sessionID 由传输层写成 opaque session cookie。
func (s *Service) WebAuthnAuthenticationFinish(ctx context.Context, command WebAuthnAuthenticationFinishCommand) (*LoginResult, string, error) {
	if strings.TrimSpace(command.ChallengeID) == "" {
		return nil, "", apperrors.NewInvalidArgument("challengeId is required — call authentication:begin first")
	}
	if strings.TrimSpace(command.Username) == "" {
		return nil, "", apperrors.NewInvalidArgument("username is required")
	}
	if strings.TrimSpace(command.CredentialID) == "" {
		return nil, "", apperrors.NewInvalidArgument("credentialId is required")
	}

	// origin 校验依赖服务端配置的来源集合；为空说明部署缺配置，先给出可操作的错误，
	// 而不是让每一次 ceremony 都倒在「origin 不在允许清单」上（也避免白白消费挑战）。
	if err := s.requireWebAuthnOriginConfig(); err != nil {
		return nil, "", err
	}

	// 与 begin 同口径：仅组织 slug 解析使用跨租户读取，随后立即收窄到目标组织。
	if strings.TrimSpace(command.OrganizationSlug) == "" {
		return nil, "", apperrors.NewInvalidArgument("organizationSlug is required")
	}
	organization, err := s.UOW.Organizations().GetBySlug(tenantctx.System(ctx), command.OrganizationSlug)
	if err != nil {
		return nil, "", err
	}
	ctx = tenantctx.WithTenantScope(ctx, organization.ID, organization.Slug, tenantctx.TenantFilterCross)
	if !passkeyEnabled(organization) {
		return nil, "", apperrors.NewForbidden("passkey is disabled for this organization")
	}
	if organization.DisableSignin {
		return nil, "", apperrors.NewForbidden("organization has sign-in disabled")
	}

	user, err := s.UOW.Users().GetByUsername(ctx, organization.ID, command.Username)
	if err != nil {
		return nil, "", mapWebAuthnUserLookupError(err)
	}
	if !isUserAllowedToLogin(user.Status, user.IsForbidden) {
		return nil, "", apperrors.NewForbidden("user cannot sign in")
	}

	// 原子消费挑战行：只有「未消费且未过期」的行能被抢到，并发 finish 只有一个能赢。
	sessionData, consumeErr := s.UOW.WebAuthnCredentials().ConsumeChallenge(ctx, command.ChallengeID, s.Clock.Now())
	if consumeErr != nil {
		// 缺失 / 已消费 / 已过期在仓储层同为 NotFound，这里统一 410 且消息一致，
		// 调用方无法据此枚举挑战是否曾经存在。
		return nil, "", apperrors.NewWithMeta(apperrors.CodeGone,
			"authentication challenge is missing, expired or already used",
			map[string]any{"reason": "challenge_expired"})
	}
	// 两个 ceremony 共用一张表：拿注册挑战来完成认证同样是跨 ceremony 重放。
	if sessionData.CeremonyType != webauthndomain.CeremonyTypeAuthentication {
		return nil, "", apperrors.NewForbidden("challenge was issued for another ceremony")
	}
	// 明文挑战不落库，用客户端回传的明文重算哈希比对；空明文一律拒绝。
	if strings.TrimSpace(command.Challenge) == "" || !sessionData.MatchesChallenge(command.Challenge) {
		return nil, "", apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
			"challenge does not match the one issued by begin",
			map[string]any{"reason": "challenge_mismatch"})
	}

	// S2: bind the submitted username to the session-bound user ID.
	if sessionData.UserID != user.ID {
		return nil, "", apperrors.NewForbidden("challenge session user does not match submitted username")
	}

	// Decode the credential ID and look up the stored credential.
	credentialIDBytes, err := base64.RawURLEncoding.DecodeString(command.CredentialID)
	if err != nil {
		return nil, "", apperrors.NewInvalidArgument("invalid credentialId encoding")
	}

	credential, err := s.UOW.WebAuthnCredentials().GetByCredentialID(ctx, credentialIDBytes)
	if err != nil {
		return nil, "", apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
			"credential not found",
			map[string]any{"reason": "credential_not_found"})
	}

	// S3 guard: revoked/deactivated credentials must not authenticate.
	// CHECK constraint on webauthn_credentials.status enforces only the
	// three canonical values; anything but Active here is rejected.
	if credential.Status != webauthndomain.CredentialStatusActive {
		return nil, "", apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("credential is %s; authentication denied", credential.Status),
			map[string]any{"reason": "credential_inactive"})
	}

	// S3 guard (约束 4)：凭据属主必须与命令里的 username 解析出的用户一致。
	// begin 对任意 username 都签发挑战，若此处不比对，攻击者可用自己的密钥对
	// 为他人 username 完成断言并拿到他人会话。
	if credential.UserID != user.ID {
		return nil, "", apperrors.NewWithMeta(apperrors.CodeForbidden,
			"credential does not belong to this user",
			map[string]any{"reason": "credential_owner_mismatch"})
	}

	// Perform full WebAuthn assertion verification:
	// - Validate clientDataJSON (type, challenge, origin)
	// - Validate authenticatorData (rpIdHash, flags, signCount)
	// - Verify cryptographic signature using stored public key
	// - Detect cloned authenticators via sign count regression
	newSignCount, err := platformwebauthn.VerifyAssertion(
		command.Challenge,
		sessionData.RelyingPartyID,
		command.ClientDataJSON,
		command.AuthenticatorData,
		command.Signature,
		credential.PublicKey,
		credential.SignCount,
		s.webAuthnAllowedOrigins(),
		s.webAuthnUVRequired(organization),
	)
	if err != nil {
		// 克隆嫌疑（签名计数未递增）单独处置，见 AC-AUTH-008：撤销凭证 + 写 critical
		// 安全事件，二者同一事务——不能出现「撤了没记」或「记了没撤」。撤销后该凭证
		// 永远过不了 S3 守卫，即使攻击者手上的私钥是真的也只能用这一次。
		if errors.Is(err, platformwebauthn.ErrSignCountRegression) {
			revokeErr := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
				if err := tx.WebAuthnCredentials().UpdateStatus(ctx, credential.ID, webauthndomain.CredentialStatusRevoked); err != nil {
					return err
				}
				return s.embedAuditOutboxPlain(
					tx,
					ctx,
					command.IPAddress,
					command.UserAgent,
					"",
					organization.ID,
					user.ID,
					"webauthn.credential.clone_suspected",
					"webauthn_credential",
					credential.ID,
					"webauthn_credential",
					"webauthn.credential.clone_suspected",
					map[string]any{
						"reason":            "sign_count_not_increased",
						"storedSignCount":   credential.SignCount,
						"receivedSignCount": newSignCount,
					},
				)
			})
			if revokeErr != nil {
				return nil, "", apperrors.NewInternalf("revoke credential after sign count regression: %v", revokeErr)
			}
			return nil, "", apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
				"WebAuthn assertion sign count did not increase; credential revoked and security event recorded",
				map[string]any{"reason": "sign_count_regression"})
		}
		// 这些 WebAuthn 端点走 OAuth 协议面（error + error_description），
		// details.reason 会被协议 writer 丢弃；把同一 reason 以稳定标记追加到
		// description，前端既能从 details（管理面）也能从 description 解析。
		reason := webAuthnFailureReason(err)
		return nil, "", apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("WebAuthn assertion verification failed: %s [reason: %s]", err.Error(), reason),
			map[string]any{"reason": reason})
	}

	// 协议上下文的目标应用。普通控制台登录不带 applicationId，此时解析组织的
	// 默认应用（app-<orgSlug>）——sessions.application_id 是 NOT NULL 外键，
	// 留空会让会话创建在 Postgres 侧以 22P02（invalid uuid）失败并变成 500。
	var application *applicationdomain.Application
	if strings.TrimSpace(command.ApplicationID) != "" {
		application, err = s.UOW.Applications().GetByID(ctx, command.ApplicationID)
		if err != nil {
			return nil, "", err
		}
	} else if strings.TrimSpace(command.ResponseType) == "" {
		// 仅普通控制台登录（无协议上下文）才补默认应用；带 responseType 的
		// 协议登录必须显式给出 applicationId，否则不能臆测目标应用。
		var source applicationReferenceSource
		application, source, err = resolveApplicationReferenceWithSource(ctx, s.UOW.Applications(), organization, organization.Slug)
		if err != nil {
			return nil, "", err
		}
		slog.InfoContext(ctx, "default application resolved",
			"organization_id", organization.ID, "application_id", application.ID, "source", string(source))
	}
	// 协议产物必须以目标应用为输入（回调地址白名单、client_id、SAML SP 等都在
	// 应用上）。缺应用时 handleLoggedIn 会在 nil 指针上取字段，这里提前拦成 422。
	if application == nil && strings.TrimSpace(command.ResponseType) != "" {
		return nil, "", apperrors.NewInvalidArgument("applicationId is required when responseType is set")
	}
	if application != nil && application.DisableSignin {
		return nil, "", apperrors.NewForbidden("application has sign-in disabled")
	}

	loginCommand := LoginCommand{
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
	var issuedSessionID string
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		now := s.Clock.Now()

		// 签名计数器与登录态同事务推进：要么「计数器 + 会话」一起生效，
		// 要么都不生效，不留下计数器已前进但没有登录态的中间态。
		//
		// 认证器可以不上报计数器（received=0，规范允许），此时不写回：仓储的
		// `sign_count < $1` 单调性谓词在 stored=0 / received=0 时命中 0 行，
		// 会把合法登录误判成回退。真正的回退（received>0 且 < stored）已在
		// VerifyAssertion 里于写入前拦截。
		if newSignCount > 0 {
			if err := tx.WebAuthnCredentials().UpdateSignCount(ctx, credential.ID, newSignCount); err != nil {
				return err
			}
		}

		// Layer 2: touch passkey → EVM mapping last_used_at (if any) and
		// surface the derived address in DeviceInfo for downstream consumers.
		web3Address := ""
		if mapping, merr := tx.Web3Identities().GetByCredentialID(ctx, credential.ID); merr == nil {
			web3Address = mapping.WalletAddress
			if terr := tx.Web3Identities().TouchLastUsedAt(ctx, mapping.ID, now); terr != nil {
				// Non-fatal — touch failure shouldn't block login.
				_ = terr
			}
		} else if !errors.Is(merr, web3identitydomain.ErrNotFound) {
			// Unexpected DB error — surface it (will roll back the transaction).
			return apperrors.WrapInternalf(merr, "lookup web3 identity for credential %s", credential.ID)
		}

		sessionID, err := s.UUIDGenerator.NewString()
		if err != nil {
			return apperrors.NewInternal("generate session id")
		}
		sid, err := newSessionSid()
		if err != nil {
			return err
		}

		sess := &sessionsdomain.Session{
			ID:               sessionID,
			Sid:              sid,
			UserID:           user.ID,
			Username:         user.Username,
			OrganizationID:   organization.ID,
			OrganizationSlug: organization.Slug,
			SessionState:     statusActive,
			DeviceInfo: func() map[string]any {
				info := map[string]any{paramLoginMethod: "webauthn"}
				if web3Address != "" {
					info["web3Address"] = web3Address
					info["web3Derivation"] = "keccak256-passkey"
				}
				return info
			}(),
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

		loginResponse, err := s.handleLoggedIn(ctx, tx, loginCommand, organization, application, user, sid, now)
		if err != nil {
			return err
		}

		loginResponse.DeviceInfo = sess.DeviceInfo
		result = &LoginResult{SessionID: sessionID, Response: *loginResponse}
		issuedSessionID = sessionID
		return nil
	})
	if err != nil {
		return nil, "", err
	}

	return result, issuedSessionID, nil
}

// webAuthnFailureReason 把 VerifyAssertion 的哨兵错误映射为稳定的 details.reason。
// 状态码保持 422/401 不变；前端据此给出可操作文案，而不是把一切断言失败显示成
// 「请检查标红的字段后重试」。
func webAuthnFailureReason(err error) string {
	switch {
	case errors.Is(err, platformwebauthn.ErrChallengeMismatch):
		return "challenge_mismatch"
	case errors.Is(err, platformwebauthn.ErrOriginNotAllowed):
		return "origin_not_allowed"
	case errors.Is(err, platformwebauthn.ErrRPIDMismatch):
		return "rp_id_mismatch"
	case errors.Is(err, platformwebauthn.ErrUserPresenceRequired):
		return "user_presence_required"
	case errors.Is(err, platformwebauthn.ErrUserVerificationRequired):
		return "user_verification_required"
	case errors.Is(err, platformwebauthn.ErrSignCountRegression):
		return "sign_count_regression"
	case errors.Is(err, platformwebauthn.ErrSignatureInvalid):
		return "signature_invalid"
	case errors.Is(err, platformwebauthn.ErrMalformedAssertion):
		return "malformed_assertion"
	default:
		return "assertion_invalid"
	}
}

// mapWebAuthnUserLookupError 把「用户不存在」折叠成与「未注册凭证」相同的 422，
// 避免匿名 begin 沦为用户名枚举探针；其它数据库错误原样上抛，不被误吞。
func mapWebAuthnUserLookupError(err error) error {
	var appErr *apperrors.Error
	if errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
		return apperrors.NewWithMeta(apperrors.CodeInvalidArgument,
			"no active WebAuthn credentials registered for this user",
			map[string]any{"reason": "credential_not_found"})
	}
	return err
}
