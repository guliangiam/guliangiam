// Package recover provides panic containment for background goroutines.
//
// Rule: any goroutine started with `go <fn>()` that is NOT a short-lived
// fan-out (parallel map/reduce) MUST be wrapped by SafeGo — a panic in an
// unwrapped goroutine crashes the whole process. HTTP request paths are
// covered by chi.Recoverer, but worker loops, periodic cleanup loops, and
// scheduler goroutines are not.
//
// Usage:
//
//	// Before (process crash risk):
//	go w.run()
//
//	// After (logs + keeps process alive):
//	recover.SafeGo("webhook-worker", w.run)
package recover

import (
	"log/slog"
	"runtime/debug"
	"time"
)

// SafeGo launches fn in a new goroutine with a panic handler that logs
// the panic + stack trace and lets the process continue.
//
// The `name` argument identifies the goroutine in logs so operators can
// route the alert to the right component (outbox-worker, cleanup-ttl, ...).
func SafeGo(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("goroutine panic recovered",
					"goroutine", name,
					"panic", r,
					"stack", string(debug.Stack()),
				)
			}
		}()
		fn()
	}()
}

// SafeGoWithLogger is like SafeGo but accepts a caller-supplied logger.
// Useful when the caller already has a component-scoped logger.
func SafeGoWithLogger(l *slog.Logger, name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if l != nil {
					l.Error("goroutine panic recovered",
						"goroutine", name,
						"panic", r,
						"stack", string(debug.Stack()),
					)
				}
			}
		}()
		fn()
	}()
}

// SuperviseOption 配置 SupervisedGo 的重启策略。
type SuperviseOption func(*superviseConfig)

type superviseConfig struct {
	maxRestarts int
	baseBackoff time.Duration
	maxBackoff  time.Duration
	stableAfter time.Duration
}

// WithMaxRestarts 设置连续 panic 重启上限（默认 10）。达到上限后放弃并
// 记 Error 日志——进程仍存活，但组件停摆必须可被告警捕获。
func WithMaxRestarts(n int) SuperviseOption { return func(c *superviseConfig) { c.maxRestarts = n } }

// WithSuperviseBackoff 设置重启退避基准/上限（默认 1s / 30s，指数递增）。
func WithSuperviseBackoff(base, max time.Duration) SuperviseOption {
	return func(c *superviseConfig) { c.baseBackoff = base; c.maxBackoff = max }
}

// WithStableAfter 设置「稳定运行」阈值（默认 1min）：fn 单次运行超过该时长
// 后 panic，连续重启计数清零，避免长寿命 worker 偶发 panic 耗尽预算。
func WithStableAfter(d time.Duration) SuperviseOption {
	return func(c *superviseConfig) { c.stableAfter = d }
}

// SupervisedGo 是 SafeGo 的监督式变体（BE-E-15）：fn panic 后按指数退避
// 重启，而不是只记一条日志就永久停摆。fn 正常返回则监督结束（不重启）。
// 返回的 channel 在监督最终结束（正常退出或放弃重启）时关闭。
func SupervisedGo(name string, fn func(), opts ...SuperviseOption) <-chan struct{} {
	cfg := superviseConfig{
		maxRestarts: 10,
		baseBackoff: time.Second,
		maxBackoff:  30 * time.Second,
		stableAfter: time.Minute,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		restarts := 0
		backoff := cfg.baseBackoff
		for {
			start := time.Now()
			panicked := runWithRecover(name, fn)
			if !panicked {
				return
			}
			if time.Since(start) >= cfg.stableAfter {
				restarts = 0
				backoff = cfg.baseBackoff
			}
			restarts++
			if restarts > cfg.maxRestarts {
				slog.Error("supervised goroutine gave up after repeated panics",
					"goroutine", name, "restarts", restarts-1)
				return
			}
			slog.Warn("supervised goroutine restarting after panic",
				"goroutine", name, "restart", restarts, "backoff", backoff.String())
			time.Sleep(backoff)
			if backoff < cfg.maxBackoff {
				backoff *= 2
				if backoff > cfg.maxBackoff {
					backoff = cfg.maxBackoff
				}
			}
		}
	}()
	return done
}

// runWithRecover 同步执行 fn，返回是否发生 panic（panic 已记日志并吞掉）。
func runWithRecover(name string, fn func()) (panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true
			slog.Error("goroutine panic recovered",
				"goroutine", name,
				"panic", r,
				"stack", string(debug.Stack()),
			)
		}
	}()
	fn()
	return false
}
