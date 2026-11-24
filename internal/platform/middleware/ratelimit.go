package middleware

import (
	"log/slog"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"

	"github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	httperr "github.com/guliangiam/guliangiam/internal/platform/http"
	"github.com/guliangiam/guliangiam/internal/platform/ratelimit"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
)

// RateLimiterConfig configures rate limiting parameters.
type RateLimiterConfig struct {
	// Rate is the number of requests allowed per second.
	Rate rate.Limit
	// Burst is the maximum number of requests that can be made at once.
	Burst int
	// CleanupInterval controls how often stale entries are removed.
	CleanupInterval time.Duration
	// MaxAge controls how long an entry is kept after its last activity.
	MaxAge time.Duration
	// PolicyID is the client-visible policy identifier written to
	// RateLimit-Policy (comment parameter). It must identify the dimension so a
	// client hitting several stacked limits can tell which one denied it.
	PolicyID string
}

// DefaultAPIRateLimit is the default rate limit for general API endpoints.
var DefaultAPIRateLimit = RateLimiterConfig{
	Rate:            100, // 100 requests per second
	Burst:           200,
	CleanupInterval: 5 * time.Minute,
	MaxAge:          10 * time.Minute,
	PolicyID:        "global-ip",
}

// AuthRateLimit is the rate limit for authentication endpoints.
var AuthRateLimit = RateLimiterConfig{
	Rate:            10.0 / 60.0, // ~10 requests per minute
	Burst:           5,
	CleanupInterval: 3 * time.Minute,
	MaxAge:          10 * time.Minute,
	PolicyID:        "auth-ip",
}

// SignupRateLimit is the rate limit for signup endpoints.
var SignupRateLimit = RateLimiterConfig{
	Rate:            5.0 / 60.0, // ~5 requests per minute
	Burst:           3,
	CleanupInterval: 3 * time.Minute,
	MaxAge:          10 * time.Minute,
	PolicyID:        "signup-ip",
}

// PasswordResetRateLimit is the rate limit for password reset endpoints.
var PasswordResetRateLimit = RateLimiterConfig{
	Rate:            3.0 / 60.0, // ~3 requests per minute
	Burst:           2,
	CleanupInterval: 3 * time.Minute,
	MaxAge:          10 * time.Minute,
	PolicyID:        "password-reset-ip",
}

// TokenEndpointRateLimit is the rate limit for token introspection/revocation endpoints.
var TokenEndpointRateLimit = RateLimiterConfig{
	Rate:            30.0 / 60.0, // ~30 requests per minute
	Burst:           10,
	CleanupInterval: 3 * time.Minute,
	MaxAge:          10 * time.Minute,
	PolicyID:        "token-endpoint-ip",
}

// OtlpRateLimit is the rate limit for OTLP ingestion endpoints (higher throughput).
var OtlpRateLimit = RateLimiterConfig{
	Rate:            200, // 200 requests per second
	Burst:           400,
	CleanupInterval: 5 * time.Minute,
	MaxAge:          10 * time.Minute,
	PolicyID:        "otlp-ip",
}

// ipRateLimiter tracks per-IP rate limiters.
type ipRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*ipLimiterEntry
	config   RateLimiterConfig
}

type ipLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// newIPRateLimiter creates a new per-IP rate limiter with the given config.
//
// 未设置清理周期/最大存活时间的配置按默认值补齐：构造出的配置一旦漏填，
// `time.NewTicker(0)` 会 panic，而 panic 发生在后台 goroutine 里——进程在启动
// 阶段直接崩溃，堆栈只指向 ticker，看不出是限流配置缺字段。
func newIPRateLimiter(cfg RateLimiterConfig) *ipRateLimiter {
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = 5 * time.Minute
	}
	if cfg.MaxAge <= 0 {
		cfg.MaxAge = 10 * time.Minute
	}
	rl := &ipRateLimiter{
		limiters: make(map[string]*ipLimiterEntry),
		config:   cfg,
	}
	recover.SafeGo("middleware-ratelimit-cleanup", rl.cleanup)
	return rl
}

// getLimiter returns the rate limiter for the given IP, creating one if needed.
func (rl *ipRateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if entry, exists := rl.limiters[ip]; exists {
		entry.lastSeen = time.Now()
		return entry.limiter
	}

	limiter := rate.NewLimiter(rl.config.Rate, rl.config.Burst)
	rl.limiters[ip] = &ipLimiterEntry{limiter: limiter, lastSeen: time.Now()}
	return limiter
}

// cleanup removes stale entries periodically.
func (rl *ipRateLimiter) cleanup() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		for ip, entry := range rl.limiters {
			if time.Since(entry.lastSeen) > rl.config.MaxAge {
				delete(rl.limiters, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// rlDimension 把配置投影为对客户端可见的限流维度。
//
// 窗口统一取 60 秒：配置以 req/s 表达，而多条配额低于 1 req/s（认证面 10/min、
// 重置密码 3/min）。若按 1 秒窗口折算，`int64(Rate)` 会把这些维度截断为 0，
// 客户端读到的配额就是「不允许任何请求」；换算到分钟再向上取整既保真又可读。
func rlDimension(cfg RateLimiterConfig) ratelimit.Dimension {
	quota := int64(math.Ceil(float64(cfg.Rate) * 60))
	if quota < 1 {
		quota = 1
	}
	return ratelimit.Dimension{
		ID:     cfg.PolicyID,
		Quota:  quota,
		Burst:  int64(cfg.Burst),
		Window: time.Minute,
	}
}

// RateLimitOption 配置 RateLimit middleware 的行为。
//
// WithRedis(*redis.Client) 启用 Redis 分布式令牌桶（ADR-019）。
// 不传或传 nil 则退回进程内令牌桶（golang.org/x/time/rate）。
type RateLimitOption func(*rateLimitOptions)

type rateLimitOptions struct {
	redisClient *redis.Client
	keyFunc     func(*http.Request) string
}

func WithRedis(client *redis.Client) RateLimitOption {
	return func(o *rateLimitOptions) {
		o.redisClient = client
	}
}

// WithKeyFunc 覆盖限流主体的取法。默认按可信客户端 IP 计数；需要「已登录按用户、
// 匿名按 IP」的端点（如 WebAuthn ceremony）注入自己的解析函数。
//
// 解析结果必须是服务端可验证的值（会话用户 ID、socket/可信代理出来的 IP），
// 不能直接用 X-Forwarded-For 这类裸请求头。返回空串时回落到默认 IP。
func WithKeyFunc(fn func(*http.Request) string) RateLimitOption {
	return func(o *rateLimitOptions) {
		o.keyFunc = fn
	}
}

// rateLimitSubject 解析本次请求的限流主体。默认与既有认证路由一致：使用
// ClientIP（仅当 socket peer 是可信代理时才采信转发头）。
func rateLimitSubject(r *http.Request, keyFunc func(*http.Request) string) string {
	if keyFunc != nil {
		if key := strings.TrimSpace(keyFunc(r)); key != "" {
			return key
		}
	}
	return httperr.ClientIP(r)
}

// globalRateLimitRedis 允许 bootstrap 在启动时统一注入 Redis client，
// 所有后续 RateLimit(config) 调用（10+ 个路由层点）自动走分布式桶，
// 不需要每个调用都传 WithRedis。nil 时退回到进程内实现。
var (
	globalRateLimitRedisMu   sync.RWMutex
	globalRateLimitRedis_    *redis.Client
	globalRateLimitRedisDown bool
)

// SetRateLimitRedis 在 bootstrap 启动时注册 Redis client。
// 只设置一次；后续 RateLimit 调用都会自动走 Redis 后端。
func SetRateLimitRedis(c *redis.Client) {
	globalRateLimitRedisMu.Lock()
	defer globalRateLimitRedisMu.Unlock()
	globalRateLimitRedis_ = c
}

// SetRateLimitRedisUnavailable 标记「分布式限流已配置但启动时不可达」。
// 此后未注入 Redis client 的 RateLimit 调用走进程内兜底，且阈值收紧为配置的
// 一半（ADR-019 装配级保守收敛：多实例总放行量上界 = 实例数 × 半额，宁紧勿松）。
// 未配置 Redis 的单实例部署不标记，保持全额。
func SetRateLimitRedisUnavailable() {
	globalRateLimitRedisMu.Lock()
	defer globalRateLimitRedisMu.Unlock()
	globalRateLimitRedisDown = true
}

func rateLimitRedisUnavailable() bool {
	globalRateLimitRedisMu.RLock()
	defer globalRateLimitRedisMu.RUnlock()
	return globalRateLimitRedisDown
}

func getRateLimitRedis() *redis.Client {
	globalRateLimitRedisMu.RLock()
	defer globalRateLimitRedisMu.RUnlock()
	return globalRateLimitRedis_
}

// conservativeFallbackConfig 把进程内兜底阈值收紧一半（Rate/Burst 各 50%，
// Burst 至少 1，避免截断为「不允许任何请求」）。
func conservativeFallbackConfig(cfg RateLimiterConfig) RateLimiterConfig {
	cfg.Rate /= 2
	cfg.Burst /= 2
	if cfg.Burst < 1 {
		cfg.Burst = 1
	}
	return cfg
}

// RateLimit 返回一个按 IP 限流的 middleware。
//
// 默认（无 WithRedis 选项）：进程内令牌桶 —— 单实例语义正确，多实例下等于"阈值 × N"。
// 传 WithRedis(client)：Redis + Lua 原子令牌桶 —— 多实例共享同一个桶，阈值语义精确。
//
// Redis 故障姿态：Fail-Close（ADR-019 § 理由与后果）—— Redis 连不上时等效"桶空，
// 本轮拒绝 + 60s 后重试"，而不是放行所有。认证防爆破是红线，宁紧勿松。
// 装配级回退（启动 ping 失败，SetRateLimitRedisUnavailable 标记）：进程内兜底
// 阈值收紧 50%；未配置 Redis 时保持全额（单实例假设由 bootstrap WARN 宣告）。
func RateLimit(cfg RateLimiterConfig, opts ...RateLimitOption) func(http.Handler) http.Handler {
	var ro rateLimitOptions
	for _, o := range opts {
		o(&ro)
	}

	// 优先级：显式 WithRedis(client) > 全局 SetRateLimitRedis 注入 > 进程内
	client := ro.redisClient
	if client == nil {
		client = getRateLimitRedis()
	}
	if client != nil {
		return newRedisRateLimitMiddleware(cfg, client, ro.keyFunc)
	}
	if rateLimitRedisUnavailable() {
		cfg = conservativeFallbackConfig(cfg)
	}
	return newIPRateLimitMiddleware(cfg, ro.keyFunc)
}

// newIPRateLimitMiddleware 是原有进程内实现（保持不变，独立测试绿色）。
func newIPRateLimitMiddleware(cfg RateLimiterConfig, keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	limiter := newIPRateLimiter(cfg)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subject := rateLimitSubject(r, keyFunc)

			l := limiter.getLimiter(subject)
			allowed := l.Allow()
			// 先扣减再取令牌数：这样 Remaining 与本次请求的真实结果一致。
			snapshot := ratelimit.Consume(rlDimension(cfg), l.Tokens(), float64(cfg.Rate))
			ratelimit.Write(w.Header(), snapshot)
			if !allowed {
				ratelimit.WriteRetryAfter(w.Header(), snapshot.Reset)
				httperr.WriteError(w, r, apperrors.NewTooManyRequests("rate limit exceeded"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// newRedisRateLimitMiddleware 用 Redis + Lua 实现分布式令牌桶。
// 失败语义见 redisLimiter.Try 的 Fail-Close 注释。
func newRedisRateLimitMiddleware(cfg RateLimiterConfig, client *redis.Client, keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	rl := newRedisLimiter(client, cfg)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subject := rateLimitSubject(r, keyFunc)

			allowed, remaining, resetIn, err := rl.Try(r.Context(), cfg.PolicyID, subject)
			if err != nil {
				// Fail-close（ADR-019）：Redis 故障时本轮拒绝；留 WARN 供运维发现。
				slog.Warn("rate limit redis backend error, failing closed",
					"policy_id", cfg.PolicyID, "subject", subject, "error", err)
			}

			// 无论 allowed / denied / err 都写 IETF 头（客户端能根据 Policy 维度自己判断）
			dim := rlDimension(cfg)
			snapshot := ratelimit.Consume(dim, remaining, float64(cfg.Rate))
			// Redis 侧 remaining 可能比进程内略不同步（Lua 原子里已扣减），Reset 用我们算的
			if resetIn > 0 {
				snapshot.Reset = time.Duration(resetIn) * time.Second
			}
			ratelimit.Write(w.Header(), snapshot)

			if !allowed {
				ratelimit.WriteRetryAfter(w.Header(), snapshot.Reset)
				httperr.WriteError(w, r, apperrors.NewTooManyRequests("rate limit exceeded"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// InitRateLimitsFromEnv overrides default rate limit configurations from
// environment variables. This allows operators to tune limits without
// recompiling the binary.
//
// Environment variables:
//   - RATE_LIMIT_GLOBAL_RATE:       requests/sec for default API (default 100)
//   - RATE_LIMIT_GLOBAL_BURST:      burst for default API (default 200)
//   - RATE_LIMIT_AUTH_RATE:         requests/min for auth endpoints (default 10)
//   - RATE_LIMIT_AUTH_BURST:        burst for auth endpoints (default 5)
//   - RATE_LIMIT_SIGNUP_RATE:       requests/min for signup endpoints (default 5)
//   - RATE_LIMIT_SIGNUP_BURST:      burst for signup endpoints (default 3)
//   - RATE_LIMIT_PASSWORD_RESET_RATE: requests/min for password reset (default 3)
//   - RATE_LIMIT_PASSWORD_RESET_BURST: burst for password reset (default 2)
//   - RATE_LIMIT_TOKEN_RATE:        requests/min for token endpoints (default 30)
//   - RATE_LIMIT_TOKEN_BURST:       burst for token endpoints (default 10)
//   - RATE_LIMIT_OTLP_RATE:         requests/sec for OTLP ingestion (default 200)
//   - RATE_LIMIT_OTLP_BURST:        burst for OTLP ingestion (default 400)
func InitRateLimitsFromEnv() {
	if rl, ok := config.RateLimitGlobal(); ok {
		if rl.Rate > 0 {
			DefaultAPIRateLimit.Rate = rate.Limit(rl.Rate)
		}
		if rl.Burst > 0 {
			DefaultAPIRateLimit.Burst = rl.Burst
		}
	}
	if rl, ok := config.RateLimitAuth(); ok {
		if rl.Rate > 0 {
			AuthRateLimit.Rate = rate.Limit(rl.Rate / 60.0)
		}
		if rl.Burst > 0 {
			AuthRateLimit.Burst = rl.Burst
		}
	}
	if rl, ok := config.RateLimitSignup(); ok {
		if rl.Rate > 0 {
			SignupRateLimit.Rate = rate.Limit(rl.Rate / 60.0)
		}
		if rl.Burst > 0 {
			SignupRateLimit.Burst = rl.Burst
		}
	}
	if rl, ok := config.RateLimitPasswordReset(); ok {
		if rl.Rate > 0 {
			PasswordResetRateLimit.Rate = rate.Limit(rl.Rate / 60.0)
		}
		if rl.Burst > 0 {
			PasswordResetRateLimit.Burst = rl.Burst
		}
	}
	if rl, ok := config.RateLimitToken(); ok {
		if rl.Rate > 0 {
			TokenEndpointRateLimit.Rate = rate.Limit(rl.Rate / 60.0)
		}
		if rl.Burst > 0 {
			TokenEndpointRateLimit.Burst = rl.Burst
		}
	}
	if rl, ok := config.RateLimitOTLP(); ok {
		if rl.Rate > 0 {
			OtlpRateLimit.Rate = rate.Limit(rl.Rate)
		}
		if rl.Burst > 0 {
			OtlpRateLimit.Burst = rl.Burst
		}
	}
}
