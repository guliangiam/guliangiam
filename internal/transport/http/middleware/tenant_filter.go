package middleware

import (
	"net/http"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// OrgUUIDResolver 是一个能把 slug 解析成 UUID 的最小接口。
// 它由 bootstrap 注入 TenantFilterMiddleware，避免引入完整 Repo 依赖。
type OrgUUIDResolver interface {
	ResolveUUIDBySlug(slug string) (string, error)
}

// DefaultBuiltinOrgSlug is the fallback organization for unauthenticated
// (truly public) endpoints under the protected group. Without it,
// session-less requests pass through with an empty tenant scope and RLS
// policies evaluate to fail-closed, hiding built-in app metadata that the
// login SPA needs to discover before the user has a session.
const DefaultBuiltinOrgSlug = "built-in"

type TenantFilterConfig struct {
	StrictMode bool
	// FallbackOrgSlug is the organization slug injected when the request
	// has no session and StrictMode is false. Empty string disables the
	// fallback; the request then passes through with an empty scope and
	// downstream RLS may block it.
	FallbackOrgSlug string
}

// TenantFilterMiddleware injects a TenantScope into the request context for
// downstream RLS tenant isolation (ADR-016).
//
// Resolution order:
//  1. Session organization (authenticated user) → session.OrganizationSlug
//  2. FallbackOrgSlug (unauthenticated truly-public endpoint, StrictMode=false)
//  3. StrictMode → 401; otherwise → empty scope (pass-through, RLS may block)
func TenantFilterMiddleware(cfg TenantFilterConfig) func(http.Handler) http.Handler {
	fallback := cfg.FallbackOrgSlug
	if fallback == "" {
		fallback = DefaultBuiltinOrgSlug
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// ── ScopeAuthorizerMiddleware 已放行跨租户请求 → pass-through ─────
			// ScopeAuthorizerMiddleware 放行显式 target_tenant 请求后，
			// 已注入 TenantFilterSystem（或 TenantFilterSuperAdmin）。
			// TenantFilterMiddleware 不应覆盖 — 否则跨租户 RLS 提升会被撤销。
			if existing := tenantctx.TenantScopeFromContext(ctx); existing.Mode == tenantctx.TenantFilterSystem || existing.Mode == tenantctx.TenantFilterSuperAdmin || existing.Mode == tenantctx.TenantFilterCross {
				next.ServeHTTP(w, r)
				return
			}

			session, ok := auth.SessionContextFromContext(ctx)
			if !ok || session.OrganizationSlug == "" {
				if cfg.StrictMode {
					route.WriteError(w, r, apperrors.NewUnauthorized("missing organization context"))
					return
				}

				// ── Fallback: truly public endpoint ──────────────────────────────
				// Requests that reach here without a session have already passed
				// APIAuthorizationMiddleware's Gate 2a (isTrulyPublicEndpoint),
				// Gate 2b fails-fast with 401, and Gate 3 also 401. So this is
				// always a pre-login discovery endpoint (applications/resolve,
				// OIDC discovery, DCR, CAS/SAML handshake). Give it a tenant
				// scope so RLS sees built-in rows.
				if fallback != "" {
					ctx = tenantctx.WithTenantScope(ctx, fallback, fallback, tenantctx.TenantFilterOptional)
				}
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			mode := tenantctx.TenantFilterOptional
			if cfg.StrictMode {
				mode = tenantctx.TenantFilterStrict
			}

			// platform_admin → 仅在 platform_admin_scopes 为 all 时提升 RLS。
			// PrincipalType 由 PrincipalResolver 在 SessionContextMiddleware 注入
			// （platform_admins 表 membership + 60s 缓存），但它只证明「是平台管理员」，
			// 不证明「可以跨所有租户」。作用域分类由 ScopeAuthorizerMiddleware 在
			// 本中间件之前写入 context：
			//   - all      → TenantFilterSuperAdmin
			//   - targeted → 不提升，保持会话本租户作用域（fail-closed）
			//   - none/unknown（含解析失败）→ 不提升（fail-closed）
			//   - 无分类（ScopeAuthorizer 未装配/未写入）→ 按 ClassNone 处理，不提升
			//
			// 生产链顺序（bootstrap/chi.go）：ScopeAuthorizer → TenantFilter，分类必然存在。
			if session.PrincipalType == "platform_admin" {
				if adminScope, classified := scopeauthorizer.AdminScopeFromContext(ctx); classified && adminScope.GrantsAllTenants() {
					mode = tenantctx.TenantFilterSuperAdmin
				}
			}

			// 会话作用域里的 OrganizationID 是 organizations.id（UUID），
			// OrganizationSlug 是 slug。下游 handler 通过 auth.TrustedOrganizationID
			// 直接把它写进 uuid 列（rebac / idv / records 等），因此这里必须优先
			// 传 UUID；只有历史会话行缺 UUID 时才退回 slug，让 uow.setTenantContext
			// 在事务内做 slug→UUID 解析。
			orgID := session.OrganizationID
			if !looksLikeUUID(orgID) {
				orgID = session.OrganizationSlug
			}
			ctx = tenantctx.WithTenantScope(ctx, orgID, session.OrganizationSlug, mode)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// looksLikeUUID 判断作用域值是否已是 36 位 UUID 形态（8-4-4-4-12）。
// 不做严格解析——这里只用于决定「直接下传 UUID」还是「交给 DB 解析 slug」。
func looksLikeUUID(v string) bool {
	if len(v) != 36 {
		return false
	}
	for i, ch := range v {
		switch i {
		case 8, 13, 18, 23:
			if ch != '-' {
				return false
			}
		default:
			if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') && (ch < 'A' || ch > 'F') {
				return false
			}
		}
	}
	return true
}
