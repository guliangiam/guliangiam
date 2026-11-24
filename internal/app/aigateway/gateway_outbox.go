package aigateway

import (
	"context"
	"encoding/json"
	"time"

	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
)

// enqueueOutboxEvent 把一个审计事件写到 Outbox Repo（Fail-Open）。
// - outbox 为 nil 时直接 return（开发环境友好）
// - payload 是业务字段 map —— 序列化成 JSON 写入 OutboxEvent.Payload
// - 任何 error 都被吞掉，不阻断主链路（和 UsageLog.Create 的设计一致）
//
// Design doc §13: ai.usage.recorded / ai.quota.exceeded / ai.provider.circuit_opened
// 通过 Outbox 投递到 metering_events + 通知下游。
func (s *GatewayService) enqueueOutboxEvent(
	ctx context.Context,
	aggregateType, aggregateID, eventType, orgID string,
	availableAt time.Time,
	payload map[string]any,
) {
	if s.Outbox == nil {
		return // Outbox 未配置 → 跳过（Fail-Open）
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		// 序列化失败不阻断主链路 — 这个事件就丢了
		return
	}
	var orgPtr *string
	if orgID != "" {
		orgPtr = &orgID
	}
	id, _ := s.UUID.NewString()
	_ = s.Outbox.Enqueue(ctx, &domainoutbox.OutboxEvent{
		ID:             id,
		OrganizationID: orgPtr,
		AggregateType:  aggregateType,
		AggregateID:    aggregateID,
		EventType:      eventType,
		EventVersion:   1,
		Payload:        payloadBytes,
		MaxAttempts:    3,
		AvailableAt:    availableAt,
	})
}

// enqueueQuotaExceeded 是 enqueueOutboxEvent 的便捷封装——RPM / TPM 两个
// 超限路径复用同一个 payload schema。
func (s *GatewayService) enqueueQuotaExceeded(
	ctx context.Context, orgID, apiKeyID, limitType string, current int64, limit int,
) {
	s.enqueueOutboxEvent(ctx, "llm_quota", apiKeyID, "ai.quota.exceeded", orgID, s.Clock.Now(), map[string]any{
		metaOrganizationID: orgID,
		"api_key_id":       apiKeyID,
		metaQuotaScope:     "api_key",
		"quota_limit_type": limitType,
		"current_value":    current,
		"limit_value":      limit,
	})
}
