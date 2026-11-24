package auth

import (
	"context"
	"encoding/json"
	"log/slog"

	actionsapp "github.com/guliangiam/guliangiam/internal/app/actions"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ActionsDispatcher 是 ADR-029 同步扩展点端口（pre_login / pre_token /
// post_registration / consent / mfa）。生产实现是 *actionsapp.Dispatcher；
// nil → 扩展点关闭，所有流程保持原有行为。方法签名与 Dispatcher 完全一致，
// bootstrap 直接字段注入即可。
type ActionsDispatcher interface {
	DispatchPreLogin(ctx context.Context, orgID string, input json.RawMessage) (*actionsapp.DispatchResult, error)
	DispatchPreToken(ctx context.Context, orgID string, input json.RawMessage) (*actionsapp.DispatchResult, error)
	DispatchPostRegistration(ctx context.Context, orgID string, input json.RawMessage) (*actionsapp.DispatchResult, error)
	DispatchConsent(ctx context.Context, orgID string, input json.RawMessage) (*actionsapp.DispatchResult, error)
	DispatchMFA(ctx context.Context, orgID string, input json.RawMessage) (*actionsapp.DispatchResult, error)
}

// 安全红线：execution_logs.Input 会落库 —— input 只允许元数据
// （user_id/username/organization_id/application_id/channel/grant_type/
// scopes/ip/user_agent）。密码、令牌、授权码、assertion、
// deviceCredentialID 等凭据一律不得进入 input 或日志。

func actionsInput(input map[string]any) json.RawMessage {
	b, _ := json.Marshal(input)
	return b
}

func actionsBaseInput(orgID, userID, username, applicationID string) map[string]any {
	input := map[string]any{"organization_id": orgID}
	if userID != "" {
		input["user_id"] = userID
		input["username"] = username
	}
	if applicationID != "" {
		input["application_id"] = applicationID
	}
	return input
}

// enforceActionsPreLogin 是凭证验证后的登录门禁（fail-closed）：
// deny → 401 通用文案；dispatch 出错 → 拒绝登录。Reasons 只进服务端
// 日志与审计，不回显给调用方。
func (s Service) enforceActionsPreLogin(ctx context.Context, channel, orgID, userID, username, applicationID, ip, userAgent string) error {
	if s.Actions == nil {
		return nil
	}
	input := actionsBaseInput(orgID, userID, username, applicationID)
	input["channel"] = channel
	if ip != "" {
		input["ip"] = ip
	}
	if userAgent != "" {
		input["user_agent"] = userAgent
	}
	result, err := s.Actions.DispatchPreLogin(ctx, orgID, actionsInput(input))
	return s.actionsGateOutcome(ctx, "actions.pre_login.denied", orgID, userID, input, result, err,
		apperrors.NewUnauthorized("login denied by policy"))
}

// enforceActionsPreToken 是令牌签发前的门禁（fail-closed）。denyErr 由
// 各 grant 按自身协议错误风格传入（invalid_grant / access_denied）。
func (s Service) enforceActionsPreToken(ctx context.Context, orgID, userID, username, applicationID, grantType string, scopes []string, denyErr error) error {
	if s.Actions == nil {
		return nil
	}
	input := actionsBaseInput(orgID, userID, username, applicationID)
	input["grant_type"] = grantType
	if len(scopes) > 0 {
		input["scopes"] = scopes
	}
	result, err := s.Actions.DispatchPreToken(ctx, orgID, actionsInput(input))
	return s.actionsGateOutcome(ctx, "actions.pre_token.denied", orgID, userID, input, result, err, denyErr)
}

// enforceActionsConsent 是 consent 写入门禁（fail-closed）：deny → 403。
func (s Service) enforceActionsConsent(ctx context.Context, orgID, userID, username, applicationID string, scopes []string) error {
	if s.Actions == nil {
		return nil
	}
	input := actionsBaseInput(orgID, userID, username, applicationID)
	if len(scopes) > 0 {
		input["scopes"] = scopes
	}
	result, err := s.Actions.DispatchConsent(ctx, orgID, actionsInput(input))
	return s.actionsGateOutcome(ctx, "actions.consent.denied", orgID, userID, input, result, err,
		apperrors.NewForbidden("consent denied by policy"))
}

// enforceActionsMFA 是 MFA 挑战验证通过后的门禁（fail-closed）：deny → 401。
func (s Service) enforceActionsMFA(ctx context.Context, orgID, userID, username, applicationID, mfaType string) error {
	if s.Actions == nil {
		return nil
	}
	input := actionsBaseInput(orgID, userID, username, applicationID)
	if mfaType != "" {
		input["mfa_type"] = mfaType
	}
	result, err := s.Actions.DispatchMFA(ctx, orgID, actionsInput(input))
	return s.actionsGateOutcome(ctx, "actions.mfa.denied", orgID, userID, input, result, err,
		apperrors.NewUnauthorized("login denied by policy"))
}

// dispatchPostRegistration 在注册事务提交后触发（ADR-029 EXT-004，
// fail-open）：失败只记日志 + 审计，绝不阻塞注册结果。
func (s Service) dispatchPostRegistration(ctx context.Context, orgID, userID, username, applicationID, registerType string) {
	if s.Actions == nil {
		return
	}
	input := actionsBaseInput(orgID, userID, username, applicationID)
	if registerType != "" {
		input["register_type"] = registerType
	}
	result, err := s.Actions.DispatchPostRegistration(ctx, orgID, actionsInput(input))
	if err == nil && (result == nil || result.Allow) {
		return
	}
	s.recordActionsOutcome(ctx, "actions.post_registration.failed", orgID, userID, input, result, err)
}

// actionsGateOutcome 把 dispatch 结果映射成门禁裁决：出错 → fail-closed
// 503；Allow=false → 调用方协议风格的拒绝错误。两种拒绝都先落审计。
func (s Service) actionsGateOutcome(ctx context.Context, event, orgID, actorID string, input map[string]any, result *actionsapp.DispatchResult, dispatchErr, denyErr error) error {
	if dispatchErr != nil {
		s.recordActionsOutcome(ctx, event, orgID, actorID, input, nil, dispatchErr)
		return apperrors.NewUnavailable("actions policy engine unavailable")
	}
	if result != nil && !result.Allow {
		s.recordActionsOutcome(ctx, event, orgID, actorID, input, result, nil)
		return denyErr
	}
	return nil
}

// recordActionsOutcome 在独立事务里写拒绝/失败审计：门禁拒绝会让外层
// 业务事务回滚，同事务审计会被一并丢弃，因此这里必须用根 UoW 另开事务
// （与 persistLoginFailureAudit 的 post-rollback sidecar 同一模式）。
// 审计写入本身 fail-open：失败只告警，不改变门禁裁决。
func (s Service) recordActionsOutcome(ctx context.Context, event, orgID, actorID string, input map[string]any, result *actionsapp.DispatchResult, dispatchErr error) {
	payload := map[string]any{"input": input}
	if dispatchErr != nil {
		payload["outcome"] = "error"
		payload["error"] = dispatchErr.Error()
		slog.WarnContext(ctx, "actions dispatch failed",
			"event", event, "organization_id", orgID, "error", dispatchErr)
	} else {
		payload["outcome"] = "denied"
		if result != nil {
			// Reasons 只落服务端审计，绝不进入 HTTP 响应。
			payload["action_ids"] = result.ActionIDs
			payload["reasons"] = result.Reasons
		}
		slog.InfoContext(ctx, "actions policy denied",
			"event", event, "organization_id", orgID, "actor_id", actorID)
	}
	if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		return s.embedAuditOutboxPlain(
			tx, ctx,
			"", "", "",
			orgID, actorID, event, "action", event, "action", event,
			payload,
		)
	}); err != nil {
		slog.WarnContext(ctx, "actions outcome audit failed", "event", event, "error", err)
	}
}
