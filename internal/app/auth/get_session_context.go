package auth

import (
	"context"
	"strings"
	"time"

	"github.com/guliangiam/guliangiam/internal/app/sessionpolicy"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	domainuow "github.com/guliangiam/guliangiam/internal/domain/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// ResolveSessionContext 根据 session cookie ID 查完整会话身份。
//
// ⚠️ 关键设计：缓存未命中时必须走 TenantFilterSystem RLS 绕过。
// sessions / users / organizations / applications 四张表全 FORCE ROW LEVEL SECURITY。
// 在尚未知道"当前用户是谁"之前（还没查 session），我们不可能提前注入
// app.current_organization_id — 此时 RLS tenant_isolation policy 求值：
//
//	is_super_admin() = false（没设 role）
//	AND current_organization_id() IS NULL
//
// → UNKNOWN → RLS 默认 DENY ALL → session 永远查不到 → 所有带 cookie 的请求 401。
//
// TenantFilterSystem 是框架层系统任务语义：根据 cookie 反查身份是认证中间件的
// 固有职责，不涉及业务层跨租户特权。映射到 super_admin role → is_super_admin()=true
// → RLS 放行。
//
// 性能（PERF-01）：会话缓存命中时不开启事务。此时只需要读取组织的 session
// policy，用会话自身的租户作用域在连接池上执行一次窄查询（单列单行），
// 不读 organizations 聚合（8 表 JOIN + 密钥解密），也不再产生 BEGIN/COMMIT。
// 缓存未命中时保持原有事务回源路径，语义完全一致。
// ResolveSessionContext resolves a session without request-client metadata.
// Callers that have the HTTP request (or another client identity) should use
// ResolveSessionContextForClient so an enabled client-binding policy can be
// enforced.
func (s *Service) ResolveSessionContext(ctx context.Context, sessionID string) (*SessionIdentity, error) {
	return s.resolveSessionContext(ctx, sessionID, "", "")
}

// ResolveSessionContextForClient resolves a session and enforces the
// organization's optional IP/user-agent binding: when the bindings are enabled
// and the request client differs, the session is rejected with 401 so the user
// re-authenticates (ADR-013; re-auth instead of silent logout).
func (s *Service) ResolveSessionContextForClient(ctx context.Context, sessionID, clientIP, clientUserAgent string) (*SessionIdentity, error) {
	return s.resolveSessionContext(ctx, sessionID, clientIP, clientUserAgent)
}

func (s *Service) resolveSessionContext(ctx context.Context, sessionID, clientIP, clientUserAgent string) (*SessionIdentity, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, apperrors.NewUnauthorized("session id is required")
	}

	now := s.Clock.Now().UTC()

	// 快路径：会话来自进程级缓存 → 不需要事务，也不需要读 sessions 表。
	// CachedSession 只在条目未过期、未撤销时返回 true，与 CachedRepository
	// 的判定完全一致；任何偏差都会回落到下面的事务路径。
	if cached, ok := s.cachedSession(ctx, sessionID); ok {
		return s.resolveCachedSession(ctx, cached, now, clientIP, clientUserAgent)
	}

	return s.resolveSessionFromStore(ctx, sessionID, now, clientIP, clientUserAgent)
}

// cachedSession reads the optional session-cache capability without touching
// the database. UoWs that do not implement it (MySQL today, test fakes) simply
// fall back to the transactional path.
func (s *Service) cachedSession(ctx context.Context, sessionID string) (*sessiondomain.Session, bool) {
	cacheUoW, ok := s.UOW.(appuow.SessionCacheUoW)
	if !ok {
		return nil, false
	}
	return cacheUoW.CachedSession(ctx, sessionID)
}

// resolveCachedSession is the PERF-01 fast path: session row from cache, one
// narrow RLS-scoped policy read (single-query transaction) for the session's
// own organization; sessions/users are never touched.
func (s *Service) resolveCachedSession(ctx context.Context, session *sessiondomain.Session, now time.Time, clientIP, clientUserAgent string) (*SessionIdentity, error) {
	policyCtx := ctx
	if session.OrganizationID != "" {
		// Scope the read to the session's own tenant so the policy lookup
		// stays behind RLS instead of needing a system bypass.
		policyCtx = tenantctx.WithTenantScope(ctx, session.OrganizationID, session.OrganizationSlug, tenantctx.TenantFilterStrict)
	}

	// organizations 为 FORCE RLS 表：tenantctx 仅随 ctx 传导，池级裸读不会
	// 获得事务局部 GUC，必须经 WithinTransaction 由 setTenantContext 设置
	// app.current_organization_id 后才能看见本组织行。
	var policy sessionpolicy.Policy
	if txErr := s.UOW.WithinTransaction(policyCtx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		var resolveErr error
		policy, resolveErr = sessionpolicy.ResolvePolicy(ctx, tx.Organizations(), session.OrganizationID)
		return resolveErr
	}); txErr != nil {
		return nil, wrapTransactionError(txErr, "resolve session policy")
	}

	identity, mismatch, binding, touch, evalErr := evaluateSession(session, policy, now, clientIP, clientUserAgent)
	if evalErr != nil {
		if mismatch != nil {
			s.recordSessionClientMismatch(ctx, mismatch, clientIP, clientUserAgent, binding)
		}
		return nil, wrapTransactionError(evalErr, "resolve session")
	}
	if touch {
		// last_seen 更新移出鉴权读路径：它只在节流窗口（≥1 分钟、默认 5 分钟）
		// 到达时触发，且使用独立事务，不再把读请求变成读写事务。
		if err := s.touchSessionActivity(ctx, session.ID, now); err != nil {
			return nil, wrapTransactionError(err, "touch session activity")
		}
	}

	return identity, nil
}

// resolveSessionFromStore is the authoritative cache-miss path. It keeps the
// existing transactional read (RLS needs the system role) but performs no
// writes inside that transaction: the activity touch runs afterwards in its
// own short transaction.
func (s *Service) resolveSessionFromStore(ctx context.Context, sessionID string, now time.Time, clientIP, clientUserAgent string) (*SessionIdentity, error) {
	// 用 TenantFilterSystem ctx 包事务：WithinTransaction 内部 setTenantContext
	// 会把 system mode 映射到 super_admin role → 所有 FORCE RLS 表放行。
	systemCtx := tenantctx.System(ctx)

	var (
		result         *SessionIdentity
		clientMismatch *sessiondomain.Session
		clientBinding  sessionpolicy.ClientBinding
		touch          bool
	)

	err := s.UOW.WithinTransaction(systemCtx, func(txCtx context.Context, tx domainuow.UnitOfWork) error {
		// ⚠️ 必须用 tx.Sessions()，不能用 s.UOW.Sessions()！
		// s.UOW 是原始 pgxpool Pool，调它的 repo.GetByID 会在 Pool 连接上跑，
		// 那个连接没有 set_tenant_context 注入的 super_admin role → RLS 仍 fail-closed。
		// tx 是 WithinTransaction 里创建的事务内 UOW，用 tx (pgx.Tx) 跑 SQL —
		// setTenantContext 已经在 tx 上调用了 set_config('app.current_role','super_admin',true)
		// （第三个参数 true = LOCAL，事务内有效），所以 tx 里所有查询 RLS 放行。
		session, err := tx.Sessions().GetByID(txCtx, sessionID)
		if err != nil {
			return err
		}

		policy, err := sessionpolicy.ResolvePolicy(txCtx, tx.Organizations(), session.OrganizationID)
		if err != nil {
			return err
		}

		identity, mismatch, binding, shouldTouch, evalErr := evaluateSession(session, policy, now, clientIP, clientUserAgent)
		result, clientMismatch, clientBinding, touch = identity, mismatch, binding, shouldTouch
		return evalErr
	})
	if err != nil {
		if clientMismatch != nil {
			s.recordSessionClientMismatch(ctx, clientMismatch, clientIP, clientUserAgent, clientBinding)
		}
		return nil, wrapTransactionError(err, "resolve session transaction")
	}

	if touch {
		if err := s.touchSessionActivity(ctx, sessionID, now); err != nil {
			return nil, wrapTransactionError(err, "touch session activity")
		}
	}

	return result, nil
}

// evaluateSession applies the session policy in the same order as before:
// revoked → absolute/idle expiry → client binding. The touch decision is
// returned rather than performed so callers can keep the read path read-only.
func evaluateSession(session *sessiondomain.Session, policy sessionpolicy.Policy, now time.Time, clientIP, clientUserAgent string) (*SessionIdentity, *sessiondomain.Session, sessionpolicy.ClientBinding, bool, error) {
	if session.RevokedAt != nil {
		return nil, nil, policy.Binding, false, apperrors.NewUnauthorized("session is revoked")
	}
	if !session.ExpiresAt.After(now) || sessionpolicy.IsIdleExpired(now, session.LastSeenAt, policy.IdleTimeout) {
		return nil, nil, policy.Binding, false, apperrors.NewUnauthorized("session is expired")
	}
	if sessionpolicy.ClientMismatch(session.IPAddress, session.UserAgent, clientIP, clientUserAgent, policy.Binding) {
		return nil, session, policy.Binding, false, apperrors.NewUnauthorized("session client changed; re-authentication required")
	}

	touch := sessionpolicy.ShouldTouchActivity(now, session.LastSeenAt, policy.IdleTimeout)
	identity := toSessionIdentity(session)
	return &identity, nil, policy.Binding, touch, nil
}

// touchSessionActivity persists last_seen in a dedicated short transaction.
// It is throttled by sessionpolicy.ShouldTouchActivity (default 5 minutes), so
// the extra transaction only happens on a small fraction of requests, and the
// authentication read itself no longer becomes a write transaction.
func (s *Service) touchSessionActivity(ctx context.Context, sessionID string, now time.Time) error {
	systemCtx := tenantctx.System(ctx)
	return s.UOW.WithinTransaction(systemCtx, func(txCtx context.Context, tx domainuow.UnitOfWork) error {
		return tx.Sessions().Touch(txCtx, sessionID, now)
	})
}
