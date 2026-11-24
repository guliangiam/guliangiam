package sessions

import "testing"

// ──── IsValid ────

func TestIsValid(t *testing.T) {
	for _, st := range AllStates {
		if !IsValid(string(st)) {
			t.Errorf("IsValid(%q) = false, want true", st)
		}
	}
	if IsValid("") {
		t.Error("IsValid(\"\") = true, want false")
	}
	if IsValid("pending") {
		t.Error("IsValid(\"pending\") = true, want false")
	}
	if IsValid("ACTIVE") { // case sensitive
		t.Error("IsValid(\"ACTIVE\") = true, want false (case-sensitive)")
	}
}

// ──── IsTerminal ────

func TestIsTerminal(t *testing.T) {
	if IsTerminal(string(StateActive)) {
		t.Error("active should not be terminal")
	}
	if IsTerminal(string(StateIdle)) {
		t.Error("idle should not be terminal")
	}
	if !IsTerminal(string(StateExpired)) {
		t.Error("expired should be terminal")
	}
	if !IsTerminal(string(StateRevoked)) {
		t.Error("revoked should be terminal")
	}
}

// ──── TryTransition: 合法路径 ────

// 显式列出所有合法迁移，防止未来有人改 validTransitions 时意外删掉。
var legalTransitions = []struct {
	from, to State
}{
	{StateActive, StateIdle},
	{StateActive, StateExpired},
	{StateActive, StateRevoked},
	{StateIdle, StateActive},
	{StateIdle, StateRevoked},
}

func TestTryTransition_Legal(t *testing.T) {
	for _, tc := range legalTransitions {
		t.Run(string(tc.from)+"→"+string(tc.to), func(t *testing.T) {
			if !TryTransition(tc.from, tc.to) {
				t.Errorf("TryTransition(%q, %q) = false, want true", tc.from, tc.to)
			}
		})
	}
}

// ──── TryTransition: 非法路径（核心！ADR-013 出口条件） ────

var illegalTransitions = []struct {
	from, to State
	reason   string
}{
	// 终态无出边
	{StateExpired, StateActive, "终态 expired 不能复活为 active"},
	{StateExpired, StateIdle, "终态 expired 不能变为 idle"},
	{StateExpired, StateRevoked, "终态 expired 不能再 revoke（已失效）"},
	{StateRevoked, StateActive, "终态 revoked 不能复活为 active"},
	{StateRevoked, StateIdle, "终态 revoked 不能变为 idle"},
	{StateRevoked, StateExpired, "终态 revoked 不能再 expire"},

	// idle 不能直接 expire（闲置≠过期）
	{StateIdle, StateExpired, "idle→expired 非法：闲置≠过期，绝对超时从 active 进 expired"},

	// 自环是 no-op，显式拒绝让调用方知道应该什么都不做
	{StateActive, StateActive, "自环 active→active 无意义，显式拒绝"},
	{StateIdle, StateIdle, "自环 idle→idle 无意义"},
	{StateExpired, StateExpired, "终态自环显式拒绝"},
	{StateRevoked, StateRevoked, "终态自环显式拒绝"},
}

func TestTryTransition_Illegal(t *testing.T) {
	for _, tc := range illegalTransitions {
		t.Run(string(tc.from)+"→"+string(tc.to)+" ("+tc.reason+")", func(t *testing.T) {
			if TryTransition(tc.from, tc.to) {
				t.Errorf("TryTransition(%q, %q) = true, want false — %s",
					tc.from, tc.to, tc.reason)
			}
		})
	}
}

// ──── TryTransition: 非法输入 ────

func TestTryTransition_InvalidInput(t *testing.T) {
	if TryTransition(State(""), StateActive) {
		t.Error("empty from should be illegal")
	}
	if TryTransition(StateActive, State("")) {
		t.Error("empty to should be illegal")
	}
	if TryTransition(State("pending"), StateActive) {
		t.Error("unknown from should be illegal")
	}
	if TryTransition(StateActive, State("pending")) {
		t.Error("unknown to should be illegal")
	}
}

// ──── AllStates 完整性 ────

func TestAllStates_CoversTransitions(t *testing.T) {
	// validTransitions 里出现的所有 from/to 都应该在 AllStates 里
	all := make(map[State]bool)
	for _, s := range AllStates {
		all[s] = true
	}
	// 检查源状态
	for from, tos := range validTransitions {
		if !all[from] {
			t.Errorf("validTransitions 里的源 %q 不在 AllStates 里", from)
		}
		for to := range tos {
			if !all[to] {
				t.Errorf("validTransitions %q→%q 里的目标不在 AllStates 里", from, to)
			}
		}
	}
	// AllStates 里的每个状态要么是 validTransitions 的 key，要么是终态（无出边）
	for _, s := range AllStates {
		_, hasOut := validTransitions[s]
		if !hasOut && !IsTerminal(string(s)) {
			t.Errorf("状态 %q 既无出边也不是终态 — 矛盾", s)
		}
	}
}
