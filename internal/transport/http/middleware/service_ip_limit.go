package middleware

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/ratelimit"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/guliangiam/guliangiam/internal/transport/http/util"
)

// ServiceIPLimitConfig defines per-endpoint rate limiting by client IP.
type ServiceIPLimitConfig struct {
	// Rate is the number of tokens added per second per IP.
	Rate float64
	// Burst is the maximum burst size per IP.
	Burst int
	// TTL is how long an idle IP entry stays in the map before eviction.
	TTL time.Duration
}

// DefaultServiceIPLimitConfigs provides sensible defaults for sensitive endpoints.
var DefaultServiceIPLimitConfigs = map[string]ServiceIPLimitConfig{
	"/api/v1/messages/send-email": {Rate: 1.0 / 3600, Burst: 1, TTL: 24 * time.Hour},   // 1 email per IP per hour
	"/api/v1/messages/send-sms":   {Rate: 1.0 / 3600, Burst: 1, TTL: 24 * time.Hour},   // 1 SMS per IP per hour
	pathAuthLogin:                 {Rate: 10.0 / 60, Burst: 10, TTL: 15 * time.Minute}, // 10 login attempts per IP per minute
	pathAuthSignup:                {Rate: 5.0 / 60, Burst: 5, TTL: 15 * time.Minute},   // 5 signup attempts per IP per minute
	"/api/v1/verifications":       {Rate: 3.0 / 60, Burst: 3, TTL: 10 * time.Minute},   // 3 verification requests per IP per minute
}

// ipLimiter holds a rate.Limiter and the last access time for an (IP, path) pair.
// ttl is the owning endpoint's idle-eviction TTL (BE-E-02): entries must be
// evicted per their own config, not a global maximum.
type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
	ttl      time.Duration
}

// ServiceIPLimiter provides per-IP rate limiting for specific endpoints.
type ServiceIPLimiter struct {
	mu              sync.Mutex
	limiters        map[string]*ipLimiter
	configs         map[string]ServiceIPLimitConfig
	cleanupInterval time.Duration
	maxTTL          time.Duration
	clock           func() time.Time
	stopCh          chan struct{}
}

// computeMaxTTL returns the longest TTL across all configs, defaulting to 1 hour if empty.
func computeMaxTTL(configs map[string]ServiceIPLimitConfig) time.Duration {
	maxTTL := time.Hour
	for _, cfg := range configs {
		if cfg.TTL > maxTTL {
			maxTTL = cfg.TTL
		}
	}
	return maxTTL
}

// NewServiceIPLimiter creates a new per-IP rate limiter with the given configurations.
// If cleanupInterval is zero, it defaults to 5 minutes.
func NewServiceIPLimiter(configs map[string]ServiceIPLimitConfig, cleanupInterval ...time.Duration) *ServiceIPLimiter {
	ci := 5 * time.Minute
	if len(cleanupInterval) > 0 && cleanupInterval[0] > 0 {
		ci = cleanupInterval[0]
	}
	l := &ServiceIPLimiter{
		limiters:        make(map[string]*ipLimiter),
		configs:         configs,
		cleanupInterval: ci,
		maxTTL:          computeMaxTTL(configs),
		clock:           time.Now,
		stopCh:          make(chan struct{}),
	}
	recover.SafeGo("service-ip-limiter-cleanup", l.cleanup)
	return l
}

// quotaPerMinute 把「每秒补充速率」折算为每分钟配额，供 RateLimit-Limit 使用。
// 该端点的配置以 req/s 表达（见 DefaultServiceIPLimitConfigs 的注释），
// 客户端看到的额度单位必须是分钟，否则 1 小时的邮件限额会被报成 "0"。
func quotaPerMinute(cfg ServiceIPLimitConfig) int64 {
	quota := int64(cfg.Rate * 60)
	if quota < 1 {
		quota = 1
	}
	return quota
}

// Middleware returns an HTTP middleware that applies per-IP rate limiting
// to the request based on the endpoint path.
func (l *ServiceIPLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg, ok := l.configs[r.URL.Path]
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		ip := util.ClientIP(r)
		limiter := l.getLimiter(ip, r.URL.Path, cfg)
		allowed := limiter.Allow()
		// 维度标识包含路径：同一 IP 可能同时命中多个端点族配额，客户端与网关
		// 必须能区分是哪一维拒绝的（limit-registry §2 维度纪律）。
		dim := ratelimit.Dimension{
			ID:     "service-ip:" + r.URL.Path,
			Quota:  quotaPerMinute(cfg),
			Burst:  int64(cfg.Burst),
			Window: time.Minute,
		}
		snapshot := ratelimit.Consume(dim, limiter.Tokens(), cfg.Rate)
		ratelimit.Write(w.Header(), snapshot)

		if !allowed {
			ratelimit.WriteRetryAfter(w.Header(), snapshot.Reset)
			route.WriteError(w, r, apperrors.NewTooManyRequests("too many requests from this IP"))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getLimiter retrieves or creates a rate limiter for the given (IP, path) pair.
// BE-E-02: the key includes the endpoint path — an IP-only key let the first
// endpoint's bucket (e.g. login burst=10) serve every later endpoint (e.g.
// send-email 1/hour), and a spent email bucket locked the same NAT IP out of
// login. Each path class keeps its own tokens and TTL.
func (l *ServiceIPLimiter) getLimiter(ip, path string, cfg ServiceIPLimitConfig) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	key := ip + "|" + path
	if entry, ok := l.limiters[key]; ok {
		entry.lastSeen = l.clock()
		return entry.limiter
	}

	limiter := rate.NewLimiter(rate.Limit(cfg.Rate), cfg.Burst)
	l.limiters[key] = &ipLimiter{limiter: limiter, lastSeen: l.clock(), ttl: cfg.TTL}
	return limiter
}

// cleanup removes stale IP entries that haven't been seen for their TTL.
func (l *ServiceIPLimiter) cleanup() {
	ticker := time.NewTicker(l.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.mu.Lock()
			now := l.clock()
			for key, entry := range l.limiters {
				ttl := entry.ttl
				if ttl <= 0 {
					ttl = l.maxTTL
				}
				if now.Sub(entry.lastSeen) > ttl {
					delete(l.limiters, key)
				}
			}
			l.mu.Unlock()
		case <-l.stopCh:
			return
		}
	}
}

// Stop terminates the background cleanup goroutine.
func (l *ServiceIPLimiter) Stop() {
	close(l.stopCh)
}

// ServiceIPLimitEnvConfig holds the rate limit configs and cleanup interval loaded from env.
type ServiceIPLimitEnvConfig struct {
	Configs         map[string]ServiceIPLimitConfig
	CleanupInterval time.Duration
}

// LoadConfigFromEnv builds per-endpoint rate limit configurations from environment variables.
// Supported environment variables:
//   - RATE_LIMIT_PER_IP_REQUESTS: max requests per window (default: 10)
//   - RATE_LIMIT_PER_IP_WINDOW: window duration in seconds (default: 60)
//   - RATE_LIMIT_PER_IP_CLEANUP_INTERVAL: cleanup interval in seconds (default: 300)
func LoadConfigFromEnv() ServiceIPLimitEnvConfig {
	requests := config.RateLimitPerIPRequests()
	window := config.RateLimitPerIPWindowSeconds()
	cleanupSeconds := config.RateLimitPerIPCleanupIntervalSeconds()

	if requests <= 0 || window <= 0 {
		slog.Warn("invalid rate limit config, using defaults",
			"requests", requests, "window", window)
		requests = 10
		window = 60
	}
	if cleanupSeconds <= 0 {
		slog.Warn("invalid cleanup interval, using default",
			"cleanup_interval", cleanupSeconds)
		cleanupSeconds = 300
	}

	windowDuration := time.Duration(window) * time.Second
	ratePerSec := float64(requests) / float64(window)

	return ServiceIPLimitEnvConfig{
		CleanupInterval: time.Duration(cleanupSeconds) * time.Second,
		Configs: map[string]ServiceIPLimitConfig{
			pathAuthLogin:                 {Rate: ratePerSec, Burst: requests, TTL: windowDuration},
			pathAuthSignup:                {Rate: ratePerSec * 0.5, Burst: max(1, requests/2), TTL: windowDuration},
			"/api/v1/messages/send-email": {Rate: 1.0 / 3600, Burst: 1, TTL: 24 * time.Hour},
			"/api/v1/messages/send-sms":   {Rate: 1.0 / 3600, Burst: 1, TTL: 24 * time.Hour},
			"/api/v1/verifications":       {Rate: ratePerSec * 0.3, Burst: max(1, requests/3), TTL: windowDuration},
		},
	}
}
