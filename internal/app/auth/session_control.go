package auth

import (
	"context"
	"log/slog"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// 并发会话超限时的处置动作。revoke_oldest / reject_new 已实现；
// require_mfa 是《开发设计》22.5 的目标态动作，尚未实现——执行时 fail-closed 拒绝新会话，
// 不静默降级为 revoke_oldest（那等于绕过租户策略）。
const (
	concurrentActionRevokeOldest = "revoke_oldest"
	concurrentActionRejectNew    = "reject_new"
	concurrentActionRequireMFA   = "require_mfa"

	// defaultMaxActiveSessions 是 concurrentSessionPolicy=limit 未显式给出上限时的缺省值
	// （《开发设计》22.5 的 max_active_sessions 缺省 10）。
	defaultMaxActiveSessions = 10
)

// ConcurrentSessionPolicyConfig represents the concurrent session policy for an organization.
// MaxSessions = 0 表示不限。
type ConcurrentSessionPolicyConfig struct {
	MaxSessions int    `json:"maxSessions"`
	Action      string `json:"action"`
}

// defaultConcurrentSessionPolicy returns the default concurrent session policy.
func defaultConcurrentSessionPolicy() ConcurrentSessionPolicyConfig {
	return ConcurrentSessionPolicyConfig{
		MaxSessions: 0, // 0 means unlimited
		Action:      "revoke_oldest",
	}
}

// resolveConcurrentSessionPolicy reads the concurrent session policy from the organization's security policy.
//
// 支持两套键，目标态键优先（《开发设计》22.5）：
//   - 目标态：concurrentSessionPolicy ∈ unlimited|limit|single，上限 maxActiveSessions，
//     动作 evict_oldest|reject_new|require_mfa；
//   - 历史键：maxSessions + action（revoke_oldest|reject_new）。仅在未写
//     concurrentSessionPolicy 时使用，保证既有租户配置语义不变。
//
// 目标态键写了一个无法识别的值时按 limit 处理（宁可从严也不静默放行），缺省上限 10。
func resolveConcurrentSessionPolicy(org *orgdomain.Organization) ConcurrentSessionPolicyConfig {
	policy := defaultConcurrentSessionPolicy()

	if org == nil || len(org.Security.ConcurrentSessionPolicy) == 0 {
		return policy
	}

	csp := org.Security.ConcurrentSessionPolicy

	policyKind := ""
	if v, ok := strFromMap(csp, "concurrentSessionPolicy"); ok {
		policyKind = strings.ToLower(strings.TrimSpace(v))
	}

	switch policyKind {
	case "":
		// 未使用目标态键：按历史键解析（0 或缺省 = 不限）。
		if v, ok := intFromMap(csp, "maxSessions"); ok {
			policy.MaxSessions = v
		}
	case "unlimited":
		policy.MaxSessions = 0
	case "single":
		// single 是 limit 在 maxActiveSessions=1 时的特例，共用同一实现（22.5）。
		policy.MaxSessions = 1
	default: // "limit" 及无法识别的取值
		policy.MaxSessions = defaultMaxActiveSessions
		if v, ok := intFromMap(csp, "maxActiveSessions"); ok && v > 0 {
			policy.MaxSessions = v
		} else if v, ok := intFromMap(csp, "maxSessions"); ok && v > 0 {
			policy.MaxSessions = v
		}
	}

	if v, ok := strFromMap(csp, "action"); ok {
		switch action := strings.ToLower(strings.TrimSpace(v)); action {
		case "evict_oldest":
			// 目标态动作名，语义等同已实现的 revoke_oldest。
			policy.Action = concurrentActionRevokeOldest
		case concurrentActionRevokeOldest, concurrentActionRejectNew, concurrentActionRequireMFA:
			policy.Action = action
		}
	}

	return policy
}

// preCheckRequireMFA 在 session 创建前检查并发会话策略是否触发 require_mfa。
// 这是 require_mfa 两阶段拆分的第一阶段（pre-session）——还没有新会话存在，
// 所以不需要 excludeSessionID。
//
// 返回值：
//   - (true, nil)    → 超限 + require_mfa + authLevel < mfa + 用户有 MFA 方法
//     调用方必须创建 MFA challenge 并返回中间态。
//   - (false, nil)   → 不触发（未超限 / 不是 require_mfa / authLevel 已够高）。
//   - (false, err)   → 用户没有可用的 MFA 方法（无法满足 require_mfa 策略），
//     或其他内部错误。调用方应 fail-closed。
//
// 此函数**不锁用户行**——假设调用方已经在事务内（如 login_flow.go 的 WithinTransaction），
// 并且用户行锁会在后续 enforceConcurrentSessionLimitWithTx 里获取。
// 这里只做 read-only 检查，不需要互斥（真正的互斥在 enforce 里）。
//
// 竞态说明：pre-check 读到的活跃会话数可能在 session 创建前被其他会话的并发操作（如
// 另一个 revoke）改变——如果另一个会话被撤销导致降到限额以下，用户会收到一个不必要的
// MFA challenge（但 MFA 完成后仍然会成功创建 session）。这是 fail-safe 行为：
// 宁可多挑战一次，也不要漏放过超限用户。
func preCheckRequireMFA(ctx context.Context, uow appuow.UnitOfWork, user *userdomain.User, orgID, currentAuthLevel string) (bool, error) {
	// require_mfa 语义：只有并发策略明确要求才检查
	org, err := uow.Organizations().GetByID(ctx, orgID)
	if err != nil {
		return false, err
	}

	policy := resolveConcurrentSessionPolicy(org)
	if policy.MaxSessions <= 0 || policy.Action != concurrentActionRequireMFA {
		return false, nil
	}

	// 用户已经过 MFA 或 fido2（本次登录中），直接放行——不需要再挑战一次
	if authLevelRank(currentAuthLevel) >= authLevelRank(loginMethodMFA) {
		return false, nil
	}

	// 管理员 + 已有 MFA 认证的会话 → 豁免 require_mfa（管理员豁免扩展：require_mfa 也豁免）
	// 但这是 pre-check，不知道管理员之前的会话是不是 MFA 级别——所以不做豁免。
	// 管理员豁免只在 revoke_oldest 里生效（已经有会话被淘汰时跳过管理员 MFA 会话）。

	activeSessions, _, err := uow.Sessions().List(ctx, sessiondomain.ListQuery{
		UserID:     user.ID,
		PageSize:   policy.MaxSessions + 10,
		ActiveOnly: boolPtr(true),
	})
	if err != nil {
		return false, apperrors.WrapInternal(err, "pre-check require_mfa: list active sessions")
	}

	// pre-session：当前活跃数 + 即将创建的新会话 = 超限判定
	if len(activeSessions) < policy.MaxSessions {
		return false, nil
	}

	// 超限 + require_mfa + authLevel < mfa → 需要 MFA challenge
	// 但首先检查用户有没有可用的 MFA 方法——没有就无法满足策略，fail-closed
	if len(enabledVerifiedMfaMethods(user)) == 0 {
		return false, apperrors.NewInvalidArgumentf(
			"concurrent session policy require_mfa requires verified MFA methods, but user has none configured (max %d)",
			policy.MaxSessions,
		)
	}

	slog.InfoContext(ctx,
		"require_mfa pre-check triggered",
		"user_id", user.ID,
		"organization_id", orgID,
		"active_sessions", len(activeSessions),
		"policy_max_sessions", policy.MaxSessions,
	)

	return true, nil
}

// enforceConcurrentSessionLimitWithTx enforces the concurrent session limit within an existing transaction.
//
// 这里没有非事务的公开变体：行级锁只在事务内有效，脱离事务的「限流」既拿不到锁也无法与
// 会话写入原子，留着只会被误用（原先的 EnforceConcurrentSessionLimit 全仓无调用方，已删）。
//
// userIsAdmin 用于管理员会话豁免淘汰（OPEN-API-18）：当 revoke_oldest 动作执行时，
// authentication_level >= "mfa" 且 user.IsAdmin 的会话不参与淘汰，仅在审计中告警。
//
// currentAuthLevel 是**本次新创建会话**的认证等级：
//   - require_mfa 分支：authLevel >= "mfa" → 放行（用户刚过了 MFA challenge）；
//     authLevel < "mfa" → fail-closed（pre-check 应该已经拦住了，兜底）
//   - revoke_oldest 分支：不关心 authLevel（管理员豁免用的是**已有会话**的 level）
func enforceConcurrentSessionLimitWithTx(ctx context.Context, tx appuow.UnitOfWork, userID, orgID, excludeSessionID string, now time.Time, userIsAdmin bool, currentAuthLevel string) error {
	org, err := tx.Organizations().GetByID(ctx, orgID)
	if err != nil {
		return err
	}

	policy := resolveConcurrentSessionPolicy(org)

	if policy.MaxSessions <= 0 {
		return nil
	}

	// 《开发设计》22.5 要求「计数 + 淘汰」在同一 user_id 的锁内完成：否则两个并发登录
	// 都会读到「未超限」而同时放行（single 策略下就是两个会话共存）。锁锚点是用户行，
	// 不是会话行——首次登录时会话行还不存在，锁不存在的行不会互斥。
	if err := tx.Users().LockByID(ctx, userID); err != nil {
		return err
	}

	return enforceSessionLimit(ctx, tx, userID, excludeSessionID, policy, now, userIsAdmin, currentAuthLevel)
}

func enforceSessionLimit(ctx context.Context, uow appuow.UnitOfWork, userID, excludeSessionID string, policy ConcurrentSessionPolicyConfig, now time.Time, userIsAdmin bool, currentAuthLevel string) error {
	activeSessions, _, err := uow.Sessions().List(ctx, sessiondomain.ListQuery{
		UserID:     userID,
		PageSize:   policy.MaxSessions + 10, // fetch a few extra to be safe
		ActiveOnly: boolPtr(true),
	})
	if err != nil {
		return apperrors.WrapInternal(err, "list active sessions for concurrent session limit")
	}

	// Count active sessions excluding the newly created one.
	var currentActive []*sessiondomain.Session
	for _, sess := range activeSessions {
		if sess.ID != excludeSessionID {
			currentActive = append(currentActive, sess)
		}
	}

	// If we are within the limit, no action needed.
	if len(currentActive) < policy.MaxSessions {
		return nil
	}

	// Determine how many sessions to revoke.
	// The new session counts as one, so we need currentActive < maxSessions.
	excess := len(currentActive) - policy.MaxSessions + 1 // +1 because the new session counts too
	if excess <= 0 {
		return nil
	}

	// Apply the configured action.
	switch policy.Action {
	case concurrentActionRejectNew:
		// Reject the new login by returning an error.
		return apperrors.NewInvalidArgumentf("maximum number of concurrent sessions (%d) exceeded", policy.MaxSessions)

	case concurrentActionRequireMFA:
		// OPEN-API-18: require_mfa 两阶段拆分的 post-session 校验。
		// 此时新会话刚创建，authLevel 是本次登录的认证等级。
		// authLevel >= "mfa" → 用户刚过了 MFA challenge（pre-check 触发的中间态完成），放行。
		// authLevel < "mfa" → pre-check 应该已经拦住了（fail-closed 并返回 challenge），
		//   能走到这里是异常路径（非密码登录路径 + require_mfa 策略），兜底 fail-closed。
		if authLevelRank(currentAuthLevel) >= authLevelRank(loginMethodMFA) {
			slog.InfoContext(ctx, "require_mfa satisfied by post-login MFA verification",
				"user_id", userID, "auth_level", currentAuthLevel)
			return nil
		}
		return apperrors.NewInvalidArgumentf(
			"concurrent session policy require_mfa requires MFA verification before new session (max %d)",
			policy.MaxSessions,
		)

	default:
		// Revoke the oldest sessions. Sessions are typically returned ordered
		// by created_at DESC (newest first), so the oldest are at the end.
		toRevokeCount := excess
		if toRevokeCount > len(currentActive) {
			toRevokeCount = len(currentActive)
		}

		// OPEN-API-18: 管理员会话豁免淘汰。
		// authentication_level >= "mfa" 且 user.IsAdmin 的会话不参与淘汰。
		// 淘汰时跳过这些会话，继续从下一个 oldest 选。
		// 注意：这里的 userIsAdmin 是当前登录用户（所有被淘汰的会话都属于同一用户）。
		skippedAdminMFA := 0
		for i := len(currentActive) - 1; i >= 0 && toRevokeCount > 0; i-- {
			sess := currentActive[i]
			// 用 authLevelRank 做数值比较，避免 Go 字典序陷阱
			// ('password' >= 'mfa' → TRUE, 因为 p > m)
			if userIsAdmin && authLevelRank(sess.AuthenticationLevel) >= authLevelRank(loginMethodMFA) {
				skippedAdminMFA++
				continue
			}
			_ = uow.Sessions().Revoke(ctx, sess.ID, now)
			toRevokeCount--
		}
		if skippedAdminMFA > 0 {
			slog.WarnContext(ctx,
				"admin MFA-authenticated sessions exempted from concurrent-session revocation",
				"user_id", userID,
				"skipped", skippedAdminMFA,
				"policy_max_sessions", policy.MaxSessions,
			)
		}
	}

	return nil
}

// authLevelRank 把 authentication_level 映射为数值等级，用于比较"是否至少 MFA 级别"。
// Go 字符串字典序比较 ('password' >= 'mfa' → TRUE, 因为 p > m) 是错的，
// 必须显式用数值等级比较。未知值返回 0（最低），fallback 安全。
func authLevelRank(level string) int {
	switch level {
	case loginMethodFido2:
		return 3
	case loginMethodMFA:
		return 2
	case loginMethodPassword:
		return 1
	default:
		return 0
	}
}

// strFromMap extracts a string value from a map[string]any.
func strFromMap(m map[string]any, key string) (string, bool) {
	if m == nil {
		return "", false
	}
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return s, true
}
