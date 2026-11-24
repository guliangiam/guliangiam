package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/ratelimit"
	"github.com/guliangiam/guliangiam/internal/transport/http/util"
)

func TestServiceIPLimiter_DefaultConfigs(t *testing.T) {
	configs := DefaultServiceIPLimitConfigs
	if len(configs) == 0 {
		t.Error("expected default configs to be non-empty")
	}

	for path, config := range configs {
		if config.Rate <= 0 {
			t.Errorf("config for %s: Rate must be > 0, got %f", path, config.Rate)
		}
		if config.Burst <= 0 {
			t.Errorf("config for %s: Burst must be > 0, got %d", path, config.Burst)
		}
		if config.TTL <= 0 {
			t.Errorf("config for %s: TTL must be > 0, got %v", path, config.TTL)
		}
	}
}

func TestServiceIPLimiter_Middleware_AllowsRequest(t *testing.T) {
	configs := map[string]ServiceIPLimitConfig{
		"/api/v1/auth/login": {Rate: 10.0 / 60, Burst: 10, TTL: 15 * time.Minute},
	}
	limiter := NewServiceIPLimiter(configs)
	defer limiter.Stop()

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", http.NoBody)
	req.RemoteAddr = "192.168.1.1:1234"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestServiceIPLimiter_Middleware_UnlimitedPath(t *testing.T) {
	configs := map[string]ServiceIPLimitConfig{
		"/api/v1/auth/login": {Rate: 1.0 / 60, Burst: 1, TTL: 1 * time.Minute},
	}
	limiter := NewServiceIPLimiter(configs)
	defer limiter.Stop()

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	req.RemoteAddr = "192.168.1.1:1234"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 for unlimited path, got %d", rec.Code)
	}
}

func TestServiceIPLimiter_ExtractClientIP(t *testing.T) {
	limiter := NewServiceIPLimiter(nil)
	defer limiter.Stop()

	tests := []struct {
		name       string
		remoteAddr string
		xForwarded string
		xRealIP    string
		expected   string
	}{
		{"RemoteAddr with port", "192.168.1.1:1234", "", "", "192.168.1.1"},
		{"RemoteAddr without port", "192.168.1.1", "", "", "192.168.1.1"},
		{"X-Forwarded-For", "10.0.0.1:1234", "192.168.1.2", "", "192.168.1.2"},
		{"X-Real-IP", "10.0.0.1:1234", "", "192.168.1.3", "192.168.1.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.RemoteAddr = tt.remoteAddr
			if tt.xForwarded != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwarded)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			ip := util.ClientIP(req)
			if ip != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, ip)
			}
		})
	}
}

func TestServiceIPLimiter_DifferentIPsIndependent(t *testing.T) {
	configs := map[string]ServiceIPLimitConfig{
		"/api/v1/auth/login": {Rate: 1.0 / 60, Burst: 1, TTL: 1 * time.Minute},
	}
	limiter := NewServiceIPLimiter(configs)
	defer limiter.Stop()

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, ip := range []string{"192.168.1.1:1234", "192.168.1.2:1234", "192.168.1.3:1234"} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", http.NoBody)
		req.RemoteAddr = ip
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request from %s: expected 200, got %d", ip, rec.Code)
		}
	}
}

func TestServiceIPLimiter_RateLimitHeaders(t *testing.T) {
	configs := map[string]ServiceIPLimitConfig{
		"/api/v1/auth/login": {Rate: 10.0 / 60, Burst: 10, TTL: 15 * time.Minute},
	}
	limiter := NewServiceIPLimiter(configs)
	defer limiter.Stop()

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", http.NoBody)
	req.RemoteAddr = "192.168.1.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	limitHeader := rec.Header().Get(ratelimit.HeaderLimit)
	if limitHeader != "10" {
		t.Errorf("expected RateLimit-Limit=10, got %s", limitHeader)
	}

	remainingHeader := rec.Header().Get(ratelimit.HeaderRemaining)
	if remainingHeader == "" {
		t.Error("expected RateLimit-Remaining header to be set on success, not only on 429")
	}

	// 成功响应也必须携带完整字段：客户端只有拿到 Reset/Policy 才能提前降速。
	// 曾经只在 429 时给出额度，客户端只能等到被拒才感知配额。
	for _, name := range []string{ratelimit.HeaderReset, ratelimit.HeaderPolicy} {
		if rec.Header().Get(name) == "" {
			t.Errorf("expected %s header to be set on success", name)
		}
	}
	// RateLimit-Policy 声明窗口配额（rate×60=10，单位秒=60）；Burst 与配额相同，
	// 因此按 RFC 9331 的可选参数规则省略，不写出冗余字段。
	if policy := rec.Header().Get(ratelimit.HeaderPolicy); policy != `10;w=60;comment="service-ip:/api/v1/auth/login"` {
		t.Errorf("unexpected RateLimit-Policy: %s", policy)
	}
	if rec.Header().Get(ratelimit.HeaderRetryAfter) != "" {
		t.Error("Retry-After must not be sent on a successful response")
	}
}

func TestServiceIPLimiter_RateLimitHeaders_WhenLimited(t *testing.T) {
	configs := map[string]ServiceIPLimitConfig{
		"/api/v1/auth/login": {Rate: 1.0 / 60, Burst: 1, TTL: 1 * time.Minute},
	}
	limiter := NewServiceIPLimiter(configs)
	defer limiter.Stop()

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", http.NoBody)
	req.RemoteAddr = "192.168.1.1:1234"

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req)
	if rec1.Code != http.StatusOK {
		t.Errorf("first request: expected 200, got %d", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("second request: expected 429, got %d", rec2.Code)
	}

	remainingHeader := rec2.Header().Get(ratelimit.HeaderRemaining)
	if remainingHeader != "0" {
		t.Errorf("expected RateLimit-Remaining=0 when rate limited, got %s", remainingHeader)
	}

	// Retry-After 只有两种合法形式：delta-seconds 与 HTTP-date。Go 时长字符串
	// （此前的 `1m0s`）两者都不是，客户端解析失败后只能固定间隔重试。
	retryAfter := rec2.Header().Get(ratelimit.HeaderRetryAfter)
	seconds, err := strconv.Atoi(retryAfter)
	if err != nil {
		t.Fatalf("Retry-After must be integer delta-seconds, got %q", retryAfter)
	}
	if seconds < 1 {
		t.Errorf("Retry-After must be positive, got %d", seconds)
	}

	if reset := rec2.Header().Get(ratelimit.HeaderReset); reset != retryAfter {
		t.Errorf("RateLimit-Reset (%s) must agree with Retry-After (%s) when denied", reset, retryAfter)
	}
}

func TestLoadConfigFromEnv_Defaults(t *testing.T) {
	// Clear env vars
	os.Unsetenv("RATE_LIMIT_PER_IP_REQUESTS")
	os.Unsetenv("RATE_LIMIT_PER_IP_WINDOW")
	os.Unsetenv("RATE_LIMIT_PER_IP_CLEANUP_INTERVAL")

	envConfig := LoadConfigFromEnv()
	configs := envConfig.Configs

	if len(configs) == 0 {
		t.Error("expected non-empty configs")
	}

	loginConfig, ok := configs["/api/v1/auth/login"]
	if !ok {
		t.Error("expected login config")
	}
	if loginConfig.Rate <= 0 {
		t.Errorf("expected positive rate, got %f", loginConfig.Rate)
	}
	if loginConfig.Burst <= 0 {
		t.Errorf("expected positive burst, got %d", loginConfig.Burst)
	}
	if envConfig.CleanupInterval != 300*time.Second {
		t.Errorf("expected default cleanup interval 300s, got %v", envConfig.CleanupInterval)
	}
}

func TestLoadConfigFromEnv_CustomValues(t *testing.T) {
	os.Setenv("RATE_LIMIT_PER_IP_REQUESTS", "20")
	os.Setenv("RATE_LIMIT_PER_IP_WINDOW", "120")
	os.Setenv("RATE_LIMIT_PER_IP_CLEANUP_INTERVAL", "600")
	defer os.Unsetenv("RATE_LIMIT_PER_IP_REQUESTS")
	defer os.Unsetenv("RATE_LIMIT_PER_IP_WINDOW")
	defer os.Unsetenv("RATE_LIMIT_PER_IP_CLEANUP_INTERVAL")

	envConfig := LoadConfigFromEnv()
	configs := envConfig.Configs

	loginConfig, ok := configs["/api/v1/auth/login"]
	if !ok {
		t.Error("expected login config")
	}
	// With 20 requests per 120 seconds, rate should be 20/120
	expectedRate := 20.0 / 120.0
	if loginConfig.Rate != expectedRate {
		t.Errorf("expected rate %f, got %f", expectedRate, loginConfig.Rate)
	}
	if loginConfig.Burst != 20 {
		t.Errorf("expected burst 20, got %d", loginConfig.Burst)
	}
	if envConfig.CleanupInterval != 600*time.Second {
		t.Errorf("expected cleanup interval 600s, got %v", envConfig.CleanupInterval)
	}
}

func TestLoadConfigFromEnv_InvalidValues(t *testing.T) {
	os.Setenv("RATE_LIMIT_PER_IP_REQUESTS", "0")
	os.Setenv("RATE_LIMIT_PER_IP_WINDOW", "-1")
	os.Setenv("RATE_LIMIT_PER_IP_CLEANUP_INTERVAL", "-5")
	defer os.Unsetenv("RATE_LIMIT_PER_IP_REQUESTS")
	defer os.Unsetenv("RATE_LIMIT_PER_IP_WINDOW")
	defer os.Unsetenv("RATE_LIMIT_PER_IP_CLEANUP_INTERVAL")

	envConfig := LoadConfigFromEnv()
	configs := envConfig.Configs

	// Should fall back to defaults (10 requests, 60 seconds)
	loginConfig, ok := configs["/api/v1/auth/login"]
	if !ok {
		t.Error("expected login config")
	}
	if loginConfig.Burst != 10 {
		t.Errorf("expected default burst 10, got %d", loginConfig.Burst)
	}
	// Cleanup interval should fall back to default 300s
	if envConfig.CleanupInterval != 300*time.Second {
		t.Errorf("expected default cleanup interval 300s, got %v", envConfig.CleanupInterval)
	}
}

func TestLoadConfigFromEnv_EmailEndpoints(t *testing.T) {
	os.Unsetenv("RATE_LIMIT_PER_IP_REQUESTS")
	os.Unsetenv("RATE_LIMIT_PER_IP_WINDOW")
	os.Unsetenv("RATE_LIMIT_PER_IP_CLEANUP_INTERVAL")

	envConfig := LoadConfigFromEnv()
	configs := envConfig.Configs

	emailConfig, ok := configs["/api/v1/messages/send-email"]
	if !ok {
		t.Error("expected email config")
	}
	// Email should be heavily rate limited: 1 per hour
	if emailConfig.Rate != 1.0/3600 {
		t.Errorf("expected email rate 1/3600, got %f", emailConfig.Rate)
	}
	if emailConfig.Burst != 1 {
		t.Errorf("expected email burst 1, got %d", emailConfig.Burst)
	}
}
