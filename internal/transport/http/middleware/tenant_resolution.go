// TenantResolutionMiddleware — 解析请求中的显式 target_tenant。
//
// Phase B-D4：跨租户操作时，platform_admin 需要明确指定"操作哪个租户"。
// 本中间件负责从请求中提取显式 target_tenant，写入 context 供 ScopeAuthorizer 使用。
//
// 与现有 TenantFilterMiddleware 的职责分离（评审 H3）：
//
//   - TenantResolutionMiddleware（本文件）：解析显式 target_tenant（URL/body）
//     → 跨租户场景有值 → 触发 ScopeAuthorizer 校验
//
//   - TenantFilterMiddleware（tenant_filter.go）：session  fallback（本租户场景）
//     → 无显式 target_tenant 时 → 继续用 session.OrganizationSlug
//
//     TenantResolutionMiddleware 在中间件链中排在 TenantFilterMiddleware 之前：
//     SessionContext → TenantResolutionMiddleware → APIAuthorizationMiddleware
//     → ScopeAuthorizerMiddleware → TenantFilterMiddleware → Handler
//
// 无显式 target_tenant 时中间件直接 pass-through，不注入任何额外 TenantScope。
// 本租户操作由 TenantFilterMiddleware 的 session fallback 处理。
package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	platformaudit "github.com/guliangiam/guliangiam/internal/platform/auditdenial"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	httputil "github.com/guliangiam/guliangiam/internal/transport/http/util"
)

type targetTenantCtxKey struct{}

// WithTargetTenant 将解析到的显式 target_tenant 写入 context。
func WithTargetTenant(ctx context.Context, tenantID uuid.UUID) context.Context {
	return context.WithValue(ctx, targetTenantCtxKey{}, tenantID)
}

// TargetTenantFromContext 读取显式 target_tenant。
// ok=false 表示本次请求没有显式指定 target_tenant（= 本租户操作）。
func TargetTenantFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(targetTenantCtxKey{}).(uuid.UUID)
	return id, ok
}

// OrganizationSlugResolver resolves an organization slug to its UUID.
//
// It is injected by the composition root (bootstrap) from the organization
// repository, so the transport layer never imports domain/infra directly. A
// nil resolver keeps the non-UUID target fail-closed.
type OrganizationSlugResolver func(ctx context.Context, slug string) (uuid.UUID, error)

// RegionResolver resolves organization IDs to deployment regions. It mirrors
// domain/regions.RegionResolver but stays a transport-local port so the
// middleware does not import infra.
type RegionResolver interface {
	ResolveBatch(ctx context.Context, orgIDs []string) (map[string]string, error)
}

// RegionResidencyMode controls cross-region enforcement.
type RegionResidencyMode string

const (
	// RegionResidencyObserve logs violations but allows the request.
	RegionResidencyObserve RegionResidencyMode = "observe"
	// RegionResidencyEnforce rejects cross-region requests with 403.
	RegionResidencyEnforce RegionResidencyMode = "enforce"
	// RegionResidencyOff disables the check.
	RegionResidencyOff RegionResidencyMode = "off"
)

// NormalizeRegionResidencyMode parses REGION_RESIDENCY_MODE; unknown values
// fall back to observe (never silently enforce).
func NormalizeRegionResidencyMode(raw string) RegionResidencyMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "enforce":
		return RegionResidencyEnforce
	case "off":
		return RegionResidencyOff
	default:
		return RegionResidencyObserve
	}
}

// TenantResolutionMiddleware 解析显式 target_tenant。
//
// 解析顺序（浅层，不递归嵌套 — 防 DoS）：
//  1. URL path param  {org_tenant_id}（chi router 约定）
//  2. Query param     ?targetOrganizationId= / ?tenantId= / ?organizationId=
//  3. Header          X-Target-Organization-Id
//  4. JSON body       {targetOrganizationId|tenantId|organizationId: "xxx"}（仅顶层 key 浅扫描）
//
// 找到 UUID → 若不是本租户则写入 context，ScopeAuthorizer 启用；
// 找到非 UUID（组织 slug）→ 本租户 slug 放行；他租户 slug 必须先解析为组织
// UUID 再交给 ScopeAuthorizer，解析失败或无解析器一律 deny（fail-closed）；
// 全部没找到 → pass-through（本租户操作，由 TenantFilterMiddleware 处理）
//
// 本构造器不携带 slug 解析器，仅用于没有组织仓储依赖的调用方（如单元测试）。
// 生产链必须使用 TenantResolutionMiddlewareWithResolver。
func TenantResolutionMiddleware() func(http.Handler) http.Handler {
	return TenantResolutionMiddlewareWithResolver(nil)
}

// TenantResolutionMiddlewareWithResolver 是生产构造器：平台管理员的非 UUID
// target 先解析为组织 UUID，ScopeAuthorizerMiddleware 才能校验真实目标租户。
func TenantResolutionMiddlewareWithResolver(resolver OrganizationSlugResolver) func(http.Handler) http.Handler {
	return TenantResolutionMiddlewareWithRegionGuard(resolver, nil, RegionResidencyOff)
}

// TenantResolutionMiddlewareWithRegionGuard additionally enforces the
// cross-region residency rule once a cross-tenant target is identified.
func TenantResolutionMiddlewareWithRegionGuard(slugResolver OrganizationSlugResolver, regionResolver RegionResolver, mode RegionResidencyMode) func(http.Handler) http.Handler {
	if mode == "" {
		mode = RegionResidencyObserve
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			targetID, raw, found := resolveRequestedTenant(ctx, r)
			if found && raw != "" && !looksLikeUUID(raw) {
				session, ok := authtransport.SessionContextFromContext(ctx)
				if !ok {
					// 无会话时保持既有 pass-through；认证/授权由
					// APIAuthorizationMiddleware 负责。
					next.ServeHTTP(w, r)
					return
				}

				// 本租户 slug 是正常请求，不标记为跨租户目标。
				if raw == session.OrganizationSlug {
					next.ServeHTTP(w, r)
					return
				}

				if session.PrincipalType != "platform_admin" {
					// 普通租户只能用本租户 slug；他租户 slug 无从授权，fail-closed。
					// S-08：跨租户访问必须与「资源不存在」不可区分 → 404（DEF-19），
					// 不能返回 403 泄露目标租户存在性。
					// S-10 L1 显式打点：本层明确知道这是跨租户目标，必须写拒绝审计
					// （中间件兜底层无法从 404 区分「不存在」与「跨租户」）。
					meta := route.RequestMetadataFrom(ctx)
					platformaudit.Record(ctx, platformaudit.Event{
						OrganizationID: session.OrganizationID,
						ActorID:        session.UserID,
						TargetTenant:   raw,
						Action:         "authz.deny",
						ResourceType:   "route",
						ResourceID:     r.URL.Path,
						Reason:         "cross_tenant_slug_denied",
						IP:             httputil.ClientIP(r),
						UserAgent:      r.UserAgent(),
						Surface:        string(meta.Surface),
						RequestID:      meta.RequestID,
						TraceID:        meta.TraceID,
					})
					route.WriteError(w, r, apperrors.NewNotFound("organization not found"))
					return
				}

				// 平台管理员：slug 必须先解析成组织 UUID；解析失败 = deny，
				// 不能退回“无 target”而绕过 ScopeAuthorizer。
				if slugResolver == nil {
					route.WriteError(w, r, apperrors.NewForbidden("target organization slug cannot be resolved"))
					return
				}
				resolvedID, err := slugResolver(ctx, raw)
				if err != nil || resolvedID == uuid.Nil {
					route.WriteError(w, r, apperrors.NewForbidden("target organization slug cannot be resolved"))
					return
				}
				targetID = resolvedID
			}

			// 本租户 UUID 不标记为跨租户目标：ScopeAuthorizer 的 Gate 6 只有在
			// 非 target 请求时才放行普通用户，若把自租户 UUID 当作 target 会被
			// Gate 2（非 platform_admin 不得跨租户）误拒。
			if targetID != uuid.Nil {
				session, sessionOK := authtransport.SessionContextFromContext(ctx)
				crossTenant := !sessionOK || session.OrganizationID == "" || targetID.String() != session.OrganizationID
				if crossTenant && regionResolver != nil && sessionOK && session.OrganizationID != "" && mode != RegionResidencyOff {
					if decisionErr := enforceCrossRegion(ctx, r, regionResolver, mode, session.OrganizationID, targetID.String()); decisionErr != nil {
						route.WriteError(w, r, decisionErr)
						return
					}
				}
				if crossTenant {
					ctx = WithTargetTenant(ctx, targetID)
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// resolveRequestedTenant 按解析顺序返回第一个显式租户：UUID 用于 ScopeAuthorizer，
// 原始字符串用于区分「本租户 slug」与「他租户 slug」。
//
// 与「只处理 UUID」的旧实现相比：额外识别 query `organizationId`（列表端点长期用它
// 取租户，此前只有 body 会被作用域授权覆盖），并在无法解析为 UUID 时保留原始值供
// 调用方判定本租户/他租户 slug。
func resolveRequestedTenant(ctx context.Context, r *http.Request) (uuid.UUID, string, bool) {
	if id, raw, ok := getTargetTenantFromPath(ctx, r); ok {
		return id, raw, true
	}
	for _, key := range []string{"targetOrganizationId", "tenantId", "organizationId"} {
		if v := strings.TrimSpace(r.URL.Query().Get(key)); v != "" {
			return parseTenantValue(v)
		}
	}
	if v := strings.TrimSpace(r.Header.Get("X-Target-Organization-Id")); v != "" {
		return parseTenantValue(v)
	}
	if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
		if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			if v := tenantStringFromBody(r); v != "" {
				return parseTenantValue(v)
			}
		}
	}
	return uuid.Nil, "", false
}

func parseTenantValue(v string) (uuid.UUID, string, bool) {
	if id, err := uuid.Parse(v); err == nil {
		return id, v, true
	}
	return uuid.Nil, v, true
}

// getTargetTenantFromPath 从 chi URL params 提取 target_tenant。
//
// 常见命名约定：
//   - org_tenant_id   — 路由模板 /api/v1/admin/organizations/{org_tenant_id}/users
//   - organization_id — /api/v1/gateway/{organization_id}/servers
//   - targetOrgID     — /api/v1/users/{targetOrgID}/impersonate
//
// 优先级最高的非空 param 生效（路由模板通常只注册一个）。UUID 直接作为
// target；非 UUID 形态（slug）交给上方统一解析，不能静默忽略。
func getTargetTenantFromPath(_ context.Context, r *http.Request) (uuid.UUID, string, bool) {
	for _, name := range []string{"org_tenant_id", "organization_id", "targetOrgID", "orgId", "tenant_id"} {
		if v := chi.URLParam(r, name); v != "" {
			id, raw, _ := parseTenantValue(v)
			return id, raw, true
		}
	}
	// organizations 的 by-ID 路由使用 {id}：UUID 形态才视为显式 target，让
	// ScopeAuthorizer 校验 targeted 管理员是否有权访问该组织；非 UUID 的
	// {id} 保持原有 404 路径（不能把 slug/非法值变成 403 探测面）。
	if r.URL.Path != "" && strings.HasPrefix(r.URL.Path, "/api/v1/organizations/") {
		if v := chi.URLParam(r, "id"); v != "" {
			if id, err := uuid.Parse(v); err == nil {
				return id, v, true
			}
		}
	}
	return uuid.Nil, "", false
}

// tenantStringFromBody 返回 JSON body 顶层第一个非空租户字符串（可能是 UUID，
// 也可能是组织 slug）。扫描后完整还原 r.Body，不截断后续 handler 的读取。
func tenantStringFromBody(r *http.Request) string {
	if r.Body == nil {
		return ""
	}

	originalBody := r.Body
	body, err := io.ReadAll(io.LimitReader(originalBody, 1<<16)) // 扫描上限 64KB，不是请求体上限
	// 已扫描前缀 + 原始剩余流必须完整交还给下游；不能因租户浅扫描截断大请求。
	r.Body = struct {
		io.Reader
		io.Closer
	}{
		Reader: io.MultiReader(bytes.NewReader(body), originalBody),
		Closer: originalBody,
	}
	if err != nil {
		return ""
	}

	var top map[string]interface{}
	if err := json.Unmarshal(body, &top); err != nil {
		return ""
	}

	for _, key := range []string{"targetOrganizationId", "tenantId", "targetTenant", "organizationId", "orgId", "targetOrg"} {
		if v, ok := top[key]; ok {
			if s, ok := v.(string); ok {
				if value := strings.TrimSpace(s); value != "" {
					return value
				}
			}
		}
	}
	return ""
}

// ScopeAuthorizerMiddleware Phase C-Step 2 正式版 ScopeAuthorizer 接线。
//
// 必须在 APIAuthorizationMiddleware（Casbin）之后执行：
//
//	Casbin 先放行资源级权限 → ScopeAuthorizer 再校验跨租户作用域
//
// Gate 流程：
//  1. 无显式 target_tenant → pass-through（本租户操作，Casbin + RLS 已覆盖）
//  2. 有显式 target_tenant + Casbin 已放行 → 调用 ScopeAuthorizer.Authorize()
//     deny → 403；allow → 继续下游
//
// AuthorizationRequest 构造：
//   - ActorID / ActorType / SourceTenant 来自 SessionContext（principal_resolver 已填充）
//   - TargetTenant 来自 TenantResolutionMiddleware 解析
//   - ResourceType 来自 URL path 推导（复用 authorization.resourceFromPath 语义）
//   - Action = HTTP method
//   - RiskLevel = action 映射（delete/impersonate → high，get → low，默认 medium）
func ScopeAuthorizerMiddleware(authorizer scopeauthorizer.ScopeAuthorizer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// BE-E-14：请求级 membership 缓存——Authorize 与放行后的
			// annotateAuthorizedAdminScope 共享同一条 platform_admins 查询。
			ctx := scopeauthorizer.WithAdminLookupCache(r.Context())

			// Gate 1: 无显式 target_tenant → 本租户 → pass-through。
			// 但 platform_admin 必须在这里先解析 platform_admin_scopes：只有 all
			// 作用域才允许 TenantFilterMiddleware 提升 RLS；targeted / none /
			// unknown 一律不提升（P1：不能仅凭 PrincipalType 就绕过租户隔离）。
			targetTenant, hasTarget := TargetTenantFromContext(ctx)
			if !hasTarget {
				ctx = annotateAdminScope(ctx, authorizer)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Gate 2: ScopeAuthorizer 未装配 → fail-closed。
			// 显式 target_tenant 意味着跨租户操作，依赖缺失时放行等于绕过
			// TenantFilter/RLS 的跨租户裁决；与 PEP 引擎未初始化同口径返回
			// 503（auth_middleware.go Gate 1），并保留 reason 便于排查。
			if authorizer == nil {
				slog.WarnContext(ctx, "scope authorizer is not configured; denying cross-tenant request",
					"method", r.Method,
					"path", r.URL.Path,
					"target_tenant", targetTenant.String(),
				)
				route.WriteError(w, r, apperrors.NewUnavailable("scope authorizer not configured"))
				return
			}

			// Gate 3: 取 SessionContext（由 SessionContextMiddleware 注入）
			session, ok := authtransport.SessionContextFromContext(ctx)
			if !ok {
				// 无 session 但有显式 target_tenant — 不可能（SessionContextMiddleware 已经 gate）
				next.ServeHTTP(w, r)
				return
			}

			// Gate 4: 构造 AuthorizationRequest
			req := scopeauthorizer.AuthorizationRequest{
				ActorID:      sessionUUID(session.UserID),
				ActorType:    scopeauthorizer.PrincipalType(session.PrincipalType),
				SourceTenant: sessionUUID(session.OrganizationID),
				TargetTenant: targetTenant,
				ResourceType: resourceTypeFromPath(r.URL.Path),
				Action:       strings.ToLower(r.Method),
				RiskLevel:    riskLevelFor(r.URL.Path, strings.ToLower(r.Method)),
			}

			result, err := authorizer.Authorize(ctx, req)
			if err != nil {
				// fail-closed: ScopeAuthorizer 异常 → 403
				route.WriteError(w, r, apperrors.NewForbidden("scope authorization failed"))
				return
			}
			if !result.Allowed {
				route.WriteError(w, r, apperrors.NewForbidden(result.Reason))
				return
			}

			// ── 放行后注入 tenant scope ─────────────────────────────────────
			// ScopeAuthorizer 放行了跨租户请求，显式设置 TenantFilterSystem
			// 跳过 TenantFilterMiddleware 的本租户强制注入（后者会覆盖）。
			//
			// 为什么用 TenantFilterSystem？
			//   - 语义上：框架层授权的跨租户访问（业务层不再有 RLS 提升 helper）
			//   - RLS 效果与 TenantFilterSuperAdmin 相同：映射到 super_admin role
			//   - TenantFilterMiddleware 检测到 system/super_admin 时 pass-through
			// OPEN-API-42 C1：显式跨租户收紧为单一目标租户精确过滤——
			// role 固定 member + 非空 target，不再落入 super_admin 空组织旁路。
			ctx = tenantctx.WithTenantScope(ctx, targetTenant.String(), "", tenantctx.TenantFilterCross)
			// 显式 target 的放行路径统一注入 AdminScope：
			//   - organizations 名录需要它回收可见范围；
			//   - OPEN-API-42 C1 把跨租户注入从 System 收紧为 Cross 后，
			//     payments/orders 等业务层的 requireUserScopeAccess 不再能靠
			//     Mode==System 放行，必须依赖 AdminScopeFromContext 的
			//     scope.Covers(target) 判定；缺注入会把合法的平台管理员
			//     跨租户访问误判为 403。
			// annotateAuthorizedAdminScope 内部对非 platform_admin 不注入，
			// 作用域解析失败按 unknown fail-closed。
			ctx = annotateAuthorizedAdminScope(ctx, authorizer, targetTenant)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// annotateAdminScope 在无显式 target 的请求上解析 platform_admin_scopes 并写入
// context。解析失败写入 unknown（不提升），而不是放行。
func annotateAdminScope(ctx context.Context, authorizer scopeauthorizer.ScopeAuthorizer) context.Context {
	session, ok := authtransport.SessionContextFromContext(ctx)
	if !ok || session.PrincipalType != "platform_admin" {
		return ctx
	}
	// nil authorizer = 该链路未挂载 ScopeAuthorizerMiddleware（测试/旧组合根）。
	// 保持无分类的旧行为；生产链始终注入非 nil authorizer。
	if authorizer == nil {
		return ctx
	}
	resolver, ok := authorizer.(scopeauthorizer.AdminScopeResolver)
	if !ok {
		// 非 nil 但不支持分类（例如 DenyAll）→ 视为无作用域，fail-closed。
		return scopeauthorizer.WithAdminScope(ctx, scopeauthorizer.AdminScope{
			Class: scopeauthorizer.AdminScopeClassNone,
		})
	}

	scope, err := resolver.ResolveAdminScope(ctx, session.UserID)
	if err != nil {
		return scopeauthorizer.WithAdminScope(ctx, scopeauthorizer.AdminScope{
			Class: scopeauthorizer.AdminScopeClassUnknown,
		})
	}
	ctx = scopeauthorizer.WithAdminScope(ctx, scope)
	if scope.GrantsAllTenants() {
		if auditor, ok := authorizer.(scopeauthorizer.TenantBypassAuditor); ok {
			actorID, actorErr := uuid.Parse(session.UserID)
			sourceTenant, tenantErr := uuid.Parse(session.OrganizationID)
			if actorErr == nil && tenantErr == nil {
				auditor.AuditTenantBypass(ctx, actorID, scopeauthorizer.PrincipalType(session.PrincipalType), sourceTenant,
					"platform_admin_all_tenants_no_explicit_target")
			}
		}
	}
	return ctx
}

// annotateAuthorizedAdminScope 在 ScopeAuthorizer 放行显式 target 后记录管理员的
// 作用域分类。非 all 作用域同时记录「本次请求经授权的显式 target」，供
// organizations 列表/名录在 RLS 提升后的全量读取中回收可见范围。
func annotateAuthorizedAdminScope(ctx context.Context, authorizer scopeauthorizer.ScopeAuthorizer, target uuid.UUID) context.Context {
	session, ok := authtransport.SessionContextFromContext(ctx)
	if !ok || session.PrincipalType != "platform_admin" {
		return ctx
	}
	resolver, ok := authorizer.(scopeauthorizer.AdminScopeResolver)
	if !ok {
		return ctx
	}

	scope, err := resolver.ResolveAdminScope(ctx, session.UserID)
	if err != nil {
		scope = scopeauthorizer.AdminScope{Class: scopeauthorizer.AdminScopeClassUnknown}
	}
	ctx = scopeauthorizer.WithAdminScope(ctx, scope)
	if !scope.GrantsAllTenants() {
		ctx = scopeauthorizer.WithAuthorizedTargetTenant(ctx, target)
	}
	return ctx
}

// sessionUUID 把 session 里的 UUID string 转成 uuid.UUID。
// 解析失败 → uuid.Nil（AllowIfScoped 会处理这种情况）。
func sessionUUID(s string) uuid.UUID {
	id, _ := uuid.Parse(s)
	return id
}

// resourceTypeFromPath 从 URL path 推导 resource type（与 authorization.resourceFromPath 语义对齐）。
// 简化版：取 /api/v1/ 之后第一段路径作为 resource type。
func resourceTypeFromPath(path string) string {
	// /api/v1/users → users; /api/v1/applications/{id} → applications
	trimmed := strings.TrimPrefix(path, "/api/v1/")
	if idx := strings.Index(trimmed, "/"); idx > 0 {
		return trimmed[:idx]
	}
	if trimmed != "" {
		return trimmed
	}
	return "unknown"
}

// riskLevelOverrides — path 子串匹配表，在 method 推导之前检查。
// pattern 不匹配时 method 继续用默认 switch。method="" 表示所有方法。
// 新增高风险 endpoint 直接往这里加一行，不需要改 chi 路由注册。
var riskLevelOverrides = []struct {
	pattern string
	method  string
	level   scopeauthorizer.RiskLevel
}{
	{"impersonate", "", scopeauthorizer.RiskCritical},
	{"batch/", "", scopeauthorizer.RiskCritical},                // /api/v1/batch/users, /batch/organizations, ...
	{"batch-enforce", methodPost, scopeauthorizer.RiskCritical}, // /enforcers/{id}/batch-enforce, /permissions/batch-enforce
	{"batch-check", methodPost, scopeauthorizer.RiskCritical},   // /rebac/namespaces/{id}/batch-check
	{"consents", methodPost, scopeauthorizer.RiskHigh},          // consent 写入属于 medium+
}

// riskLevelFor 根据 HTTP path + method 推导 RiskLevel。
// 先用 path-pattern 覆盖表处理 impersonate/batch 等高风险特例，再按 method 推默认值：
// GET/HEAD/OPTIONS → low, DELETE → high, POST/PUT/PATCH → medium。
//
// 为什么不搞 chi route annotation？因为 scopeauthorizer 只看到 path + method，
// 看不到 chi 路由 handler 上下文。path 子串匹配足够覆盖已知敏感 endpoint 名字模式。
func riskLevelFor(path, method string) scopeauthorizer.RiskLevel {
	method = strings.ToLower(method)
	for _, ov := range riskLevelOverrides {
		if ov.pattern != "" && !strings.Contains(strings.ToLower(path), ov.pattern) {
			continue
		}
		if ov.method != "" && ov.method != method {
			continue
		}
		return ov.level
	}
	switch method {
	case methodGet, methodHead, methodOptions:
		return scopeauthorizer.RiskLow
	case methodDelete:
		return scopeauthorizer.RiskHigh
	case methodPost, methodPut, methodPatch:
		return scopeauthorizer.RiskMedium
	default:
		return scopeauthorizer.RiskMedium
	}
}

// enforceCrossRegion compares the session organization's region with the target
// organization's region. Missing bindings fail closed only in enforce mode.
func enforceCrossRegion(ctx context.Context, r *http.Request, resolver RegionResolver, mode RegionResidencyMode, sourceOrgID, targetOrgID string) error {
	codes, err := resolver.ResolveBatch(ctx, []string{sourceOrgID, targetOrgID})
	if err != nil {
		slog.WarnContext(ctx, "region residency check failed to resolve organizations",
			"event", "region_residency_lookup_failed", "mode", string(mode),
			"source_organization_id", sourceOrgID, "target_organization_id", targetOrgID, "error", err)
		if mode == RegionResidencyEnforce {
			return apperrors.NewForbidden("cross-region residency cannot be verified")
		}
		return nil
	}
	sourceCode, sourceOK := codes[sourceOrgID]
	targetCode, targetOK := codes[targetOrgID]
	if sourceOK && targetOK && sourceCode == targetCode {
		return nil
	}
	slog.WarnContext(ctx, "cross-region tenant access detected",
		"event", "region_residency_violation", "mode", string(mode),
		"source_organization_id", sourceOrgID, "source_region", sourceCode,
		"target_organization_id", targetOrgID, "target_region", targetCode,
		"path", r.URL.Path, "method", r.Method)
	if mode == RegionResidencyEnforce {
		return apperrors.NewForbidden("cross-region tenant access is not allowed")
	}
	return nil
}
