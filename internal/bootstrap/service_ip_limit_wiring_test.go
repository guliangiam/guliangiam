package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// 服务级 per-IP 限流器必须读取 RATE_LIMIT_PER_IP_REQUESTS/WINDOW。
// 之前 runtime 直接传 DefaultServiceIPLimitConfigs，导致这两个已文档化的
// 环境变量在生产路径上完全不生效——E2E 连跑十几个登录就被 429 挡住。
func TestNewServiceIPLimiterFromEnvHonorsRequestsEnv(t *testing.T) {
	t.Setenv("RATE_LIMIT_PER_IP_REQUESTS", "40")
	t.Setenv("RATE_LIMIT_PER_IP_WINDOW", "60")

	limiter := newServiceIPLimiterFromEnv()
	t.Cleanup(limiter.Stop)

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", http.NoBody)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d got 429; RATE_LIMIT_PER_IP_REQUESTS=40 should allow 20", i+1)
		}
	}
}

// 未设置环境变量时必须保持默认的严格登录限流（10/min per IP）。
func TestNewServiceIPLimiterFromEnvKeepsLoginDefault(t *testing.T) {
	t.Setenv("RATE_LIMIT_PER_IP_REQUESTS", "")
	t.Setenv("RATE_LIMIT_PER_IP_WINDOW", "")

	limiter := newServiceIPLimiterFromEnv()
	t.Cleanup(limiter.Stop)

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	statuses := make([]int, 0, 12)
	for i := 0; i < 12; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", http.NoBody)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		statuses = append(statuses, rec.Code)
	}

	got429 := false
	for _, status := range statuses {
		if status == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	if !got429 {
		t.Fatalf("default login limit should reject after burst, statuses=%v", statuses)
	}
}
