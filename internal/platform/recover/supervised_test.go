package recover

import (
	"sync/atomic"
	"testing"
	"time"
)

// BE-E-15 回归：SupervisedGo panic 后退避重启、正常返回不重启、
// 连续 panic 达上限后放弃、稳定运行后计数清零。

func waitDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("supervision did not finish in time")
	}
}

func TestSupervisedGo_RestartsAfterPanic(t *testing.T) {
	var calls atomic.Int32
	done := SupervisedGo("test-restart", func() {
		if calls.Add(1) < 3 {
			panic("boom")
		}
	}, WithSuperviseBackoff(time.Millisecond, 2*time.Millisecond), WithMaxRestarts(5))
	waitDone(t, done)
	if got := calls.Load(); got != 3 {
		t.Errorf("calls = %d, want 3 (two panics then success)", got)
	}
}

func TestSupervisedGo_NormalReturnNoRestart(t *testing.T) {
	var calls atomic.Int32
	done := SupervisedGo("test-normal", func() { calls.Add(1) },
		WithSuperviseBackoff(time.Millisecond, 2*time.Millisecond))
	waitDone(t, done)
	time.Sleep(10 * time.Millisecond)
	if got := calls.Load(); got != 1 {
		t.Errorf("calls = %d, want 1", got)
	}
}

func TestSupervisedGo_GivesUpAfterMaxRestarts(t *testing.T) {
	var calls atomic.Int32
	done := SupervisedGo("test-giveup", func() {
		calls.Add(1)
		panic("always")
	}, WithSuperviseBackoff(time.Millisecond, 2*time.Millisecond), WithMaxRestarts(2))
	waitDone(t, done)
	if got := calls.Load(); got != 3 {
		t.Errorf("calls = %d, want 3 (initial + 2 restarts)", got)
	}
}

func TestSupervisedGo_StableRunResetsRestartBudget(t *testing.T) {
	var calls atomic.Int32
	done := SupervisedGo("test-stable", func() {
		n := calls.Add(1)
		if n == 2 {
			// 第二次运行超过 stableAfter 后 panic → 计数清零，
			// 预算耗尽前还能再重启一次（第三次运行）。
			time.Sleep(30 * time.Millisecond)
		}
		panic("boom")
	}, WithSuperviseBackoff(time.Millisecond, 2*time.Millisecond),
		WithMaxRestarts(1), WithStableAfter(20*time.Millisecond))
	waitDone(t, done)
	if got := calls.Load(); got != 3 {
		t.Errorf("calls = %d, want 3 (stable run must reset the restart budget)", got)
	}
}
