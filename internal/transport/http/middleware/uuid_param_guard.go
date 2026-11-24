package middleware

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	guuid "github.com/google/uuid"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	httperr "github.com/guliangiam/guliangiam/internal/platform/http"
)

// uuidParamGuardPrefixes 是已确认「{id} 必须是 UUID」的管理面资源前缀。
// 采用白名单而不是全局规则：i18n 等路由的 {id} 是业务键，不能按 UUID 校验。
var uuidParamGuardPrefixes = []string{
	"/api/v1/users",
	"/api/v1/groups",
	"/api/v1/applications",
	"/api/v1/providers",
	"/api/v1/roles",
	"/api/v1/keys",
	"/api/v1/invitations",
	"/api/v1/webhooks",
	"/api/v1/permissions",
	"/api/v1/ldaps",
	"/api/v1/identity-pools",
	"/api/v1/plans",
	// BE-E-10：以下模块的 {id}/{xxxId} 均为 UUID（schema/UUID 生成器核实），
	// 此前非法 UUID 直达 SQL 22P02 → 500。豁免：organizations（{id} 是 slug，
	// 仅其 domains 子路由的 orgId/domainId 是 UUID）、webhook-events（自校验）；
	// coupons/forms/models/rules 已由 pgerror.Map 映射，不重复接管。
	"/api/v1/actions",
	"/api/v1/adapters",
	"/api/v1/agents",
	"/api/v1/captcha-providers",
	"/api/v1/certs",
	"/api/v1/enforcers",
	"/api/v1/entries",
	"/api/v1/face-id-providers",
	"/api/v1/ldap-sync-configs",
	"/api/v1/mcp-servers",
	"/api/v1/notification-providers",
	"/api/v1/notifications",
	"/api/v1/orders",
	"/api/v1/pay-attempts",
	"/api/v1/pay-channel-configs",
	"/api/v1/pay-recon-batches",
	"/api/v1/pay-recon-items",
	"/api/v1/pay-refunds",
	"/api/v1/payments",
	"/api/v1/pricings",
	"/api/v1/products",
	"/api/v1/radius-accountings",
	"/api/v1/radius-configs",
	"/api/v1/resources",
	"/api/v1/sessions",
	"/api/v1/subscriptions",
	"/api/v1/syncers",
	"/api/v1/tickets",
	"/api/v1/tokens",
	"/api/v1/transactions",
	"/api/v1/verifications",
	"/api/v1/idv/verifications",
	"/api/v1/privacy",
	"/api/v1/iga",
	"/api/v1/metering",
	"/api/v1/risk",
	"/api/v1/rebac",
	"/api/v1/ai",
	"/api/v1/organizations/{orgId}/domains",
}

// UUIDParamGuard 在路由匹配后校验白名单资源中的 {id}、{xxxId} 路径参数：
// 非法 UUID 返回 400 invalid_argument（DEF-44），而不是把 pgx 的
// "invalid input syntax for type uuid" 包装成 500。
func UUIDParamGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pattern := chi.RouteContext(r.Context()).RoutePattern()
		if pattern == "" || !matchesGuardPrefix(pattern) {
			next.ServeHTTP(w, r)
			return
		}
		for _, name := range idParamNames(pattern) {
			value := strings.TrimSpace(chi.URLParam(r, name))
			if value == "" {
				continue
			}
			if _, err := guuid.Parse(value); err != nil {
				httperr.WriteError(w, r, apperrors.NewWithMeta(
					apperrors.CodeInvalidArgument,
					"path parameter "+name+" must be a UUID",
					map[string]any{"field": name},
				))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func matchesGuardPrefix(pattern string) bool {
	// 豁免：/api/v1/applications/by-client-id/{clientId} 的 {clientId} 是 OAuth
	// client_id（如 gl_cid_xxx / built-in-client），不是 UUID；它命中了
	// /api/v1/applications 前缀，但不能按 UUID 校验，否则 OIDC 登录页直接 400。
	// 按完整路由模板精确豁免：子串匹配会连带放过未来任何含 /by-client-id/
	// 片段的路由（如其下再挂 {id} 子资源），使 UUID 守卫出现静默盲区。
	if pattern == "/api/v1/applications/by-client-id/{clientId}" {
		return false
	}
	for _, prefix := range uuidParamGuardPrefixes {
		if pattern == prefix || strings.HasPrefix(pattern, prefix+"/") {
			return true
		}
	}
	return false
}

// idParamNames 从路由模板中取 {id} / {xxxId} 形式的参数名。
// chi 的自定义方法段形如 {id}:transition，先剥掉 :verb 后缀再识别。
func idParamNames(pattern string) []string {
	names := make([]string, 0, 2)
	for _, segment := range strings.Split(pattern, "/") {
		if idx := strings.IndexByte(segment, ':'); idx >= 0 {
			segment = segment[:idx]
		}
		if len(segment) < 3 || segment[0] != '{' || segment[len(segment)-1] != '}' {
			continue
		}
		name := segment[1 : len(segment)-1]
		if name == "id" || strings.HasSuffix(name, "Id") {
			names = append(names, name)
		}
	}
	return names
}
