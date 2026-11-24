package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/guliangiam/guliangiam/internal/platform/ratelimit"
)

func corsTestConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{
			"https://console.example.com",
			"http://localhost:7101",
			"https://*.example.org",
		},
		AllowedMethods:   corsDefaultAllowedMethods,
		AllowCredentials: true,
	}
}

func serveCORS(t *testing.T, config CORSConfig, method, origin string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	nextCalled := false
	handler := CORS(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(method, "/api/v1/users", http.NoBody)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if method == http.MethodOptions && rec.Code == http.StatusNoContent && nextCalled {
		t.Error("预检必须由中间件终结，不得下发到业务链")
	}
	return rec
}

func TestCORS_ExactOriginIsEchoedWithCredentials(t *testing.T) {
	rec := serveCORS(t, corsTestConfig(), http.MethodGet, "https://console.example.com", nil)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://console.example.com" {
		t.Errorf("Allow-Origin = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Allow-Credentials = %q", got)
	}
	if !strings.Contains(rec.Header().Get("Vary"), "Origin") {
		t.Errorf("Vary = %q, must contain Origin", rec.Header().Get("Vary"))
	}
	if rec.Code != http.StatusOK {
		t.Errorf("实际请求应继续下发，got %d", rec.Code)
	}
}

// 未命中白名单绝不能回显来源：回显等于放行任意站点。
func TestCORS_UnlistedOriginIsNotEchoed(t *testing.T) {
	for _, origin := range []string{
		"https://evil.example.net",
		"https://evilexample.com",    // 通配条目的点边界
		"https://example.org",        // 通配条目要求非空子域
		"http://console.example.com", // scheme 不匹配
		"https://console.example.com.evil.net",
	} {
		rec := serveCORS(t, corsTestConfig(), http.MethodGet, origin, nil)
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("origin %q 不在白名单却回显了 %q", origin, got)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("origin %q：未命中白名单不应报错，got %d", origin, rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Vary"), "Origin") {
			t.Errorf("origin %q：未命中也必须声明 Vary: Origin", origin)
		}
	}
}

func TestCORS_WildcardSubdomainRequiresNonEmptyLabelAndDotBoundary(t *testing.T) {
	for origin, want := range map[string]bool{
		"https://app.example.org":      true,
		"https://a.b.example.org":      true,
		"https://example.org":          false,
		"https://evilexample.org":      false,
		"http://app.example.org":       false,
		"https://app.example.org.evil": false,
	} {
		rec := serveCORS(t, corsTestConfig(), http.MethodGet, origin, nil)
		got := rec.Header().Get("Access-Control-Allow-Origin") != ""
		if got != want {
			t.Errorf("origin %q: allowed = %v, want %v", origin, got, want)
		}
	}
}

func TestCORS_PreflightDoesNotReflectRequestedHeaders(t *testing.T) {
	rec := serveCORS(t, corsTestConfig(), http.MethodOptions, "https://console.example.com", map[string]string{
		"Access-Control-Request-Method":  "PATCH",
		"Access-Control-Request-Headers": "X-Evil-Header, Authorization",
	})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("预检应返回 204，got %d", rec.Code)
	}
	allowHeaders := rec.Header().Get("Access-Control-Allow-Headers")
	if strings.Contains(allowHeaders, "X-Evil-Header") {
		t.Error("Allow-Headers 反射了客户端请求头（CSRF 温床）")
	}
	for _, required := range []string{"Authorization", "Content-Type", "Idempotency-Key", "If-Match"} {
		if !strings.Contains(allowHeaders, required) {
			t.Errorf("Allow-Headers 缺少固定清单项 %q：%q", required, allowHeaders)
		}
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got != "86400" {
		t.Errorf("Max-Age = %q, want %q", got, "86400")
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "PATCH") {
		t.Errorf("Allow-Methods = %q", got)
	}
}

func TestCORS_PreflightForUnlistedOriginIsRejectedWithoutAllowHeaders(t *testing.T) {
	rec := serveCORS(t, corsTestConfig(), http.MethodOptions, "https://evil.example.net", map[string]string{
		"Access-Control-Request-Method": "POST",
	})
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("未命中白名单的预检不应回显来源，got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "" {
		t.Errorf("未命中白名单的预检不应给出方法清单，got %q", got)
	}
}

// 前端依赖这些头做退避与问题定位；漏暴露等于限流反馈在浏览器侧不可见。
// `ETag` 单独成由：控制台默认跨域部署，而 42.3.11 的条件请求必须携带
// `If-None-Match`——读不到 `ETag` 就无从携带，轮询会退化为每次全量响应。
func TestCORS_ExposesRateLimitAndRequestIDHeaders(t *testing.T) {
	rec := serveCORS(t, corsTestConfig(), http.MethodGet, "https://console.example.com", nil)

	exposed := rec.Header().Get("Access-Control-Expose-Headers")
	for _, required := range []string{
		ratelimit.HeaderLimit,
		ratelimit.HeaderRemaining,
		ratelimit.HeaderReset,
		ratelimit.HeaderPolicy,
		ratelimit.HeaderRetryAfter,
		"ETag",
		"Location",
		"X-Request-Id",
	} {
		if !strings.Contains(exposed, required) {
			t.Errorf("Expose-Headers 缺少 %q：%q", required, exposed)
		}
	}
}

func TestCORS_NoOriginHeaderIsNoop(t *testing.T) {
	rec := serveCORS(t, corsTestConfig(), http.MethodGet, "", nil)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("无 Origin 头不应写入 CORS 头，got %q", got)
	}
	if rec.Header().Get("Vary") != "" {
		t.Errorf("同源请求不应引入 Vary，got %q", rec.Header().Get("Vary"))
	}
}

func TestCORS_EmptyWhitelistDeniesEverything(t *testing.T) {
	rec := serveCORS(t, CORSConfig{AllowedMethods: corsDefaultAllowedMethods}, http.MethodGet, "https://console.example.com", nil)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("空白名单必须完全不跨域，got %q", got)
	}
}

func TestCORSConfig_Validate(t *testing.T) {
	valid := []string{"https://console.example.com", "http://localhost:7101", "https://*.example.org"}
	for _, pattern := range valid {
		if err := (CORSConfig{AllowedOrigins: []string{pattern}}).Validate(); err != nil {
			t.Errorf("%q 应合法，得到 %v", pattern, err)
		}
	}
	invalid := []string{"*", "http://*", "http://*.example.com", "https://*.", "console.example.com", "https://*/path"}
	for _, pattern := range invalid {
		if err := (CORSConfig{AllowedOrigins: []string{pattern}}).Validate(); err == nil {
			t.Errorf("%q 应被判为非法", pattern)
		}
	}
}

func TestCORSConfigFromEnv_Defaults(t *testing.T) {
	t.Setenv(EnvCORSAllowedOrigins, "")
	t.Setenv(EnvCORSAllowedMethods, "")
	config := CORSConfigFromEnv()
	if config.Enabled() {
		t.Error("未配置白名单时不得启用跨域")
	}
	if len(config.AllowedMethods) != len(corsDefaultAllowedMethods) {
		t.Errorf("AllowedMethods = %v", config.AllowedMethods)
	}

	t.Setenv(EnvCORSAllowedOrigins, " https://console.example.com , http://localhost:7101 ")
	t.Setenv(EnvCORSAllowedMethods, "GET,POST")
	config = CORSConfigFromEnv()
	if len(config.AllowedOrigins) != 2 {
		t.Fatalf("AllowedOrigins = %v", config.AllowedOrigins)
	}
	if config.AllowedOrigins[0] != "https://console.example.com" {
		t.Errorf("空白未裁剪：%q", config.AllowedOrigins[0])
	}
	if err := config.Validate(); err != nil {
		t.Errorf("环境装配出的配置应合法：%v", err)
	}
	if len(config.AllowedMethods) != 2 {
		t.Errorf("AllowedMethods = %v", config.AllowedMethods)
	}
}
