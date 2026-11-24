// wire_cache.go — Session Cache (ADR-010) + Rate Limit Redis (ADR-019) 初始化。
// 从 runtime.go 的 NewRuntime 中抽离，保持 bootstrap 包级 private function。
package bootstrap

import (
	"context"
	"io"
	"log/slog"
	"time"

	mwplatform "github.com/guliangiam/guliangiam/internal/platform/middleware"
	"github.com/redis/go-redis/v9"

	"github.com/guliangiam/guliangiam/internal/infra/cache"
	postgresuow "github.com/guliangiam/guliangiam/internal/infra/postgres/uow"
	"github.com/guliangiam/guliangiam/internal/platform/config"
)

// wireCacheDeps 初始化 session cache（ADR-010）+ 分布式限流 Redis（ADR-019）。
// 两个都 fail-open：Redis 不可用时 session cache 退化内存、限流退化进程内令牌桶。
// 返回 (sessionCache, sessionCacheCloser) — 后者仅 Redis 实现有 Close，内存实现为 nil。
func wireCacheDeps() (cache.SessionCache, io.Closer) {
	// ── Session Cache wiring (ADR-010) ──────────────────────────────────────
	// Resolve cache driver via SESSION_CACHE_DRIVER env (memory|redis, default memory).
	// cache.FactoryWithConfig is fail-open — if SESSION_CACHE_DRIVER=redis but Redis is
	// unreachable, it logs + falls back to memory instead of panicking.
	//
	// Observability hooks (Redis driver only):
	//   - CircuitOpen / CircuitClose fire on circuit state transitions (Warn/Info).
	//   - GetHit/GetMiss are intentionally silent — hot path, would spam logs.
	//   - GetError/SetError/DeleteError fire on actual failures (Warn).
	// Keep hooks lightweight; they run synchronously in the caller goroutine.
	sessionCache, cacheDriver, err := cache.FactoryWithConfig(cache.FactoryConfig{
		Hooks: cache.RedisHooks{
			CircuitOpen: func(failures int64, lastErr error) {
				slog.Warn("session cache circuit opened — Redis temporarily unavailable, all ops failing open to DB",
					"failures", failures, "last_error", lastErr)
			},
			CircuitClose: func() {
				slog.Info("session cache circuit closed — Redis recovered")
			},
			GetError: func(_ context.Context, id string, err error) {
				slog.Warn("session cache GET error", "session_id", id, "err", err)
			},
			SetError: func(_ context.Context, id string, err error) {
				slog.Warn("session cache SET error", "session_id", id, "err", err)
			},
			DeleteError: func(_ context.Context, id string, err error) {
				slog.Warn("session cache DEL error", "session_id", id, "err", err)
			},
		},
	})
	var sessionCacheCloser io.Closer
	if c, ok := sessionCache.(io.Closer); ok {
		sessionCacheCloser = c // RedisSessionCache implements Close
	}
	if err != nil {
		// Factory already fell back to memory; log the reason for operators.
		slog.Warn("session cache driver fallback",
			"requested", config.SessionCacheDriver(),
			"active", cacheDriver,
			"reason", err.Error())
	} else {
		slog.Info("session cache initialized", "driver", cacheDriver)
	}
	postgresuow.SetSessionCache(sessionCache)

	// ── Rate Limit Redis wiring (ADR-019) ────────────────────────────────────
	// 分布式限流：所有 RateLimit(config) 调用（10+ 路由点）自动走 Redis 后端。
	// RATE_LIMIT_REDIS_ADDR 未设置 → 进程内令牌桶（单实例假设，WARN 显式宣告；
	// 多实例部署必须配置，否则总放行量 = 单实例阈值 × N）。
	// 限流 Redis 与 session cache 独立初始化——用 DB 索引 1（cache 默认用 0）避免 key 冲突。
	// 启动 ping 失败 → 进程内兜底阈值收紧 50%（装配级保守收敛，宁紧勿松），不 panic；
	// 请求级 Redis 故障仍由 redisLimiter.Try Fail-Close（429）。
	if rlAddr := config.RateLimitRedisAddr(); rlAddr != "" {
		rlClient := redis.NewClient(&redis.Options{
			Addr:         rlAddr,
			Password:     config.RateLimitRedisPassword(),
			DB:           config.RateLimitRedisDB(),
			DialTimeout:  2 * time.Second,
			ReadTimeout:  2 * time.Second,
			WriteTimeout: 2 * time.Second,
		})
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := rlClient.Ping(ctx).Err(); err != nil {
			cancel()
			_ = rlClient.Close()
			mwplatform.SetRateLimitRedisUnavailable()
			slog.Warn("rate limit Redis ping failed; falling back to in-process at 50% quota",
				"reason", err.Error())
		} else {
			cancel()
			mwplatform.SetRateLimitRedis(rlClient)
			slog.Info("rate limit Redis initialized", "addr", rlAddr)
		}
	} else {
		slog.Warn("rate limit Redis not configured; in-process buckets assume a single instance",
			"hint", "multi-instance deployments must set RATE_LIMIT_REDIS_ADDR")
	}

	return sessionCache, sessionCacheCloser
}
