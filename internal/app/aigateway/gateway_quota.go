package aigateway

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ─── Quota enforcement helpers ─────────────────────────────────────────

// enforceQuota 在 upstream 调用前检查所有适用的配额维度：
//
//  1. API key 级别的 RPM 上限（key.RPMLimit）—— TryIncr 原子预扣 +1，
//     超限直接 429；预扣成功后 incrementCounters 不再重复 Incr RPM
//  2. API key 级别的 TPM 上限（key.TPMLimit，基于请求内容粗估）—— Get 检查
//  3. API key 级别的月度预算（key.MonthlyBudget，货币单位）—— 与
//     incrementCounters 的 cost 计数器（分）同 key 比对，超预算 429
//  4. QuotaPoliciesRepo 里 org 维度的策略 —— 只匹配 status=active 且
//     scope/scopeRef 命中本 key/alias 的策略（RPM → TryIncr 原子扣，
//     TPM/Cost → Get 检查）
//
// Meter == nil 时直接返回 nil（Fail-Open）。
// Meter 查询失败时也返回 nil——不因为 Redis 临时抖动阻断主链路。
func (s *GatewayService) enforceQuota(ctx context.Context, key *domain.LlmApiKey, alias string, req *domain.CanonicalRequest) error {
	if s.Meter == nil || key == nil {
		return nil // 配额治理未启用 → 直接放行
	}

	nowMin := nowMinute()

	// ── 1. API key RPM（原子预扣）──
	if key.RPMLimit != nil {
		rpmKey := fmt.Sprintf("rpm:api_key:%s:%d", key.ID, nowMin)
		allowed, cur, _ := s.Meter.TryIncr(ctx, rpmKey, 1, int64(*key.RPMLimit), time.Minute)
		if !allowed {
			s.enqueueQuotaExceeded(ctx, key.OrganizationID, key.ID, "rpm", cur, *key.RPMLimit)
			RecordQuotaExceeded(key.OrganizationID, key.ID, "api_key", "rpm")
			return apperrors.NewTooManyRequestsf("RPM limit exceeded: %d/%d", cur, *key.RPMLimit)
		}
	}

	// ── 2. API key TPM（粗估，仍用 Get 检查）──
	// 注：TPM 粗估不适合原子扣（estimateInputTokens 与实际 tokens 差异大），
	// 这里只做前置屏障拒绝明显会超的请求，真正计数由 incrementCounters post-call 完成。
	if key.TPMLimit != nil {
		estimatedInputTokens := estimateInputTokens(req)
		tpmKey := fmt.Sprintf("tpm:api_key:%s:%d", key.ID, nowMin)
		cur, _ := s.Meter.Get(ctx, tpmKey)
		if cur+estimatedInputTokens >= int64(*key.TPMLimit) {
			s.enqueueQuotaExceeded(ctx, key.OrganizationID, key.ID, "tpm", cur+estimatedInputTokens, *key.TPMLimit)
			RecordQuotaExceeded(key.OrganizationID, key.ID, "api_key", "tpm")
			return apperrors.NewTooManyRequestsf("TPM limit exceeded (estimated): %d/%d", cur+estimatedInputTokens, *key.TPMLimit)
		}
	}

	// ── 3. API key 月度预算（BE-C-04）──
	// incrementCounters 以「分」为单位累加 cost:api_key:{id}:{YYYY-MM}；
	// MonthlyBudget 是货币单位，×100 后同口径比对。<=0 视为不设预算。
	if key.MonthlyBudget != nil && *key.MonthlyBudget > 0 {
		monthKey := fmt.Sprintf("cost:api_key:%s:%s", key.ID, s.Clock.Now().Format("2006-01"))
		cur, _ := s.Meter.Get(ctx, monthKey)
		budgetCents := int64(*key.MonthlyBudget * 100)
		if cur >= budgetCents {
			s.enqueueQuotaExceeded(ctx, key.OrganizationID, key.ID, "cost", cur, int(budgetCents))
			RecordQuotaExceeded(key.OrganizationID, key.ID, "api_key", "cost")
			return apperrors.NewTooManyRequestsf("monthly budget exceeded: %d/%d cents", cur, budgetCents)
		}
	}

	// ── 4. QuotaPolicy 维度 ──
	if s.Quotas != nil {
		policies, err := s.listEnforceablePolicies(ctx, key.OrganizationID)
		if err == nil {
			for _, p := range policies {
				if !policyApplies(p, key, alias) {
					continue
				}
				scopeKey := buildQuotaKey(p, s.Clock.Now())
				// RPM 类型的 policy 也用 TryIncr 原子扣；其他维度（TPM/cost）用 Get 检查
				var errStr string
				if p.LimitType == domain.LimitTypeRPM {
					allowed, cur, _ := s.Meter.TryIncr(ctx, scopeKey, 1, int64(p.LimitValue), time.Minute)
					if !allowed {
						errStr = fmt.Sprintf("rpm policy limit exceeded: %d/%d", cur, int64(p.LimitValue))
					}
				} else {
					cur, _ := s.Meter.Get(ctx, scopeKey)
					if err := p.CheckLimit(float64(cur)); err != nil {
						errStr = err.Error()
					}
				}
				if errStr != "" {
					s.enqueueOutboxEvent(ctx, "llm_quota", p.ID, "ai.quota.exceeded", key.OrganizationID, s.Clock.Now(), map[string]any{
						metaOrganizationID: key.OrganizationID,
						"api_key_id":       key.ID,
						"quota_policy_id":  p.ID,
						metaQuotaScope:     p.Scope,
						"quota_limit_type": p.LimitType,
						"limit_value":      p.LimitValue,
						"error":            errStr,
					})
					RecordQuotaExceeded(key.OrganizationID, key.ID, p.Scope, p.LimitType)
					return apperrors.NewTooManyRequestsf("quota policy: %s", errStr)
				}
			}
		}
	}

	return nil
}

// listEnforceablePolicies 读取 org 的配额策略。仓储若实现 ListActive
// （只取 status='active'）则优先使用；否则回退 List 并由 policyApplies
// 在应用层过滤 status —— 两条路径的 enforcement 语义一致。
func (s *GatewayService) listEnforceablePolicies(ctx context.Context, orgID string) ([]*domain.LlmQuotaPolicy, error) {
	if la, ok := s.Quotas.(domain.ActiveQuotaPoliciesLister); ok {
		return la.ListActive(ctx, orgID)
	}
	return s.Quotas.List(ctx, orgID)
}

// policyApplies 判定一条配额策略是否命中当前请求（BE-C-03）：
//   - 必须是本组织、status=active 的策略（disabled 策略不 enforce，
//     RPM 的 TryIncr 路径同样不得绕过该检查）
//   - scope=organization → 对本组织所有请求生效
//   - scope=api_key → scopeRef 必须等于 key.ID；scopeRef 为空视为
//     组织内全部 key（与管理面「留空=全部」语义一致）
//   - scope=alias → scopeRef 必须等于请求的 alias 名；留空同上
func policyApplies(p *domain.LlmQuotaPolicy, key *domain.LlmApiKey, alias string) bool {
	if p == nil || p.Status != domain.QuotaStatusActive {
		return false
	}
	if p.OrganizationID != key.OrganizationID {
		return false
	}
	ref := ""
	if p.ScopeRef != nil {
		ref = *p.ScopeRef
	}
	switch p.Scope {
	case domain.QuotaScopeOrganization:
		return true
	case domain.QuotaScopeAPIKey:
		return ref == "" || ref == key.ID
	case domain.QuotaScopeAlias:
		return ref == "" || ref == alias
	default:
		return false
	}
}

// incrementCounters 在 upstream 成功返回后递增 post-call 才能知道的计数器：
//   - TPM + actual tokens（分钟窗口）
//   - Monthly cost + cost_amount_cents（月窗口）
//
// RPM 已经在 enforceQuota 里用 TryIncr 原子预扣了（准入即扣），这里不再重复 Incr。
//
// 失败都是 best-effort：Meter 错误被静默吞掉——已向用户返回成功响应，
// 配额计数器的暂时不精确比回滚一次成功调用的代价小得多。
func (s *GatewayService) incrementCounters(ctx context.Context, key *domain.LlmApiKey, resolution *domain.RouteResolution, resp *domain.CanonicalResponse) {
	if s.Meter == nil || key == nil || resp == nil {
		return
	}

	nowMin := nowMinute()

	// ── TPM + actual tokens（实际消耗，只有 upstream 返回后才知道）──
	totalTokens := int64(resp.InputTokens + resp.OutputTokens)
	if totalTokens > 0 {
		tpmKey := fmt.Sprintf("tpm:api_key:%s:%d", key.ID, nowMin)
		_, _ = s.Meter.Incr(ctx, tpmKey, totalTokens, time.Minute)
	}

	// ── Monthly cost + cost_cents ──
	// 与 recordUsage 同样的成本计算逻辑，避免 CanonicalResponse 引入 CostAmount 字段。
	if resolution != nil && resolution.Model != nil && s.Prices != nil {
		if pv, _ := s.Prices.GetByModel(ctx, resolution.Model.ID); pv != nil {
			costCents := int64(pv.CalculateCost(resp.InputTokens, resp.OutputTokens) * 100)
			if costCents > 0 {
				now := s.Clock.Now()
				monthKey := fmt.Sprintf("cost:api_key:%s:%s", key.ID, now.Format("2006-01"))
				// TTL 覆盖到下个自然月开始 + 1 天缓冲，避免固定 30 天在
				// 31 天月份提前过期导致预算计数清零（BE-C-16）。
				nextMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, 1, 1)
				ttl := nextMonth.Sub(now)
				_, _ = s.Meter.Incr(ctx, monthKey, costCents, ttl)
			}
		}
	}
}

// estimateInputTokens 用 4 chars ≈ 1 token 的粗估比估算请求 token。
// 只用于 enforceQuota 的前置检查（拒绝明显会超 TPM 的请求）；
// 真正的 TPM 计数用 upstream 返回的 actual tokens。
func estimateInputTokens(req *domain.CanonicalRequest) int64 {
	if req == nil {
		return 0
	}
	var chars int
	for _, m := range req.Messages {
		for _, b := range m.Content {
			if b.Type == domain.ContentBlockText {
				chars += len(b.Text)
			}
		}
	}
	// 额外加上 system prompt 长度
	chars += len(req.System)
	// 粗略：4 chars ≈ 1 token，向下取整让检查略宽松
	return int64(chars / 4)
}

// nowMinute 返回当前时间的"分钟桶"——Unix 秒 / 60。
// 同一分钟内的所有请求映射到同一个 Redis key。
func nowMinute() int64 {
	return time.Now().Unix() / 60
}

// buildQuotaKey 为 LlmQuotaPolicy 构造 Redis counter key。
// 格式：{limitType}:{scope}[:{scopeRef}]:{window}
//   - RPM/TPM 用 minute bucket，Cost 用 YYYY-MM
func buildQuotaKey(p *domain.LlmQuotaPolicy, now time.Time) string {
	scope := p.Scope
	if p.ScopeRef != nil {
		scope += ":" + *p.ScopeRef
	}
	window := now.Format("2006-01")
	if p.LimitType == domain.LimitTypeRPM || p.LimitType == domain.LimitTypeTPM {
		window = fmt.Sprintf("%d", now.Unix()/60)
	}
	return fmt.Sprintf("%s:%s:%s", p.LimitType, scope, window)
}

// newProviderHealth 为尚无 health 行的 provider 分配一条新记录。
// llm_provider_health.id 是 UUID 列，因此 id 必须来自平台 UUID 生成器，
// 不能用字符串前缀拼接（否则 Postgres 插入会因类型转换失败而丢行）。
func (s *GatewayService) newProviderHealth(provider *domain.LlmProvider) (*domain.ProviderHealth, error) {
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "generate provider health id for provider %s", provider.ID)
	}
	return &domain.ProviderHealth{
		ID:         id,
		ProviderID: provider.ID,
		State:      domain.HealthStateHealthy,
	}, nil
}

// updateHealthOnSuccess 上游成功后更新 Health 状态（清零 ConsecutiveFailures）。
// 返回 Upsert 或 ID 生成失败；调用方记录日志但不得让该失败影响已完成的
// 数据面响应（health 是旁路状态，不是计费/计量的前置条件）。
func (s *GatewayService) updateHealthOnSuccess(ctx context.Context, resolution *domain.RouteResolution) error {
	if s.Health == nil || resolution == nil || resolution.Provider == nil {
		return nil
	}
	// 构造/获取 health record
	h := resolution.Health
	if h == nil {
		var err error
		h, err = s.newProviderHealth(resolution.Provider)
		if err != nil {
			return err
		}
	}
	h.RecordSuccess(nil)
	var persistErr error
	if err := s.Health.Upsert(ctx, h); err != nil {
		persistErr = apperrors.WrapInternalf(err, "persist provider health for provider %s", h.ProviderID)
	}

	// ── Outbox: ai.provider.circuit_closed（数据面成功恢复）──
	s.enqueueOutboxEvent(ctx, "llm_provider", h.ProviderID, "ai.provider.circuit_closed",
		resolution.Provider.OrganizationID, s.Clock.Now(), map[string]any{
			metaOrganizationID: resolution.Provider.OrganizationID,
			metaProviderID:     h.ProviderID,
			metaEventType:      "ai.provider.circuit_closed",
			metaLatencyMS:      nil,
			metaTrigger:        "upstream_success",
		})

	// ── Prometheus: circuit state gauge ──
	RecordCircuitState(resolution.Provider.OrganizationID, h.ProviderID, h.State)
	return persistErr
}

// updateHealthOnFailure 上游失败后更新 Health 状态（递增 ConsecutiveFailures → 可能触发 Open）。
// 错误语义与 updateHealthOnSuccess 一致：向上返回，由调用方记录。
func (s *GatewayService) updateHealthOnFailure(ctx context.Context, resolution *domain.RouteResolution) error {
	if s.Health == nil || resolution == nil || resolution.Provider == nil {
		return nil
	}
	h := resolution.Health
	if h == nil {
		var err error
		h, err = s.newProviderHealth(resolution.Provider)
		if err != nil {
			return err
		}
	}
	h.RecordFailure()
	var persistErr error
	if err := s.Health.Upsert(ctx, h); err != nil {
		persistErr = apperrors.WrapInternalf(err, "persist provider health for provider %s", h.ProviderID)
	}

	// ── Outbox: ai.provider.circuit_opened（数据面失败触发熔断）──
	s.enqueueOutboxEvent(ctx, "llm_provider", h.ProviderID, "ai.provider.circuit_opened",
		resolution.Provider.OrganizationID, s.Clock.Now(), map[string]any{
			metaOrganizationID: resolution.Provider.OrganizationID,
			metaProviderID:     h.ProviderID,
			metaEventType:      "ai.provider.circuit_opened",
			metaLatencyMS:      nil,
			metaTrigger:        "upstream_failure",
		})

	// ── Prometheus: circuit state gauge ──
	RecordCircuitState(resolution.Provider.OrganizationID, h.ProviderID, h.State)
	return persistErr
}

// logHealthPersistError 记录 health 行持久化失败。键名使用 snake_case，
// 只带 provider/organization 标识与错误分类，不记录任何凭据。
func (s *GatewayService) logHealthPersistError(ctx context.Context, provider *domain.LlmProvider, err error) {
	slog.WarnContext(ctx, "aigateway provider health upsert failed",
		metaProviderID, provider.ID,
		metaOrganizationID, provider.OrganizationID,
		"error_class", string(apperrors.FromError(err).Code),
		"error", err)
}

// route 把 alias 解析为具体 RouteResolution 并拿到 UpstreamAdapter。
// orgID 从 authenticate() 拿到的 API key 来 —— 用于精确匹配本组织的 alias。
func (s *GatewayService) route(ctx context.Context, orgID, alias string, req *domain.CanonicalRequest) (domain.UpstreamAdapter, *domain.RouteResolution, error) {
	if s.Routers == nil {
		return nil, nil, apperrors.NewUnavailable("router engine not configured")
	}
	resolution, err := s.Routers.Resolve(ctx, orgID, alias, req)
	if err != nil {
		return nil, nil, apperrors.WrapInternalf(err, "resolve alias %s", alias)
	}
	if resolution == nil || resolution.Provider == nil {
		return nil, nil, apperrors.NewUnavailablef("no route for alias %s", alias)
	}
	if s.Upstreams == nil {
		return nil, nil, apperrors.NewUnavailable("upstream registry not configured")
	}
	// BuildAdapter 用 provider 的真实 baseURL/apiKey/timeout 动态构造 adapter
	// —— 而不是从 factory 里拿一个空参数单例。
	upstream, err := s.Upstreams.BuildAdapter(resolution.Provider, resolution.ProtocolFace)
	if err != nil {
		return nil, nil, apperrors.NewUnavailablef("build upstream for protocol %s: %v", resolution.ProtocolFace, err)
	}
	return upstream, resolution, nil
}

// recordUsage 把一次调用写成 UsageLog 行。失败也写 — 计量口径统一。
func (s *GatewayService) recordUsage(ctx context.Context, res *domain.RouteResolution, resp *domain.CanonicalResponse, status string, callErr error) {
	if s.UsageLogs == nil {
		return // 没实现就跳过，不阻断主链路
	}

	// 计算 token
	var inputTokens, outputTokens, totalTokens int
	var cost float64
	if resp != nil {
		inputTokens = resp.InputTokens
		outputTokens = resp.OutputTokens
		totalTokens = resp.TotalTokens
		if res.Model != nil && s.Prices != nil {
			if pv, _ := s.Prices.GetByModel(ctx, res.Model.ID); pv != nil {
				cost = pv.CalculateCost(inputTokens, outputTokens)
			}
		}
	}

	now := s.Clock.Now()
	uuidStr, err := s.UUID.NewString()
	if err != nil {
		// 无合法 ID 无法落库（uuid 列），记录告警后跳过本条计量；
		// 不阻断已完成的响应，但绝不静默写入空 ID 行。
		slog.ErrorContext(ctx, "aigateway usage log id generation failed, skipping usage record",
			metaOrganizationID, res.Provider.OrganizationID, "error", err)
		return
	}

	log := &domain.UsageLog{
		ID:             uuidStr,
		OrganizationID: res.Provider.OrganizationID,
		AliasID:        aliasFromResolution(res),
		ProviderID:     res.Provider.ID,
		ModelID:        res.Model.ID,
		ProtocolFace:   res.ProtocolFace,
		AttemptNo:      1,
		InputTokens:    inputTokens,
		OutputTokens:   outputTokens,
		TotalTokens:    totalTokens,
		CostAmount:     cost,
		Status:         status,
		CreatedAt:      now,
	}
	if callErr != nil {
		// error_code is a normalized code, never callErr.Error(): upstream
		// transport errors may embed the full URL (Gemini's ?key=) and upstream
		// bodies may echo credentials. The same code is written to the outbox
		// payload below and later to metering_events.metadata.
		code := normalizeUsageErrorCode(callErr)
		log.ErrorCode = &code
	}
	if err := s.UsageLogs.Create(ctx, log); err != nil {
		// Usage log 落库失败不回滚主响应 — Outbox 有自身重试机制；
		// 但必须留下告警，否则计量丢失完全不可见（BE-C-16）。
		slog.ErrorContext(ctx, "aigateway usage log persist failed",
			"usage_log_id", log.ID,
			metaOrganizationID, log.OrganizationID,
			"error_class", string(apperrors.FromError(err).Code),
			"error", err)
	}

	// ── Outbox: ai.usage.recorded ──
	s.enqueueOutboxEvent(ctx, "llm_usage", log.ID, "ai.usage.recorded", log.OrganizationID, now, map[string]any{
		metaOrganizationID: log.OrganizationID,
		metaProviderID:     log.ProviderID,
		"model_id":         log.ModelID,
		"alias_id":         log.AliasID,
		metaProtocolFace:   log.ProtocolFace,
		"input_tokens":     log.InputTokens,
		"output_tokens":    log.OutputTokens,
		"total_tokens":     log.TotalTokens,
		"cost_amount":      log.CostAmount,
		metaStatus:         log.Status,
		"error_code":       log.ErrorCode,
	})
}
