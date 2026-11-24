package bootstrap

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authv1 "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
)

// rootPublicPatterns = chi.go 根 router 上直接注册、不在任何 api/v1/* 模块
// routes.go 里的公开端点。合约测试里用 allowlist 跳过对它们的"routes.go 必须声明"检查。
// 规则：凡是 isTrulyPublicEndpoint 返回 true 但 routes.go 里找不到的，必须在这里说明。
var rootPublicPatterns = map[string]bool{
	"/healthz":                          true, // chi.go 根 router 直接注册 k8s 探针
	"/livez":                            true,
	"/.well-known/openid-configuration": true, // auth.RegisterChiRoutes 里直接注册
	"/uploads/*":                        true, // chi.go 静态文件代理
	"/openapi/":                         true, // OpenAPI spec 挂载
}

// TestPublicWhitelist_NoRouteDrift 是"chi 注册 ↔ 白名单字符串"防漂移护栏。
//
// 历史（2026-09-30）：chi 注册了 /applications/resolve（斜杠），
// isTrulyPublicEndpoint 白名单里写成了 /applications:resolve（冒号），
// 前端也跟着用冒号。三层同时漂移 → 登录前应用发现端点直接 401。
//
// 本测试分正反两向：
//
//	(A) routes.go 声明的 Pattern 里"看起来应该公开"的候选（协议面端点、DCR、
//	    system health、应用发现、auth 登录流端点）必须在白名单里。
//	    captcha-provider 管理面是受认证的租户管理 CRUD，不在候选中；登录页
//	    读取的 /api/v1/auth/captcha* 由 auth 登录流候选覆盖。
//	(B) 白名单里标为 truly public 的关键路径，routes.go 必须真有声明（exact 或前缀），
//	    防止"白名单有但路由没注册"的静默漂移。根路由 allowlist 里的条目例外。
func TestPublicWhitelist_NoRouteDrift(t *testing.T) {
	type declared struct {
		module  string
		method  string
		pattern string
	}
	var all []declared
	for _, m := range declaredRouteModules {
		for _, d := range m.routes() {
			all = append(all, declared{m.name, d.Method, d.Pattern})
		}
	}

	t.Run("A_public_candidates_must_be_in_whitelist", func(t *testing.T) {
		isPublicCandidate := func(p string) bool {
			switch {
			case strings.HasPrefix(p, "/oauth2/"),
				strings.HasPrefix(p, "/.well-known/"),
				strings.HasPrefix(p, "/cas/"),
				strings.HasPrefix(p, "/kerberos/"),
				strings.HasPrefix(p, "/scim/v2/"),
				strings.HasPrefix(p, "/api/v1/saml/"):
				return true
			case strings.HasPrefix(p, "/api/v1/dcr/"),
				p == "/api/v1/invitations/verify",
				p == "/api/v1/system/health",
				strings.HasPrefix(p, "/api/v1/payments/notify/"):
				return true
			case p == "/api/v1/applications/resolve",
				strings.HasPrefix(p, "/api/v1/applications/by-client-id/"):
				return true
			case strings.HasPrefix(p, "/api/v1/auth/"):
				return authv1.IsTrulyPublicEndpoint(p)
			}
			return false
		}

		var missing []string
		for _, r := range all {
			if !isPublicCandidate(r.pattern) {
				continue
			}
			if !authv1.IsTrulyPublicEndpoint(r.pattern) {
				missing = append(missing, r.method+" "+r.pattern+"  (module "+r.module+")")
			}
		}
		sort.Strings(missing)
		assert.Empty(t, missing,
			"这些 routes.go 声明的端点看起来应该是公开的（登录前可访问），\n"+
				"但 isTrulyPublicEndpoint 白名单里没找到。\n"+
				"要么补上白名单（威胁模型必须说明为什么完全不需要认证），\n"+
				"要么把 routes.go 的 Pattern 改成与白名单完全一致（字符串漂移修复）。")
	})

	t.Run("B_whitelist_paths_must_exist_in_routes", func(t *testing.T) {
		declared := make(map[string]bool, len(all))
		for _, r := range all {
			declared[r.pattern] = true
		}
		prefixDeclared := []string{
			"/oauth2/", "/.well-known/", "/cas/", "/kerberos/", "/scim/v2/",
			"/uploads/", "/openapi/", "/api/v1/dcr/",
			"/api/v1/applications/by-client-id/", "/api/v1/payments/notify/",
		}

		criticalPublicPaths := []string{
			"/.well-known/openid-configuration",
			"/healthz",
			"/api/v1/auth/login",
			"/api/v1/auth/signup",
			"/api/v1/auth/social/login",
			"/api/v1/auth/social/callback",
			"/api/v1/applications/resolve",
			"/api/v1/applications/by-client-id/abc123",
			"/api/v1/dcr/register",
			"/api/v1/invitations/verify",
			"/api/v1/system/health",
			"/api/v1/payments/notify/wechat",
			"/cas/login",
			"/oauth2/token",
			"/scim/v2/Users",
		}
		for _, p := range criticalPublicPaths {
			require.True(t, authv1.IsTrulyPublicEndpoint(p),
				"critical path %q is expected to be truly public but the whitelist says otherwise", p)

			// 根路由 allowlist —— chi.go 直接注册的不走 routes.go
			if rootPublicPatterns[p] {
				continue
			}
			// 前缀匹配（协议面、静态文件等）
			matchedPrefix := false
			for _, prefix := range prefixDeclared {
				if strings.HasPrefix(p, prefix) {
					matchedPrefix = true
					break
				}
			}
			if !declared[p] && !matchedPrefix {
				t.Errorf("白名单标记为 truly public 的路径 %q 在任何 routes.go 里都没声明 "+
					"(既无 exact 匹配，也不匹配前缀组)。要么 routes.go 漏了注册，"+
					"要么白名单里这条过时了该删掉。"+
					"如果是 chi.go 根 router 直接注册的，加到 rootPublicPatterns allowlist 里并说明原因。", p)
			}
		}
	})
}

// TestCaptchaProviderAdmin_NotTrulyPublic 防止管理面再次被前缀白名单放行。
// 登录页的验证码读取走 /api/v1/auth/captcha*；captcha-provider CRUD 必须
// 经正常会话 + Casbin 链路。
func TestCaptchaProviderAdmin_NotTrulyPublic(t *testing.T) {
	for _, p := range []string{
		"/api/v1/captcha-providers",
		"/api/v1/captcha-providers/generate",
		"/api/v1/captcha-providers/abc",
	} {
		assert.False(t, authv1.IsTrulyPublicEndpoint(p),
			"%s must require authentication; it is tenant-admin configuration", p)
	}
}

// TestApplicationsResolve_IsTrulyPublic 是 /applications/resolve 的定向护栏。
// 这条端点是登录前端必须能访问的 pre-login discovery——如果它被误改成受保护，
// 用户根本拿不到 app 配置，登录页直接崩。
func TestApplicationsResolve_IsTrulyPublic(t *testing.T) {
	for _, p := range []string{
		"/api/v1/applications/resolve",
		"/api/v1/applications/by-client-id/built-in-client",
	} {
		assert.True(t, authv1.IsTrulyPublicEndpoint(p),
			"%s MUST be truly public — it's pre-login app discovery", p)
	}
	// 反向：冒号变体是历史漂移错误，必须被拒
	assert.False(t, authv1.IsTrulyPublicEndpoint("/api/v1/applications:resolve"),
		"/api/v1/applications:resolve 是 2026-09-30 漂移错误，绝不能再被放进白名单")
}
