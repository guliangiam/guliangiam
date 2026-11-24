package auth

import (
	"context"
	"fmt"
	"log/slog"
	pathpkg "path"
	"strings"
)

// NamespaceResolver resolves the default ReBAC namespace UUID for an
// organization. The transport layer declares the port locally (Clean
// Architecture Rule 2: transport does not import domain); the pgx/RLS adapter
// lives in infra/postgres/rebac and is injected by bootstrap.
type NamespaceResolver interface {
	Resolve(ctx context.Context, organizationID string) (string, error)
}

// PermissionConfig maps HTTP method + path pattern to a required permission.
// This is used by the APIAuthorizationMiddleware to enforce fine-grained access control.
type PermissionConfig struct {
	Method     string
	Pattern    string
	Permission string
}

// CasbinPermissionConfig holds the configuration for RequirePermission middleware
// that integrates with the Casbin enforcer.
type CasbinPermissionConfig struct {
	EnforcerID string
	ModelText  string
}

// defaultRBACModel returns the standard 4-parameter RBAC Casbin model.
// 用于普通 user 主体：org 维度强制匹配（防止跨租户）。
func defaultRBACModel() string {
	return `[request_definition]
r = sub, org, obj, act

[policy_definition]
p = sub, org, obj, act

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = (r.sub == p.sub || g(r.sub, p.sub, r.org)) && r.org == p.org && (p.obj == "*" || r.obj == p.obj) && (p.act == "*" || r.act == p.act)
`
}

// platformAdminRBACModel 3 参数 matcher — org 维度通配符。
// 用于 platform_admin 主体：Casbin 只做 resource_type × action 基础授权，
// 跨租户校验交给 ScopeAuthorizer（在 Casbin 之后跑）。
//
// 评审 C1 修复：Casbin matcher 去掉 org 维度检查，platform_admin 的
// Casbin policy 用 p, platform_admin, *, *, * 通配符覆盖所有 org。
func platformAdminRBACModel() string {
	return `[request_definition]
r = sub, org, obj, act

[policy_definition]
p = sub, org, obj, act

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = (r.sub == p.sub || g(r.sub, p.sub, "*")) && (p.obj == "*" || r.obj == p.obj) && (p.act == "*" || r.act == p.act)
`
}

// principalModel 根据 SessionContext.PrincipalType 选择 Casbin model。
//   - "platform_admin" → platformAdminRBACModel（org 通配符）
//   - "service_account" / "user" / 空 → defaultRBACModel（org 强制匹配）
func principalModel(principalType string) string {
	switch principalType {
	case "platform_admin":
		return platformAdminRBACModel()
	default:
		return defaultRBACModel()
	}
}

// matchPermissionOverride returns the declared permission pin for a request.
// Pattern follows path.Match semantics with "{param}" treated as a single
// segment wildcard, so "/api/v1/users/{id}/set-password" also accepts the
// literal chi form written in route definitions.
func matchPermissionOverride(method, path string, overrides []PermissionConfig) (PermissionConfig, bool) {
	for _, override := range overrides {
		if override.Permission == "" {
			continue
		}
		if override.Method != "" && !strings.EqualFold(override.Method, method) {
			continue
		}
		// chi 参数段写成 "{id}"，path.Match 的通配符是单个 "*"。
		// 逐段替换：只把整段参数占位符换成 "*"，避免留下 "*id" 这种
		// 永远匹配不上的模式。
		segments := strings.Split(override.Pattern, "/")
		for i, segment := range segments {
			if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
				segments[i] = "*"
			}
		}
		pattern := strings.Join(segments, "/")
		if matched, err := pathpkg.Match(pattern, path); err != nil || !matched {
			continue
		}
		return override, true
	}
	return PermissionConfig{}, false
}

// splitDeclaredPermission splits "resource:action" at the FIRST colon, so a
// two-part action such as "users:password:reset" becomes ("users", "password:reset").
func splitDeclaredPermission(permission string) (string, string) {
	trimmed := strings.TrimSpace(permission)
	if idx := strings.Index(trimmed, ":"); idx >= 0 {
		return trimmed[:idx], trimmed[idx+1:]
	}
	return trimmed, ""
}

// casbinActionCandidates returns the Casbin action names to try for an HTTP
// request, in order.
//
// Two policy families coexist: seeded RBAC roles grant the documented CRUD
// vocabulary (read/write/delete), while permissions created through the API
// (for example AI Gateway) use HTTP verb names. Trying the verb first and the
// documented alias second lets both authorize without rewriting either family;
// a policy that matches no candidate still falls through to default-deny.
func casbinActionCandidates(method string) []string {
	verb := strings.ToLower(strings.TrimSpace(method))
	alias := ""
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "GET", "HEAD":
		alias = "read"
	case "POST", "PUT", "PATCH":
		alias = "write"
	case "DELETE":
		alias = "delete"
	}
	if alias == "" || alias == verb {
		return []string{verb}
	}
	return []string{verb, alias}
}

// resourceFromPath extracts a resource identifier from the URL path.
//
// Normal mappings:
//
//	"/api/v1/users/123"       -> "users"
//	"/api/v1/organizations"   -> "organizations"
//	"/api/v1/global/roles"    -> "roles"
//
// AI Gateway special — design doc §管理面路由定义 uses /api/v1/ai/*
// but Casbin policies are scoped under the "aigateway:" prefix so a single
// Casbin role (e.g. aigateway.admin) covers the whole subsystem regardless of
// how we shard permissions.yaml today.
//
//	"/api/v1/ai/providers"           -> "aigateway:providers"
//	"/api/v1/ai/providers/{id}/test" -> "aigateway:providers"
//	"/api/v1/ai/api-keys"            -> "aigateway:api_keys"
//	"/api/v1/ai/usage-logs"          -> "aigateway:usage_logs"
func resourceFromPath(path string) string {
	trimmed := strings.TrimPrefix(path, "/api/v1/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 {
		return path
	}

	// AI Gateway: /api/v1/ai/{entity} → "aigateway:{entity}"
	if parts[0] == "ai" && len(parts) > 1 {
		// Data-plane faces: /api/v1/ai/faces/{provider}/... → "aigateway:chat"
		// All three protocol faces (openai / anthropic / gemini) share the same
		// resource — they're all data-plane chat/embeddings endpoints.
		if parts[1] == "faces" {
			return "aigateway:chat"
		}
		entity := parts[1]
		// Strip action suffix — chi uses /providers/{id}/test, not {id}:test,
		// but we also tolerate the design-doc colon form for safety.
		if idx := strings.Index(entity, ":"); idx >= 0 {
			entity = entity[:idx]
		}
		// Normalize hyphenated segments (api-keys, usage-logs) to underscore.
		entity = strings.ReplaceAll(entity, "-", "_")
		return "aigateway:" + entity
	}

	// /api/v1/global/{resource} → the concrete resource (no prefix)
	if parts[0] == "global" && len(parts) > 1 {
		resource := parts[1]
		if idx := strings.Index(resource, ":"); idx >= 0 {
			resource = resource[:idx]
		}
		return resource
	}

	// Default: /api/v1/{module}/... → module unchanged (orders, payments, etc.)
	resource := parts[0]
	if idx := strings.Index(resource, ":"); idx >= 0 {
		resource = resource[:idx]
	}
	return resource
}

// isRebacEndpoint returns true for /api/v1/rebac/* endpoints.
// These endpoints expose the ReBAC evaluator itself — running a ReBAC
// override against them would be circular (the API IS the engine).
// Casbin still runs on these endpoints (Gate 5), only Gate 5b (ReBAC Check) is skipped.
func isRebacEndpoint(path string) bool {
	return strings.HasPrefix(path, "/api/v1/rebac/")
}

// logWarn is a thin slog wrapper so the auth middleware package doesn't
// need to depend on the platform logging package.
func logWarn(ctx context.Context, format string, args ...any) {
	slog.WarnContext(ctx, fmt.Sprintf(format, args...))
}
