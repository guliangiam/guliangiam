package workpool

import (
	"log/slog"
	"runtime/debug"
)

// runTask 隔离单个任务的 panic（BE-E-19）：一个坏任务不能杀死 worker
// goroutine，否则池容量静默缩水直至 Submit 全部 ErrPoolFull。
func runTask(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("workpool task panic recovered",
				"panic", r,
				"stack", string(debug.Stack()),
			)
		}
	}()
	fn()
}
