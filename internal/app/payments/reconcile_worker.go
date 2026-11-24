// reconcile_worker.go — 对账 Worker（设计文档 §8 G11）。
//
// 职责：
//  1. 轮询 ReconRepo 里 status=pending 的对账批次
//  2. 对每个 batch 调用 ChannelGateway.DownloadStatement(batch_date)
//  3. 拉本地 payments (CreatedFrom=batch_date 00:00, CreatedTo=batch_date+1 00:00)
//  4. 按 OutTradeNo 做 diff → 分类差异 (missing_local/missing_channel/amount_mismatch/state_mismatch)
//  5. 差异 → 写 ReconItem；batch.DiffCount/TotalLocal/TotalChannel 填充 → 标 completed/failed
//
// 设计原则（docs/design/commerce/payment.md §6 "对账 Worker + 人工处置"）：
//
//	逐笔对比, 差异入队人工处置 (§8.2 审批流由 transport 层 ResolveItem handler 负责)。
//
// Clean Architecture：本文件只依赖 domain/payments 的 Port 接口 + ChannelGateway SPI。
// 不感知 Postgres/SQLite/任何渠道适配器实现。
package payments

import (
	"context"
	"log/slog"
	"sync"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ReconcileWorker 周期性轮询待处理对账批次, 拉渠道对账单并与本地支付记录对比。
// 遵循 QueryWorker / HealthProbeWorker 的 ticker + ctx.Done() select 生命周期模式。
type ReconcileWorker struct {
	Payments domain.Repository
	Recon    domain.ReconRepo
	Clock    clock.Clock
	UUID     uuidGenerator
	Registry *GatewayRegistry

	// Interval 两次 tick 的间隔。生产建议 5min；每日 02:00 UTC 对账通常由 cron 触发 CreateBatch
	// 来产生 pending batch, 本 worker 负责实际执行。
	Interval time.Duration

	// BatchSize 每轮最多处理多少个 pending batch。
	BatchSize int

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	logger  *slog.Logger

	// runFn 仅用于单测注入异常返回值；nil 时执行 Run。
	runFn func(context.Context) error
}

// NewReconcileWorker 构造一个 ReconcileWorker。
func NewReconcileWorker(
	payments domain.Repository,
	recon domain.ReconRepo,
	ck clock.Clock,
	uuidGen uuidGenerator,
	registry *GatewayRegistry,
	interval time.Duration,
	batchSize int,
) *ReconcileWorker {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	if batchSize <= 0 {
		batchSize = 10
	}
	return &ReconcileWorker{
		Payments:  payments,
		Recon:     recon,
		Clock:     ck,
		UUID:      uuidGen,
		Registry:  registry,
		Interval:  interval,
		BatchSize: batchSize,
		logger:    slog.Default().With("component", "payments-reconcile-worker"),
	}
}

// Start 启动 worker goroutine (幂等)。传入外部 ctx 用于优雅关闭；
// 若传入 context.Background() 则退化为不可终止模式（仅 Stop() 可停止）。
func (w *ReconcileWorker) Start(ctx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	parent := ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	w.cancel = cancel
	w.mu.Unlock()

	run := w.Run
	if w.runFn != nil {
		run = w.runFn
	}
	runWorkerLoop(ctx, w.logger, "payments-reconcile", run)
	w.logger.Info("payments-reconcile worker started", "interval", w.Interval.String())
}

// Stop 停止 worker goroutine (幂等)。
func (w *ReconcileWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return
	}
	if w.cancel != nil {
		w.cancel()
	}
	w.running = false
	w.logger.Info("payments-reconcile worker stopped")
}

// Run 阻塞主循环, 遵循项目通用 ticker + ctx.Done() select 模式。
func (w *ReconcileWorker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()

	w.logger.Debug("payments-reconcile run loop entering")
	for {
		select {
		case <-ctx.Done():
			w.logger.Debug("payments-reconcile run loop exiting", "reason", ctx.Err())
			return ctx.Err()
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				w.logger.Error("payments-reconcile tick failed", "error", err)
			}
		}
	}
}

// tick 执行一轮完整对账: 拉 pending batches → 逐批对账 → 写 ReconItem → 标 batch 状态。
// 单个 batch 失败不应中断整个 tick — 记日志后继续下一个。
func (w *ReconcileWorker) tick(ctx context.Context) error {
	// ReconRepo.Batches 拉全量后本地过滤 pending (ReconRepo 接口尚未暴露 GetPending)
	batches, _, err := w.Recon.Batches(ctx, "", w.BatchSize, 0)
	if err != nil {
		return apperrors.WrapInternalf(err, "list recon batches")
	}
	var pending []*domain.ReconBatch
	for _, b := range batches {
		if b.Status == string(domain.ReconBatchStatusPending) {
			pending = append(pending, b)
		}
	}
	if len(pending) == 0 {
		return nil
	}

	w.logger.Info("payments-reconcile tick: processing pending batches", "count", len(pending))
	for _, b := range pending {
		err := w.reconcileOne(ctx, b)
		if err == nil {
			continue
		}
		w.logger.Error("payments-reconcile: single batch failed, continuing",
			"batch_id", b.ID, "channel", b.Channel, "err", err)
		// 标记 batch 失败但继续处理下一个
		b.Status = string(domain.ReconBatchStatusFailed)
		b.UpdatedAt = w.Clock.Now()
		if updateErr := w.Recon.UpdateBatch(ctx, b); updateErr != nil {
			slog.WarnContext(ctx, "reconcile: persist failed batch failed",
				"batch_id", b.ID, "error", updateErr)
		}
	}
	return nil
}

// RunManual 手动触发指定 ReconBatch 的完整对账流程。用于 HTTP handler
// 的 POST /api/v1/pay-recon:run 端点（design doc §8.2 手动触发）。
//
// 如果 batch 尚不存在 → 调用方（handler）负责先 CreateBatch；此处假设
// batch 已通过 ReconRepo.GetBatch 找到。Fail-Open：Registry 空或渠道不支持
// 对账单时直接标 batch failed 持久化，不 panic。
func (w *ReconcileWorker) RunManual(ctx context.Context, batchID string) error {
	b, err := w.Recon.GetBatch(ctx, batchID)
	if err != nil {
		return apperrors.WrapInternalf(err, "get recon batch %s", batchID)
	}
	return w.reconcileOne(ctx, b)
}

// reconcileOne 对单个 ReconBatch 执行完整对账: DownloadStatement → 本地 List → 逐笔 diff → 写 ReconItem。
func (w *ReconcileWorker) reconcileOne(ctx context.Context, b *domain.ReconBatch) (err error) {
	// failBatch 是 panic-safe 的 defer：任何非 nil error 都保证 batch 被标 failed 持久化。
	// 正常完成（return nil）时由 step 9 显式标 completed。
	defer func() {
		if err != nil {
			b.Status = string(domain.ReconBatchStatusFailed)
			b.UpdatedAt = w.Clock.Now()
			if updateErr := w.Recon.UpdateBatch(ctx, b); updateErr != nil {
				// The failure could not be persisted: the batch would otherwise be
				// retried forever with no trace of why the write was lost.
				slog.WarnContext(ctx, "reconcile worker: failed to persist failed batch",
					"batch_id", b.ID, "error", updateErr)
			}
		}
	}()

	// 1. 标记 batch → processing 防重复处理
	b.Status = string(domain.ReconBatchStatusProcessing)
	b.UpdatedAt = w.Clock.Now()
	if err = w.Recon.UpdateBatch(ctx, b); err != nil {
		return apperrors.WrapInternalf(err, "mark batch %s processing", b.ID)
	}

	// 2. 找到对应渠道网关
	gw, ok := w.Registry.Get(b.Channel)
	if !ok {
		return apperrors.NewUnavailablef("no channel gateway registered for %s", b.Channel)
	}
	meta := gw.Metadata()
	if !meta.SupportsStatement {
		return apperrors.NewUnavailablef("channel %s does not support statement download", b.Channel)
	}

	// 3. 拉渠道对账单
	lines, chErr := gw.DownloadStatement(ctx, domain.StatementRequest{
		OrganizationID: b.OrganizationID,
		Channel:        b.Channel,
		BatchDate:      b.BatchDate,
	})
	if chErr != nil {
		return apperrors.NewUnavailablef("download statement for %s/%s: %s", b.Channel, b.BatchDate, chErr.Message)
	}

	// 4. 拉本地 payments (batch_date 范围)
	from := b.BatchDate
	to := from.AddDate(0, 0, 1) // T+1 00:00
	local, _, err := w.Payments.List(ctx, domain.ListQuery{
		OrganizationID: b.OrganizationID,
		CreatedFrom:    &from,
		CreatedTo:      &to,
	})
	if err != nil {
		return apperrors.WrapInternalf(err, "list local payments for batch %s", b.ID)
	}

	// 5. 构建本地索引
	localByTradeNo := make(map[string]*domain.Payment, len(local))
	for _, p := range local {
		if p.OutOrderId != "" {
			localByTradeNo[p.OutOrderId] = p
		}
	}

	// 6. 构建渠道索引
	channelByTradeNo := make(map[string]*domain.StatementLine, len(lines))
	for i := range lines {
		if lines[i].OutTradeNo != "" {
			channelByTradeNo[lines[i].OutTradeNo] = &lines[i]
		}
	}

	b.TotalLocal = len(local)
	b.TotalChannel = len(lines)

	// 7. 逐笔 diff — 先遍历渠道侧 (渠道的笔数通常少于全量)
	now := w.Clock.Now()
	for _, line := range lines {
		if line.OutTradeNo == "" {
			continue
		}
		lp, exists := localByTradeNo[line.OutTradeNo]
		if !exists {
			// 渠道有, 本地没有 → missing_local
			// createDiffItem 签名: (batchID, diffType, channelOrderNo, localPaymentID, channelAmount, localAmount, channelState, localState, createdAt)
			// 渠道有本地没有 → 只填渠道侧字段
			chAmt := line.Amount
			chSt := line.State
			if err := w.createDiffItem(ctx, b.ID, domain.DiffMissingLocal, &line.OutTradeNo, nil, &chAmt, nil, &chSt, nil, now); err != nil {
				return err
			}
			continue
		}
		// 两边都有 → 比对金额和状态
		if lp.Price != line.Amount {
			la := lp.Price
			chAmt := line.Amount
			chSt := line.State
			ls := lp.State
			if err := w.createDiffItem(ctx, b.ID, domain.DiffAmountMismatch, &line.OutTradeNo, nil, &chAmt, &la, &chSt, &ls, now); err != nil {
				return err
			}
			continue
		}
		if domain.AttemptState(line.State) != attemptStateFromPayment(lp.State) {
			la := lp.Price
			chAmt := line.Amount
			chSt := line.State
			ls := lp.State
			if err := w.createDiffItem(ctx, b.ID, domain.DiffStateMismatch, &line.OutTradeNo, nil, &chAmt, &la, &chSt, &ls, now); err != nil {
				return err
			}
		}
	}

	// 8. 本地有, 渠道没有 → missing_channel
	for _, p := range local {
		if p.OutOrderId == "" {
			continue
		}
		if _, exists := channelByTradeNo[p.OutOrderId]; exists {
			continue
		}
		pid := p.ID
		amt := p.Price
		st := p.State
		if err := w.createDiffItem(ctx, b.ID, domain.DiffMissingChannel, nil, &pid, nil, &amt, nil, &st, now); err != nil {
			return err
		}
	}

	// 9. 更新 batch 汇总 + 标 completed
	items, err := w.Recon.ListItems(ctx, b.ID)
	if err != nil {
		return apperrors.WrapInternalf(err, "list recon items for batch %s", b.ID)
	}
	b.DiffCount = len(items)
	b.Status = string(domain.ReconBatchStatusCompleted)
	b.UpdatedAt = now
	return w.Recon.UpdateBatch(ctx, b)
}

func (w *ReconcileWorker) createDiffItem(
	ctx context.Context,
	batchID string,
	diffType domain.DiffType,
	channelOrderNo *string,
	localPaymentID *string,
	channelAmount *float64,
	localAmount *float64,
	channelState *string,
	localState *string,
	createdAt time.Time,
) error {
	id, err := w.UUID.NewString()
	if err != nil {
		return apperrors.WrapInternalf(err, "gen recon item id")
	}
	item := &domain.ReconItem{
		ID:             id,
		BatchID:        batchID,
		DiffType:       string(diffType),
		ChannelOrderNo: channelOrderNo,
		LocalPaymentID: localPaymentID,
		ChannelAmount:  channelAmount,
		LocalAmount:    localAmount,
		ChannelState:   channelState,
		LocalState:     localState,
		CreatedAt:      createdAt,
	}
	if err := item.Validate(); err != nil {
		return err
	}
	return w.Recon.CreateItem(ctx, item)
}

// attemptStateFromPayment maps Payment state → AttemptState for channel comparison.
// Local Payment states are strings in domain.State*; channel lines use AttemptState
// (succeeded/failed/refunded/unknown/pending).
func attemptStateFromPayment(s string) domain.AttemptState {
	switch s {
	case string(domain.StateSucceeded):
		return domain.AttemptStateSucceeded
	case string(domain.StateFailed):
		return domain.AttemptStateFailed
	case string(domain.StateRefunded):
		return "refunded" // channel-side normalises refunds differently per vendor
	case string(domain.StateProcessing):
		return domain.AttemptStatePending
	default:
		return domain.AttemptStateUnknown
	}
}
