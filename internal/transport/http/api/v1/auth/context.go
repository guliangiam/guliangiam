package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/actorctx"
	platformaudit "github.com/guliangiam/guliangiam/internal/platform/auditdenial"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

type SessionContext struct {
	// ID is the internal session row's primary key. Transport-layer code that
	// needs to act on a specific session (e.g. exclude "the current session"
	// when revoking consent-related sessions) reads this.
	ID string
	// UserID is the application user's UUID — used by ScopeAuthorizer to check
	// platform_admins membership and by impersonate flows. Phase C-Step 1 added.
	UserID string
	// OrganizationID is the UUID of the organization (not the slug). It drives
	// enforcer resolution in APIAuthorizationMiddleware Gate 5.
	OrganizationID   string
	OrganizationSlug string
	Username         string
	AccessToken      string
	Source           string
	// AuthTime is when the user authenticated, carried so the authorize endpoint
	// can put it in the ID token's auth_time claim (设计报告 14.8).
	AuthTime time.Time
	// Sid is the OIDC session identifier, carried so the logout token will use (设计报告 14.12.3).
	Sid string
	// PrincipalType identifies the subject's authorization tier.
	// "user"            = 普通组织级用户 — Casbin 用 4 参数 matcher（org 维度强制匹配）
	// "platform_admin"  = 平台管理员   — Casbin 用 3 参数 matcher（org 维度通配符，跨租户授权交给 ScopeAuthorizer）
	// "service_account" = 服务账号   — 类似 user 但可能绑定多个 org
	// "system"          = 系统任务   — 跳过 Casbin，由 TenantFilterSystem 处理
	// Phase B-D6 引入，Phase C-Step 1 从 platform_admins 表查真实值（含 60s 缓存）。
	PrincipalType string
	IsPrivileged  bool
	LifecycleID   string
	TicketID      string
}

type sessionContextKey struct{}

func WithSessionContext(ctx context.Context, session SessionContext) context.Context {
	// BE-D-13：会话即可信操作者来源，随会话一并注入审计 actor 通道，
	// 各 app 模块 embedAuditEvent 空 actorID 时回落读取。
	ctx = actorctx.WithActorID(ctx, session.UserID)
	return context.WithValue(ctx, sessionContextKey{}, session)
}

func SessionContextFromContext(ctx context.Context) (SessionContext, bool) {
	session, ok := ctx.Value(sessionContextKey{}).(SessionContext)
	return session, ok
}

// HasValidatedBearerSession 仅当 SessionContextMiddleware 已验证请求头 JWT 时返回 true。
// Cookie 会话和未经认证的原始请求头都不能用于 CSRF 豁免。
func HasValidatedBearerSession(ctx context.Context) bool {
	session, ok := SessionContextFromContext(ctx)
	return ok && session.Source == "header" && strings.TrimSpace(session.AccessToken) != ""
}

// TrustedOrganizationID resolves the organization a request may act on, and it is
// the function tenant-scoped handlers must use when a request carries its own
// organizationId (body or query).
//
// The authenticated tenant scope injected by TenantFilterMiddleware is the only
// authority: a regular user's body/query/header value is ignored, so a forged
// organizationId cannot move a write into another tenant (MySQL has no RLS to
// catch that, and on PostgreSQL the write would fail only by accident of policy).
//
// The single legitimate override is a platform administrator: ScopeAuthorizer /
// TenantFilter escalate those requests to super_admin (or system) mode for an
// explicit cross-tenant operation, which is exactly when `requested` is honoured.
func TrustedOrganizationID(r *http.Request, requested string) string {
	scope := tenantctx.TenantScopeFromContext(r.Context())
	requestedOrg := strings.TrimSpace(requested)
	if scope.Mode == tenantctx.TenantFilterSuperAdmin || scope.Mode == tenantctx.TenantFilterSystem {
		if requested != "" {
			return requested
		}
	}
	effectiveOrg := scope.OrganizationID
	session, hasSession := SessionContextFromContext(r.Context())
	if effectiveOrg == "" && hasSession {
		effectiveOrg = session.OrganizationID
	}
	if requestedOrg != "" && effectiveOrg != "" && requestedOrg != effectiveOrg && hasSession {
		meta := route.RequestMetadataFrom(r.Context())
		platformaudit.Record(r.Context(), platformaudit.Event{
			OrganizationID: effectiveOrg,
			ActorID:        session.UserID,
			TargetTenant:   requestedOrg,
			Action:         "authz.cross_tenant_denied",
			ResourceType:   "route",
			ResourceID:     r.URL.Path,
			Reason:         "cross_tenant",
			IP:             r.RemoteAddr,
			UserAgent:      strings.TrimSpace(r.UserAgent()),
			Surface:        string(meta.Surface),
			RequestID:      meta.RequestID,
			TraceID:        meta.TraceID,
		})
	}
	if effectiveOrg != "" {
		return effectiveOrg
	}
	return ""
}
