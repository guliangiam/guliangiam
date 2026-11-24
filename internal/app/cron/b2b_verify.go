package cron

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	appb2b "github.com/guliangiam/guliangiam/internal/app/b2b"
	domainb2b "github.com/guliangiam/guliangiam/internal/domain/b2b"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// 单轮认领预算。租约必须大于一轮 DNS 查询耗时；实例崩溃后到期自动恢复。
const (
	b2bClaimBatch = 50
	b2bClaimLease = 5 * time.Minute
)

// b2bVerificationPolicy 沿用应用层默认续验预算（5 次失败；退避
// 1/2/4/8/16 分钟，上限 60 分钟）。真正进入 failed 才计 failed 指标。
var b2bVerificationPolicy = appb2b.DefaultVerificationPolicy()

// b2bVerified / b2bFailed 是包级单例 Counter —— 避免多 Worker 实例时重复注册。
// promauto 注册到 prometheus.DefaultRegisterer；bootstrap 的 /metrics 使用
// promhttp.Handler()（默认 registry），两者因此共享同一采集面。
var (
	b2bVerified = promauto.NewCounter(prometheus.CounterOpts{
		Name: "b2b_domain_verified_total",
		Help: "B2B DNS TXT verification success count",
	})
	b2bFailed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "b2b_domain_failed_total",
		Help: "B2B DNS TXT verification terminal failure count after retry budget exhausted",
	})
)

// B2BVerifyWorker 是 B2B 域名治理 DNS TXT 验证 Cron。
//
// 流程：
//
//	ticker(2min) → svc.ClaimPending(50, 5min) → for each:
//		svc.VerifySingle(policy) → verified / pending(retry) / failed → Prometheus counter
//
// 多实例：ClaimPending 用单语句 CTE + FOR UPDATE SKIP LOCKED 原子认领并加租约，
// 同一 pending 行不会被两个实例重复查询。Update 仍带 `status='pending'`；
// 冲突方得到 Conflict，视为其他实例已终态化，不计 failed 指标。
type B2BVerifyWorker struct {
	svc      appb2b.Service
	interval time.Duration
	log      *slog.Logger

	// Prometheus 指标 —— verified 成功 +1，failed 仅预算耗尽终态 +1。
	verified prometheus.Counter
	failed   prometheus.Counter

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewB2BVerifyWorker 构造。默认 interval 2min。
// Prometheus counter 由包级 promauto 注册一次 —— 多 Worker 实例共享。
func NewB2BVerifyWorker(svc appb2b.Service, opts ...WorkerOption) *B2BVerifyWorker {
	w := &B2BVerifyWorker{
		svc:      svc,
		interval: 2 * time.Minute,
		log:      slog.Default(),
		verified: b2bVerified,
		failed:   b2bFailed,
	}
	for _, o := range opts {
		o(w)
	}
	return w
}

// WorkerOption 调整 Worker 参数。
type WorkerOption func(*B2BVerifyWorker)

func WithInterval(d time.Duration) WorkerOption {
	return func(w *B2BVerifyWorker) { w.interval = d }
}
func WithLogger(l *slog.Logger) WorkerOption {
	return func(w *B2BVerifyWorker) { w.log = l }
}

// Start 起 goroutine 进入 ticker 循环。幂等 —— 已启动时 no-op。
func (w *B2BVerifyWorker) Start(ctx context.Context) {
	w.mu.Lock()
	if w.cancel != nil {
		w.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.done = make(chan struct{})
	done := w.done
	w.mu.Unlock()

	recover.SafeGo("b2b-verify", func() { w.loop(ctx, done) })
	w.log.Info("b2b verify worker started", "interval", w.interval)
}

// Stop 发 cancel 并等待 goroutine 退出（最多 interval + 1s）。
func (w *B2BVerifyWorker) Stop() {
	w.mu.Lock()
	cancel := w.cancel
	done := w.done
	w.cancel = nil
	w.done = nil
	w.mu.Unlock()

	if cancel == nil {
		return
	}
	cancel()

	select {
	case <-done:
	case <-time.After(w.interval + time.Second):
		w.log.Warn("b2b verify worker did not stop within grace period")
	}
	w.log.Info("b2b verify worker stopped")
}

// loop 持有启动时的 done channel —— Stop 会清空 w.done 字段，
// 若 defer 再读字段会 close(nil) panic。
func (w *B2BVerifyWorker) loop(ctx context.Context, done chan struct{}) {
	defer close(done)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// 启动立即跑一次，不等首个 ticker。
	w.run(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.run(ctx)
		}
	}
}

// run 单次 Cron 循环：ClaimPending → VerifySingle。
// 全程带 ctx —— ctx.Done() 时立即返回，不做新的 DNS 查询。
// organization_domains 受 FORCE RLS：跨租户读写必须显式 system scope，
// 不能依赖 HTTP 请求上下文。ClaimPending 自身在事务内执行以应用 GUC。
func (w *B2BVerifyWorker) run(ctx context.Context) {
	ctx = tenantctx.System(ctx)
	pending, err := w.svc.ClaimPending(ctx, b2bClaimBatch, b2bClaimLease)
	if err != nil {
		w.log.Error("b2b verify: claim pending failed", "err", err)
		return
	}

	if len(pending) == 0 {
		return
	}

	w.log.Info("b2b verify: starting batch", "count", len(pending))
	var verified, failed, retried, conflicts int

	for i := range pending {
		p := &pending[i]
		select {
		case <-ctx.Done():
			return
		default:
		}

		// DTO → domain entity —— VerifySingle 签名是 *domainb2b.OrganizationDomain。
		ent := dtoToEnt(p)
		out, err := w.svc.VerifySingle(ctx, ent, b2bVerificationPolicy)
		if err != nil {
			// 已被其他实例终态化：不是业务失败，不计 failed 指标。
			if apperrors.FromError(err).Code == apperrors.CodeConflict {
				conflicts++
				w.log.Debug("b2b verify: already terminalized", "domain_id", p.ID)
				continue
			}
			// 未进入终态的异常（DB 等）同样不冒充失败指标。
			w.log.Error("b2b verify: single failed", "domain_id", p.ID, "err", err)
			continue
		}

		switch out.Status {
		case domainb2b.StatusVerified:
			verified++
			w.verified.Inc()
		case domainb2b.StatusFailed:
			failed++
			w.failed.Inc()
			w.log.Warn("b2b verify: retry budget exhausted",
				"domain_id", out.ID, "attempts", out.Attempts, "code", failureCode(out.LastError))
		default:
			retried++
			w.log.Info("b2b verify: retry scheduled",
				"domain_id", out.ID, "attempts", out.Attempts, "code", failureCode(out.LastError))
		}
	}

	w.log.Info("b2b verify: batch done",
		"verified", verified, "failed", failed, "retried", retried, "conflicts", conflicts)
}

// failureCode 取 last_error 的分类码前缀（"dns_nxdomain: ..." → "dns_nxdomain"）。
// 日志只记录 domain_id/attempts/分类码，不记录 token 或完整错误文本。
func failureCode(lastError string) string {
	if lastError == "" {
		return "unknown"
	}
	if i := strings.IndexByte(lastError, ':'); i > 0 {
		return lastError[:i]
	}
	return lastError
}

// dtoToEnt 是 DTO → domain entity 转换。
// cron 包不持有 Repository —— 全部通过 Service 操作。
func dtoToEnt(dto *appb2b.DomainDTO) *domainb2b.OrganizationDomain {
	return &domainb2b.OrganizationDomain{
		ID:                dto.ID,
		OrganizationID:    dto.OrganizationID,
		Domain:            dto.Domain,
		VerificationToken: dto.VerificationToken,
		Status:            dto.Status,
		VerifiedAt:        dto.VerifiedAt,
		CreatedAt:         dto.CreatedAt,
		UpdatedAt:         dto.UpdatedAt,
		Attempts:          dto.Attempts,
		LastError:         dto.LastError,
		NextRetryAt:       dto.NextRetryAt,
		LastCheckedAt:     dto.LastCheckedAt,
	}
}
