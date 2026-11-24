package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	apprebac "github.com/guliangiam/guliangiam/internal/app/rebac"
	rebacapp "github.com/guliangiam/guliangiam/internal/app/rebac"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	mw "github.com/guliangiam/guliangiam/internal/platform/middleware"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// AuthorizationPDP 是本包对 Casbin PDP 能力的本地接口依赖。
// 由 bootstrap/wire_transport.go 注入 infra/casbin.Manager 具体实现。
// 定义在 transport 包内而非 domain，避免 transport→domain 穿透（Rule 2）。
type AuthorizationPDP interface {
	ResolveEnforcerUUID(ctx context.Context, orgUUID string) (string, error)
	Enforce(ctx context.Context, enforcerID, modelText string, rvals ...interface{}) (bool, error)
}

// PrivilegedAuthorizationPDP is the optional capability required for JIT-backed
// privileged sessions. Managers without it fail closed for those sessions.
type PrivilegedAuthorizationPDP interface {
	EnforcePrivileged(ctx context.Context, enforcerID, modelText, lifecycleID string, rvals ...interface{}) (bool, error)
}

// APIAuthorizationMiddleware returns a middleware that enforces permission-based access control.
//
// Security model (fail-close per 架构设计 §8.8 / S11):
//   - casbinManager == nil → 503 Service Unavailable (never fail-open)
//   - Truly public endpoints → allow through (白名单见 isTrulyPublicEndpoint)
//   - Auth endpoints requiring session → 检查 session 存在但跳过 Casbin (见 isAuthRequiringSession)
//   - ReBAC endpoints (/api/v1/rebac/*) → 跳过 ReBAC 覆盖检查（端点自身就是 ReBAC 引擎入口）
//   - No session / expired session → 401
//   - Username 为空 → 401 (subject must be unambiguous)
//   - Casbin Enforce 出错 → 500 (fail-closed, misconfigured engine does NOT grant access)
//   - Casbin denies → 403
//   - Casbin allows + ReBAC denies → 403 (ReBAC fail-close 覆盖 Casbin allow)
func APIAuthorizationMiddleware(casbinManager AuthorizationPDP, rebacEngine *apprebac.Engine, nsResolver NamespaceResolver, overrides ...PermissionConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// ── Gate 1: PEP 引擎必须就绪 ──────────────────────────────────
			// fail-close：manager 为 nil 说明权限引擎未正确初始化，
			// 绝不能放行（架构 §8.8 安全默认值）。
			if casbinManager == nil {
				// Authz audit: engine not initialized (fail-close).
				audit.SlogAudit(r.Context(), audit.EventAuthzCasbinInitFail,
					audit.RequestAuditFields(r, "", "")...)
				route.WriteError(w, r, apperrors.NewUnavailable("authorization engine not initialized"))
				return
			}

			// ── Gate 2a: 完全公开端点（无需任何认证） ────────────────────
			// 判定键是 method + path：captcha-provider 管理面在旧前缀下
			// 曾把匿名 GET/PATCH/DELETE 一并放行。
			if isTrulyPublicRequest(r.Method, r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// ── Gate 2a.5: Logout — 幂等无 session 端点 ───────────────────
			// logout handler 自身是幂等的（从 cookie 读 sessionID；没有 cookie 也会
			// ClearSessionCookie 并返回 200）。允许无 session 调用是为了避免：
			// session 过期 → middleware Gate 2b/3 拦截 → handler 永远不执行 →
			// HttpOnly cookie 残留 → 下次 bootstrap 自动"重新登录"。
			if r.URL.Path == "/api/v1/auth/logout" {
				next.ServeHTTP(w, r)
				return
			}

			// ── Gate 2b: Auth 端点 — 需要 session 但跳过 Casbin ───────────
			// 这些端点（account, consents:* 等）在 auth 包内部自行处理
			// session/token 校验，但依赖 SessionContextMiddleware 注入的上下文。
			// 跳过 Casbin 是因为 auth 端点自身已经在做更细粒度的认证流程，
			// 但如果 session context 不存在 → 401（fail-close），绝不放行。
			if isAuthRequiringSession(r.URL.Path) {
				session, ok := SessionContextFromContext(r.Context())
				if !ok {
					route.WriteError(w, r, apperrors.NewUnauthorized("authentication required"))
					return
				}
				// Subject 必须明确（与 Gate 4 同一 fail-close 逻辑）
				if strings.TrimSpace(session.Username) == "" {
					route.WriteError(w, r, apperrors.NewUnauthorized("authentication required"))
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// ── Gate 3: 会话上下文必须存在 ───────────────────────────────
			session, ok := SessionContextFromContext(r.Context())
			if !ok {
				route.WriteError(w, r, apperrors.NewUnauthorized("authentication required"))
				return
			}

			// Repository-backed sessions are fully validated by database lookup
			// (checks revoked, expired, idle timeout), so AccessToken is not required.
			// Cookie-backed and header-backed sessions lack server-side validation,
			// so they must present a valid AccessToken.
			if session.AccessToken == "" && session.Source != "repository" {
				route.WriteError(w, r, apperrors.NewUnauthorized("authentication required"))
				return
			}

			// ── Gate 4: subject 必须有明确 Username ──────────────────────
			// fail-close：subject 为空时拒绝，而不是回退到 AccessToken。
			// AccessToken 可能过期/伪造，用作 subject 会导致审计追踪混乱。
			// （之前的回退逻辑是一个隐蔽的 fail-open 风险）
			subject := strings.TrimSpace(session.Username)
			if subject == "" {
				route.WriteError(w, r, apperrors.NewUnauthorized("authentication required"))
				return
			}

			// ── 资源 + 动作推导 ─────────────────────────────────────────
			resource := resourceFromPath(r.URL.Path)
			// Casbin actions: the seeded RBAC policies use the documented CRUD
			// vocabulary (read/write/delete), while dynamically-created
			// permissions (e.g. AI Gateway) use HTTP method names. Try the
			// method first, then the documented alias, so both conventions
			// authorize without rewriting either policy family.
			actions := casbinActionCandidates(r.Method)
			action := actions[0]
			// 显式权限点覆盖：少数子资源动作不能被父资源的 write 顺带覆盖
			// （例如 POST /users/{id}/set-password 必须用 users:password:reset，
			// 而 POST 的方法别名是 users:write）。覆盖规则来自 route 声明层，
			// 未命中的请求仍走路径 + 方法推导。
			if override, ok := matchPermissionOverride(r.Method, r.URL.Path, overrides); ok {
				resource, action = splitDeclaredPermission(override.Permission)
				actions = []string{action}
			}

			// Resolve enforcer UUID from organization UUID.
			// The `casbin_rules.enforcer_id` column is UUID + FK → enforcers(id),
			// so we cannot pass a string like "default" directly to the adapter —
			// PostgreSQL would reject the cast (SQLSTATE 22P02).
			if strings.TrimSpace(session.OrganizationID) == "" {
				route.WriteError(w, r, apperrors.NewInternal("organization id missing from session"))
				return
			}
			enforcerUUID, err := casbinManager.ResolveEnforcerUUID(r.Context(), session.OrganizationID)
			if err != nil {
				// Fail-closed: enforcer not found or resolver broken — do NOT grant access.
				route.WriteError(w, r, apperrors.NewInternal("authorization engine misconfigured"))
				return
			}
			modelText := principalModel(session.PrincipalType)

			// ── Gate 5: Casbin 裁决 ───────────────────────────────────────
			// OTel span: child of TracingMiddleware top-level HTTP span.
			{
				span := mw.RecordSpan(r.Context(), "authz.casbin.enforce")
				var allowed bool
				var err error
				for _, candidate := range actions {
					action = candidate
					if session.IsPrivileged {
						privilegedManager, ok := casbinManager.(PrivilegedAuthorizationPDP)
						if !ok {
							span.End()
							route.WriteError(w, r, apperrors.NewUnavailable("privileged authorization is not configured"))
							return
						}
						allowed, err = privilegedManager.EnforcePrivileged(r.Context(), enforcerUUID, modelText, session.LifecycleID, subject, session.OrganizationSlug, resource, action)
					} else {
						allowed, err = casbinManager.Enforce(r.Context(), enforcerUUID, modelText, subject, session.OrganizationSlug, resource, action)
					}
					if err != nil || allowed {
						break
					}
				}
				span.End()
				if err != nil {
					// Fail-closed 且语义正确：策略引擎不可用是依赖类故障 → 503
					// （S-07 / AC-PERM-003），不是 500 未知内部错误（DEF-20）。
					route.WriteError(w, r, apperrors.NewUnavailable("authorization engine unavailable"))
					return
				}
				if !allowed {
					// Authz audit: Casbin denied every candidate action.
					fields := audit.RequestAuditFields(r, session.Username, resource)
					fields = append(fields, "audit.action", strings.Join(actions, "|"))
					audit.SlogAudit(r.Context(), audit.EventAuthzDeny, fields...)
					route.WriteError(w, r, apperrors.NewForbidden("insufficient permissions"))
					return
				}
			}

			// ── Gate 5b: ReBAC 覆盖检查（Casbin allow 后 ReBAC deny → 403）──
			// 端点自身就是 ReBAC API → 跳过（否则死循环 / 自证）。
			// nsUUID 为空 = 该 org 尚未 seed default namespace → clean skip
			// （不是 fail-open；只是没有 ReBAC policy 可评估）。
			if !isRebacEndpoint(r.URL.Path) {
				if rebacEngine == nil {
					logWarn(r.Context(), "rebac engine unavailable")
					fields := audit.RequestAuditFields(r, session.Username, resource)
					fields = append(fields, "audit.action", action, "audit.rebac.error", "engine_unavailable")
					audit.SlogAudit(r.Context(), audit.EventAuthzDeny, fields...)
					route.WriteError(w, r, apperrors.NewUnavailable("authorization engine unavailable"))
					return
				}
				if nsResolver == nil {
					logWarn(r.Context(), "rebac namespace resolver unavailable")
					fields := audit.RequestAuditFields(r, session.Username, resource)
					fields = append(fields, "audit.action", action, "audit.rebac.error", "namespace_resolver_unavailable")
					audit.SlogAudit(r.Context(), audit.EventAuthzDeny, fields...)
					route.WriteError(w, r, apperrors.NewUnavailable("authorization engine unavailable"))
					return
				}
				nsUUID, nsResolveErr := nsResolver.Resolve(r.Context(), session.OrganizationID)
				if nsResolveErr != nil {
					logWarn(r.Context(), "rebac namespace resolution failed: %v", nsResolveErr)
					fields := audit.RequestAuditFields(r, session.Username, resource)
					fields = append(fields, "audit.action", action, "audit.rebac.error", "namespace_resolution_failed")
					audit.SlogAudit(r.Context(), audit.EventAuthzDeny, fields...)
					route.WriteError(w, r, apperrors.NewUnavailable("authorization engine unavailable"))
					return
				}
				if nsUUID != "" {
					reBACAllowed, reBACErr := rebacEngine.Check(r.Context(), rebacapp.CheckRequest{
						OrganizationID: session.OrganizationSlug,
						NamespaceID:    nsUUID,
						Tuple: rebacapp.TupleKey{
							Subject:  fmt.Sprintf("user:%s", subject),
							Relation: action,
							Object:   fmt.Sprintf("%s:*", resource),
						},
						DepthLimit: 10,
					})
					if reBACErr != nil {
						logWarn(r.Context(), "rebac check failed: %v", reBACErr)
						fields := audit.RequestAuditFields(r, session.Username, resource)
						fields = append(fields, "audit.action", action, "audit.rebac.error", "evaluation_failed")
						audit.SlogAudit(r.Context(), audit.EventAuthzDeny, fields...)
						route.WriteError(w, r, apperrors.NewUnavailable("authorization engine unavailable"))
						return
					} else if !reBACAllowed.Allowed {
						// ReBAC deny 覆盖 Casbin allow —— 审计追踪两层裁决
						fields := audit.RequestAuditFields(r, session.Username, resource)
						fields = append(fields, "audit.action", action, "audit.rebac.override", "casbin_allow_rebac_deny")
						audit.SlogAudit(r.Context(), audit.EventAuthzDeny, fields...)
						route.WriteError(w, r, apperrors.NewForbidden("insufficient resource-level permissions"))
						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermission returns a middleware that requires a specific permission.
// It checks the authenticated user's permission via Casbin Enforce(),
// then applies the ReBAC resource-level override (Casbin allow + ReBAC deny → 403).
//
// Security model (fail-close per 架构设计 §8.8 / S11):
//   - manager == nil → 503 (never fail-open)
//   - No session → 401
//   - Username 为空 → 401
//   - Enforce 出错 → 503 (fail-closed，依赖类故障)
//   - Casbin deny → 403
//   - Casbin allow + ReBAC deny → 403
//
// casbinVerbActions is the closed set of permission actions Casbin policies may
// name explicitly. Route permissions outside this set (e.g. "aigateway:providers")
// describe a sub-resource, so the effective action comes from the HTTP method.
var casbinVerbActions = map[string]bool{
	"read": true, "write": true, "delete": true, "admin": true,
	"check": true, "run": true, "enforce": true, "resolve": true,
	"pay": true, "buy": true, "reset": true,
}

// casbinObjectAndAction normalizes a route permission ("webauthn:write") into
// the (obj, act) pair Casbin policies use (resource + CRUD verb). The PEP used
// to pass the raw permission string and the HTTP method, so non-wildcard
// policies could never match (OPEN-API-72).
func casbinObjectAndAction(permission, method string) (string, string) {
	trimmed := strings.TrimSpace(permission)
	obj := trimmed
	act := ""
	if index := strings.Index(trimmed, ":"); index > 0 {
		obj = trimmed[:index]
		act = trimmed[strings.LastIndex(trimmed, ":")+1:]
	}
	if !casbinVerbActions[act] {
		act = httpMethodAction(method)
	}
	return obj, act
}

func httpMethodAction(method string) string {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodHead:
		return "read"
	case http.MethodDelete:
		return "delete"
	default:
		return "write"
	}
}

func RequirePermission(permission string, manager AuthorizationPDP, rebacEngine *apprebac.Engine, cfg CasbinPermissionConfig, nsResolver NamespaceResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if manager == nil {
				// Authz audit: engine not initialized (fail-close).
				audit.SlogAudit(r.Context(), audit.EventAuthzCasbinInitFail,
					audit.RequestAuditFields(r, "", "")...)
				route.WriteError(w, r, apperrors.NewUnavailable("authorization engine not initialized"))
				return
			}

			session, ok := SessionContextFromContext(r.Context())
			if !ok {
				route.WriteError(w, r, apperrors.NewUnauthorized("authentication required"))
				return
			}

			// Subject must be unambiguous — fail-close on empty Username.
			subject := strings.TrimSpace(session.Username)
			if subject == "" {
				route.WriteError(w, r, apperrors.NewUnauthorized("authentication required"))
				return
			}

			action := strings.ToLower(r.Method)
			casbinObject, casbinAction := casbinObjectAndAction(permission, r.Method)

			// ── Casbin 裁决 ──────────────────────────────────────────
			{
				span := mw.RecordSpan(r.Context(), "authz.casbin.enforce")
				var allowed bool
				var err error
				if session.IsPrivileged {
					privilegedManager, ok := manager.(PrivilegedAuthorizationPDP)
					if !ok {
						span.End()
						route.WriteError(w, r, apperrors.NewUnavailable("privileged authorization is not configured"))
						return
					}
					allowed, err = privilegedManager.EnforcePrivileged(r.Context(), cfg.EnforcerID, cfg.ModelText, session.LifecycleID, subject, session.OrganizationSlug, casbinObject, casbinAction)
				} else {
					allowed, err = manager.Enforce(r.Context(), cfg.EnforcerID, cfg.ModelText, subject, session.OrganizationSlug, casbinObject, casbinAction)
				}
				span.End()
				if err != nil {
					route.WriteError(w, r, apperrors.NewUnavailable("authorization engine unavailable"))
					return
				}
				if !allowed {
					// Authz audit: Casbin denied this subject/resource/action.
					fields := audit.RequestAuditFields(r, session.Username, permission)
					fields = append(fields, "audit.action", action)
					audit.SlogAudit(r.Context(), audit.EventAuthzDeny, fields...)
					route.WriteError(w, r, apperrors.NewForbidden("insufficient permissions"))
					return
				}
			}

			// ── ReBAC 覆盖（Casbin allow 后 ReBAC deny → 403）────────
			if !isRebacEndpoint(r.URL.Path) {
				if rebacEngine == nil {
					logWarn(r.Context(), "rebac engine unavailable")
					fields := audit.RequestAuditFields(r, session.Username, permission)
					fields = append(fields, "audit.action", action, "audit.rebac.error", "engine_unavailable")
					audit.SlogAudit(r.Context(), audit.EventAuthzDeny, fields...)
					route.WriteError(w, r, apperrors.NewUnavailable("authorization engine unavailable"))
					return
				}
				if nsResolver == nil {
					logWarn(r.Context(), "rebac namespace resolver unavailable")
					fields := audit.RequestAuditFields(r, session.Username, permission)
					fields = append(fields, "audit.action", action, "audit.rebac.error", "namespace_resolver_unavailable")
					audit.SlogAudit(r.Context(), audit.EventAuthzDeny, fields...)
					route.WriteError(w, r, apperrors.NewUnavailable("authorization engine unavailable"))
					return
				}
				nsUUID, nsResolveErr := nsResolver.Resolve(r.Context(), session.OrganizationID)
				if nsResolveErr != nil {
					logWarn(r.Context(), "rebac namespace resolution failed: %v", nsResolveErr)
					fields := audit.RequestAuditFields(r, session.Username, permission)
					fields = append(fields, "audit.action", action, "audit.rebac.error", "namespace_resolution_failed")
					audit.SlogAudit(r.Context(), audit.EventAuthzDeny, fields...)
					route.WriteError(w, r, apperrors.NewUnavailable("authorization engine unavailable"))
					return
				}
				if nsUUID != "" {
					reBACAllowed, reBACErr := rebacEngine.Check(r.Context(), rebacapp.CheckRequest{
						OrganizationID: session.OrganizationSlug,
						NamespaceID:    nsUUID,
						Tuple: rebacapp.TupleKey{
							Subject:  fmt.Sprintf("user:%s", subject),
							Relation: action,
							Object:   fmt.Sprintf("%s:*", permission),
						},
						DepthLimit: 10,
					})
					if reBACErr != nil {
						logWarn(r.Context(), "rebac check failed: %v", reBACErr)
						fields := audit.RequestAuditFields(r, session.Username, permission)
						fields = append(fields, "audit.action", action, "audit.rebac.error", "evaluation_failed")
						audit.SlogAudit(r.Context(), audit.EventAuthzDeny, fields...)
						route.WriteError(w, r, apperrors.NewUnavailable("authorization engine unavailable"))
						return
					} else if !reBACAllowed.Allowed {
						fields := audit.RequestAuditFields(r, session.Username, permission)
						fields = append(fields, "audit.action", action, "audit.rebac.override", "casbin_allow_rebac_deny")
						audit.SlogAudit(r.Context(), audit.EventAuthzDeny, fields...)
						route.WriteError(w, r, apperrors.NewForbidden("insufficient resource-level permissions"))
						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
