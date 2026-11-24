// Package ratelimit defines ADR-019 统一限流的域端口接口。
//
// 设计原则（Experience 1322436 整合方案）：
//   - 按职责拆域：前置关口（Allow）/ 反馈呈现（Snapshot）与实现解耦
//   - domain 接口只定义"限流决策"语义：Allow / Reserve / Wait
//   - 反馈头呈现（IETF RateLimit-*）留在 platform/ratelimit 包，不动
//   - 进程内实现保留为故障降级底座（Fail-Close），Redis Lua 实现等 ADR-010 引入
//
// 三个已有实现将逐步迁移到本接口下：
//   - platform/middleware/ratelimit  — TokenBucketLimiter（主框架）
//   - transport/middleware/service_ip_limit — ServiceIPLimiter（服务-IP 维度）
//   - app/verifications/ratelimit    — 验证码窗口计数（语义特殊，保留独立方法）
//
// 新代码**只用本接口**，不再直接 import golang.org/x/time/rate。
package ratelimit

import (
	"context"
	"time"
)

// Quota 定义一个限流策略的配置。
// 所有限流器（进程内 / Redis Lua / 网关）必须能表达的最小字段集。
type Quota struct {
	// Limit 是每秒允许的令牌数。
	Limit float64
	// Burst 是令牌桶最大容量（允许瞬时突发）。
	Burst int
	// KeyPrefix 是限流键的前缀，用于区分维度。
	// 例如 "ip:" / "user:" / "org:"
	KeyPrefix string
	// PolicyID 是策略标识，写入 RateLimit-Policy header。
	PolicyID string
}

// DefaultWindowSeconds 是窗口配额转令牌桶的默认窗口（1 秒）。
// 若调用方用窗口配额（"100 请求/分钟"），需要在构造 Quota 时手动换算成 Limit=100/60。
const DefaultWindowSeconds = 1

// Decision 是 Allow/Reserve/Wait 的决策结果。
// 与 platform/ratelimit.Dimension/Snapshot 解耦 — 本接口只管"允许/拒绝 + 剩余令牌"，
// HTTP 反馈头由 platform/ratelimit 包根据本决策写回。
type Decision struct {
	// Allowed = true 表示放行；false 表示拒绝。
	Allowed bool
	// Remaining 是剩余令牌数（>=0）。
	Remaining int
	// ResetAfter 是客户端应该等多少秒后再试（Allowed=false 时用）。
	// Allowed=true 时为 0。
	ResetAfter time.Duration
	// CurrentTokens 是决策后的瞬时令牌量（调试/日志用）。
	CurrentTokens int
	// PolicyID 是触发决策的策略标识（与 Quota.PolicyID 对应）。
	PolicyID string
}

// Limiter 是统一限流的域端口接口。
// 三种实现：
//   - local: 进程内 golang.org/x/time/rate（降级底座）
//   - redis: Redis + Lua 原子令牌桶（ADR-010 引入时补）
//   - stub: 测试零依赖（始终允许 / 始终拒绝）
//
// 方法语义：
//   - Allow:   非阻塞，立即返回决策（HTTP 中间件用这个）
//   - Reserve: 非阻塞，返回一个 Reserve 对象（调用方可以 Wait / Cancel）
//   - Wait:    阻塞直到拿到令牌或 ctx 取消（后台任务重试用）
type Limiter interface {
	// Allow 尝试立即消费 tokens 个令牌，不阻塞。
	// Returns Decision 描述决策；error 只有 Redis 网络故障等场景才会返回
	// （进程内实现永不返回 error）。
	Allow(ctx context.Context, key string, tokens int, quota Quota) (Decision, error)

	// Reserve 预留 tokens 个令牌，返回 Reserve 对象让调用方决定等不等。
	Reserve(ctx context.Context, key string, tokens int, quota Quota) (Reserve, error)

	// Wait 阻塞直到拿到 tokens 个令牌或 ctx 取消。
	Wait(ctx context.Context, key string, tokens int, quota Quota) error
}

// Reserve 表示一个"已预留但尚未消费"的令牌请求。
// 调用方可以 Wait() 实际消费，或 Cancel() 释放。
type Reserve interface {
	// Delay 返回拿到令牌需要等多久（0 表示立即可用）。
	Delay() time.Duration
	// Wait 阻塞直到令牌可用或 ctx 取消。
	Wait(ctx context.Context) error
	// Cancel 释放预留的令牌（不归还到桶里 — 令牌桶没有退款语义）。
	Cancel()
}

// 编译期断言：接口可用
var _ Limiter = (*StubLimiter)(nil)

// ──────────────────────────────────────────────────────────────
//  StubLimiter — 测试零依赖实现
// ──────────────────────────────────────────────────────────────

// StubLimiter 是测试用零依赖实现。
// 两种模式：AlwaysAllow / AlwaysDeny。
type StubLimiter struct {
	AlwaysAllow bool
	// AllowFunc 可以用自定义函数覆盖（最灵活）。
	AllowFunc func(ctx context.Context, key string, tokens int, quota Quota) (Decision, error)
}

func (s *StubLimiter) Allow(ctx context.Context, key string, tokens int, quota Quota) (Decision, error) {
	if s.AllowFunc != nil {
		return s.AllowFunc(ctx, key, tokens, quota)
	}
	if s.AlwaysAllow {
		return Decision{Allowed: true, Remaining: quota.Burst - tokens, PolicyID: quota.PolicyID}, nil
	}
	return Decision{Allowed: false, ResetAfter: time.Second, PolicyID: quota.PolicyID}, nil
}

func (s *StubLimiter) Reserve(ctx context.Context, key string, tokens int, quota Quota) (Reserve, error) {
	dec, err := s.Allow(ctx, key, tokens, quota)
	if err != nil {
		return nil, err
	}
	return &stubReserve{allowed: dec.Allowed, delay: dec.ResetAfter}, nil
}

func (s *StubLimiter) Wait(ctx context.Context, key string, tokens int, quota Quota) error {
	if !s.AlwaysAllow && s.AllowFunc == nil {
		return context.DeadlineExceeded
	}
	return nil
}

type stubReserve struct {
	allowed bool
	delay   time.Duration
}

func (r *stubReserve) Delay() time.Duration { return r.delay }
func (r *stubReserve) Wait(ctx context.Context) error {
	if !r.allowed {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(r.delay):
			return nil
		}
	}
	return nil
}
func (r *stubReserve) Cancel() {}

var _ Limiter = (*StubLimiter)(nil)
