package users

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	sessionsdomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	tokensdomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// SetPasswordCommand allows an admin to reset a user's password.
type SetPasswordCommand struct {
	OrganizationID string
	UserID         string
	Password       string
	Reason         string
	// ActorID 是已认证调用方的 userID，写入审计归属。
	ActorID string
	// ActorIsPlatformAdmin 由 transport 从 session.PrincipalType 判定。
	ActorIsPlatformAdmin bool
	ActorOrganizationID  string
}

// SetUserPassword 是管理员重置他人口令的入口；口令变更的唯一写路径。
func (s *Service) SetUserPassword(ctx context.Context, command SetPasswordCommand) error {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return err
	}
	if command.UserID == "" {
		return apperrors.NewInvalidArgument("userId is required")
	}
	if command.Password == "" {
		return apperrors.NewInvalidArgument("password is required")
	}

	_, err := s.ResetUserPassword(ctx, ResetUserPasswordCommand{
		OrganizationID:       command.OrganizationID,
		UserID:               command.UserID,
		Password:             command.Password,
		Reason:               command.Reason,
		ActorID:              command.ActorID,
		ActorIsPlatformAdmin: command.ActorIsPlatformAdmin,
		ActorOrganizationID:  command.ActorOrganizationID,
	})
	return err
}

// ListUserWebAuthnCredentials returns WebAuthn credentials for a user.
func (s *Service) ListUserWebAuthnCredentials(ctx context.Context, organizationID, userID string) ([]WebAuthnCredentialItem, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	if userID == "" {
		return nil, apperrors.NewInvalidArgument("userId is required")
	}

	// webauthn_credentials has no organization column; the parent user is the
	// tenant anchor. A foreign user must look exactly like a missing one.
	if _, err := s.UOW.Users().GetByIDForOrganization(ctx, userID, organizationID); err != nil {
		return nil, err
	}

	credentials, err := s.UOW.WebAuthnCredentials().ListByUserID(ctx, userID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "list webauthn credentials")
	}

	items := make([]WebAuthnCredentialItem, 0, len(credentials))
	for _, cred := range credentials {
		items = append(items, toWebAuthnCredentialItem(cred))
	}

	return items, nil
}

// DeleteUserWebAuthnCredential removes a WebAuthn credential for a user.
// AC-AUTH-009 compliance: after deletion, revoke the user's ENTIRE active
// session set + all refresh token families + all remaining access tokens —
// no device keeps working. A fresh session is issued for the caller so they
// don't get logged out of the management console mid-operation.
func (s *Service) DeleteUserWebAuthnCredential(ctx context.Context, organizationID, userID, credentialID, previousSessionID string) (string, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return "", err
	}
	if userID == "" {
		return "", apperrors.NewInvalidArgument("userId is required")
	}
	if credentialID == "" {
		return "", apperrors.NewInvalidArgument("credentialId is required")
	}

	// credentialId in the URL is base64url-encoded authenticator credentialId.
	// Repository.Delete takes the internal UUID primary key — resolve via GetByCredentialID.
	decoded, err := base64.RawURLEncoding.DecodeString(credentialID)
	if err != nil {
		return "", apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "invalid credentialId encoding (expect base64url)", map[string]any{metaKeyField: "credentialId", metaKeyValue: credentialID, metaKeyFormat: "base64url"})
	}

	var newSessionID string
	err = runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// {id} comes from the URL, so ownership must be resolved through the
		// trusted tenant before the credential row is touched. sessions has no
		// organization column; the parent user is the tenant anchor.
		user, err := tx.Users().GetByIDForOrganization(ctx, userID, organizationID)
		if err != nil {
			return err
		}

		cred, err := tx.WebAuthnCredentials().GetByCredentialID(ctx, decoded)
		if err != nil {
			return apperrors.NewNotFound("credential not found")
		}

		// Guard: credential must belong to the target user — prevents cross-user deletion.
		if cred.UserID != userID {
			return apperrors.NewNotFound("credential not found") // don't leak ownership
		}

		if err := tx.WebAuthnCredentials().Delete(ctx, cred.ID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			user.OrganizationID,
			"",
			"webauthn.credential.deleted",
			"WebAuthnCredential",
			cred.ID,
			"webauthn",
			"webauthn.credential.deleted",
			map[string]any{},
		); err != nil {
			return err
		}
		// AC-AUTH-009：删除即级联（撤销全部会话与令牌），并为调用者轮换一个新会话。
		rotated, cascadeErr := s.revokeUserAccessCascade(ctx, tx, userID, previousSessionID)
		if cascadeErr != nil {
			return cascadeErr
		}
		newSessionID = rotated
		return nil
	})
	if err != nil {
		return "", err
	}
	return newSessionID, nil
}

// newSessionSid generates a fresh OIDC session identifier for the rotated session.
// Mirrors the same helper in app/auth — lives here because users.Service cannot
// import auth without a circular dependency.
func newSessionSid() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "sid-" + base64.RawURLEncoding.EncodeToString(b), nil
}

// WebAuthnCredentialStatus 是凭证生命周期状态（别名转发 domain 常量，供 transport
// 层构造命令，避免 transport 直接依赖 domain）。
type WebAuthnCredentialStatus = webauthndomain.CredentialStatus

const (
	WebAuthnCredentialStatusActive      = webauthndomain.CredentialStatusActive
	WebAuthnCredentialStatusRevoked     = webauthndomain.CredentialStatusRevoked
	WebAuthnCredentialStatusDeactivated = webauthndomain.CredentialStatusDeactivated
)

// allowedWebAuthnCredentialStatuses is the closed set a status update accepts;
// the switch below and the 422 `allowed` metadata read the same list.
var allowedWebAuthnCredentialStatuses = []string{
	string(WebAuthnCredentialStatusActive),
	string(WebAuthnCredentialStatusRevoked),
	string(WebAuthnCredentialStatusDeactivated),
}

// UpdateWebAuthnCredentialStatusCommand 迁移凭证生命周期状态（§31.3.3）。
type UpdateWebAuthnCredentialStatusCommand struct {
	OrganizationID    string
	UserID            string
	CredentialID      string
	Status            webauthndomain.CredentialStatus
	PreviousSessionID string
}

// UpdateWebAuthnCredentialStatus 把凭证置为 revoked（吊销）或 deactivated（停用），
// 或把 deactivated 的凭证恢复为 active（恢复）。
//
// 状态机（§31.3.3，revoked 为终态）：
//
//	active      → revoked      吊销（不可恢复）
//	active      → deactivated  停用（可恢复）
//	deactivated → active       恢复
//	deactivated → revoked      吊销
//
// 吊销与停用会走 AC-AUTH-009 级联：撤销该用户全部会话与令牌——失效的认证因素不能让
// 任何设备继续持有会话；恢复不动会话（用户下次自行登录）。
//
// 恢复本应要求 step-up 重认证（§31.3.3）；平台的 step-up 设施尚未实现，因此当前只校验
// 调用方的 users:write 权限并写审计，差异已登记在 §31.2 缺陷总表。
func (s *Service) UpdateWebAuthnCredentialStatus(ctx context.Context, command UpdateWebAuthnCredentialStatusCommand) (string, error) {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return "", err
	}
	if command.UserID == "" {
		return "", apperrors.NewInvalidArgument("userId is required")
	}
	if command.CredentialID == "" {
		return "", apperrors.NewInvalidArgument("credentialId is required")
	}
	switch command.Status {
	case webauthndomain.CredentialStatusRevoked, webauthndomain.CredentialStatusDeactivated, webauthndomain.CredentialStatusActive:
	default:
		return "", apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, fmt.Sprintf("unsupported credential status %q", command.Status), map[string]any{metaKeyField: "status", metaKeyValue: string(command.Status), "allowed": append([]string{}, allowedWebAuthnCredentialStatuses...)})
	}

	decoded, err := base64.RawURLEncoding.DecodeString(command.CredentialID)
	if err != nil {
		return "", apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "invalid credentialId encoding (expect base64url)", map[string]any{metaKeyField: "credentialId", metaKeyValue: command.CredentialID, metaKeyFormat: "base64url"})
	}

	var newSessionID string
	err = runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// {id} comes from the URL, so ownership must be resolved through the
		// trusted tenant before the credential row is touched. sessions has no
		// organization column; the parent user is the tenant anchor.
		user, err := tx.Users().GetByIDForOrganization(ctx, command.UserID, command.OrganizationID)
		if err != nil {
			return err
		}
		cred, err := tx.WebAuthnCredentials().GetByCredentialID(ctx, decoded)
		if err != nil {
			return apperrors.NewNotFound("credential not found")
		}
		// 属主不符按「找不到」处理，不泄露凭据归属。
		if cred.UserID != command.UserID {
			return apperrors.NewNotFound("credential not found")
		}

		auditAction, err := credentialStatusTransition(cred.Status, command.Status)
		if err != nil {
			return err
		}

		if err := tx.WebAuthnCredentials().UpdateStatus(ctx, cred.ID, command.Status); err != nil {
			return apperrors.WrapInternal(err, "update webauthn credential status")
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			user.OrganizationID,
			"",
			auditAction,
			"WebAuthnCredential",
			cred.ID,
			auditAction,
			auditAction,
			map[string]any{"from": string(cred.Status), "to": string(command.Status)},
		); err != nil {
			return err
		}

		if command.Status == webauthndomain.CredentialStatusActive {
			return nil // 恢复不触碰会话
		}

		rotated, cascadeErr := s.revokeUserAccessCascade(ctx, tx, command.UserID, command.PreviousSessionID)
		if cascadeErr != nil {
			return cascadeErr
		}
		newSessionID = rotated
		return nil
	})
	if err != nil {
		return "", err
	}
	return newSessionID, nil
}

// credentialStatusTransition 校验状态迁移并返回审计动作名。
// 同状态重复操作与「revoked → 任何状态」都按冲突拒绝：前者掩盖调用方的状态判断错误，
// 后者会破坏「吊销是终态」这一契约（PUT/POST 幂等不能以放宽状态机为代价）。
func credentialStatusTransition(from, to webauthndomain.CredentialStatus) (string, error) {
	if from == to {
		return "", apperrors.NewConflictf("credential is already %s", from)
	}
	switch {
	case from == webauthndomain.CredentialStatusActive && to == webauthndomain.CredentialStatusRevoked:
		return "webauthn.credential.revoked", nil
	case from == webauthndomain.CredentialStatusDeactivated && to == webauthndomain.CredentialStatusRevoked:
		return "webauthn.credential.revoked", nil
	case from == webauthndomain.CredentialStatusActive && to == webauthndomain.CredentialStatusDeactivated:
		return "webauthn.credential.deactivated", nil
	case from == webauthndomain.CredentialStatusDeactivated && to == webauthndomain.CredentialStatusActive:
		return "webauthn.credential.restored", nil
	case from == webauthndomain.CredentialStatusRevoked:
		return "", apperrors.NewConflict("revoked credential is terminal and cannot be reactivated; the user must register a new credential")
	default:
		return "", apperrors.NewConflictf("illegal credential status transition %s → %s", from, to)
	}
}

// revokeUserAccessCascade 撤销用户的全部会话与令牌（AC-AUTH-009），并为调用者轮换一个
// 新会话（否则管理员会把自己踢下线）。凭证吊销/停用与删除共用这段逻辑——一处失效的
// 认证因素不能让任何设备继续持有会话。
func (s *Service) revokeUserAccessCascade(ctx context.Context, tx appuow.UnitOfWork, userID, previousSessionID string) (string, error) {
	// 先抓调用者自己的会话上下文：下面的 RevokeByUser 会让这行消失。
	var oldSess *sessionsdomain.Session
	if strings.TrimSpace(previousSessionID) != "" {
		if sess, getErr := tx.Sessions().GetByID(ctx, previousSessionID); getErr == nil {
			oldSess = sess
		}
	}

	now := s.Clock.Now().UTC()

	// 1) 该用户全部活跃会话——每个已登录设备都下线。
	if _, err := tx.Sessions().RevokeByUser(ctx, userID, now); err != nil {
		return "", err
	}
	// 2) 全部刷新令牌及其配对访问令牌（CTE 在查询内部）。
	if _, err := tx.Tokens().RevokeRefreshByUser(ctx, userID, now); err != nil {
		return "", err
	}
	// 3) 剩余访问令牌（未与刷新令牌配对的直接授权、设备码等）。
	if _, err := tx.Tokens().RevokeByQuery(ctx, tokensdomain.RevokeQuery{UserID: userID}, now); err != nil {
		return "", err
	}

	if oldSess == nil {
		// 调用者没有会话 Cookie（服务间调用），无需轮换。
		return "", nil
	}

	sessID, idErr := s.UUID.NewString()
	if idErr != nil {
		return "", apperrors.NewInternal("generate rotated session id")
	}
	sid, sidErr := newSessionSid()
	if sidErr != nil {
		return "", sidErr
	}
	if cErr := tx.Sessions().Create(ctx, &sessionsdomain.Session{
		ID:               sessID,
		Sid:              sid,
		UserID:           oldSess.UserID,
		Username:         oldSess.Username,
		OrganizationID:   oldSess.OrganizationID,
		OrganizationSlug: oldSess.OrganizationSlug,
		ApplicationID:    oldSess.ApplicationID,
		ApplicationSlug:  oldSess.ApplicationSlug,
		SessionState:     sessionsdomain.StateActive,
		DeviceInfo:       oldSess.DeviceInfo,
		IPAddress:        oldSess.IPAddress,
		UserAgent:        oldSess.UserAgent,
		ExpiresAt:        now.Add(30 * 24 * time.Hour),
		CreatedAt:        now,
		LastSeenAt:       now,
		UpdatedAt:        now,
		Revision:         1,
	}); cErr != nil {
		return "", cErr
	}
	return sessID, nil
}
