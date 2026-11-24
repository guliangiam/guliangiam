package audit

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformrecover "github.com/guliangiam/guliangiam/internal/platform/recover"
)

// ChainWorker 是审计链后台 Worker。
//
// 两个周期任务：
//  1. 每 anchorInterval 遍历所有 organization 链 → 调 AuditChainWriter.Anchor()
//     锚点写入 audit_chain_anchors，形成链头/链尾定期快照。
//  2. 每 verifyInterval 遍历所有 organization 链 → 全量重算 entry_hash
//     （与 cmd/audit-verify 共用 VerifyChainRecordBatch）：字段内容被篡改但
//     prev_hash/entry_hash 关系未断时也能发现，并定位首个失配的 chain_seq。
//
// 全量校验的失败策略：
//   - 每条链按 verifyBatchSize 条分页加载（keyset: chain_seq > 上一页末条），
//     批间按 verifyBatchPause 限流；不会一次性扫描整张表/整条链进内存。
//   - 每条链有独立 verifyTimeout；超时/查询失败/校验失败只终止当前链，
//     记 ERROR 结构化日志（chain_id、entries_verified、chain_seq 断点、
//     batch_size）并继续下一条链；下一轮 verifyInterval 重新校验。
//   - Worker 上下文取消（Stop）时立即退出。
//   - 每次链任务耗时 Observe 到既有 guliangiam_audit_chain_build_duration_seconds
//     {result=ok|error}，不新增未登记指标名。
//
// 只做本地锚定：proof="" 交给 writer 计算确定性摘要（离线可复算），
// 外部时间戳权威（RFC 3161）仍由调用方显式注入 proof，属后续增强。
//
// 启动：Start(ctx) 起一个 goroutine，两个 ticker 并行。
// 关闭：Stop() cancel ctx + 等 goroutine 退出。
type ChainWorker struct {
	writer         *AuditChainWriter
	anchorInterval time.Duration
	verifyInterval time.Duration
	log            *slog.Logger

	// verifyBatchSize / verifyTimeout / verifyBatchPause are constructor
	// defaults, overridable by package tests. They are deliberately not
	// environment variables: changing them must go through the env registry.
	verifyBatchSize  int
	verifyTimeout    time.Duration
	verifyBatchPause time.Duration

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

const (
	// defaultVerifyBatchSize bounds one chain read. 500 rows keeps the
	// verification working set small and each query short while amortising
	// round-trip cost for normal chains.
	defaultVerifyBatchSize = 500
	// defaultVerifyTimeout bounds one chain's full verification. It stays below
	// the 15-minute verify interval, so one slow chain cannot silently overlap
	// multiple cycles.
	defaultVerifyTimeout = 10 * time.Minute
	// defaultVerifyBatchPause throttles the next batch query. Verifying is a
	// background maintenance job; without a pause a large chain would issue
	// back-to-back scans against the primary database.
	defaultVerifyBatchPause = 20 * time.Millisecond
)

const (
	chainVerifyStageLoad   = "load"
	chainVerifyStageVerify = "verify"
)

// NewChainWorker 构造审计链 Worker。
//   - writer: 由 Bootstrap 注入 AuditChainWriter（持有 DB 端口）
//   - anchorInterval: 锚定间隔（默认 5 分钟）
//   - verifyInterval: 验证间隔（默认 15 分钟）
func NewChainWorker(
	writer *AuditChainWriter,
	anchorInterval, verifyInterval time.Duration,
	log *slog.Logger,
) *ChainWorker {
	if anchorInterval <= 0 {
		anchorInterval = 5 * time.Minute
	}
	if verifyInterval <= 0 {
		verifyInterval = 15 * time.Minute
	}
	if log == nil {
		log = slog.Default()
	}
	return &ChainWorker{
		writer:           writer,
		anchorInterval:   anchorInterval,
		verifyInterval:   verifyInterval,
		log:              log,
		verifyBatchSize:  defaultVerifyBatchSize,
		verifyTimeout:    defaultVerifyTimeout,
		verifyBatchPause: defaultVerifyBatchPause,
		done:             make(chan struct{}),
	}
}

// Start 启动 Worker goroutine。幂等：多次 Start 只启一个。
func (w *ChainWorker) Start(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		return
	}
	ctx, w.cancel = context.WithCancel(ctx)
	platformrecover.SafeGo("audit-chain-worker", func() { w.loop(ctx) })
}

// Stop 关闭 Worker。等 goroutine 退出（最多 anchorInterval + 2s）。
func (w *ChainWorker) Stop() {
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
	case <-time.After(w.anchorInterval + 2*time.Second):
		w.log.Warn("audit chain worker shutdown timed out")
	}
}

// loop 并行跑 anchor ticker + verify ticker，ctx.Done() 时退出。
func (w *ChainWorker) loop(ctx context.Context) {
	defer close(w.done)

	anchorTicker := time.NewTicker(w.anchorInterval)
	defer anchorTicker.Stop()
	verifyTicker := time.NewTicker(w.verifyInterval)
	defer verifyTicker.Stop()

	// 启动后立即跑一次（不等首个 ticker）— 便于开发调试。
	w.anchorAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-anchorTicker.C:
			w.anchorAll(ctx)
		case <-verifyTicker.C:
			w.verifyAll(ctx)
		}
	}
}

// listActiveChains 列出所有有 entry_hash 的 chain_id（= organization_id）。
func (w *ChainWorker) listActiveChains(ctx context.Context) ([]string, error) {
	return w.writer.ListActiveChains(ctx)
}

// refreshPendingGauge refreshes guliangiam_audit_chain_pending_entries.
// A failed count only logs: the gauge keeps its previous value because
// republishing 0 on a query error would hide a real backlog.
func (w *ChainWorker) refreshPendingGauge(ctx context.Context) {
	pending, err := w.writer.CountPending(ctx)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			w.log.Warn("audit chain pending count failed; keeping previous gauge value", "err", err)
		}
		return
	}
	setAuditChainPendingEntries(pending)
}

// anchorAll 遍历所有链 → 每条链调一次 Anchor()。
// 一条链锚定失败只记日志，不阻塞其他链。
// 每个周期先刷新 pending gauge（启动时 loop 也会立即调用一次），
// 每条链的 Anchor 耗时单独 Observe 到 build duration histogram。
func (w *ChainWorker) anchorAll(ctx context.Context) {
	w.refreshPendingGauge(ctx)

	chains, err := w.listActiveChains(ctx)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			w.log.Error("audit chain worker list chains failed", "err", err)
		}
		return
	}
	if len(chains) == 0 {
		return
	}

	w.log.Info("audit chain worker anchoring", "chains", len(chains))
	anchored := 0
	for _, chainID := range chains {
		if err := ctx.Err(); err != nil {
			if !errors.Is(err, context.Canceled) {
				w.log.Warn("audit chain anchor loop ctx error", "chain_id", chainID, "err", err)
			}
			return
		}
		started := time.Now()
		anchorErr := w.writer.Anchor(ctx, chainID, chainID, "")
		observeAuditChainWorkerDuration(started, anchorErr)
		if anchorErr != nil {
			if errors.Is(anchorErr, context.Canceled) {
				w.log.Debug("audit chain anchor skipped — context canceled", "chain_id", chainID)
				return
			}
			w.log.Error("audit chain anchor failed", "chain_id", chainID, "err", anchorErr)
			continue
		}
		anchored++
	}
	w.log.Info("audit chain worker done", "anchored", anchored, "total", len(chains))
}

// verifyAll 遍历所有链 → 每条链做一次全量哈希重算。
// 任一批次查询失败、prev_hash 断裂、算法混用或 entry_hash 与重算值不符都是
// fail-closed：记 ERROR 级结构化日志（链 ID、已验证条数、首个断点 chain_seq），
// 终止当前链并继续下一条；下一轮 verifyInterval 重试。超时同理，不会拖住其他链。
func (w *ChainWorker) verifyAll(ctx context.Context) {
	chains, err := w.listActiveChains(ctx)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			w.log.Error("audit chain worker list chains failed", "err", err)
		}
		return
	}
	if len(chains) == 0 {
		return
	}

	w.log.Info("audit chain worker verifying", "chains", len(chains), "batch_size", w.verifyBatchSize)
	verifiedChains := 0
	for _, chainID := range chains {
		if err := ctx.Err(); err != nil {
			return
		}

		chainCtx, cancel := context.WithTimeout(ctx, w.verifyTimeout)
		started := time.Now()
		result, verifyErr := w.verifyChainFull(chainCtx, chainID)
		cancel()
		observeAuditChainWorkerDuration(started, verifyErr)

		if verifyErr != nil {
			if ctx.Err() != nil {
				w.log.Debug("audit chain verify skipped — context canceled", "chain_id", chainID)
				return
			}
			if errors.Is(verifyErr, context.DeadlineExceeded) {
				w.log.Error("audit chain verification timed out",
					"chain_id", chainID,
					"stage", result.Stage,
					"entries_verified", result.Verified,
					"batch_size", w.verifyBatchSize,
					"timeout", w.verifyTimeout,
					"err", verifyErr)
				continue
			}
			attrs := []any{
				"chain_id", chainID,
				"stage", result.Stage,
				"entries_verified", result.Verified,
				"batch_size", w.verifyBatchSize,
			}
			if result.BreakSeq > 0 {
				attrs = append(attrs, "chain_seq", result.BreakSeq)
			}
			attrs = append(attrs, "err", verifyErr)
			if result.Stage == chainVerifyStageLoad {
				// A page read/decode failure is an operational error, not
				// evidence of tampering; keep the two alertable events apart.
				w.log.Error("audit chain verification failed", attrs...)
			} else {
				// chain broken / tampered — ERROR 级，运维应配置告警
				w.log.Error("audit chain integrity FAILED", attrs...)
			}
			continue
		}
		verifiedChains++
		w.log.Debug("audit chain verified", "chain_id", chainID, "entries", result.Verified)
	}
	w.log.Info("audit chain worker verified", "chains", verifiedChains, "total", len(chains))
}

// chainVerifyResult is the per-chain outcome of verifyChainFull. Stage is
// empty on success, chainVerifyStageLoad for a page read/decode failure, and
// chainVerifyStageVerify for a link/hash mismatch.
type chainVerifyResult struct {
	Verified int
	BreakSeq int64
	Stage    string
}

// verifyChainFull streams one chain in verifyBatchSize pages and runs the same
// pure verifier as cmd/audit-verify. It returns the number of entries verified
// before the first failure and, when a page fails at a located row, that row's
// chain_seq so the caller can log the breakpoint as a structured field.
func (w *ChainWorker) verifyChainFull(ctx context.Context, chainID string) (chainVerifyResult, error) {
	var (
		result   chainVerifyResult
		state    ChainVerifyState
		afterSeq int64
	)
	for {
		batch, loadErr := w.writer.ListChainRecordsBatch(ctx, chainID, afterSeq, w.verifyBatchSize)
		if loadErr != nil {
			result.Stage = chainVerifyStageLoad
			return result, apperrors.WrapInternalf(loadErr,
				"chain %s: load batch after seq %d", chainID, afterSeq)
		}

		count, next, verifyErr := VerifyChainRecordBatch(chainID, batch, state)
		result.Verified += count
		if verifyErr != nil {
			result.Stage = chainVerifyStageVerify
			// count indexes the first failing row in this page; report its
			// chain_seq instead of leaving operators to parse the message.
			if count < len(batch) {
				result.BreakSeq = batch[count].ChainSeq
			}
			return result, verifyErr
		}
		state = next

		if len(batch) < w.verifyBatchSize {
			return result, nil
		}
		afterSeq = batch[len(batch)-1].ChainSeq

		if pauseErr := waitForNextVerifyBatch(ctx, w.verifyBatchPause); pauseErr != nil {
			result.Stage = chainVerifyStageLoad
			return result, pauseErr
		}
	}
}

// waitForNextVerifyBatch throttles between batches and honours ctx cancellation
// so Stop() does not wait for a full chain to drain.
func waitForNextVerifyBatch(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
