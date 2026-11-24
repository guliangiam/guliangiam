package sessions

import (
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// DeviceInfo keys written by the social-login flow. The access token is
// envelope-encrypted before it reaches this map and is stripped from API DTOs.
const (
	DeviceInfoProviderID          = "providerId"
	DeviceInfoProviderAccessToken = "providerAccessToken"
)

// AssuranceLevel (NIST SP 800-63) values persisted in sessions.assurance_level.
const (
	AssuranceLevelAAL1 = "aal1"
	AssuranceLevelAAL2 = "aal2"
)

// AssuranceLevelForAuthenticationLevel maps a session authentication level
// (password / mfa / fido2) to the matching assurance level. Unknown or empty
// levels stay at AAL1, which is the column default.
func AssuranceLevelForAuthenticationLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "mfa", "fido2":
		return AssuranceLevelAAL2
	default:
		return AssuranceLevelAAL1
	}
}

type Session struct {
	ID               string
	UserID           string
	Username         string
	OrganizationID   string
	OrganizationSlug string
	ApplicationID    string
	ApplicationSlug  string
	// SessionState 是状态机枚举值（见 state_machine.go）。
	// DB 默认 DEFAULT 'active'；Domain 层禁止裸赋值跳过 TryTransition。
	SessionState State
	// Sid is the OIDC session identifier (设计报告 14.12.3). It is deliberately
	// not the primary key: `id` is an internal UUID that appears in the session
	// cookie and in CAS tickets, and a value handed to every relying party must
	// be a dedicated, revocable-in-isolation identifier rather than the handle we
	// use to look the row up.
	Sid        string
	DeviceInfo map[string]any
	IPAddress  string
	UserAgent  string
	// ExpiresAt 语义：绝对超时（absolute_expires_at）。
	// 会话创建时设置为最长存活期（默认 7 天），无论用户是否活跃都会到期。
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
	LastSeenAt time.Time
	UpdatedAt  time.Time
	Revision   int64
	// AuthenticationLevel 标记本次会话的认证等级：password / mfa / fido2。
	// concurrent_session_policy.action=require_mfa 会在超限时触发 MFA challenge；
	// 管理员会话豁免淘汰要求 authentication_level >= "mfa"。
	AuthenticationLevel string
	// AssuranceLevel is the NIST SP 800-63 assurance level (aal1 / aal2).
	// Empty means "not set"; the storage layer derives it from
	// AuthenticationLevel (mfa/fido2 → aal2, otherwise aal1).
	AssuranceLevel string
	// IsPrivileged marks a session elevated through an approved IGA request.
	IsPrivileged bool
	LifecycleID  string
	TicketID     string
}

// Validate checks the session fields for business rule violations.
func (s *Session) Validate() error {
	if s.ID == "" {
		return apperrors.NewInvalidArgument("sessionId is required")
	}
	if s.UserID == "" {
		return apperrors.NewInvalidArgument("userId is required")
	}
	if s.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if s.ExpiresAt.IsZero() {
		return apperrors.NewInvalidArgument("expiresAt is required")
	}
	// SessionState 必须是合法枚举值（空也不行 —— DB DEFAULT 'active' 应覆盖）
	if !IsValid(string(s.SessionState)) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"invalid session state: (want active|idle|expired|revoked)",
			map[string]any{"field": "sessionState", "value": string(s.SessionState), "allowed": AllStateStrings()},
		)
	}
	// Empty is allowed (unset, normalised by the storage layer); any non-empty
	// value must be a supported assurance level.
	if s.AssuranceLevel != "" &&
		s.AssuranceLevel != AssuranceLevelAAL1 &&
		s.AssuranceLevel != AssuranceLevelAAL2 {
		return apperrors.NewWithMeta(
			apperrors.CodeInvalidArgument,
			"invalid assurance level: "+s.AssuranceLevel+" (want aal1|aal2)",
			map[string]any{"field": "assuranceLevel", "value": s.AssuranceLevel, "allowed": []string{AssuranceLevelAAL1, AssuranceLevelAAL2}},
		)
	}
	return nil
}

// IsExpired returns true if the session has passed its absolute expiration time.
func (s *Session) IsExpired(now time.Time) bool {
	return now.After(s.ExpiresAt)
}

// IsRevoked returns true if the session has been revoked.
func (s *Session) IsRevoked() bool {
	return s.RevokedAt != nil
}

// IsActive 报告会话在 Domain 语义下是否仍可用（未过绝对期、未撤销）。
// 闲置超时是组织级策略，由 App 层用 LastSeenAt + idle_timeout 计算，
// 不属于 Domain 的可判定状态；因此这里不做 idle 判定。
func (s *Session) IsActive(now time.Time) bool {
	return !s.IsExpired(now) && !s.IsRevoked()
}

// IsTerminal 报告会话是否处于终态（expired / revoked）——不可再迁移。
func (s *Session) IsTerminal() bool {
	return IsTerminal(string(s.SessionState))
}

// CanTransition 包装 TryTransition，先校验当前 SessionState 合法 + 非终态。
// 给 app 层做业务逻辑判断用：比裸 TryTransition 多一层 Domain 语义。
func (s *Session) CanTransition(to State) bool {
	from := s.SessionState
	if !IsValid(string(from)) {
		return false // 当前状态本身非法，应该先被 Validate 挡住
	}
	return TryTransition(from, to)
}

// SetState 是 Domain 层唯一允许修改 SessionState 的入口。
// 调用方必须传入一个合法的目标状态；非法迁移会返回错误而不是静默接受。
// 这防止了 app 层裸赋值 s.SessionState = "revoked" 绕过状态机。
//
// 返回值：
//   - nil        —— 迁移合法，SessionState 已更新
//   - error      —— 未知目标状态或非法迁移，均以 422 + meta 回答（手册 4.14.1）
func (s *Session) SetState(to State) error {
	if !IsValid(string(to)) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"invalid session state: "+string(to)+" (want active|idle|expired|revoked)",
			map[string]any{"field": "sessionState", "value": string(to), "allowed": AllStateStrings()},
		)
	}
	if !TryTransition(s.SessionState, to) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"illegal session state transition: "+string(s.SessionState)+" → "+string(to),
			map[string]any{"field": "sessionState", "value": string(s.SessionState), "allowed": AllowedTransitions(s.SessionState)},
		)
	}
	s.SessionState = to
	return nil
}
