// =============================================================================
// AI Gateway — Outbox Delivery Handlers
// =============================================================================
//
// 平台 Outbox Worker 投递 AI Gateway 审计事件的 Handler 集。
// 三个 aggregate_type 对应同一个 handler（meteringHandler）：
//
//	llm_provider — ai.provider.circuit_opened / ai.provider.circuit_closed
//	llm_quota    — ai.quota.exceeded
//	llm_usage    — ai.usage.recorded
//
// 每条 outbox 事件 → 两个 sink：
//  1. llm_metering_events（AI Gateway 审计表）— 所有 aggregate_type 都写
//  2. metering_events（Commerce billing 计费表）— 仅 llm_usage + ai.usage.recorded
//
// DB 写入失败时记录结构化日志并返回错误，让 Outbox 按退避策略重试；
// 各 sink 使用稳定幂等键，避免重试产生重复审计或计费事实。
//
// 依赖注入（全部 nil-safe）:
//
//	appgateway.NewOutboxHandlers(auditRepo, logger, billingRepo, idGen)
//	  auditRepo — domain.MeteringEventRepo (llm_metering_events)
//	  billingRepo — metering.Repository (metering_events, Commerce billing)
//	  idGen — platform uuid.Generator (metering_events.id 是 UUID 列)
package aigateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	metering "github.com/guliangiam/guliangiam/internal/domain/metering"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// NewOutboxHandlers 返回 AI Gateway 三个 aggregate_type 的 Handler map。
//
// auditRepo 写入 llm_metering_events 审计表（所有 aggregate_type）。
// billingRepo 写入 metering_events 计费表（仅 llm_usage + ai.usage.recorded）。
// 两者都为 nil 时 handler 退化为日志-only。
// idGen 为 nil 时回退到平台 V7 生成器（无状态，行为不变）。
func NewOutboxHandlers(
	auditRepo domain.MeteringEventRepo,
	logger *slog.Logger,
	billingRepo metering.Repository,
	idGen platformuuid.Generator,
) map[string]platformoutbox.DeliveryHandler {
	if logger == nil {
		logger = slog.Default()
	}
	if idGen == nil {
		idGen = platformuuid.V7Generator{}
	}
	logger = logger.With("component", "ai-gateway-outbox")
	h := &meteringHandler{
		log:         logger,
		auditRepo:   auditRepo,   // nil-safe
		billingRepo: billingRepo, // nil-safe
		idGen:       idGen,
	}
	return map[string]platformoutbox.DeliveryHandler{
		"llm_provider": h,
		"llm_quota":    h,
		"llm_usage":    h,
	}
}

// meteringHandler 是三种 aggregate_type 共用的 Handler。
//
//  1. 写 llm_metering_events（审计表，所有 aggregate_type）
//  2. 写 metering_events（计费表，仅 llm_usage + ai.usage.recorded）
//     → 从 usage payload 提取 tokens + cost → 可计费事实
type meteringHandler struct {
	log         *slog.Logger
	auditRepo   domain.MeteringEventRepo // optional; nil = log only
	billingRepo metering.Repository      // optional; nil = skip billing bridge
	idGen       platformuuid.Generator   // metering_events.id is a UUID column
}

func (h *meteringHandler) Handle(ctx context.Context, item *platformoutbox.OutboxItem) error {
	// Parse payload once — for both extraction + raw JSON save
	var payload map[string]any
	var payloadBytes []byte
	if len(item.Payload) > 0 {
		payloadBytes = item.Payload
		_ = json.Unmarshal(item.Payload, &payload)
	}

	// ── 1. Audit sink: llm_metering_events ────────────────────────────
	if err := h.writeAuditEvent(ctx, item, payload, payloadBytes); err != nil {
		return err
	}

	// ── 2. Billing sink: metering_events (only usage.recorded) ────────
	if item.AggregateType == "llm_usage" && item.EventType == "ai.usage.recorded" {
		if err := h.writeBillingEvents(ctx, item, payload, payloadBytes); err != nil {
			return err
		}
	}

	return nil
}

// ─── Audit sink: llm_metering_events ──────────────────────────────────

func (h *meteringHandler) writeAuditEvent(
	ctx context.Context,
	item *platformoutbox.OutboxItem,
	payload map[string]any,
	payloadBytes []byte,
) error {
	var providerID, modelID *string
	if payload != nil {
		if v, ok := payload[metaProviderID].(string); ok && v != "" {
			s := v
			providerID = &s
		}
		if v, ok := payload["model_id"].(string); ok && v != "" {
			s := v
			modelID = &s
		}
	}

	var aggregateIDPtr *string
	if item.AggregateID != "" {
		s := item.AggregateID
		aggregateIDPtr = &s
	}

	me := &domain.MeteringEvent{
		ID:            item.ID,
		OutboxID:      item.ID,
		AggregateType: item.AggregateType,
		AggregateID:   aggregateIDPtr,
		EventType:     item.EventType,
		ProviderID:    providerID,
		ModelID:       modelID,
		PayloadJSON:   payloadBytes,
	}
	if item.OrganizationID != nil {
		org := *item.OrganizationID
		me.OrganizationID = &org
	}

	if h.auditRepo == nil {
		return nil
	}
	if err := h.auditRepo.Save(ctx, me); err != nil {
		h.log.Error("audit metering event save failed",
			"err", err, "outbox_id", item.ID,
			metaEventType, item.EventType, "aggregate_type", item.AggregateType)
		return fmt.Errorf("save AI gateway audit event: %w", err)
	}
	return nil
}

// ─── Billing sink: metering_events ────────────────────────────────────
//
// 从 ai.usage.recorded payload 里提取两个可计费事实：
//   (1) ai.usage.tokens — quantity = input + output tokens, unit = "tokens"
//   (2) ai.usage.cost   — quantity = cost * 1000 (milli-yuan), unit = "milliyuan"
//
// Subject 维度: api_key_id（计费主体）→ 若缺失降级为 organization_id。
// IdempotencyKey: outbox_id + ":tokens" / outbox_id + ":cost" —
//   uq_metering_events_org_idem 会自动跳过重复。
//
// ID: 独立随机 UUID（metering_events.id 是 UUID 列）。幂等性由
// IdempotencyKey 承担，重试时 ID 变化不影响去重；InsertEvent 的
// ON CONFLICT DO NOTHING / INSERT IGNORE 会把重复插入吞掉。

func (h *meteringHandler) writeBillingEvents(
	ctx context.Context,
	item *platformoutbox.OutboxItem,
	payload map[string]any,
	payloadBytes []byte,
) error {
	if h.billingRepo == nil {
		return nil
	}

	orgID := ""
	if item.OrganizationID != nil {
		orgID = *item.OrganizationID
	}
	if orgID == "" {
		h.log.Debug("billing bridge: skipped — no organization_id on outbox item")
		return nil
	}

	// ── Extract numeric fields from payload ──
	// JSON numbers decode as float64 in map[string]any — need strconv or direct type switch
	apiKeyID := firstString(payload, "api_key_id", "alias_id")
	subject := apiKeyID
	if subject == "" {
		subject = orgID // fallback: bill to org level
	}

	inputTokens := firstInt64(payload, "input_tokens")
	outputTokens := firstInt64(payload, "output_tokens")
	totalTokens := firstInt64(payload, "total_tokens")
	if totalTokens == 0 && (inputTokens > 0 || outputTokens > 0) {
		totalTokens = inputTokens + outputTokens
	}

	costAmount := firstFloat64(payload, "cost_amount")
	costMilli := int64(costAmount * 1000) // milli-yuan 避免浮点精度

	// OccurredAt: payload 里可能有 created_at / occurred_at → 否则 now()
	occurredAt := firstTime(payload, "occurred_at", "created_at", "usage_created_at")
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	idempotencyKey := item.ID // 原始 outbox id 作为前缀

	// ── Fact 1: token usage ──
	if totalTokens > 0 {
		tokensID, err := h.newBillingEventID("tokens")
		if err != nil {
			return err
		}
		tokensEvent := &metering.MeteringEvent{
			ID:             tokensID,
			OrganizationID: orgID,
			EventType:      "ai.usage.tokens",
			Subject:        subject,
			Quantity:       totalTokens,
			Unit:           "tokens",
			IdempotencyKey: idempotencyKey + ":tokens",
			OccurredAt:     occurredAt,
			Metadata:       payloadBytes,
		}
		if _, insertErr := h.billingRepo.InsertEvent(ctx, tokensEvent); insertErr != nil {
			h.log.Error("billing metering tokens insert failed",
				"err", insertErr, "outbox_id", item.ID, "quantity", totalTokens)
			return fmt.Errorf("insert AI token billing event: %w", insertErr)
		} else {
			h.log.Debug("billing bridge: tokens fact recorded",
				"outbox_id", item.ID, "quantity", totalTokens)
		}
	}

	// ── Fact 2: monetary cost ──
	if costMilli > 0 {
		costID, err := h.newBillingEventID("cost")
		if err != nil {
			return err
		}
		costEvent := &metering.MeteringEvent{
			ID:             costID,
			OrganizationID: orgID,
			EventType:      "ai.usage.cost",
			Subject:        subject,
			Quantity:       costMilli,
			Unit:           "milliyuan",
			IdempotencyKey: idempotencyKey + ":cost",
			OccurredAt:     occurredAt,
			Metadata:       payloadBytes,
		}
		if _, insertErr := h.billingRepo.InsertEvent(ctx, costEvent); insertErr != nil {
			h.log.Error("billing metering cost insert failed",
				"err", insertErr, "outbox_id", item.ID, "cost_milli", costMilli)
			return fmt.Errorf("insert AI cost billing event: %w", insertErr)
		} else {
			h.log.Debug("billing bridge: cost fact recorded",
				"outbox_id", item.ID, "cost_milli", costMilli)
		}
	}
	return nil
}

// newBillingEventID 生成 metering_events 的 UUID 主键。
// kind 只用于错误信息（tokens / cost），不参与 ID 语义。
func (h *meteringHandler) newBillingEventID(kind string) (string, error) {
	id, err := h.idGen.NewString()
	if err != nil {
		return "", fmt.Errorf("generate AI %s billing event id: %w", kind, err)
	}
	return id, nil
}

// ─── Payload field extractors (map[string]any helpers) ─────────────────

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func firstInt64(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		switch v := m[k].(type) {
		case float64:
			return int64(v)
		case int:
			return int64(v)
		case int64:
			return v
		case string:
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				return n
			}
		}
	}
	return 0
}

func firstFloat64(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		switch v := m[k].(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case int64:
			return float64(v)
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f
			}
		}
	}
	return 0
}

func firstTime(m map[string]any, keys ...string) time.Time {
	for _, k := range keys {
		switch v := m[k].(type) {
		case string:
			for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02T15:04:05Z"} {
				if t, err := time.Parse(layout, v); err == nil {
					return t
				}
			}
		case float64:
			return time.Unix(int64(v), 0).UTC()
		}
	}
	return time.Time{}
}
