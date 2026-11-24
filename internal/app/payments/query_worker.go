// query_worker.go — 渠道状态查询 Worker（设计文档 §10 G10）。
//
// 职责：
//  1. 定期列出所有 state=processing 的支付单
//  2. 对每张支付单调用 ChannelGateway.Query 查询渠道侧状态
//  3. 写 pay_attempts 留痕（QueryRequest/QueryResult 快照）
//  4. 如果 Query 返回终态（succeeded/failed/rejected）→ 推进 Payment 状态机
//  5. 如果 Query 返回 unknown → 仅留痕 + 不推断（等待下一轮再查）
//
// 设计原则（docs/design/commerce/payment.md §7 "UNKNOWN 状态收敛"）：
//
//	UNKNOWN 不推断 — 渠道状态未知时不得推断为成功或失败，必须由本 Worker 收敛。
//
// Clean Architecture：本文件只依赖 domain/payments 的 Port 接口 +
// ChannelGateway SPI 接口。注册具体 ChannelGateway 实现由构造时注入。
package payments

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
)

// QueryWorker 定期轮询处于 processing 状态的支付单，向渠道查询真实状态，
// 并在获取到终态时推进 Payment 状态机。遵循 HealthProbeWorker 的 Start/Stop
// 生命周期模式（同一项目通用 ticker + ctx.Done() select）。
type QueryWorker struct {
	Payments domain.Repository
	Attempts domain.AttemptsRepo
	Clock    clock.Clock
	UUID     uuidGenerator
	Registry *GatewayRegistry

	// Interval 两次 tick 的间隔。生产建议 60s；单测可设为 0 手动调用 tick。
	Interval time.Duration

	// BatchSize 每轮最多处理多少张 processing 支付单。防止一次性查爆渠道。
	BatchSize int

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	logger  *slog.Logger

	// runFn 仅用于单测注入异常返回值；nil 时执行 Run。
	runFn func(context.Context) error
}

// uuidGenerator 屏蔽具体 UUID 库依赖，单测可 stub。
type uuidGenerator interface {
	NewString() (string, error)
}

// NewQueryWorker 构造一个 QueryWorker。
func NewQueryWorker(
	payments domain.Repository,
	attempts domain.AttemptsRepo,
	ck clock.Clock,
	uuidGen uuidGenerator,
	registry *GatewayRegistry,
	interval time.Duration,
	batchSize int,
) *QueryWorker {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	return &QueryWorker{
		Payments:  payments,
		Attempts:  attempts,
		Clock:     ck,
		UUID:      uuidGen,
		Registry:  registry,
		Interval:  interval,
		BatchSize: batchSize,
		logger:    slog.Default().With("component", "payments-query-worker"),
	}
}

// Start 启动 worker goroutine（幂等）。传入外部 ctx 用于优雅关闭；
// 若传入 context.Background() 则退化为不可终止模式（仅 Stop() 可停止）。
func (w *QueryWorker) Start(ctx context.Context) {
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
	runWorkerLoop(ctx, w.logger, "payments-query", run)
	w.logger.Info("payments-query worker started", "interval", w.Interval.String())
}

// runWorkerLoop 在独立 goroutine 中执行 worker 主循环，并按退出原因分级记录日志：
// 正常返回或上下文取消/超时属预期退出（Debug），其余错误必须落 Error——
// 否则 worker 异常退出会被静默吞掉，运维侧只看到任务不再执行却无任何线索。
func runWorkerLoop(ctx context.Context, logger *slog.Logger, name string, run func(context.Context) error) {
	recover.SafeGo(name, func() {
		err := run(ctx)
		if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			logger.Debug(name+" worker loop exited", "reason", err)
			return
		}
		logger.Error(name+" worker stopped with error", "error", err)
	})
}

// Stop 停止 worker goroutine（幂等）。
func (w *QueryWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return
	}
	if w.cancel != nil {
		w.cancel()
	}
	w.running = false
	w.logger.Info("payments-query worker stopped")
}

// Run 是阻塞主循环，遵循项目通用的 ticker + ctx.Done() select 模式。
func (w *QueryWorker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()

	w.logger.Debug("payments-query run loop entering")
	for {
		select {
		case <-ctx.Done():
			w.logger.Debug("payments-query run loop exiting", "reason", ctx.Err())
			return ctx.Err()
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				w.logger.Error("payments-query tick failed", "error", err)
			}
		}
	}
}

// tick 执行一轮完整查询：列出 processing 支付 → 逐张 Query → 写 attempt → 收敛状态。
// 单张支付处理失败不应中断整个 batch — 记日志后继续下一张。
func (w *QueryWorker) tick(ctx context.Context) error {
	stateProcessing := string(domain.StateProcessing)
	items, _, err := w.Payments.List(ctx, domain.ListQuery{
		Page:     1,
		PageSize: w.BatchSize,
		State:    &stateProcessing,
	})
	if err != nil {
		return apperrors.WrapInternalf(err, "list processing payments")
	}
	if len(items) == 0 {
		return nil
	}

	w.logger.Info("payments-query tick: processing batch", "count", len(items))
	for _, p := range items {
		if err := w.queryOne(ctx, p); err != nil {
			w.logger.Error("payments-query: single payment failed, continuing",
				"payment_id", p.ID, "channel", p.Provider, "err", err)
			continue
		}
	}
	return nil
}

// queryOne 查询单个支付单的渠道侧状态，写 attempt 留痕，并在必要时推进状态机。
func (w *QueryWorker) queryOne(ctx context.Context, p *domain.Payment) error {
	gw, ok := w.Registry.Get(p.Provider)
	if !ok {
		return apperrors.NewUnavailablef("no channel gateway registered for %s", p.Provider)
	}

	req := domain.QueryRequest{
		PaymentID:      p.ID,
		ChannelOrderNo: p.ChannelOrderNo,
		OutTradeNo:     p.OutOrderId,
	}

	start := w.Clock.Now()
	res, chErr := gw.Query(ctx, req)
	latencyMs := int(w.Clock.Now().Sub(start).Milliseconds())

	attemptID, _ := w.UUID.NewString()
	channelOrderNo := p.ChannelOrderNo
	attempt := &domain.PayAttempt{
		ID:             attemptID,
		OrganizationID: p.OrganizationID,
		PaymentID:      p.ID,
		Channel:        p.Provider,
		AttemptType:    "query",
		ChannelOrderNo: &channelOrderNo,
		LatencyMs:      &latencyMs,
		OccurredAt:     w.Clock.Now(),
	}

	if chErr != nil {
		attempt.State = domain.AttemptStateFailed
		if chErr.Code != "" {
			code := chErr.Code
			attempt.ErrorCode = &code
		}
		if chErr.Message != "" {
			msg := chErr.Message
			attempt.ErrorMessage = &msg
		}
		if err := w.Attempts.Create(ctx, attempt); err != nil {
			return apperrors.WrapInternalf(err, "persist pay_attempt for payment %s", p.ID)
		}
		// Retryable 错误（网络/5xx/限流）→ 只留痕，Payment 状态不动；下一轮继续查
		return nil
	}

	attempt.State = res.State
	co := res.ChannelOrderNo
	attempt.ChannelOrderNo = &co
	if res.ErrorCode != "" {
		ec := res.ErrorCode
		attempt.ErrorCode = &ec
	}

	if err := w.Attempts.Create(ctx, attempt); err != nil {
		return apperrors.WrapInternalf(err, "persist pay_attempt for payment %s", p.ID)
	}

	// UNKNOWN → 留痕后不动 Payment 状态（下一轮继续查）
	if res.State == domain.AttemptStateUnknown {
		return nil
	}

	// 终态 → 推进 Payment 状态机（复用 callback_pipeline.go 里已有的映射函数）
	newState := attemptStateToPaymentState(res.State)
	if newState == "" {
		return nil // unknown / rejected — 不推进（callback_pipeline.go 的映射）
	}
	if p.CanTransitionTo(newState) {
		p.State = newState
		if res.ChannelOrderNo != "" {
			p.ChannelOrderNo = res.ChannelOrderNo
		}
		if res.ErrorCode != "" {
			p.LastErrorCode = res.ErrorCode
		}
		p.UpdatedAt = w.Clock.Now()
		if err := w.Payments.Update(ctx, p); err != nil {
			return apperrors.WrapInternalf(err, "update payment %s state to %s", p.ID, newState)
		}
	}
	return nil
}
