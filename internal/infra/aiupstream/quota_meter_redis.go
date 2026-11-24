// RedisQuotaMeter — AI Gateway 配额治理的 Redis 计数器实现（Round 9）。
//
// 实现 domain.QuotaMeter 端口接口：
//
//	Incr     用 INCRBY + EXPIRE pipeline 原子递增并刷新 TTL（post-call best-effort）
//	Get      用 GET 读取，key 不存在返回 0, nil（而非 redis.Nil）
//	TryIncr  Lua 原子 check-and-decrement（enforceQuota 预扣，单次 RTT）
//
// 与 rate limit Redis 共用 RATE_LIMIT_REDIS_ADDR / RATE_LIMIT_REDIS_DB=1
// （与 session cache DB=0 隔离，避免 key 冲突）。Fail-Open：任何 Redis
// 错误都向上冒泡，调用方 GatewayService.enforceQuota 在 Meter==nil 时
// 跳过全部配额——真正 fail-open 的位置在 app 层，infra 只负责返回错误。
package aiupstream

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// tryIncrScript 是原子 check-and-decrement Lua 脚本（KEYS[1] 个 key）：
//
//	ARGV[1] = delta
//	ARGV[2] = limit   (>=0)
//	ARGV[3] = window_seconds (float64 → 转 int64)
//
// 返回：{allowed_flag (1 or 0), count_after_or_count_before}
//
// 语义：
//
//	cur = tonumber(GET(key) or "0")
//	if cur + delta > limit then  —— 超限，不扣
//	  return {0, cur}
//	else                        —— 通过，扣并刷新 TTL
//	  INCRBY(key, delta)
//	  EXPIRE(key, window)
//	  return {1, cur + delta}
const tryIncrScript = `
local cur = tonumber(redis.call('GET', KEYS[1]) or '0')
local delta = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local ws = tonumber(ARGV[3])
if cur + delta > limit then
  return {0, cur}
end
redis.call('INCRBY', KEYS[1], delta)
redis.call('EXPIRE', KEYS[1], math.floor(ws))
return {1, cur + delta}
`

// RedisQuotaMeter 是 QuotaMeter 的 Redis 实现。零值不可用，必须用
// NewRedisQuotaMeter(client) 构造；若传入 nil client 所有方法都返回
// "client is nil" 错误——上游应该在 bootstrap 阶段判断 Redis 是否可用，
// 不可用时直接不传 Meter（fail-open）。
type RedisQuotaMeter struct {
	client *redis.Client
	// tryIncrCmd 是预加载的 Lua 脚本，避免每次 TryIncr 都传脚本文本。
	tryIncrCmd *redis.Script
}

// NewRedisQuotaMeter 构造一个 RedisQuotaMeter。client 必须非 nil。
func NewRedisQuotaMeter(client *redis.Client) *RedisQuotaMeter {
	return &RedisQuotaMeter{
		client:     client,
		tryIncrCmd: redis.NewScript(tryIncrScript),
	}
}

// Incr 原子递增计数器并刷新 TTL。
//
// 使用 TxPipeline 而不是 Lua —— 这里不需要强事务：我们接受"偶尔 TTL 没刷新"，
// 让 Redis 自然过期即可。Pipeline 减少 RTT 开销，同时两条命令几乎同时执行。
//
// 注意：INCRBY 在 key 不存在时自动创建并返回增量值（即 delta 本身），
// 紧接着 EXPIRE 设窗口 TTL。
func (m *RedisQuotaMeter) Incr(ctx context.Context, key string, delta int64, window time.Duration) (int64, error) {
	if m.client == nil {
		return 0, errors.New("RedisQuotaMeter: client is nil")
	}

	pipe := m.client.TxPipeline()
	incrCmd := pipe.IncrBy(ctx, key, delta)
	pipe.Expire(ctx, key, window)

	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incrCmd.Val(), nil
}

// Get 读取当前计数。key 不存在时返回 (0, nil)——与 Redis 的 GET nil 行为对齐，
// 调用方不需要处理 redis.Nil 特殊值。
func (m *RedisQuotaMeter) Get(ctx context.Context, key string) (int64, error) {
	if m.client == nil {
		return 0, errors.New("RedisQuotaMeter: client is nil")
	}

	val, err := m.client.Get(ctx, key).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, err
	}
	return val, nil
}

// TryIncr 原子地 check-and-decrement，Lua 单次往返。
//
// 语义：
//
//	allowed=true  → cur+delta ≤ limit → INCRBY + EXPIRE，返回 count=cur+delta
//	allowed=false → cur+delta > limit → 什么都不做，返回 count=cur
//
// Redis 端在 Lua 解释器里串行执行，两个并发请求同时接近 limit 时最多有一个
// allowed=true——消除 Get→判断→Incr 的竞态。
//
// nil client 时返回错误（与 Incr/Get 行为一致）；Redis 连接错误冒泡到调用方
// 让 GatewayService 决定是否 fail-open（它已经这样做了）。
func (m *RedisQuotaMeter) TryIncr(ctx context.Context, key string, delta, limit int64, window time.Duration) (allowed bool, count int64, err error) {
	if m.client == nil {
		return false, 0, errors.New("RedisQuotaMeter: client is nil")
	}

	res, err := m.tryIncrCmd.Run(
		ctx, m.client,
		[]string{key},
		delta, limit, int64(window.Seconds()),
	).Slice()
	if err != nil {
		return false, 0, err
	}
	// redis Lua 脚本返回整数切片；go-redis 解码为 []any
	if len(res) != 2 {
		return false, 0, errors.New("RedisQuotaMeter: unexpected Lua result length")
	}
	flag, _ := res[0].(int64)
	cnt, _ := res[1].(int64)
	return flag == 1, cnt, nil
}
