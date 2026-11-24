package aigateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
)

const (
	// defaultProbeInterval 后台探测 worker 的默认周期。
	defaultProbeInterval = 30 * time.Second
	// defaultProbeTimeout 单次 HTTP HEAD 探测的超时。
	defaultProbeTimeout = 5 * time.Second
)

// HealthProbeWorker 周期性地扫描所有处于 Open 状态的 ProviderHealth，
// 当 OpenUntil 过期时将其转为 HalfOpen，并向 provider.BaseURL 发送
// 轻量 HEAD 请求测试连通性。探测成功 → RecordSuccess（Healthy）；
// 探测失败 → RecordFailure（重新 Open + 新 OpenUntil）。
//
// 设计文档 §2.4 "熔断健康：llm_provider_health 状态 + 半开探测"。
// 状态转换事件通过 Outbox 投递（§13）——ai.provider.circuit_opened /
// ai.provider.circuit_closed，供 metering_events 和通知下游消费。
type HealthProbeWorker struct {
	Health    domain.ProviderHealthRepo
	Providers domain.ProvidersRepo
	Clock     clock.Clock
	HTTP      *http.Client // SSRF 保护的探测 client

	// Outbox 是审计事件投递端口。nil 表示审计投递关闭（Fail-Open，
	// 开发环境友好；生产应配置）。
	Outbox domainoutbox.Repository

	Interval time.Duration // 两次 tick 的间隔（默认 30s）

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	logger  *slog.Logger

	// runFn 仅用于单测注入异常返回值；nil 时执行 Run。
	runFn func(context.Context) error
}

// NewHealthProbeWorker 构造一个 HealthProbeWorker。httpClient 必须已经
// 配置好 SSRF 保护（infra/aiupstream.NewSSRFClient）。outbox 可以为 nil。
func NewHealthProbeWorker(
	health domain.ProviderHealthRepo,
	providers domain.ProvidersRepo,
	ck clock.Clock,
	httpClient *http.Client,
	outbox domainoutbox.Repository,
	interval time.Duration,
	logger *slog.Logger,
) *HealthProbeWorker {
	if interval <= 0 {
		interval = defaultProbeInterval
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &HealthProbeWorker{
		Health:    health,
		Providers: providers,
		Clock:     ck,
		HTTP:      httpClient,
		Outbox:    outbox,
		Interval:  interval,
		logger:    logger.With("component", "ai-gateway-health-probe"),
	}
}

// Start 启动 worker 的 goroutine（幂等）。传入外部 ctx 用于优雅关闭。
func (w *HealthProbeWorker) Start(ctx context.Context) {
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
	recover.SafeGo("aigateway-health-probe", func() {
		err := run(ctx)
		if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			w.logger.Debug("health-probe worker loop exited", "reason", err)
			return
		}
		w.logger.Error("health-probe worker stopped with error", "error", err)
	})
	w.logger.Info("health-probe worker started", "interval", w.Interval.String())
}

// Stop 停止 worker goroutine（幂等）。
func (w *HealthProbeWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return
	}
	if w.cancel != nil {
		w.cancel()
	}
	w.running = false
	w.logger.Info("health-probe worker stopped")
}

// Run 是阻塞主循环，遵循项目通用的 ticker + ctx.Done() select 模式。
// IGA worker.go 和 platform/outbox/worker.go 均采用此模式。
func (w *HealthProbeWorker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()

	w.logger.Debug("health-probe run loop entering")
	for {
		select {
		case <-ctx.Done():
			w.logger.Debug("health-probe run loop exiting", "reason", ctx.Err())
			return ctx.Err()
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				// 单轮 tick 失败不应该停止 worker — 打日志继续。
				w.logger.Error("health-probe tick failed", "error", err)
			}
		}
	}
}

// tick 执行一轮完整的半开探测：
//  1. 列出所有 state='open' 的 health 记录。
//  2. 跳过 OpenUntil 尚未到期的。
//  3. 到期 → EnterHalfOpen → Upsert。
//  4. 用 SSRF 保护 client HEAD provider.BaseURL。
//  5. StatusCode < 500 → RecordSuccess → Healthy；否则 RecordFailure → Open。
func (w *HealthProbeWorker) tick(ctx context.Context) error {
	now := w.Clock.Now()

	openHealths, err := w.Health.ListOpen(ctx)
	if err != nil {
		return apperrors.WrapInternalf(err, "list open provider health")
	}
	if len(openHealths) == 0 {
		return nil
	}

	var (
		halfOpenEntered int
		probed          int
		recovered       int
		reopened        int
	)

	for _, h := range openHealths {
		// 检查 ctx 是否已被取消 — 允许在长时间 tick 中提前退出。
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !h.IsOpenExpired(now) {
			continue
		}

		// Step 2.5: 取 provider — 在 EnterHalfOpen 之前拿是为了
		// 后续 RecordCircuitState / Outbox 事件能拿到 OrganizationID。
		provider, err := w.Providers.GetByID(ctx, h.ProviderID)
		if err != nil || provider == nil {
			w.logger.Warn("health-probe provider lookup failed; cannot probe",
				metaProviderID, h.ProviderID, "error", err)
			continue
		}
		if provider.BaseURL == "" {
			w.logger.Warn("health-probe provider has no base_url; skipping",
				metaProviderID, h.ProviderID)
			continue
		}

		// Step 3: Open → HalfOpen
		if err := h.EnterHalfOpen(now); err != nil {
			w.logger.Warn("health-probe EnterHalfOpen failed",
				metaProviderID, h.ProviderID, "error", err)
			continue
		}
		h.UpdatedAt = now
		if err := w.Health.Upsert(ctx, h); err != nil {
			w.logger.Warn("health-probe upsert half_open failed",
				metaProviderID, h.ProviderID, "error", err)
			continue
		}
		// Prometheus: circuit state 进入 half_open
		RecordCircuitState(provider.OrganizationID, h.ProviderID, h.State)
		halfOpenEntered++

		// Step 5: HEAD probe
		latencyMs, ok := w.probe(ctx, provider.BaseURL)
		probed++

		if ok {
			lat := latencyMs
			h.RecordSuccess(&lat)
			h.UpdatedAt = w.Clock.Now()
			if err := w.Health.Upsert(ctx, h); err != nil {
				w.logger.Warn("health-probe upsert success failed",
					metaProviderID, h.ProviderID, "error", err)
			} else {
				recovered++
				// Prometheus: circuit state → healthy
				RecordCircuitState(provider.OrganizationID, h.ProviderID, h.State)
				w.logger.Info("health-probe recovered provider",
					metaProviderID, h.ProviderID, metaLatencyMS, latencyMs)
				w.enqueueCircuitEvent(ctx, provider.OrganizationID, h.ProviderID,
					"ai.provider.circuit_closed", latencyMs, "probe_success")
			}
		} else {
			h.RecordFailure()
			h.UpdatedAt = w.Clock.Now()
			if err := w.Health.Upsert(ctx, h); err != nil {
				w.logger.Warn("health-probe upsert reopen failed",
					metaProviderID, h.ProviderID, "error", err)
			} else {
				reopened++
				// Prometheus: circuit state → open
				RecordCircuitState(provider.OrganizationID, h.ProviderID, h.State)
				w.logger.Info("health-probe provider reopened",
					metaProviderID, h.ProviderID, metaLatencyMS, latencyMs)
				w.enqueueCircuitEvent(ctx, provider.OrganizationID, h.ProviderID,
					"ai.provider.circuit_opened", latencyMs, "probe_failure")
			}
		}
	}

	w.logger.Debug("health-probe tick summary",
		"open_found", len(openHealths),
		"half_open_entered", halfOpenEntered,
		"probed", probed,
		"recovered", recovered,
		"reopened", reopened,
	)
	return nil
}

// probe 向 url 发送 HEAD 请求，返回 (latencyMs, ok)。
// ok=true 表示 StatusCode < 500 或 SSRF/网络层面允许连接；
// ok=false 表示 5xx、超时、或 SSRF 拒绝导致的失败。
func (w *HealthProbeWorker) probe(ctx context.Context, url string) (int, bool) {
	probeCtx, cancel := context.WithTimeout(ctx, defaultProbeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodHead, url, http.NoBody)
	if err != nil {
		w.logger.Debug("health-probe build request failed", "url", url, "error", err)
		return 0, false
	}
	req.Header.Set("User-Agent", "guliangiam-health-probe/1.0")

	start := time.Now()
	resp, err := w.HTTP.Do(req)
	latencyMs := int(time.Since(start).Milliseconds())
	if err != nil {
		w.logger.Debug("health-probe HTTP failed",
			"url", url, metaLatencyMS, latencyMs, "error", err)
		return latencyMs, false
	}
	resp.Body.Close()

	// 2xx/3xx/4xx 都算成功 — 4xx 多半是认证/权限问题，不代表服务挂了。
	// 只有 5xx 或 SSRF 拒绝才应该 reopen。
	ok := resp.StatusCode < http.StatusInternalServerError
	if !ok {
		w.logger.Debug("health-probe got 5xx",
			"url", url, metaStatus, resp.StatusCode, metaLatencyMS, latencyMs)
	}
	return latencyMs, ok
}

// enqueueCircuitEvent 把熔断器状态转换事件写到 Outbox Repo（Fail-Open）。
// - outbox 为 nil 时直接 return（开发环境友好）
// - payload 序列化成 JSON，含 organization_id / provider_id / latency_ms / trigger
// - 任何 error 都被吞掉，worker 的主循环不受影响
func (w *HealthProbeWorker) enqueueCircuitEvent(
	ctx context.Context, orgID, providerID, eventType string, latencyMs int, trigger string,
) {
	if w.Outbox == nil {
		return
	}
	payloadBytes, err := json.Marshal(map[string]any{
		metaOrganizationID: orgID,
		metaProviderID:     providerID,
		metaEventType:      eventType,
		metaLatencyMS:      latencyMs,
		metaTrigger:        trigger,
	})
	if err != nil {
		w.logger.Debug("health-probe outbox marshal failed", "error", err)
		return
	}
	id := newUUIDOrEmpty()
	var orgPtr *string
	if orgID != "" {
		orgPtr = &orgID
	}
	_ = w.Outbox.Enqueue(ctx, &domainoutbox.OutboxEvent{
		ID:             id,
		OrganizationID: orgPtr,
		AggregateType:  "llm_provider",
		AggregateID:    providerID,
		EventType:      eventType,
		EventVersion:   1,
		Payload:        payloadBytes,
		MaxAttempts:    3,
		AvailableAt:    w.Clock.Now(),
	})
}

// newUUIDOrEmpty 试图生成一个 UUID —— 失败时返回空字符串。
// 和 gateway_service.UUID.NewString 类似但 HealthProbeWorker 没有 UUID
// Generator 依赖；这里用 clock.Now() + nanoseconds 兜底即可。
func newUUIDOrEmpty() string {
	t := time.Now().UnixNano()
	return fmt.Sprintf("hp-%d", t)
}
