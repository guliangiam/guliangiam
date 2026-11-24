package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	guuid "github.com/google/uuid"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	eventdomain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
)

// 领域事件的 outbox 聚合类型。这些聚合的事件由 auditoutbox 在与业务事实
// 同一事务中写入 outbox；此前没有注册处理器，Worker 一律转死信（"no handler
// → dead"）。本处理器把它们扇出到订阅了该事件类型的租户 webhook。
const (
	AggregateOrganization = "Organization"
	AggregateRisk         = "risk"
	AggregateUser         = "user"
	AggregateSession      = "session"
)

// DomainEventAggregateTypes 是为 DomainEventOutboxHandler 注册的全部聚合类型。
var DomainEventAggregateTypes = []string{
	"Application",
	"ConsentRecord",
	"DsarRequest",
	"Order",
	"Organization",
	"Permission",
	"RefreshToken",
	"RememberMeFamily",
	"Role",
	"Session",
	"Subscription",
	"Token",
	"TokenFamily",
	"TrustedDevice",
	"User",
	"Webhook",
	"WebhookEvent",
	"access_request",
	"action",
	"actions",
	"adapters",
	"agents",
	"ai",
	"application",
	"audit",
	"auth",
	"authorization_code",
	"authz",
	"b2b",
	"b2b.domain.claimed",
	"b2b.domain.deleted",
	"captchaproviders",
	"cert",
	"certs",
	"consent",
	"control",
	"coupon",
	"coupon_usage",
	"credential",
	"decision",
	"dsar",
	"enforcers",
	"faceidproviders",
	"federation_trust",
	"forms",
	"gateway",
	"group",
	"groups",
	"idv",
	"iga",
	"invitation",
	"key",
	"ldaps",
	"lifecycle",
	"mcpservers",
	"metering",
	"metering_event",
	"mfa",
	"models",
	"namespace",
	"notificationproviders",
	"order",
	"org",
	"organization",
	"password_change_gate",
	"payment",
	"permission",
	"plans",
	"platform_admins",
	"policy",
	"pricings",
	"privacy",
	"privileged_session",
	"products",
	"protocol",
	"provider",
	"provider_config",
	"radiusaccountings",
	"radiusconfigs",
	"rebac",
	"records",
	"refresh_token",
	"resources",
	"risk",
	"risk_signal",
	"role",
	"rules",
	"secret",
	"security",
	"server",
	"service_account",
	"session",
	"session_governance",
	"site",
	"subscription",
	"syncer",
	"token",
	"transaction",
	"tuple",
	"user",
	"users",
	"web3",
	"web3_identity_mapping",
	"webauthn",
	"webauthn_credential",
	"webhook",
	"webhook_event",
	"wli",
}

// DomainEventOutboxHandler 把领域事件扇出到租户 webhook 订阅者。
//
// 失败分类：
//   - 永久失败（PermanentError）：缺失组织、event_type 非点分命名、payload 非
//     JSON 对象/数组 —— 重试不会改变结果，直接死信。
//   - 可重试失败：订阅查询、已有投递查询、入队等仓储/依赖错误。
//
// 幂等：每个订阅者使用 (事件 ID, webhook ID) 派生的确定性投递主键；重放时
// 先以一次 ListExistingIDs 批量预检 webhook_events，已存在则跳过，不会重复
// 投递；缺失项在单个事务内批量入队（要么全部落库要么全部回滚）。
//
// 无订阅者时不产生外部副作用：该事件的审计副本已在业务事务内写入 records，
// outbox 事件只需被确认消费；这是唯一允许"无副作用成功"的情形，并会记录日志。
type DomainEventOutboxHandler struct {
	uow appuow.UnitOfWork
	log *slog.Logger
}

// NewDomainEventOutboxHandler 构造领域事件处理器。log 为 nil 时使用默认 logger。
func NewDomainEventOutboxHandler(uow appuow.UnitOfWork, log *slog.Logger) *DomainEventOutboxHandler {
	if log == nil {
		log = slog.Default()
	}
	return &DomainEventOutboxHandler{uow: uow, log: log}
}

// Handle 由 Outbox Worker 调用。实现 DeliveryHandler 契约。
func (h *DomainEventOutboxHandler) Handle(ctx context.Context, item *platformoutbox.OutboxItem) error {
	var orgID string
	if item.OrganizationID != nil {
		orgID = strings.TrimSpace(*item.OrganizationID)
	}
	if orgID == "" {
		return platformoutbox.Permanent(apperrors.NewInvalidArgument("outbox event is missing organization_id"))
	}
	if strings.TrimSpace(item.EventType) == "" || !strings.Contains(item.EventType, ".") {
		return platformoutbox.Permanent(apperrors.NewInvalidArgumentf(
			"outbox event_type %q is not dot-namespaced", item.EventType))
	}
	if !jsonObjectOrArray(item.Payload) {
		return platformoutbox.Permanent(apperrors.NewInvalidArgumentf(
			"outbox payload for %s/%s is not a JSON object or array", item.AggregateType, item.EventType))
	}

	webhooks, err := h.uow.Webhooks().ListByOrganizationAndEvent(ctx, orgID, item.EventType)
	if err != nil {
		return apperrors.WrapInternal(err, "domain outbox: list subscribed webhooks")
	}
	if len(webhooks) == 0 {
		h.log.Debug("domain outbox event has no subscribers",
			"aggregate_type", item.AggregateType, "event_type", item.EventType,
			"organization_id", orgID)
		return nil
	}

	// 批量扇出（LEFT-R0-07）：一次 ListExistingIDs 预检覆盖全部订阅者
	// （替代逐订阅者 GetByID 的 N+1 读），缺失项在单个事务里
	// CreateBatch + EnqueueBatch 一次入队（替代逐条独立事务）。
	// webhook_events 无 organization_id 列，RLS 谓词经父表绑定；本 Handler
	// 由 Outbox Worker 在系统作用域 ctx 下调用，预检走池读安全。
	ids := make([]string, 0, len(webhooks))
	for _, wh := range webhooks {
		if wh == nil || !wh.IsEnabled || wh.ID == "" {
			continue
		}
		ids = append(ids, deterministicDeliveryID(item.ID, wh.ID))
	}
	if len(ids) == 0 {
		return nil
	}

	existing, err := h.uow.WebhookEvents().ListExistingIDs(ctx, ids)
	if err != nil {
		return apperrors.WrapInternal(err, "domain outbox: batch check existing deliveries")
	}
	// ListExistingIDs 契约：入参 canonical 小写、返回同为小写；
	// deterministicDeliveryID 产出的正是小写 UUID，可直接集合比对。
	existingSet := make(map[string]struct{}, len(existing))
	for _, id := range existing {
		existingSet[id] = struct{}{}
	}

	now := time.Now()
	events := make([]*eventdomain.WebhookEvent, 0, len(ids))
	deliveries := make([]*domainoutbox.OutboxEvent, 0, len(ids))
	fannedOut := make([]string, 0, len(ids))
	for _, wh := range webhooks {
		if wh == nil || !wh.IsEnabled || wh.ID == "" {
			continue
		}
		id := deterministicDeliveryID(item.ID, wh.ID)
		if _, ok := existingSet[id]; ok {
			// 上一次尝试已入队 —— 幂等重放，跳过。
			continue
		}
		maxAttempts := wh.RetryCount
		if maxAttempts <= 0 {
			maxAttempts = 5
		}
		headers := buildWebhookHeaders(wh.URL, wh.Secret, wh.Method, nil, "")
		events = append(events, &eventdomain.WebhookEvent{
			ID:        id,
			WebhookID: wh.ID,
			EventType: item.EventType,
			Payload:   json.RawMessage(item.Payload),
			Status:    "pending",
			CreatedAt: now,
			UpdatedAt: now,
		})
		deliveries = append(deliveries, &domainoutbox.OutboxEvent{
			ID:             id,
			OrganizationID: &orgID,
			AggregateType:  "webhook",
			AggregateID:    wh.ID,
			EventType:      item.EventType,
			Payload:        item.Payload,
			Headers:        headers,
			MaxAttempts:    maxAttempts,
		})
		fannedOut = append(fannedOut, wh.ID)
	}
	if len(events) == 0 {
		return nil
	}

	// 单事务批量入队：要么全部落库要么全部回滚；worker 重试整条事件时
	// 由上面的预检过滤已入队项，保持幂等。
	// 并发重放语义：两个 worker 同时预检通过时，后提交方触发主键冲突、
	// 整批回滚（错误可重试）；下一轮预检会过滤对方已提交的行后收敛。
	// 确定性主键保证冲突只发生在重复投递上，不会丢单或产生重复。
	if err := h.uow.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.WebhookEvents().CreateBatch(ctx, events); err != nil {
			return apperrors.WrapInternal(err, "domain outbox: batch create webhook_events")
		}
		if err := tx.Outbox().EnqueueBatch(ctx, deliveries); err != nil {
			return apperrors.WrapInternal(err, "domain outbox: batch enqueue deliveries")
		}
		return nil
	}); err != nil {
		return err
	}

	for _, webhookID := range fannedOut {
		h.log.Info("domain outbox event fanned out to webhook",
			"aggregate_type", item.AggregateType, "event_type", item.EventType,
			"organization_id", orgID, "webhook_id", webhookID)
	}
	return nil
}

// deterministicDeliveryID 把 (outbox 事件 ID, webhook ID) 映射为稳定 UUID：
// 同一事件重放得到同一投递主键，从而命中已存在的 webhook_events 并跳过。
func deterministicDeliveryID(eventID, webhookID string) string {
	return guuid.NewSHA1(guuid.NameSpaceOID, []byte(eventID+"|"+webhookID)).String()
}

// jsonObjectOrArray 要求载荷是 JSON 对象或数组；标量/空载荷视为契约违反。
func jsonObjectOrArray(payload []byte) bool {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		return false
	}
	return json.Valid(trimmed)
}
