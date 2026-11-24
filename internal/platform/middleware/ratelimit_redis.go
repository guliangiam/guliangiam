package middleware

import (
	"context"
	"fmt"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

// Lua 原子令牌桶脚本 —— 限流核心。
//
// Redis 令牌桶在 Go 端等价于 golang.org/x/time/rate，但状态存在 Redis 里，
// 多实例共享同一个桶。脚本原子完成「计算新 tokens → 扣 1 个 → 写回 → 续 TTL」，
// 消除多命令竞态（两个请求同时读到满桶、同时扣减 → 实际放行 2 个）。
//
// KEYS:
//
//	KEYS[1] = 限流键，格式 "rate:{policyID}:{subject}"（subject 通常是 IP）
//
// ARGV:
//
//	ARGV[1] = rate (float, tokens per second)
//	ARGV[2] = burst (int, max tokens)
//	ARGV[3] = now_seconds (float, Unix time.Second())
//	ARGV[4] = ttl (int, Redis key 过期时间秒 —— 比配置 MaxAge 略大一点，
//	          因为进程内也做清理；Redis 侧只需要防止死键泄漏)
//
// Returns (multi bulk): [allowed, remaining, reset_in_sec]
//
//	allowed    = 1 允许 / 0 拒绝
//	remaining  = 扣减后剩余 token 数（整数）
//	reset_in   = 桶回满需要的秒数（整数，最小 1，给 Retry-After 用）
const rateLimitLua = `
local key    = KEYS[1]
local rate   = tonumber(ARGV[1])
local burst  = tonumber(ARGV[2])
local now    = tonumber(ARGV[3])
local ttl    = tonumber(ARGV[4])

local tokens    = tonumber(redis.call("HGET", key, "tokens")     or burst)
local last_time = tonumber(redis.call("HGET", key, "last_time")  or now)

-- 按时间差回填 token，上限 burst
local elapsed = now - last_time
if elapsed > 0 then
    tokens = math.min(burst, tokens + elapsed * rate)
end

-- 尝试扣 1
local allowed = 0
if tokens >= 1 then
    tokens = tokens - 1
    allowed = 1
end

-- 写回状态 + 续 TTL（每次请求都续，避免活跃键被扫到过期）
redis.call("HSET", key, "tokens", tokens, "last_time", now)
redis.call("EXPIRE", key, ttl)

local reset_in = math.ceil((burst - tokens) / rate)
if reset_in < 1 then reset_in = 1 end

return {allowed, math.floor(tokens), reset_in}
`

// redisLimiter 是 Redis + Lua 实现的分布式令牌桶。
//
// 结构说明：
//   - client *redis.Client —— 可以是真实 Redis 也可以是 miniredis（测试）
//   - script *redis.Script —— 预加载 Lua 脚本；Eval 时如果 Redis 有 script
//     cache 就走 EVALSHA（快一倍），cache miss 自动 fallback EVAL
//   - config —— Rate + Burst 来自 RateLimiterConfig；TTL 用 MaxAge
//
// Fail-Close 姿态（ADR-019 § 理由与后果）：
//
//	Redis 不可用时 Try() 返回 (false, 0, 60) —— 等同"桶空了，拒绝 + 60s 后再试"。
//	这比"放行所有"安全：认证防爆破是红线，Redis 挂了 = 宁紧勿松。
//	调用方可以在 WARN 级日志里记录这个 fallback，便于运维发现 Redis 问题。
type redisLimiter struct {
	client *redis.Client
	script *redis.Script

	rate  rate.Limit
	burst int
	ttl   time.Duration
}

// newRedisLimiter 构造一个 Redis 令牌桶。client 必须非 nil。
func newRedisLimiter(client *redis.Client, config RateLimiterConfig) *redisLimiter {
	return &redisLimiter{
		client: client,
		script: redis.NewScript(rateLimitLua),
		rate:   config.Rate,
		burst:  config.Burst,
		ttl:    config.MaxAge + 30*time.Second, // 比 MaxAge 多 30s 冗余
	}
}

// Try 尝试从 subject 对应的桶里扣一个令牌。
//
// Returns:
//
//	allowed  —— true 放行 / false 拒绝
//	remaining —— 扣减后剩余 token（float，给 IETF RateLimit-Remaining 头用）
//	resetIn   —— 桶回满秒数（给 Retry-After 和 RateLimit-Reset 用）
//	err       —— Redis 网络错误；调用方应该 log + 决定是否 fail-close
func (r *redisLimiter) Try(ctx context.Context, policyID, subject string) (allowed bool, remaining float64, resetIn int, err error) {
	key := fmt.Sprintf("rate:%s:%s", policyID, subject)
	now := float64(time.Now().Unix())
	ttlSec := int(r.ttl.Seconds())
	if ttlSec < 1 {
		ttlSec = 60
	}

	res, err := r.script.Run(ctx, r.client,
		[]string{key},
		float64(r.rate),
		r.burst,
		now,
		ttlSec,
	).Result()

	if err != nil {
		// Redis 故障：Fail-Close（ADR-019）
		return false, 0, 60, err
	}

	// res 是 []interface{} 长度 3：[int64 allowed, int64 remaining, int64 reset_in]
	vals, ok := res.([]interface{})
	if !ok || len(vals) != 3 {
		return false, 0, 60, apperrors.NewInternalf("rate limiter script returned unexpected type %T", res)
	}

	allowed = vals[0].(int64) == 1
	remaining = float64(vals[1].(int64))
	resetIn = int(vals[2].(int64))
	return allowed, remaining, resetIn, nil
}
