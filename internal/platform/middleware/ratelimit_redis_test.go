package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/guliangiam/guliangiam/internal/platform/ratelimit"
)

// newMiniredisClient 给一个 miniredis server 创建 go-redis client。
func newMiniredisClient(t *testing.T, s *miniredis.Miniredis) *redis.Client {
	t.Helper()
	return redis.NewClient(&redis.Options{Addr: s.Addr()})
}

// TestRateLimit_Redis_ExhaustBurst 用真实 Redis（miniredis）验证令牌桶语义：
// burst=3 的桶 → 前 3 个 OK → 第 4 个拒绝。与进程内基准语义一致。
func TestRateLimit_Redis_ExhaustBurst(t *testing.T) {
	mr := miniredis.RunT(t)
	client := newMiniredisClient(t, mr)

	config := RateLimiterConfig{Rate: 10, Burst: 3, PolicyID: "t-ip", MaxAge: time.Minute}
	handler := RateLimit(config, WithRedis(client))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.RemoteAddr = "10.0.0.1:1234"
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, rec.Code)
		}
	}

	// 第 4 次：必须 429
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "10.0.0.1:1234"
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("4th request: expected 429, got %d", rec.Code)
	}
	if rec.Header().Get(ratelimit.HeaderRemaining) != "0" {
		t.Errorf("RateLimit-Remaining = %q, want 0", rec.Header().Get(ratelimit.HeaderRemaining))
	}
}

// TestRateLimit_Redis_DeniedWritesUnifiedErrorBody verifies the distributed
// path emits the same canonical 429 envelope as the in-process path.
func TestRateLimit_Redis_DeniedWritesUnifiedErrorBody(t *testing.T) {
	mr := miniredis.RunT(t)
	client := newMiniredisClient(t, mr)

	config := RateLimiterConfig{Rate: 10.0 / 60.0, Burst: 1, PolicyID: "e1b2-redis-ip", MaxAge: time.Minute}
	handler := RateLimit(config, WithRedis(client))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	if first := serveWithRequestID(handler, "req-e1b2-redis-ok"); first.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200 (body=%s)", first.Code, first.Body.String())
	}
	denied := serveWithRequestID(handler, "req-e1b2-redis-429")
	assertUnifiedErrorBody(t, denied, http.StatusTooManyRequests, "too_many_requests", "rate limit exceeded", "req-e1b2-redis-429")
}

// TestRateLimit_Redis_MultiInstanceConsistency 是 ADR-019 的核心验证：
// 两个进程各发一半请求 → 总量不超过单实例阈值（burst + rate*window）。
// 进程内实现会让"阈值 × N"绕过去；Redis 实现必须精确收敛到单阈值。
func TestRateLimit_Redis_MultiInstanceConsistency(t *testing.T) {
	mr := miniredis.RunT(t)

	// 两个独立 client（模拟两个进程）
	clientA := newMiniredisClient(t, mr)
	clientB := newMiniredisClient(t, mr)

	// burst=5, rate=10/s —— 阈值 = 5 + 10*0.5 ≈ 10（短窗口内主要靠 burst）
	config := RateLimiterConfig{Rate: 10, Burst: 5, PolicyID: "multi-ip", MaxAge: time.Minute}
	handlerA := RateLimit(config, WithRedis(clientA))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handlerB := RateLimit(config, WithRedis(clientB))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 同时发 20 个请求（模拟两个进程各发 10 个），全部来自同一个 IP
	var wg sync.WaitGroup
	ok := make(chan int, 20) // 统计 200 的数量
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.RemoteAddr = "10.0.0.99:1234"
			if idx%2 == 0 {
				handlerA.ServeHTTP(rec, req)
			} else {
				handlerB.ServeHTTP(rec, req)
			}
			if rec.Code == http.StatusOK {
				ok <- 1
			}
		}(i)
	}
	wg.Wait()
	close(ok)

	var okCount int
	for range ok {
		okCount++
	}

	// 阈值就是 burst=5（短窗口内 refill 来不及）
	if okCount > 5 {
		t.Fatalf("multi-instance bypass! got %d OK, want <= 5 (burst=5)", okCount)
	}
	t.Logf("OK=%d (burst=5), all other requests got 429 — multi-instance consistent ✓", okCount)
}

// TestRateLimit_Redis_FailClose 验证 Redis 不可用时的姿态：
// 必须"桶空拒绝"而不是"放行所有"。
// ADR-019 § 理由与后果：Fail-Close，宁紧勿松。
func TestRateLimit_Redis_FailClose(t *testing.T) {
	mr := miniredis.RunT(t)
	client := newMiniredisClient(t, mr)

	config := RateLimiterConfig{Rate: 1000, Burst: 1000, PolicyID: "fc-ip", MaxAge: time.Minute}
	handler := RateLimit(config, WithRedis(client))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 1. 正常情况下，高 burst 的桶不会拒绝
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "10.0.0.1:1234"
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("pre-shutdown: expected 200, got %d", rec.Code)
	}

	// 2. 关掉 Redis → Fail-Close
	mr.Close()
	time.Sleep(50 * time.Millisecond)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "10.0.0.1:1234"
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("Redis down: expected 429 (Fail-Close), got %d", rec.Code)
	}
}

// TestRateLimit_Redis_TTL 验证限流键带过期，防止 Redis 死键泄漏。
func TestRateLimit_Redis_TTL(t *testing.T) {
	mr := miniredis.RunT(t)
	client := newMiniredisClient(t, mr)

	config := RateLimiterConfig{Rate: 10, Burst: 5, PolicyID: "ttl-ip", MaxAge: 30 * time.Second}
	handler := RateLimit(config, WithRedis(client))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "10.0.0.1:1234"
	handler.ServeHTTP(rec, req)

	// 直接用 Redis 查 key 的 TTL
	ctx := context.Background()
	ttl, err := client.TTL(ctx, "rate:ttl-ip:10.0.0.1").Result()
	if err != nil {
		t.Fatalf("TTL query failed: %v", err)
	}
	if ttl <= 0 || ttl > 2*time.Minute {
		t.Fatalf("TTL = %v, want positive and reasonable (<2m)", ttl)
	}
	t.Logf("Redis key TTL = %v ✓", ttl)
}

// TestRateLimit_Redis_IETFHeaders 验证 Redis 后端同样输出 IETF 头。
func TestRateLimit_Redis_IETFHeaders(t *testing.T) {
	mr := miniredis.RunT(t)
	client := newMiniredisClient(t, mr)

	config := RateLimiterConfig{Rate: 10.0 / 60.0, Burst: 5, PolicyID: "hdr-ip"}
	handler := RateLimit(config, WithRedis(client))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "10.0.0.1:1234"
	handler.ServeHTTP(rec, req)

	// IETF RateLimit-Policy 必须带 PolicyID comment
	policy := rec.Header().Get(ratelimit.HeaderPolicy)
	if policy == "" || !contains(policy, "hdr-ip") {
		t.Errorf("RateLimit-Policy = %q, want comment=hdr-ip", policy)
	}
	if rec.Header().Get(ratelimit.HeaderLimit) == "" {
		t.Error("missing RateLimit-Limit")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || s != "" && indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// TestRateLimit_Redis_GlobalInjection 验证全局 SetRateLimitRedis 入口：
// 不传 WithRedis 的 RateLimit(config) 在全局 client 设置后自动走 Redis 后端。
// 这让 bootstrap/chi.go 等 10+ 处现有调用无需改动就能分布式限流。
func TestRateLimit_Redis_GlobalInjection(t *testing.T) {
	mr := miniredis.RunT(t)
	client := newMiniredisClient(t, mr)

	// 先清掉残留（如果有）
	SetRateLimitRedis(nil)
	t.Cleanup(func() { SetRateLimitRedis(nil) })

	// 1. 全局 nil —— 进程内实现语义正确（burst=3 → 前 3 OK → 第 4 个 429）
	//
	// Rate 取 0.001 令牌/秒：回填 1 个令牌需要 1000 秒，远长于本用例时长。用例只验证
	// burst 耗尽后拒绝，不验证回填速度；若用 1000/s，Redis Lua 侧的时间源是整秒
	// （ratelimit_redis.go 的 time.Now().Unix()），第 3、4 次请求跨秒时 elapsed=1
	// 会回填 1000 个令牌（被 burst 截断为 3）从而放行第 4 次；进程内 rate.Limiter
	// 同样每 1ms 回填 1 个令牌。两种精度窗口都会让断言假红。
	const noRefillDuringTest = 0.001 // tokens/second
	config := RateLimiterConfig{Rate: noRefillDuringTest, Burst: 3, PolicyID: "global-ip", MaxAge: time.Minute}
	handler := RateLimit(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.RemoteAddr = "10.0.0.1:1234"
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("pre-injection request %d: expected 200, got %d", i, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "10.0.0.1:1234"
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("pre-injection 4th: expected 429, got %d", rec.Code)
	}

	// 2. 注入全局 Redis —— 新 handler 应该走 Redis（旧 handler 不受影响，已 capture backend）
	SetRateLimitRedis(client)
	handler2 := RateLimit(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	// Redis key 是新的（不同 handler 但同 IP + 同 policyID）—— 但之前进程内不会污染 Redis
	// 重置测试 IP 避免残留
	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.RemoteAddr = "10.0.0.2:1234" // 新 IP，不受上一个 handler 影响
		handler2.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("global-injected request %d: expected 200, got %d", i, rec.Code)
		}
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "10.0.0.2:1234"
	handler2.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("global-injected 4th: expected 429 (Redis bucket), got %d", rec.Code)
	}
}

// 已注入可用 Redis client 时，装配级降级标记不得收紧阈值（Redis 路径全额）。
func TestRateLimit_RedisInjectionOverridesUnavailableFlag(t *testing.T) {
	mr := miniredis.RunT(t)
	client := newMiniredisClient(t, mr)
	SetRateLimitRedisUnavailable()
	t.Cleanup(resetRateLimitRedisDown)

	config := RateLimiterConfig{Rate: 10, Burst: 3, PolicyID: "t-override", MaxAge: time.Minute}
	handler := RateLimit(config, WithRedis(client))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.RemoteAddr = "10.0.0.7:1234"
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d = %d, want 200 (full burst 3, flag must not tighten Redis path)", i+1, rec.Code)
		}
	}
}
