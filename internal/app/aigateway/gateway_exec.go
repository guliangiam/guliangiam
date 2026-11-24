package aigateway

import (
	"context"
	"log/slog"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
)

// Chat executes a non-streaming chat completion request.
func (s *GatewayService) Chat(ctx context.Context, apiKeyHash, alias string, req *domain.CanonicalRequest) (*domain.CanonicalResponse, error) {
	key, err := s.authenticate(ctx, apiKeyHash)
	if err != nil {
		return nil, err
	}
	ctx = bindKeyTenant(ctx, key)
	if err := req.Validate(); err != nil {
		return nil, err
	}

	res, err := s.execute(ctx, key, alias, req, false)
	if err != nil {
		return nil, err
	}
	return res.response, nil
}

// ChatStream executes a streaming chat request. The returned channel closes
// when the stream ends or ctx is cancelled.
//
// Streaming quota & metering (all handled in tee goroutine after stream close):
//   - RPM: 前置 +1（请求发起即计入）
//   - TPM / cost: 累积 UsageMetadata 后一次性调用 incrementCounters
//   - UsageLog: 成功（拿到 usage）or 失败（ctx cancel / 无 usage）
//   - Health: 成功 Upsert Healthy / 失败 Upsert Degraded
func (s *GatewayService) ChatStream(ctx context.Context, apiKeyHash, alias string, req *domain.CanonicalRequest) (<-chan *domain.CanonicalStreamEvent, error) {
	key, err := s.authenticate(ctx, apiKeyHash)
	if err != nil {
		return nil, err
	}
	ctx = bindKeyTenant(ctx, key)
	if err := req.Validate(); err != nil {
		return nil, err
	}

	upstream, resolution, err := s.route(ctx, key.OrganizationID, alias, req)
	if err != nil {
		return nil, err
	}

	// 熔断状态门禁
	if resolution.Health != nil && resolution.Health.State == domain.HealthStateOpen {
		s.recordUsage(ctx, resolution, nil, domain.UsageStatusFailed,
			apperrors.NewUnavailablef("circuit breaker open for provider %s", resolution.Provider.ID))
		return nil, apperrors.NewUnavailablef("provider %s circuit open", resolution.Provider.ID)
	}

	// 前置配额检查（RPM + QuotaPolicy）
	if err := s.enforceQuota(ctx, key, alias, req); err != nil {
		s.recordUsage(ctx, resolution, nil, domain.UsageStatusFailed, err)
		return nil, err
	}

	upstreamCh, err := upstream.ChatStream(ctx, req)
	if err != nil {
		// The adapter already returns credential-free errors; recordUsage
		// persists only a normalized code. The returned wrapper carries the
		// code (not the raw error) so 5xx bodies stay generic.
		s.recordUsage(ctx, resolution, nil, domain.UsageStatusFailed, err)
		return nil, apperrors.NewUnavailablef("upstream stream failed: %s", normalizeUsageErrorCode(err))
	}

	// ── Tee goroutine: 转发事件给 handler + 累积 usage + 流结束闭环 + tool_use 自动执行 ──
	outCh := make(chan *domain.CanonicalStreamEvent, 32)
	streamStart := time.Now()
	streamLabels := buildGatewayLabels(key, resolution, "stream")
	recover.SafeGo("aigateway-stream-tee", func() { s.streamTee(ctx, upstreamCh, outCh, key, resolution, req, streamStart, streamLabels) })

	return outCh, nil
}

// streamTee forwards upstream events, accumulates usage, closes the stream
// accounting and executes accumulated tool_use blocks. Extracted from
// ChatStream so the latter stays below the cyclomatic-complexity gate.
func (s *GatewayService) streamTee(
	ctx context.Context,
	upstreamCh <-chan *domain.CanonicalStreamEvent,
	outCh chan<- *domain.CanonicalStreamEvent,
	key *domain.LlmApiKey,
	resolution *domain.RouteResolution,
	req *domain.CanonicalRequest,
	streamStart time.Time,
	streamLabels gatewayLabels,
) {
	defer close(outCh)

	var (
		inputTokens   int
		outputTokens  int
		totalTokens   int
		gotUsage      bool
		streamFailure error
	)

	// tool_use 增量聚合：ToolCallID → 累积的 ContentBlock
	// 流式场景下 arguments 可能分多个 delta 到达，需要 merge map
	toolUses := map[string]*domain.CanonicalContentBlock{}

	for ev := range upstreamCh {
		if ev.Type == domain.StreamEventError {
			code := apperrors.CodeUnavailable
			if ev.Error != nil && ev.Error.Code != "" {
				code = apperrors.Code(ev.Error.Code)
			}
			// Fixed message: the adapter already sanitized the event, and the
			// usage log only persists the normalized code.
			streamFailure = apperrors.New(code, "upstream stream interrupted")
		}
		// 累积 UsageMetadata（上游通常在倒数第 2 个 message_delta 事件携带）
		if ev.UsageMetadata != nil {
			if v, ok := ev.UsageMetadata["input_tokens"].(int); ok {
				inputTokens = v
			}
			if v, ok := ev.UsageMetadata["output_tokens"].(int); ok {
				outputTokens = v
			}
			if v, ok := ev.UsageMetadata["total_tokens"].(int); ok {
				totalTokens = v
			}
			if _, ok := ev.UsageMetadata["input_tokens"]; ok {
				gotUsage = true
			}
		}

		// 拦截 ToolCallDelta：聚合增量 tool_use 块
		if ev.ToolCallDelta != nil {
			delta := ev.ToolCallDelta
			existing, ok := toolUses[delta.ToolCallID]
			if !ok {
				cp := *delta
				toolUses[delta.ToolCallID] = &cp
			} else {
				// merge：ToolName / ToolCallID 直接覆盖（最后一次为准），
				// ToolInput 是 map 增量合并
				if delta.ToolName != "" {
					existing.ToolName = delta.ToolName
				}
				if delta.ToolCallID != "" {
					existing.ToolCallID = delta.ToolCallID
				}
				for k, v := range delta.ToolInput {
					existing.ToolInput[k] = v
				}
			}
		}

		// 转发给 handler；ctx.Done()（客户端断连）不影响 tee 内部继续消费
		// 但避免 goroutine 在 outCh 满时永久阻塞
		select {
		case outCh <- ev:
		case <-ctx.Done():
			// handler 已退出，继续 drain upstreamCh 做 usage 累积后退出
		}
	}

	// upstreamCh closed → 流结束或 ctx cancel 导致上游关闭
	// 执行累积的 tool_use blocks（Fail-Open：无 ToolExecutor 或无 tool_use → 跳过）
	if len(toolUses) > 0 && s.ToolExecutor != nil && ctx.Err() == nil {
		collected := make([]domain.CanonicalContentBlock, 0, len(toolUses))
		for _, tb := range toolUses {
			collected = append(collected, *tb)
		}
		// ExecuteTools 自动从 req.Tools 查 MCPBinding；Fail-Open：找不到 → 原样返回
		results := ExecuteTools(ctx, s.ToolExecutor, collected, req.Tools,
			key.OrganizationID, resolution.Provider.ID)
		for _, r := range results {
			if r.Type != domain.ContentBlockToolResult {
				continue
			}
			// 把 tool_result 包装成 stream event 回灌给 handler
			select {
			case outCh <- &domain.CanonicalStreamEvent{
				Type:         fieldToolResult,
				ContentDelta: &r,
			}:
			case <-ctx.Done():
				// handler 已断连 → 不再发 tool_result
			}
		}
	} else if len(toolUses) > 0 && s.ToolExecutor == nil {
		slog.DebugContext(ctx, "aigateway stream: tool_use blocks accumulated but ToolExecutor nil, leaving to caller",
			"count", len(toolUses))
	}
	streamLatency := time.Since(streamStart).Seconds()
	// 计量/健康闭环不能随客户端断连一起被取消：WithoutCancel 保留租户作用域
	// 等 value，仅剥离 cancel 信号，再加独立超时防止后台落库无限挂起。
	acctCtx, cancelAcct := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancelAcct()
	if ctx.Err() == nil && streamFailure == nil {
		// 成功闭环：写 UsageLog + Health + TPM/cost 计数器
		RecordGatewayCall(streamLabels, "success")
		RecordGatewayLatency(streamLabels, "success", streamLatency)
		if gotUsage {
			resp := &domain.CanonicalResponse{
				InputTokens:  inputTokens,
				OutputTokens: outputTokens,
				TotalTokens:  totalTokens,
			}
			s.recordUsage(acctCtx, resolution, resp, domain.UsageStatusSucceeded, nil)
			if err := s.updateHealthOnSuccess(acctCtx, resolution); err != nil {
				s.logHealthPersistError(acctCtx, resolution.Provider, err)
			}
			s.incrementCounters(acctCtx, key, resolution, resp)
			RecordGatewayTokens(key, resolution, "stream", resp)
			RecordGatewayCost(acctCtx, key, resolution, "stream", resp, s.Prices)
		} else {
			// 上游没发 usage metadata（少见），但流正常结束
			s.recordUsage(acctCtx, resolution, nil, domain.UsageStatusSucceeded, nil)
			if err := s.updateHealthOnSuccess(acctCtx, resolution); err != nil {
				s.logHealthPersistError(acctCtx, resolution.Provider, err)
			}
		}
	} else {
		// 客户端提前断连 / ctx cancel → 仍算部分完成但计为失败；上游读取中断
		// 同样走失败分支，不能在计量上把截断流记成成功。
		failureErr := ctx.Err()
		if failureErr == nil {
			failureErr = streamFailure
		}
		s.recordUsage(acctCtx, resolution, nil, domain.UsageStatusFailed, failureErr)
		if err := s.updateHealthOnFailure(acctCtx, resolution); err != nil {
			s.logHealthPersistError(acctCtx, resolution.Provider, err)
		}
		RecordGatewayCall(streamLabels, "failure")
		RecordGatewayLatency(streamLabels, "failure", streamLatency)
	}
}

// Embedding executes an embedding request. The flow mirrors Chat.
func (s *GatewayService) Embedding(ctx context.Context, apiKeyHash, alias string, req *domain.CanonicalRequest) (*domain.CanonicalResponse, error) {
	key, err := s.authenticate(ctx, apiKeyHash)
	if err != nil {
		return nil, err
	}
	ctx = bindKeyTenant(ctx, key)
	if err := req.Validate(); err != nil {
		return nil, err
	}

	res, err := s.execute(ctx, key, alias, req, true)
	if err != nil {
		return nil, err
	}
	return res.response, nil
}

// ─── Internal orchestration ────────────────────────────────────────────

type executionResult struct {
	response   *domain.CanonicalResponse
	resolution *domain.RouteResolution
}

// execute 封装 Resolve → QuotaCheck → UpstreamAdapter.Call → RecordUsage → HealthUpsert → Meter.Incr 的通用路径。
// key 参数由上游 Chat/ChatStream/Embedding 调用方传入——authenticate() 已完成状态/过期检查，此处只需用它做配额查询和计数器 key 构造。
func (s *GatewayService) execute(ctx context.Context, key *domain.LlmApiKey, alias string, req *domain.CanonicalRequest, isEmbedding bool) (*executionResult, error) {
	upstream, resolution, err := s.route(ctx, key.OrganizationID, alias, req)
	if err != nil {
		return nil, err
	}

	// 熔断状态门禁（RouterEngine.Resolve 已过滤 Open provider，这里防御性再兜一次）
	if resolution.Health != nil && resolution.Health.State == domain.HealthStateOpen {
		s.recordUsage(ctx, resolution, nil, domain.UsageStatusFailed,
			apperrors.NewUnavailablef("circuit breaker open for provider %s", resolution.Provider.ID))
		return nil, apperrors.NewUnavailablef("provider %s circuit open", resolution.Provider.ID)
	}

	// ── 前置配额检查（RPM / TPM 估算 / QuotaPolicy / MonthlyBudget）──
	if err := s.enforceQuota(ctx, key, alias, req); err != nil {
		s.recordUsage(ctx, resolution, nil, domain.UsageStatusFailed, err)
		return nil, err
	}

	callType := "chat"
	if isEmbedding {
		callType = "embedding"
	}

	start := time.Now()

	var (
		resp    *domain.CanonicalResponse
		errCall error
	)
	if isEmbedding {
		resp, errCall = upstream.Embedding(ctx, req)
	} else {
		resp, errCall = upstream.Chat(ctx, req)
	}

	// ── Prometheus: latency — 成功失败都记录 ──
	latencySec := time.Since(start).Seconds()
	labels := buildGatewayLabels(key, resolution, callType)

	if errCall != nil {
		// errCall crosses the adapter boundary already sanitized; recordUsage
		// additionally persists only a normalized code. The wrapper returned to
		// the handler never concatenates the raw error text.
		s.recordUsage(ctx, resolution, nil, domain.UsageStatusFailed, errCall)
		if err := s.updateHealthOnFailure(ctx, resolution); err != nil {
			s.logHealthPersistError(ctx, resolution.Provider, err)
		}
		RecordGatewayCall(labels, "failure")
		RecordGatewayLatency(labels, "failure", latencySec)
		return nil, apperrors.NewUnavailablef("upstream call failed: %s", normalizeUsageErrorCode(errCall))
	}
	if resp == nil {
		s.recordUsage(ctx, resolution, nil, domain.UsageStatusFailed,
			apperrors.NewInternal("upstream returned nil response"))
		if err := s.updateHealthOnFailure(ctx, resolution); err != nil {
			s.logHealthPersistError(ctx, resolution.Provider, err)
		}
		RecordGatewayCall(labels, "failure")
		RecordGatewayLatency(labels, "failure", latencySec)
		return nil, apperrors.NewInternal("upstream returned nil response")
	}

	// ── Prometheus: 成功 call + latency ──
	RecordGatewayCall(labels, "success")
	RecordGatewayLatency(labels, "success", latencySec)

	// ── 工具调用自动执行（Fail-Open：ToolExecutor == nil 时跳过）──
	if s.ToolExecutor != nil && HasToolUseBlocks(resp.Content) {
		resp.Content = ExecuteTools(ctx, s.ToolExecutor, resp.Content, req.Tools,
			key.OrganizationID, resolution.Provider.ID)
	}

	// ── Prometheus: tokens + cost ──
	RecordGatewayTokens(key, resolution, callType, resp)
	RecordGatewayCost(ctx, key, resolution, callType, resp, s.Prices)

	s.recordUsage(ctx, resolution, resp, domain.UsageStatusSucceeded, nil)
	if err := s.updateHealthOnSuccess(ctx, resolution); err != nil {
		s.logHealthPersistError(ctx, resolution.Provider, err)
	}

	// ── 成功后递增配额计数器（RPM + 实际 token + 月度成本）──
	s.incrementCounters(ctx, key, resolution, resp)

	return &executionResult{response: resp, resolution: resolution}, nil
}
