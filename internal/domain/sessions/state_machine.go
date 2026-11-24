package sessions

// State 是会话状态机的枚举值（ADR-013）。
//
// 生命周期：
//
//	active ──(闲置超时)──▶ idle ──(交互)──▶ active
//	   │                        │
//	   └──(绝对超时/清理)──────▶ expired
//	   └──(用户/管理员撤销)────▶ revoked
//
// idle 状态下的会话可以被新交互复活为 active（闲置超时只是"暂时不活跃"，
// 不是"已过期"）。expired / revoked 是终态——一旦进入不可复活，必须新建会话。
//
// 禁止的迁移：
//
//	idle    → active 虽然合法，但必须经 App 层在刷新 LastSeenAt 后迁移；
//	         闲置截止时间由 App 层按 LastSeenAt + 组织级 idle_timeout 计算，
//	         Domain 不持有冗余的闲置 deadline。
//	revoked / expired 是终态：任何出边都是非法。
type State string

const (
	StateActive  State = "active"
	StateIdle    State = "idle"
	StateExpired State = "expired"
	StateRevoked State = "revoked"
)

// AllStates 列出所有合法状态（用于 schema CHECK 约束 / migration 验证）。
var AllStates = []State{StateActive, StateIdle, StateExpired, StateRevoked}

// AllStateStrings returns the closed set as strings for 422 `allowed` metadata.
func AllStateStrings() []string {
	values := make([]string, 0, len(AllStates))
	for _, state := range AllStates {
		values = append(values, string(state))
	}
	return values
}

// IsValid 报告 s 是否属于已知状态集合。
// 空字符串也算无效——数据库 DEFAULT 'active' 应该覆盖，但防一下手动构造零值。
func IsValid(s string) bool {
	for _, state := range AllStates {
		if State(s) == state {
			return true
		}
	}
	return false
}

// validTransitions 是显式白名单迁移表。任何不在表中的 from→to 对都是非法。
//
// ADR-013 设计原则：
//   - 终态（expired / revoked）无出边。
//   - 闲置/活跃互转必须经过 TryTransition——不允许裸赋值跳过语义检查。
//   - "active → revoked" 和 "idle → revoked" 合并：撤销是无条件的紧急操作，
//     不关心会话此刻在活跃还是闲置。
var validTransitions = map[State]map[State]bool{
	StateActive: {
		StateIdle:    true,
		StateExpired: true,
		StateRevoked: true,
	},
	StateIdle: {
		StateActive:  true,
		StateRevoked: true, // idle → revoked 允许（管理员撤销闲置会话）
		// idle → expired 不允许：闲置状态本身就意味着"尚未到绝对超时"。
		// 绝对超时从 active 直接进 expired，清理任务定期扫描。
	},
	// StateExpired: 空 map → 无合法出边
	// StateRevoked: 空 map → 无合法出边
}

// IsTerminal 报告 s 是否是终态（不可再迁移）。
func IsTerminal(s string) bool {
	st := State(s)
	return st == StateExpired || st == StateRevoked
}

// TryTransition 尝试把状态从 from 迁移到 to。
//
// 返回 (allowed bool)：true 表示迁移合法；false 表示非法且调用方必须拒绝。
//
// 不 panic、不写 Session 字段（该职责属于 Session 方法），纯检查。
// 设计成纯函数让它在 domain 层可独立单元测试，不依赖 Session 结构体。
func TryTransition(from, to State) bool {
	if !IsValid(string(from)) || !IsValid(string(to)) {
		return false
	}
	next, ok := validTransitions[from]
	if !ok {
		return false
	}
	return next[to]
}

// AllowedTransitions returns the legal targets from `from`, in AllStates order,
// for the 422 `allowed` metadata. It reads validTransitions through
// TryTransition so the state machine stays the single source of truth.
func AllowedTransitions(from State) []string {
	allowed := make([]string, 0, len(AllStates))
	for _, state := range AllStates {
		if TryTransition(from, state) {
			allowed = append(allowed, string(state))
		}
	}
	return allowed
}
