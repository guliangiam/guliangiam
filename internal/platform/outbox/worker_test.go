package outbox

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// ──── 假 Registry ────

type fakeRegistry struct {
	mu       sync.Mutex
	handlers map[string]DeliveryHandler
}

func newFakeRegistry() *fakeRegistry { return &fakeRegistry{handlers: map[string]DeliveryHandler{}} }
func (r *fakeRegistry) Register(aggregateType string, h DeliveryHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[aggregateType] = h
}
func (r *fakeRegistry) Get(aggregateType string) (DeliveryHandler, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	h, ok := r.handlers[aggregateType]
	return h, ok
}

// ──── 假 Handler ────

type fakeHandler struct {
	mu      sync.Mutex
	calls   int
	failAt  int // 第 N 次调用失败（1-based），<=0 表示不失败
	failErr error
}

func (h *fakeHandler) Handle(ctx context.Context, item *OutboxItem) error {
	h.mu.Lock()
	h.calls++
	call := h.calls
	fail := h.failAt > 0 && call == h.failAt
	h.mu.Unlock()
	if fail {
		if h.failErr != nil {
			return h.failErr
		}
		return errors.New("boom")
	}
	return nil
}

// ──── 假 Repository（内存，精确记录每一步） ────

type fakeRepo struct {
	mu       sync.Mutex
	items    map[string]*OutboxItem
	inserted []string
	success  []string
	retried  []string // id + availableAt
	dead     []string
}

func newFakeRepo() *fakeRepo { return &fakeRepo{items: map[string]*OutboxItem{}} }

func (r *fakeRepo) Insert(ctx context.Context, item *OutboxItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[item.ID] = item
	r.inserted = append(r.inserted, item.ID)
	return nil
}

func (r *fakeRepo) LockBatch(ctx context.Context, limit int, workerID string, ttl time.Duration) ([]*OutboxItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var batch []*OutboxItem
	for _, item := range r.items {
		if item.Status != StatusPending || !item.AvailableAt.Before(time.Now()) {
			continue
		}
		item.Status = StatusProcessing
		now := time.Now()
		item.LockedBy = &workerID
		item.LockedAt = &now
		batch = append(batch, item)
		if len(batch) >= limit {
			break
		}
	}
	return batch, nil
}

func (r *fakeRepo) MarkSuccess(ctx context.Context, id string, t time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[id]
	if !ok {
		return fmt.Errorf("item %s not found", id)
	}
	item.Status = StatusSucceeded
	item.ProcessedAt = &t
	r.success = append(r.success, id)
	return nil
}

func (r *fakeRepo) MarkRetry(ctx context.Context, id, errMsg string, now, availableAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[id]
	if !ok {
		return fmt.Errorf("item %s not found", id)
	}
	item.Attempts++
	item.Status = StatusPending
	item.AvailableAt = availableAt
	item.LastError = &errMsg
	r.retried = append(r.retried, id)
	return nil
}

func (r *fakeRepo) MarkDead(ctx context.Context, id, errMsg string, deadAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[id]
	if !ok {
		return fmt.Errorf("item %s not found", id)
	}
	item.Status = StatusDead
	item.LastError = &errMsg
	r.dead = append(r.dead, id)
	return nil
}

// ──── Worker 测试 ────

func TestWorker_Success(t *testing.T) {
	repo := newFakeRepo()
	reg := newFakeRegistry()
	handler := &fakeHandler{}
	reg.Register("webhook", handler)

	item := &OutboxItem{
		ID:            "evt-1",
		AggregateType: "webhook",
		AggregateID:   "org-1",
		EventType:     "user.created",
		Status:        StatusPending,
		MaxAttempts:   3,
		AvailableAt:   time.Now().Add(-time.Second), // 立即可用
	}
	repo.items[item.ID] = item

	w := NewWorker(repo, reg, "worker-1", WithPollInterval(10*time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)

	// 等 poll 循环
	time.Sleep(50 * time.Millisecond)
	cancel()
	w.Stop()

	if len(repo.success) != 1 {
		t.Errorf("success count = %d, want 1", len(repo.success))
	}
	if handler.calls != 1 {
		t.Errorf("handler calls = %d, want 1", handler.calls)
	}
	if repo.items["evt-1"].Status != StatusSucceeded {
		t.Errorf("status = %s, want succeeded", repo.items["evt-1"].Status)
	}
}

func TestWorker_NoHandler_MarkDead(t *testing.T) {
	repo := newFakeRepo()
	reg := newFakeRegistry()
	// 不注册 webhook handler

	item := &OutboxItem{
		ID:            "evt-2",
		AggregateType: "webhook",
		AggregateID:   "org-1",
		EventType:     "user.created",
		Status:        StatusPending,
		MaxAttempts:   3,
		AvailableAt:   time.Now().Add(-time.Second),
	}
	repo.items[item.ID] = item

	w := NewWorker(repo, reg, "worker-1", WithPollInterval(10*time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)

	time.Sleep(50 * time.Millisecond)
	cancel()
	w.Stop()

	if len(repo.dead) != 1 {
		t.Errorf("dead count = %d, want 1", len(repo.dead))
	}
	if repo.items["evt-2"].Status != StatusDead {
		t.Errorf("status = %s, want dead", repo.items["evt-2"].Status)
	}
	if repo.items["evt-2"].LastError == nil {
		t.Error("LastError should be set")
	}
}

func TestWorker_MaxRetries_GoDead(t *testing.T) {
	repo := newFakeRepo()
	reg := newFakeRegistry()
	// handler 每次都失败
	handler := &fakeHandler{failAt: 1, failErr: errors.New("always fail")}
	reg.Register("webhook", handler)

	// IsDead() 检查 item.Attempts >= item.MaxAttempts（handler 调用前快照）
	// 直接把 Attempts 设为 == MaxAttempts，handler 一失败就进 MarkDead
	item := &OutboxItem{
		ID:            "evt-3",
		AggregateType: "webhook",
		AggregateID:   "org-1",
		EventType:     "user.created",
		Status:        StatusPending,
		MaxAttempts:   3,
		Attempts:      3, // == max → 立即死信
		AvailableAt:   time.Now().Add(-time.Second),
	}
	repo.items[item.ID] = item

	ctx := context.Background()
	batch, _ := repo.LockBatch(ctx, 10, "worker-1", 30*time.Second)
	if len(batch) != 1 {
		t.Fatalf("LockBatch returned %d items", len(batch))
	}

	w := NewWorker(repo, reg, "worker-1")
	w.processItem(ctx, batch[0], time.Now())

	if repo.items["evt-3"].Status != StatusDead {
		t.Errorf("status = %s, want dead", repo.items["evt-3"].Status)
	}
	if len(repo.dead) != 1 {
		t.Errorf("dead count = %d, want 1", len(repo.dead))
	}
}

func TestWorker_PermanentFailure_MarkDeadImmediately(t *testing.T) {
	repo := newFakeRepo()
	reg := newFakeRegistry()
	handler := &fakeHandler{failAt: 1, failErr: Permanent(errors.New("invalid payload"))}
	reg.Register("webhook", handler)

	item := &OutboxItem{
		ID:            "evt-permanent",
		AggregateType: "webhook",
		AggregateID:   "org-1",
		EventType:     "user.created",
		Status:        StatusPending,
		MaxAttempts:   3,
		Attempts:      0, // 仍有一次重试预算，但永久失败必须立即死信
		AvailableAt:   time.Now().Add(-time.Second),
	}
	repo.items[item.ID] = item

	ctx := context.Background()
	batch, _ := repo.LockBatch(ctx, 10, "worker-1", 30*time.Second)
	if len(batch) != 1 {
		t.Fatalf("LockBatch returned %d items", len(batch))
	}

	w := NewWorker(repo, reg, "worker-1")
	w.processItem(ctx, batch[0], time.Now())

	if repo.items["evt-permanent"].Status != StatusDead {
		t.Errorf("status = %s, want dead", repo.items["evt-permanent"].Status)
	}
	if len(repo.dead) != 1 {
		t.Errorf("dead count = %d, want 1", len(repo.dead))
	}
	if len(repo.retried) != 0 {
		t.Errorf("permanent failure must not consume retry budget, retried=%d", len(repo.retried))
	}
}

func TestWorker_MultipleWorkers_NoDuplicatePickup(t *testing.T) {
	// fakeRepo 不做真正的 SKIP LOCKED，但 LockBatch 已改为只取 Status==pending 的
	// 两个 worker 同时 LockBatch 不应该拿到同一条（一个先改为 processing）
	repo := newFakeRepo()
	reg := newFakeRegistry()
	handler := &fakeHandler{}
	reg.Register("webhook", handler)

	for i := 0; i < 5; i++ {
		repo.items[fmt.Sprintf("evt-%d", i)] = &OutboxItem{
			ID:            fmt.Sprintf("evt-%d", i),
			AggregateType: "webhook",
			AggregateID:   "org-1",
			EventType:     "user.created",
			Status:        StatusPending,
			MaxAttempts:   3,
			AvailableAt:   time.Now().Add(-time.Second),
		}
	}

	ctx := context.Background()
	b1, _ := repo.LockBatch(ctx, 10, "w1", 30*time.Second)
	b2, _ := repo.LockBatch(ctx, 10, "w2", 30*time.Second)

	// w1 先拿到 5 个，w2 拿到 0 个（都已 processing）
	if len(b1) != 5 {
		t.Errorf("w1 got %d, want 5", len(b1))
	}
	if len(b2) != 0 {
		t.Errorf("w2 got %d, want 0 (no duplicate pickup)", len(b2))
	}
}
