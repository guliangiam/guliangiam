package outbox

import (
	"context"
	"testing"
	"time"
)

// BE-E-11 回归：单项超时 × batch ≤ 租约 TTL；超时 item 的终态落库
// 脱离投递 ctx，不会因 ctx 过期而丢失 MarkRetry。

func TestWorker_ItemTimeoutDefaultsToLeaseShare(t *testing.T) {
	w := NewWorker(newFakeRepo(), newFakeRegistry(), "w-1")
	if w.itemTimeout != 3*time.Second {
		t.Errorf("default itemTimeout = %s, want 3s (30s TTL / 10 batch)", w.itemTimeout)
	}
	if w.itemTimeout*time.Duration(w.batchSize) > w.lockTTL {
		t.Error("invariant violated: itemTimeout × batch must not exceed lockTTL")
	}
}

func TestWorker_ItemTimeoutClampedWhenExplicitTooLarge(t *testing.T) {
	w := NewWorker(newFakeRepo(), newFakeRegistry(), "w-1",
		WithLockTTL(30*time.Second), WithBatchSize(10), WithItemTimeout(60*time.Second))
	if w.itemTimeout != 3*time.Second {
		t.Errorf("clamped itemTimeout = %s, want 3s", w.itemTimeout)
	}
}

func TestWorker_ItemTimeoutRespectsSmallerExplicitValue(t *testing.T) {
	w := NewWorker(newFakeRepo(), newFakeRegistry(), "w-1",
		WithLockTTL(30*time.Second), WithBatchSize(10), WithItemTimeout(time.Second))
	if w.itemTimeout != time.Second {
		t.Errorf("itemTimeout = %s, want 1s", w.itemTimeout)
	}
}

type blockingHandler struct{}

func (blockingHandler) Handle(ctx context.Context, _ *OutboxItem) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestWorker_SlowItemTimesOutAndStillMarksRetry(t *testing.T) {
	repo := newFakeRepo()
	reg := newFakeRegistry()
	reg.Register("slow", blockingHandler{})
	fast := &fakeHandler{}
	reg.Register("fast", fast)

	slowItem := &OutboxItem{
		ID: "evt-slow", AggregateType: "slow", EventType: "x",
		Status: StatusPending, MaxAttempts: 3, AvailableAt: time.Now().Add(-time.Second),
	}
	fastItem := &OutboxItem{
		ID: "evt-fast", AggregateType: "fast", EventType: "x",
		Status: StatusPending, MaxAttempts: 3, AvailableAt: time.Now().Add(-time.Second),
	}
	repo.items[slowItem.ID] = slowItem
	repo.items[fastItem.ID] = fastItem

	// 200ms 租约 / batch 2 → 每项 100ms；慢 handler 必然超时。
	w := NewWorker(repo, reg, "w-1", WithLockTTL(200*time.Millisecond), WithBatchSize(2))
	start := time.Now()
	w.poll(context.Background())
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Fatalf("poll took %s; per-item timeout did not bound the slow handler", elapsed)
	}
	if len(repo.retried) != 1 || repo.retried[0] != "evt-slow" {
		t.Errorf("retried = %v, want [evt-slow]（超时落库不能随投递 ctx 一起失效）", repo.retried)
	}
	if len(repo.success) != 1 || repo.success[0] != "evt-fast" {
		t.Errorf("success = %v, want [evt-fast]（慢项超时不得拖累同批快项）", repo.success)
	}
}
