package crypto

import "testing"

// ──── IsValidKeyStatus ────

func TestIsValidKeyStatus(t *testing.T) {
	for _, st := range AllKeyStates {
		if !IsValidKeyStatus(string(st)) {
			t.Errorf("IsValidKeyStatus(%q) = false, want true", st)
		}
	}
	if IsValidKeyStatus("") {
		t.Error("empty should be invalid")
	}
	if IsValidKeyStatus("rotten") {
		t.Error("rotten should be invalid")
	}
	// DB schema CHECK 允许的 5 值，Domain 层一一对应
}

// ──── IsTerminalKeyStatus ────

func TestIsTerminalKeyStatus(t *testing.T) {
	for _, s := range []KeyStatus{KeyStatusPending, KeyStatusActive, KeyStatusRetiring} {
		if IsTerminalKeyStatus(string(s)) {
			t.Errorf("%s should NOT be terminal", s)
		}
	}
	for _, s := range []KeyStatus{KeyStatusRetired, KeyStatusRevoked} {
		if !IsTerminalKeyStatus(string(s)) {
			t.Errorf("%s should be terminal", s)
		}
	}
}

// ──── TryKeyTransition: 合法路径 ────

var legalKeyTransitions = []struct {
	from, to KeyStatus
}{
	{KeyStatusPending, KeyStatusActive},
	{KeyStatusPending, KeyStatusRevoked},
	{KeyStatusActive, KeyStatusRetiring},
	{KeyStatusActive, KeyStatusRevoked},
	{KeyStatusRetiring, KeyStatusRetired},
	{KeyStatusRetiring, KeyStatusRevoked},
}

func TestTryKeyTransition_Legal(t *testing.T) {
	for _, tc := range legalKeyTransitions {
		t.Run(string(tc.from)+"→"+string(tc.to), func(t *testing.T) {
			if !TryKeyTransition(tc.from, tc.to) {
				t.Errorf("got false, want true")
			}
		})
	}
}

// ──── TryKeyTransition: 非法路径（核心！终态无出边 + 流程绕过拦截） ────

var illegalKeyTransitions = []struct {
	from, to KeyStatus
	reason   string
}{
	// 终态无出边
	{KeyStatusRetired, KeyStatusActive, "retired 不能复活为 active"},
	{KeyStatusRetired, KeyStatusRetiring, "retired 不能回到 retiring"},
	{KeyStatusRetired, KeyStatusRevoked, "retired 不能再 revoke（已退役）"},
	{KeyStatusRevoked, KeyStatusActive, "revoked 不能复活"},
	{KeyStatusRevoked, KeyStatusPending, "revoked 不能回到 pending"},
	{KeyStatusRevoked, KeyStatusRetiring, "revoked 不能回到 retiring"},
	{KeyStatusRevoked, KeyStatusRetired, "revoked 不能变 retired"},

	// 流程绕过拦截
	{KeyStatusActive, KeyStatusRetired, "active 不能直接 retired — 必须先 retiring 走完重加密"},
	{KeyStatusPending, KeyStatusRetiring, "pending 不能直接 retiring — 必须先 active"},
	{KeyStatusPending, KeyStatusRetired, "pending 不能直接 retired"},
	{KeyStatusPending, KeyStatusRevoked, "这条应该合法（已列入 legalKeyTransitions）"}, // 占位，后面修正
	{KeyStatusRetiring, KeyStatusActive, "retiring 不能回到 active（轮换是单向的）"},

	// 自环
	{KeyStatusPending, KeyStatusPending, "自环显式拒绝"},
	{KeyStatusActive, KeyStatusActive, "自环显式拒绝"},
	{KeyStatusRetiring, KeyStatusRetiring, "自环显式拒绝"},
	{KeyStatusRetired, KeyStatusRetired, "终态自环显式拒绝"},
	{KeyStatusRevoked, KeyStatusRevoked, "终态自环显式拒绝"},
}

func TestTryKeyTransition_Illegal(t *testing.T) {
	for _, tc := range illegalKeyTransitions {
		// pending→revoked 实际上是合法的，skip
		if tc.from == KeyStatusPending && tc.to == KeyStatusRevoked {
			continue
		}
		t.Run(string(tc.from)+"→"+string(tc.to)+" ("+tc.reason+")", func(t *testing.T) {
			if TryKeyTransition(tc.from, tc.to) {
				t.Errorf("got true, want false — %s", tc.reason)
			}
		})
	}
}

// ──── TryKeyTransition: 非法输入 ────

func TestTryKeyTransition_InvalidInput(t *testing.T) {
	if TryKeyTransition(KeyStatus(""), KeyStatusActive) {
		t.Error("empty from should be false")
	}
	if TryKeyTransition(KeyStatusActive, KeyStatus("")) {
		t.Error("empty to should be false")
	}
	if TryKeyTransition(KeyStatus("rotten"), KeyStatusActive) {
		t.Error("unknown from should be false")
	}
	if TryKeyTransition(KeyStatusActive, KeyStatus("rotten")) {
		t.Error("unknown to should be false")
	}
}

// ──── AllKeyStates 完整性 ────

func TestAllKeyStates_CoversTransitions(t *testing.T) {
	all := make(map[KeyStatus]bool)
	for _, s := range AllKeyStates {
		all[s] = true
	}
	for from, tos := range validKeyTransitions {
		if !all[from] {
			t.Errorf("validKeyTransitions 源 %q 不在 AllKeyStates", from)
		}
		for to := range tos {
			if !all[to] {
				t.Errorf("validKeyTransitions %q→%q 目标不在 AllKeyStates", from, to)
			}
		}
	}
	for _, s := range AllKeyStates {
		_, hasOut := validKeyTransitions[s]
		if !hasOut && !IsTerminalKeyStatus(string(s)) {
			t.Errorf("状态 %q 既无出边也不是终态 — 矛盾", s)
		}
	}
}
