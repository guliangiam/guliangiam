package outbox

import "time"

// Status 是 guliangiam.outbox.status 列的枚举（ADR-008 + schema CHECK）。
//
// 生命周期：
//
//	pending ──(worker 租约取件)──▶ processing ──(成功)──▶ succeeded
//	                                     │
//	                                     └──(失败 attempts<max)──▶ pending (退避后)
//	                                     └──(失败 attempts>=max)──▶ dead
//
// 退避后回 pending 是显式允许的——这是 ADR-008 指数退避重试的核心：
// worker 递增 attempts 并设置 available_at = now + backoff，然后把行
// 放回 pending，等待下次轮询 pickup。
//
// CHECK 约束：
//
//	ck_outbox_status CHECK (status IN ('pending','processing','succeeded','failed','dead'))
//
// 注意：DB CHECK 里有 failed，但 Domain 层状态机用 pending 代表"可重试"
// （含刚创建 + 退避后）。failed 在 DB 层保留供兼容旧代码 / 审计查询。
type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusSucceeded  Status = "succeeded"
	StatusDead       Status = "dead"
	// StatusFailed 在 DB CHECK 中保留但 Domain 层不再使用——重试统一走 pending。
	// 这个常量保留是为了 IsValidStatus 能通过 DB CHECK 兼容旧行。
	StatusFailed Status = "failed"
)

// AllStatuses 匹配 DB ck_outbox_status CHECK 里的 5 值。
var AllStatuses = []Status{
	StatusPending, StatusProcessing, StatusSucceeded, StatusDead, StatusFailed,
}

// IsValidStatus 报告 s 是否属于 DB CHECK 允许的集合。
func IsValidStatus(s string) bool {
	switch Status(s) {
	case StatusPending, StatusProcessing, StatusSucceeded, StatusDead, StatusFailed:
		return true
	default:
		return false
	}
}

// IsTerminalStatus 报告是否终态（不可再被 worker 拾取）。
// succeeded = 投递成功；dead = 超过 max_attempts 进入死信队列。
func IsTerminalStatus(s string) bool {
	st := Status(s)
	return st == StatusSucceeded || st == StatusDead || st == StatusFailed
}

// validTransitions 是 Domain 层允许的状态迁移。
// 相比 DB CHECK 的 5 值，Domain 更严格：
//   - failed 是兼容值，Domain 不主动迁移到它，全部走 pending 退避重试。
//   - 终态无出边。
//   - pending→processing 需要 Worker 租约（在 Repo 层实现，TryTransition 只管状态合法性）。
var validTransitions = map[Status]map[Status]bool{
	StatusPending: {
		StatusProcessing: true, // worker 租约取件
	},
	StatusProcessing: {
		StatusSucceeded: true, // 投递成功
		StatusPending:   true, // 投递失败 → 退避后回到 pending（attempts<max）
		StatusDead:      true, // 投递失败 → 死信（attempts>=max）
		StatusFailed:    true, // 兼容旧代码
	},
	// StatusSucceeded: 空 map → 终态
	// StatusDead:      空 map → 终态
	// StatusFailed:    空 map → 兼容终态
}

// TryTransition 校验 from→to 是否合法。纯函数。
func TryTransition(from, to Status) bool {
	if !IsValidStatus(string(from)) || !IsValidStatus(string(to)) {
		return false
	}
	next, ok := validTransitions[from]
	if !ok {
		return false
	}
	return next[to]
}

// BackoffDuration 计算指数退避时长：base * 2^(attempts)，上限 cap。
// attempts=0 → base，attempts=1 → 2*base，...
// 与 ADR-008 "指数退避 + jitter" 对齐。
func BackoffDuration(attempts int, base, maxBackoff time.Duration) time.Duration {
	if attempts <= 0 {
		return base
	}
	d := base
	for i := 0; i < attempts; i++ {
		d *= 2
		if d >= maxBackoff {
			return maxBackoff
		}
	}
	return d
}
