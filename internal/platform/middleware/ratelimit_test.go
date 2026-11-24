package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/guliangiam/guliangiam/internal/platform/ratelimit"
	"github.com/guliangiam/guliangiam/internal/transport/http/util"
)

func TestExtractIP_XForwardedForUsesRightmostUntrustedHop(t *testing.T) {
	r := &http.Request{
		Header:     http.Header{"X-Forwarded-For": []string{"1.2.3.4, 5.6.7.8"}},
		RemoteAddr: "9.10.11.12",
	}
	ip := util.ClientIP(r)
	if ip != "5.6.7.8" {
		t.Errorf("ClientIP() = %q, want 5.6.7.8", ip)
	}
}

func TestExtractIP_XForwardedForSingle(t *testing.T) {
	r := &http.Request{
		Header:     http.Header{"X-Forwarded-For": []string{"1.2.3.4"}},
		RemoteAddr: "9.10.11.12",
	}
	ip := util.ClientIP(r)
	if ip != "1.2.3.4" {
		t.Errorf("ClientIP() = %q, want 1.2.3.4", ip)
	}
}

func TestExtractIP_RemoteAddr(t *testing.T) {
	r := &http.Request{
		Header:     http.Header{},
		RemoteAddr: "9.10.11.12:12345",
	}
	ip := util.ClientIP(r)
	if ip != "9.10.11.12" {
		t.Errorf("ClientIP() = %q, want 9.10.11.12", ip)
	}
}

func TestExtractIP_Unknown(t *testing.T) {
	r := &http.Request{
		Header:     http.Header{},
		RemoteAddr: "",
	}
	ip := util.ClientIP(r)
	if ip != "" {
		t.Errorf("ClientIP() = %q, want empty", ip)
	}
}

func TestRateLimiterConfig_Defaults(t *testing.T) {
	if DefaultAPIRateLimit.Rate <= 0 {
		t.Error("DefaultAPIRateLimit.Rate should be positive")
	}
	if DefaultAPIRateLimit.Burst <= 0 {
		t.Error("DefaultAPIRateLimit.Burst should be positive")
	}
	if AuthRateLimit.Rate <= 0 {
		t.Error("AuthRateLimit.Rate should be positive")
	}
	if SignupRateLimit.Rate <= 0 {
		t.Error("SignupRateLimit.Rate should be positive")
	}
	if PasswordResetRateLimit.Rate <= 0 {
		t.Error("PasswordResetRateLimit.Rate should be positive")
	}
	if TokenEndpointRateLimit.Rate <= 0 {
		t.Error("TokenEndpointRateLimit.Rate should be positive")
	}
	if OtlpRateLimit.Rate <= 0 {
		t.Error("OtlpRateLimit.Rate should be positive")
	}
}

// 预置限流器必须各自带策略 id：三层限流（全局/认证/注册）串联时，客户端与网关
// 只有靠 RateLimit-Policy 的 comment 才能判断是哪一维拒绝的。
func TestRateLimiterConfig_PolicyIDs(t *testing.T) {
	for name, config := range map[string]RateLimiterConfig{
		"DefaultAPIRateLimit":    DefaultAPIRateLimit,
		"AuthRateLimit":          AuthRateLimit,
		"SignupRateLimit":        SignupRateLimit,
		"PasswordResetRateLimit": PasswordResetRateLimit,
		"TokenEndpointRateLimit": TokenEndpointRateLimit,
		"OtlpRateLimit":          OtlpRateLimit,
	} {
		if config.PolicyID == "" {
			t.Errorf("%s.PolicyID must be set", name)
		}
	}
}

// 低于 1 req/s 的配额按秒截断会变成 0，客户端读到的就是「不允许任何请求」。
func TestRLDimension_SubSecondRatesKeepAPositiveQuota(t *testing.T) {
	for name, config := range map[string]RateLimiterConfig{
		"AuthRateLimit":          AuthRateLimit,
		"SignupRateLimit":        SignupRateLimit,
		"PasswordResetRateLimit": PasswordResetRateLimit,
		"TokenEndpointRateLimit": TokenEndpointRateLimit,
	} {
		dim := rlDimension(config)
		if dim.Quota < 1 {
			t.Errorf("%s → Quota %d, must be at least 1 per reported window", name, dim.Quota)
		}
	}
}

func TestRateLimit_DeniedWritesUnifiedErrorBody(t *testing.T) {
	// Explicitly detach the global Redis client so this test exercises the
	// in-process limiter regardless of what earlier tests injected.
	SetRateLimitRedis(nil)
	t.Cleanup(func() { SetRateLimitRedis(nil) })

	config := RateLimiterConfig{Rate: 10.0 / 60.0, Burst: 1, PolicyID: "e1b2-ip"}
	handler := RateLimit(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	if first := serveWithRequestID(handler, "req-e1b2-429-ok"); first.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200 (body=%s)", first.Code, first.Body.String())
	}
	denied := serveWithRequestID(handler, "req-e1b2-429")
	assertUnifiedErrorBody(t, denied, http.StatusTooManyRequests, "too_many_requests", "rate limit exceeded", "req-e1b2-429")

	body := denied.Body.String()
	for _, internal := range []string{"redis", "miniredis", "policy", "bucket"} {
		if contains(body, internal) {
			t.Errorf("429 body leaked limiter internals (%q): %s", internal, body)
		}
	}
}

func TestRateLimit_EmitsIETFHeaders(t *testing.T) {
	config := RateLimiterConfig{Rate: 10.0 / 60.0, Burst: 5, PolicyID: "auth-ip"}
	handler := RateLimit(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.RemoteAddr = "10.0.0.1:1234"
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, rec.Code)
		}
		if rec.Header().Get(ratelimit.HeaderPolicy) != `10;w=60;burst=5;comment="auth-ip"` {
			t.Errorf("request %d: RateLimit-Policy = %q", i, rec.Header().Get(ratelimit.HeaderPolicy))
		}
		if rec.Header().Get(ratelimit.HeaderLimit) == "" || rec.Header().Get(ratelimit.HeaderReset) == "" {
			t.Errorf("request %d: incomplete rate limit headers", i)
		}
	}

	// 第 6 次超出 burst：必须是 429（限流语义）而不是 503，并给出整数秒重试指引。
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "10.0.0.1:1234"
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 when the bucket is empty, got %d", rec.Code)
	}
	if rec.Header().Get(ratelimit.HeaderRemaining) != "0" {
		t.Errorf("RateLimit-Remaining = %q, want 0", rec.Header().Get(ratelimit.HeaderRemaining))
	}
	if _, err := strconv.Atoi(rec.Header().Get(ratelimit.HeaderRetryAfter)); err != nil {
		t.Errorf("Retry-After must be integer delta-seconds, got %q", rec.Header().Get(ratelimit.HeaderRetryAfter))
	}
}

// WithKeyFunc 让端点按业务主体（如已登录用户）计数，而不是永远按 IP：
// 同一主体换 IP 共享额度，不同主体（含匿名 IP）互不影响。
func TestRateLimit_WithKeyFuncSeparatesSubjects(t *testing.T) {
	SetRateLimitRedis(nil)
	t.Cleanup(func() { SetRateLimitRedis(nil) })

	config := RateLimiterConfig{Rate: 10.0 / 60.0, Burst: 2, PolicyID: "subject-test"}
	keyFunc := func(r *http.Request) string { return r.Header.Get("X-Test-Subject") }
	handler := RateLimit(config, WithKeyFunc(keyFunc))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	serve := func(subject, remoteAddr string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.RemoteAddr = remoteAddr
		req.Header.Set("X-Test-Subject", subject)
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	for i := 0; i < 2; i++ {
		if code := serve("user:a", "10.0.0.1:1234"); code != http.StatusOK {
			t.Fatalf("request %d for subject user:a = %d, want 200", i+1, code)
		}
	}
	if code := serve("user:a", "10.0.0.2:1234"); code != http.StatusTooManyRequests {
		t.Errorf("same subject from another IP = %d, want 429", code)
	}
	if code := serve("user:b", "10.0.0.1:1234"); code != http.StatusOK {
		t.Errorf("different subject on the same IP = %d, want 200", code)
	}
}

// resetRateLimitRedisDown 清除装配级降级标记（仅测试用）。
func resetRateLimitRedisDown() {
	globalRateLimitRedisMu.Lock()
	defer globalRateLimitRedisMu.Unlock()
	globalRateLimitRedisDown = false
}

// 装配级保守收敛（ADR-019）：配置了 Redis 但启动不可达时，进程内兜底阈值减半，
// 客户端可见的 Policy 配额同步减半（不是只改内部桶而对外谎报全额）。
func TestRateLimit_RedisUnavailableTightensInProcessFallback(t *testing.T) {
	SetRateLimitRedis(nil)
	SetRateLimitRedisUnavailable()
	t.Cleanup(func() {
		SetRateLimitRedis(nil)
		resetRateLimitRedisDown()
	})

	cfg := RateLimiterConfig{Rate: 10.0 / 60.0, Burst: 4, PolicyID: "fallback-test"}
	handler := RateLimit(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	serve := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.RemoteAddr = "10.9.9.9:1234"
		handler.ServeHTTP(rec, req)
		return rec
	}

	for i := 0; i < 2; i++ {
		if rec := serve(); rec.Code != http.StatusOK {
			t.Fatalf("request %d = %d, want 200 (halved burst 4→2)", i+1, rec.Code)
		}
	}
	rec := serve()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("request 3 = %d, want 429 (halved burst exhausted)", rec.Code)
	}
	if got := rec.Header().Get(ratelimit.HeaderPolicy); got != `5;w=60;burst=2;comment="fallback-test"` {
		t.Errorf("RateLimit-Policy = %q, want halved quota 5;w=60;burst=2", got)
	}
}

// Burst 减半不得截断为 0（「不允许任何请求」）：最小保留 1。
func TestRateLimit_RedisUnavailableKeepsMinimumBurstOne(t *testing.T) {
	SetRateLimitRedis(nil)
	SetRateLimitRedisUnavailable()
	t.Cleanup(func() {
		SetRateLimitRedis(nil)
		resetRateLimitRedisDown()
	})

	cfg := RateLimiterConfig{Rate: 3.0 / 60.0, Burst: 1, PolicyID: "fallback-min"}
	handler := RateLimit(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	serve := func() int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.RemoteAddr = "10.9.9.10:1234"
		handler.ServeHTTP(rec, req)
		return rec.Code
	}
	if code := serve(); code != http.StatusOK {
		t.Fatalf("first request = %d, want 200 (burst clamped to 1, not 0)", code)
	}
	if code := serve(); code != http.StatusTooManyRequests {
		t.Errorf("second request = %d, want 429", code)
	}
}
