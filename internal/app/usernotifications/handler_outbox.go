package usernotifications

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	domain "github.com/guliangiam/guliangiam/internal/domain/usernotifications"
	"github.com/guliangiam/guliangiam/internal/platform/outbox"
	tentxtx "github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// inboxEventNamespace 为确定性 ID 提供固定命名空间，保证同一条 outbox 事件
// 重投时派生出同一个收件箱主键（配合 ExistsBySource 双重幂等）。
var inboxEventNamespace = uuid.MustParse("6f3f0f1e-5a63-4b2f-9a6a-0c8f2a5d1e77")

const (
	inboxEventSourceType = "outbox"
	// inboxAdminPageSize 限制一次告警扇出的管理员数量，避免超大租户写放大。
	inboxAdminPageSize = 50
)

// InboxEventHandler turns curated domain events into per-user inbox messages.
// It is registered alongside the existing audit/webhook handler; the registry
// only allows one handler per aggregate, so bootstrap composes them.
type InboxEventHandler struct {
	Service Service
	Logger  *slog.Logger
}

func NewInboxEventHandler(service Service, logger *slog.Logger) *InboxEventHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &InboxEventHandler{Service: service, Logger: logger.With("component", "inbox-event-handler")}
}

// Handle implements outbox.DeliveryHandler.
//
// 事件接入范围：
//   - user.created                → 注册欢迎站内信
//   - ai.quota.exceeded           → 组织管理员的配额告警
//   - ai.provider.circuit_opened  → 组织管理员的熔断告警
//
// 其余业务事件（工单回复、管理员公告、密码重置）在各自用例内通过
// Service.Publish 直接发布，避免用空 payload 猜测收件人。
func (h *InboxEventHandler) Handle(ctx context.Context, item *outbox.OutboxItem) error {
	if item == nil || item.OrganizationID == nil || *item.OrganizationID == "" {
		return nil
	}
	organizationID := *item.OrganizationID

	switch {
	case item.AggregateType == "user" && item.EventType == "user.created":
		if item.AggregateID == "" || item.ID == "" {
			return nil
		}
		return h.handleUserCreated(ctx, item, organizationID)
	case item.AggregateType == "llm_quota" && item.EventType == "ai.quota.exceeded":
		if item.ID == "" {
			return nil
		}
		return h.handleOrgAlert(ctx, item, organizationID, orgAlertSpec{
			category:   domain.CategorySystem,
			eventType:  "ai.quota.exceeded",
			title:      "AI 网关配额告警",
			link:       "/ai/quotas",
			sourceType: "llm_quota",
			content: func(payload map[string]any) string {
				return fmt.Sprintf(
					"API Key %s 的 %s 配额已触达上限（当前 %s / 上限 %s）。请调整配额或通知使用方降低调用量。",
					payloadString(payload, "api_key_id"),
					payloadString(payload, "quota_limit_type"),
					payloadString(payload, "current_value"),
					payloadString(payload, "limit_value"),
				)
			},
		})
	case item.AggregateType == "llm_provider" && item.EventType == "ai.provider.circuit_opened":
		if item.ID == "" {
			return nil
		}
		return h.handleOrgAlert(ctx, item, organizationID, orgAlertSpec{
			category:   domain.CategorySystem,
			eventType:  "ai.provider.circuit_opened",
			title:      "AI 提供方熔断告警",
			link:       "/ai/health",
			sourceType: "llm_provider",
			content: func(payload map[string]any) string {
				return fmt.Sprintf(
					"提供方 %s 的熔断器已打开（触发原因：%s，探测延迟 %s ms）。请检查上游服务可用性与密钥配置。",
					payloadString(payload, "provider_id"),
					payloadString(payload, "trigger"),
					payloadString(payload, "latency_ms"),
				)
			},
		})
	}
	return nil
}

func (h *InboxEventHandler) handleUserCreated(ctx context.Context, item *outbox.OutboxItem, organizationID string) error {
	userID := item.AggregateID

	// outbox worker 没有请求租户上下文；收件箱表是 FORCE RLS，必须在系统
	// 作用域事务内写，否则会 0 行写入或被 RLS 拒绝。
	ctx = tentxtx.System(ctx)

	// 事件重投幂等：同源消息已经存在则直接成功。
	if inboxNotificationExists(ctx, h.Service.UOW, organizationID, userID, inboxEventSourceType, item.ID) {
		return nil
	}

	_, err := h.Service.Publish(ctx, PublishCommand{
		NotificationID: uuid.NewSHA1(inboxEventNamespace, []byte(item.ID+":"+userID)).String(),
		OrganizationID: organizationID,
		UserID:         userID,
		Category:       string(domain.CategorySystem),
		EventType:      "user.registered",
		Title:          "欢迎使用 GuliangIAM",
		Content:        "你的账号已注册成功。可在「通知中心」查看系统消息，并在「通知偏好」中开启邮件、短信或企业微信提醒。",
		SourceType:     inboxEventSourceType,
		SourceID:       item.ID,
	})
	if err != nil {
		h.Logger.ErrorContext(ctx, "failed to publish welcome notification",
			"organization_id", organizationID, "user_id", userID, "error", err)
		return err
	}
	return nil
}

type orgAlertSpec struct {
	category   domain.Category
	eventType  string
	title      string
	link       string
	sourceType string
	content    func(payload map[string]any) string
}

// handleOrgAlert 把运维告警扇出到组织管理员。收件人解析与写入都在系统作用域
// 事务内完成（RLS）；发布经 Service.Publish 以获得偏好驱动的多渠道扇出。
func (h *InboxEventHandler) handleOrgAlert(ctx context.Context, item *outbox.OutboxItem, organizationID string, spec orgAlertSpec) error {
	ctx = tentxtx.System(ctx)

	payload := decodePayload(item.Payload)
	adminIDs, err := inboxAdminUserIDs(ctx, h.Service.UOW, organizationID)
	if err != nil {
		h.Logger.WarnContext(ctx, "inbox alert: administrator lookup failed",
			"organization_id", organizationID, "event_type", item.EventType, "error", err)
		return err
	}
	if len(adminIDs) == 0 {
		return nil
	}

	content := spec.content(payload)
	var firstErr error
	for _, userID := range adminIDs {
		if inboxNotificationExists(ctx, h.Service.UOW, organizationID, userID, inboxEventSourceType, item.ID) {
			continue
		}
		_, pubErr := h.Service.Publish(ctx, PublishCommand{
			NotificationID: uuid.NewSHA1(inboxEventNamespace, []byte(item.ID+":"+userID)).String(),
			OrganizationID: organizationID,
			UserID:         userID,
			Category:       string(spec.category),
			EventType:      spec.eventType,
			Title:          spec.title,
			Content:        content,
			Link:           spec.link,
			SourceType:     inboxEventSourceType,
			SourceID:       item.ID,
		})
		if pubErr != nil {
			h.Logger.ErrorContext(ctx, "failed to publish inbox alert",
				"organization_id", organizationID, "user_id", userID,
				"event_type", item.EventType, "error", pubErr)
			if firstErr == nil {
				firstErr = pubErr
			}
		}
	}
	return firstErr
}

func decodePayload(raw []byte) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	return payload
}

func payloadString(payload map[string]any, key string) string {
	if payload == nil {
		return "-"
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return "-"
	}
	switch typed := value.(type) {
	case string:
		if typed == "" {
			return "-"
		}
		return typed
	case float64:
		// JSON 数字统一为 float64；整数值去掉小数尾巴，避免 "1000 / 1000.000000"。
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
		return fmt.Sprintf("%g", typed)
	default:
		return fmt.Sprintf("%v", typed)
	}
}
