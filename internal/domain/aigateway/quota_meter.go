package aigateway

import (
	"context"
	"time"
)

// QuotaMeter 存储按 API key / org 维度的 RPM / TPM / cost 计数器。
// Domain 端口 —— infra 层提供 Redis 实现（RedisQuotaMeter）。
//
// key 约定格式：
//
//	rpm:api_key:{keyID}:{minuteBucket}          ← 每分钟请求数
//	tpm:api_key:{keyID}:{minuteBucket}          ← 每分钟 token 数
//	cost:api_key:{keyID}:{YYYY-MM}              ← 月度成本（分）
//	{limitType}:{scope}:{scopeRef}:{window}     ← QuotaPolicy 维度
//
// Fail-Open：infra 实现应在 Redis 不可用时返回 (0, nil) 或退化为内存计数器，
// 调用方（GatewayService.enforceQuota）在 Meter == nil 时跳过所有配额检查。
type QuotaMeter interface {
	// Incr 将 key 对应的计数器增加 delta，返回递增后的值。
	// window 是 Redis key 的 TTL；若 key 首次创建则设为该值，
	// 已存在则刷新 TTL。Fail-Open：错误向上冒泡，调用方吞。
	Incr(ctx context.Context, key string, delta int64, window time.Duration) (int64, error)

	// Get 读取 key 当前值。不存在的 key 返回 0 和 nil error（不是 redis.Nil）。
	Get(ctx context.Context, key string) (int64, error)

	// TryIncr 原子地 check-and-decrement —— 单次 RTT。
	//
	// Lua 脚本等价：
	//   local cur = tonumber(redis.call("GET", KEYS[1]) or "0")
	//   if cur + ARGV[1] > ARGV[2] then return {0, cur} end
	//   redis.call("INCRBY", KEYS[1], ARGV[1])
	//   redis.call("EXPIRE", KEYS[1], ARGV[3])
	//   return {1, cur + ARGV[1]}
	//
	// 返回值：(allowed, count, err)
	//   allowed=true  → 预扣成功，count 是递增后的新值
	//   allowed=false → 已达上限，count 是当前值（未递增）
	//
	// 竞态语义：两个并发请求同时 cur 接近 limit 时，最多有一个 allowed=true
	// —— 真正达到 metering.md §配额与限流协调 要求的"原子扣减 + 单次往返"。
	//
	// 适用场景：enforceQuota 前置预扣（RPM/TPM/QuotaPolicy 维度）。
	// 不适用于 post-call incrementCounters（那里用 Incr 就够）。
	TryIncr(ctx context.Context, key string, delta int64, limit int64, window time.Duration) (allowed bool, count int64, err error)
}
