package outbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/recover"
)

// Worker 是 outbox 投递 Worker。
//
// 启动：Start() 起一个 goroutine，每 pollInterval 轮询一次 Repository.LockBatch()。
// 关闭：Stop() 发 ctx cancel + 等 goroutine 退出（最多 pollInterval + 1s）。
//
// 投递流程（每个 item）：
//
//  1. LockBatch(limit, workerID, lockTTL)   租约取件（FOR UPDATE SKIP LOCKED）
//  2. registry.Get(aggregateType)           找 Handler
//     - 找不到 → MarkDead + last_error="no handler registered for aggregate_type=xxx"
//  3. handler.Handle(item)                  投递
//     - success → MarkSuccess
//     - err → 算 attempts+1:
//     - < max → MarkRetry（attempts+1, available_at 指数退避, status=pending）
//     - >= max → MarkDead
//
// 并发安全：Worker 本身不持有 item，全部通过 Repository 操作。多 Worker 并行
// 由 DB FOR UPDATE SKIP LOCKED 保证不重复取件。
type Worker struct {
	repo        Repository
	registry    DeliveryHandlerRegistry
	observer    DeliveryObserver
	workerID    string
	pollInt     time.Duration
	lockTTL     time.Duration
	batchSize   int
	itemTimeout time.Duration
	backoffBase time.Duration
	backoffCap  time.Duration

	log *slog.Logger

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewWorker 构造 Worker。参数：
//   - workerID: 唯一名（用于 locked_by 列，多 worker 时区分）
//   - pollInt: 轮询间隔（默认 5s）
//   - lockTTL: 租约时长（默认 30s，超过则认为 worker 崩溃，租约超时后行会被下一轮 pickup）
//   - batchSize: 每次最多取多少（默认 10）
//   - backoffBase/Cap: 指数退避基准/上限（默认 1s / 32s）
func NewWorker(
	repo Repository,
	registry DeliveryHandlerRegistry,
	workerID string,
	opts ...WorkerOption,
) *Worker {
	w := &Worker{
		repo:        repo,
		registry:    registry,
		workerID:    workerID,
		pollInt:     5 * time.Second,
		lockTTL:     30 * time.Second,
		batchSize:   10,
		backoffBase: time.Second,
		backoffCap:  32 * time.Second,
		log:         slog.Default(),
		done:        make(chan struct{}),
	}
	for _, opt := range opts {
		opt(w)
	}
	if w.batchSize < 1 {
		w.batchSize = 10
	}
	// BE-E-11：单项超时 × batch 必须 < 租约 TTL，否则串行投递还没跑完
	// 租约就过期，行会被其他 worker 重复领取。默认均分租约；显式配置
	// 违反不变量时同样收紧到均分值。
	if maxPerItem := w.lockTTL / time.Duration(w.batchSize); w.itemTimeout <= 0 || w.itemTimeout > maxPerItem {
		w.itemTimeout = maxPerItem
	}
	return w
}

// WorkerOption 是构造选项。
type WorkerOption func(*Worker)

func WithPollInterval(d time.Duration) WorkerOption { return func(w *Worker) { w.pollInt = d } }
func WithLockTTL(d time.Duration) WorkerOption      { return func(w *Worker) { w.lockTTL = d } }
func WithBatchSize(n int) WorkerOption              { return func(w *Worker) { w.batchSize = n } }
func WithItemTimeout(d time.Duration) WorkerOption  { return func(w *Worker) { w.itemTimeout = d } }
func WithBackoff(base, maxBackoff time.Duration) WorkerOption {
	return func(w *Worker) { w.backoffBase = base; w.backoffCap = maxBackoff }
}
func WithLogger(l *slog.Logger) WorkerOption { return func(w *Worker) { w.log = l } }

// WithDeliveryObserver registers a best-effort post-delivery observer.
func WithDeliveryObserver(o DeliveryObserver) WorkerOption {
	return func(w *Worker) { w.observer = o }
}

// Start 启动 Worker goroutine。幂等：多次 Start 只启一个 goroutine。
func (w *Worker) Start(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		return // 已启动
	}
	ctx, w.cancel = context.WithCancel(ctx)
	// BE-E-15：panic 后监督式重启 loop，而不是静默停摆。
	finished := recover.SupervisedGo("outbox-worker", func() { w.loop(ctx) })
	recover.SafeGo("outbox-worker-done", func() { <-finished; close(w.done) })
}

// Stop 关闭 Worker。最多等 pollInt + 1s。
func (w *Worker) Stop() {
	w.mu.Lock()
	if w.cancel == nil {
		w.mu.Unlock()
		return
	}
	cancel := w.cancel
	w.cancel = nil
	w.mu.Unlock()
	cancel()
	select {
	case <-w.done:
	case <-time.After(w.pollInt + time.Second):
		w.log.Warn("outbox worker shutdown timed out", "worker_id", w.workerID)
	}
}

// loop 是主循环。正常退出（ctx 取消）由 Start 的监督器关闭 done；
// panic 退出时监督器会重启 loop，done 保持开放。
func (w *Worker) loop(ctx context.Context) {
	ticker := time.NewTicker(w.pollInt)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.poll(ctx)
		}
	}
}

// poll 一次轮询：取 batch 个 item → 逐个投递。
func (w *Worker) poll(ctx context.Context) {
	items, err := w.repo.LockBatch(ctx, w.batchSize, w.workerID, w.lockTTL)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			w.log.Error("outbox LockBatch failed", "worker_id", w.workerID, "err", err)
		}
		return
	}
	if len(items) == 0 {
		return
	}

	w.log.Debug("outbox worker picked items", "worker_id", w.workerID, "count", len(items))

	now := time.Now()
	for _, item := range items {
		itemCtx, cancel := context.WithTimeout(ctx, w.itemTimeout)
		w.processItem(itemCtx, item, now)
		cancel()
	}
}

// processItem 投递单个 item。
func (w *Worker) processItem(ctx context.Context, item *OutboxItem, now time.Time) {
	// 投递 ctx 可能已超时/取消；终态落库必须脱离它，否则超时 item 的
	// MarkRetry/MarkDead 也会失败，行滞留到租约过期被重复投递。
	markCtx, cancelMark := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancelMark()

	handler, ok := w.registry.Get(item.AggregateType)
	if !ok {
		errMsg := fmt.Sprintf("no handler registered for aggregate_type=%q", item.AggregateType)
		w.log.Error("outbox item has no handler", "aggregate_type", item.AggregateType, "event_type", item.EventType)
		if err := w.repo.MarkDead(markCtx, item.ID, errMsg, now); err != nil {
			w.log.Error("outbox MarkDead failed (no handler)", "err", err)
		}
		return
	}

	err := handler.Handle(ctx, item)
	if err == nil {
		if merr := w.repo.MarkSuccess(markCtx, item.ID, now); merr != nil {
			w.log.Error("outbox MarkSuccess failed", "err", merr)
			return
		}
		if w.observer != nil {
			if oerr := w.observer.Observe(ctx, item); oerr != nil {
				w.log.Warn("outbox observer failed",
					"aggregate_type", item.AggregateType, "event_type", item.EventType, "err", oerr)
			}
		}
		return
	}

	// 永久失败（契约违反/载荷不可解析）→ 立即死信，不消耗重试预算。
	// 可重试失败（依赖不可用、瞬时错误）→ 继续走 attempts/backoff。
	if IsPermanent(err) {
		if merr := w.repo.MarkDead(markCtx, item.ID, err.Error(), now); merr != nil {
			w.log.Error("outbox MarkDead failed (permanent)", "err", merr)
		}
		return
	}

	// 投递失败 → 递增 attempts
	if item.IsDead() {
		if merr := w.repo.MarkDead(markCtx, item.ID, err.Error(), now); merr != nil {
			w.log.Error("outbox MarkDead failed", "err", merr)
		}
		return
	}

	availableAt := item.NextAvailableTime(now, w.backoffBase, w.backoffCap)
	if merr := w.repo.MarkRetry(markCtx, item.ID, err.Error(), now, availableAt); merr != nil {
		w.log.Error("outbox MarkRetry failed", "err", merr)
	}
}
