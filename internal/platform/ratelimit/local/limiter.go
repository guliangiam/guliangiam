// Package local 实现 domain/ratelimit.Limiter 的进程内版本。
//
// 用 golang.org/x/time/rate + sync.Map 管理每个 key 的 token bucket。
// 这是 ADR-019 的**降级底座** — Redis 不可用时的 Fail-Close 兜底。
// 生产环境多实例部署时应优先用 Redis Lua 版本（等 ADR-010 引入）。
package local

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"

	domain "github.com/guliangiam/guliangiam/internal/domain/ratelimit"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
)

// Limiter 是进程内令牌桶实现。
// 每个 key 有独立的 *rate.Limiter；空闲条目按 maxAge 清理。
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	cleanup *time.Ticker
	maxAge  time.Duration
	stopCh  chan struct{}
}

type bucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewLimiter 构造进程内限流器。
//
// maxAge: 空闲条目在 map 里保留多久（超时后清理）。
// cleanupInterval: 清理 goroutine 周期（推荐 maxAge/2 ~ maxAge）。
func NewLimiter(maxAge, cleanupInterval time.Duration) *Limiter {
	if cleanupInterval <= 0 {
		cleanupInterval = maxAge / 2
	}
	l := &Limiter{
		buckets: make(map[string]*bucket),
		maxAge:  maxAge,
		cleanup: time.NewTicker(cleanupInterval),
		stopCh:  make(chan struct{}),
	}
	recover.SafeGo("local-ratelimit-cleanup", l.cleanupLoop)
	return l
}

// Close 停止清理 goroutine（应用退出时调）。
func (l *Limiter) Close() {
	select {
	case <-l.stopCh: // 已关
	default:
		close(l.stopCh)
		l.cleanup.Stop()
	}
}

func (l *Limiter) cleanupLoop() {
	for {
		select {
		case <-l.cleanup.C:
			l.prune()
		case <-l.stopCh:
			return
		}
	}
}

func (l *Limiter) prune() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for k, b := range l.buckets {
		if now.Sub(b.lastSeen) > l.maxAge {
			delete(l.buckets, k)
		}
	}
}

// getOrCreate 获取或创建一个 key 的 bucket。
func (l *Limiter) getOrCreate(key string, quota domain.Quota) *bucket {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		burst := quota.Burst
		if burst <= 0 {
			burst = int(quota.Limit) // 桶容量 = 每秒速率（默认行为）
		}
		b = &bucket{
			limiter:  rate.NewLimiter(rate.Limit(quota.Limit), burst),
			lastSeen: time.Now(),
		}
		l.buckets[key] = b
	}
	b.lastSeen = time.Now()
	return b
}

// Allow 实现 domain.Limiter.Allow。
func (l *Limiter) Allow(ctx context.Context, key string, tokens int, quota domain.Quota) (domain.Decision, error) {
	fullKey := quota.KeyPrefix + key
	b := l.getOrCreate(fullKey, quota)

	allowed := b.limiter.AllowN(time.Now(), tokens)

	// 计算剩余令牌（简化：用 reserve 的 Delay 反推 reset 时间）
	remaining := 0
	if allowed {
		remaining = b.limiter.Burst() - tokens
	}
	resetAfter := time.Duration(0)
	if !allowed {
		// 预估需要等多久 — 用 Reserve 的 Delay 字段
		r := b.limiter.ReserveN(time.Now(), tokens)
		if !r.OK() {
			// burst 太小
			resetAfter = time.Second
		} else {
			resetAfter = r.Delay()
			r.Cancel() // 不实际消费 — Allow 只做决策
		}
	}

	return domain.Decision{
		Allowed:       allowed,
		Remaining:     remaining,
		ResetAfter:    resetAfter,
		CurrentTokens: b.limiter.Burst() - remaining,
		PolicyID:      quota.PolicyID,
	}, nil
}

// Reserve 实现 domain.Limiter.Reserve。
func (l *Limiter) Reserve(ctx context.Context, key string, tokens int, quota domain.Quota) (domain.Reserve, error) {
	fullKey := quota.KeyPrefix + key
	b := l.getOrCreate(fullKey, quota)

	r := b.limiter.ReserveN(time.Now(), tokens)
	if !r.OK() {
		// burst 太小，无法 reserve
		return nil, context.DeadlineExceeded // 但我们不想暴露 rate 包错误 — 包装成自定义
	}
	return &localReserve{rl: b.limiter, tokens: tokens, reserve: r}, nil
}

// Wait 实现 domain.Limiter.Wait。
func (l *Limiter) Wait(ctx context.Context, key string, tokens int, quota domain.Quota) error {
	fullKey := quota.KeyPrefix + key
	b := l.getOrCreate(fullKey, quota)
	return b.limiter.WaitN(ctx, tokens)
}

// localReserve 实现 domain.Reserve。
type localReserve struct {
	rl      *rate.Limiter
	tokens  int
	reserve *rate.Reservation
}

func (r *localReserve) Delay() time.Duration { return r.reserve.Delay() }
func (r *localReserve) Wait(ctx context.Context) error {
	// Wait 到 reserve 可用 — golang.org/x/time/rate 没有直接 API，用定时器
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(r.reserve.Delay()):
		// 到点了，实际消费
		r.rl.ReserveN(time.Now(), r.tokens) // 已经在时间点上，应该立即通过
		return nil
	}
}
func (r *localReserve) Cancel() { r.reserve.Cancel() }

// 编译期断言
var _ domain.Limiter = (*Limiter)(nil)
