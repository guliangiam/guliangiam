package outbox

import (
	"testing"
	"time"
)

// ──── IsValidStatus ────

func TestIsValidStatus(t *testing.T) {
	for _, s := range AllStatuses {
		if !IsValidStatus(string(s)) {
			t.Errorf("IsValidStatus(%q) = false", s)
		}
	}
	if IsValidStatus("") || IsValidStatus("unknown") {
		t.Error("should reject invalid")
	}
}

// ──── IsTerminalStatus ────

func TestIsTerminalStatus(t *testing.T) {
	for _, s := range []Status{StatusSucceeded, StatusDead, StatusFailed} {
		if !IsTerminalStatus(string(s)) {
			t.Errorf("%s should be terminal", s)
		}
	}
	for _, s := range []Status{StatusPending, StatusProcessing} {
		if IsTerminalStatus(string(s)) {
			t.Errorf("%s should NOT be terminal", s)
		}
	}
}

// ──── TryTransition: 合法 ────

var legalOutboxTransitions = []struct{ from, to Status }{
	{StatusPending, StatusProcessing},
	{StatusProcessing, StatusSucceeded},
	{StatusProcessing, StatusPending}, // 退避重试
	{StatusProcessing, StatusDead},    // 死信
	{StatusProcessing, StatusFailed},  // 兼容
}

func TestTryTransition_Legal(t *testing.T) {
	for _, tc := range legalOutboxTransitions {
		t.Run(string(tc.from)+"→"+string(tc.to), func(t *testing.T) {
			if !TryTransition(tc.from, tc.to) {
				t.Errorf("got false, want true")
			}
		})
	}
}

// ──── TryTransition: 非法 ────

var illegalOutboxTransitions = []struct {
	from, to Status
	reason   string
}{
	// 终态无出边
	{StatusSucceeded, StatusPending, "终态 succeeded 不能回到 pending"},
	{StatusSucceeded, StatusProcessing, "终态不能再被处理"},
	{StatusSucceeded, StatusDead, "终态不能变死信"},
	{StatusDead, StatusPending, "死信不能复活"},
	{StatusDead, StatusProcessing, "死信不能再处理"},
	{StatusFailed, StatusPending, "兼容终态不能复活"},

	// pending 不能直接终态（必须先 processing）
	{StatusPending, StatusSucceeded, "pending 不能直接成功（必须先 processing）"},
	{StatusPending, StatusDead, "pending 不能直接死信"},

	// 自环
	{StatusPending, StatusPending, "自环拒绝"},
	{StatusProcessing, StatusProcessing, "自环拒绝"},
	{StatusSucceeded, StatusSucceeded, "终态自环拒绝"},
	{StatusDead, StatusDead, "终态自环拒绝"},
}

func TestTryTransition_Illegal(t *testing.T) {
	for _, tc := range illegalOutboxTransitions {
		t.Run(string(tc.from)+"→"+string(tc.to), func(t *testing.T) {
			if TryTransition(tc.from, tc.to) {
				t.Errorf("got true, want false — %s", tc.reason)
			}
		})
	}
}

// ──── AllStatuses 完整性 ────

func TestAllStatuses_CoversTransitions(t *testing.T) {
	all := make(map[Status]bool)
	for _, s := range AllStatuses {
		all[s] = true
	}
	for from, tos := range validTransitions {
		if !all[from] {
			t.Errorf("源 %q 不在 AllStatuses", from)
		}
		for to := range tos {
			if !all[to] {
				t.Errorf("目标 %q 不在 AllStatuses", to)
			}
		}
	}
	for _, s := range AllStatuses {
		_, hasOut := validTransitions[s]
		if !hasOut && !IsTerminalStatus(string(s)) {
			t.Errorf("状态 %q 既无出边也不是终态 — 矛盾", s)
		}
	}
}

// ──── BackoffDuration ────

func TestBackoffDuration(t *testing.T) {
	base := time.Second
	maxBackoff := 8 * time.Second

	cases := []struct {
		attempts int
		want     time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 8 * time.Second}, // 到 cap 就停
		{10, 8 * time.Second},
		{-1, 1 * time.Second}, // 防御性
	}
	for _, tc := range cases {
		got := BackoffDuration(tc.attempts, base, maxBackoff)
		if got != tc.want {
			t.Errorf("attempts=%d: got %v, want %v", tc.attempts, got, tc.want)
		}
	}
}

// ──── OutboxItem.Validate ────

func TestOutboxItem_Validate(t *testing.T) {
	good := OutboxItem{
		ID:            "evt-1",
		AggregateType: "webhook",
		AggregateID:   "org-1",
		EventType:     "user.created",
		Status:        StatusPending,
		MaxAttempts:   12,
		Attempts:      0,
	}
	if err := good.Validate(); err != nil {
		t.Errorf("good item rejected: %v", err)
	}

	// 缺字段
	bad := good
	bad.ID = ""
	if err := bad.Validate(); err == nil {
		t.Error("missing ID should fail")
	}

	// 非法状态
	bad = good
	bad.Status = Status("unknown")
	if err := bad.Validate(); err == nil {
		t.Error("unknown status should fail")
	}

	// attempts 超界
	bad = good
	bad.Attempts = 13
	if err := bad.Validate(); err == nil {
		t.Error("attempts > max should fail")
	}

	// attempts 负数
	bad = good
	bad.Attempts = -1
	if err := bad.Validate(); err == nil {
		t.Error("attempts < 0 should fail")
	}
}
