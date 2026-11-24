package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	eventdomain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	webhookdomain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
	"github.com/guliangiam/guliangiam/internal/platform/redact"
	tentxtx "github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

const defaultDeliveryTimeoutMs = 10000

// DeliveryOutboxHandler 是 aggregate_type="webhook" 的唯一投递处理器（H40 退役
// 遗留直轮询轨后，webhook 投递统一走 outbox 单轨）。每次 Handle 恰好一次 HTTP
// 尝试；重试调度（attempts/backoff/死信）由 platform/outbox.Worker 拥有，
// webhook_events 台账在此同步维护供 UI/API 查询：
//
//   - 2xx → delivered
//   - 永久失败（配置缺失/禁用/无签名密钥/不可重试 4xx，DEF-28）→ failed +
//     同事务 webhook_event.failed 审计，返回 Permanent → Worker 死信
//   - 可重试失败且预算未尽 → 台账保持 pending，retry_count=attempts+1，
//     返回可重试错误 → Worker MarkRetry；next_retry_at 留空（调度事实在
//     outbox.available_at，台账不再自建第二套调度）
//   - 可重试失败且本次即最后一次（attempts+1 >= max_attempts）→ failed + 审计
//
// 台账行定位：扇出路径 webhook_events 与 outbox 同主键（domain_outbox.go）；
// 重放路径 outbox 是新行，Headers.event_id 指回原台账行。
type DeliveryOutboxHandler struct {
	uow    appuow.UnitOfWork
	sender webhookdomain.DeliverySender
	log    *slog.Logger
}

func NewDeliveryOutboxHandler(uow appuow.UnitOfWork, sender webhookdomain.DeliverySender, log *slog.Logger) *DeliveryOutboxHandler {
	if log == nil {
		log = slog.Default()
	}
	return &DeliveryOutboxHandler{uow: uow, sender: sender, log: log.With("component", "webhook-outbox-handler")}
}

// webhooks / webhook_events 均受 FORCE RLS 保护，Worker 无请求租户上下文，
// 读写必须在系统作用域事务内执行（DEF-35）。
func (h *DeliveryOutboxHandler) withSystemTx(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return h.uow.WithinTransaction(tentxtx.System(ctx), fn)
}

func (h *DeliveryOutboxHandler) Handle(ctx context.Context, item *platformoutbox.OutboxItem) error {
	ledgerID := item.ID
	var meta struct {
		EventID string `json:"event_id"`
	}
	if len(item.Headers) > 0 {
		_ = json.Unmarshal(item.Headers, &meta)
	}
	if meta.EventID != "" {
		ledgerID = meta.EventID
	}

	var wh *webhookdomain.Webhook
	var ev *eventdomain.WebhookEvent
	loadErr := h.withSystemTx(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		var err error
		ev, err = tx.WebhookEvents().GetByID(ctx, ledgerID)
		if err != nil {
			return err
		}
		wh, err = tx.Webhooks().GetByID(ctx, item.AggregateID)
		return err
	})
	if loadErr != nil {
		var appErr *apperrors.Error
		if errors.As(loadErr, &appErr) && appErr.Code == apperrors.CodeNotFound {
			if ev == nil {
				// 台账行已被删除（DeleteWebhookEvent）：无投递对象，直接死信。
				return platformoutbox.Permanent(apperrors.NewNotFoundf("webhook event %s not found", ledgerID))
			}
			// 父配置已删除：事件永远无法投递，终结台账而不是滞留 pending。
			h.finalizeFailed(ctx, ledgerID, "", 0, "webhook config not found")
			return platformoutbox.Permanent(apperrors.NewNotFoundf("webhook config %s not found", item.AggregateID))
		}
		// 瞬时读取失败：记录本次尝试进度后交给 Worker 退避重试。
		if ev != nil {
			h.recordAttempt(ctx, item, ledgerID, 0, "webhook config lookup failed: "+redact.SanitizeText(loadErr.Error()))
		}
		return apperrors.WrapInternal(loadErr, "webhook delivery: load config/event")
	}

	if !wh.IsEnabled {
		h.finalizeFailed(ctx, ledgerID, wh.OrganizationID, 0, "webhook is disabled")
		return platformoutbox.Permanent(apperrors.NewInvalidArgumentf("webhook %s is disabled", wh.ID))
	}
	// 无签名密钥的 webhook 绝不投递：任何回退密钥（如 webhook.ID）都会被
	// 列表 API 暴露，接收方无法区分真实投递与伪造请求。
	if strings.TrimSpace(wh.Secret) == "" {
		reason := "refusing delivery: webhook has no signing secret configured"
		h.log.Error("webhook delivery blocked", "event_id", ledgerID, "webhook_id", wh.ID, "reason", reason)
		h.finalizeFailed(ctx, ledgerID, wh.OrganizationID, 0, reason)
		return platformoutbox.Permanent(apperrors.NewInvalidArgument(reason))
	}
	if h.sender == nil {
		return platformoutbox.Permanent(apperrors.NewUnimplemented("delivery sender is not configured"))
	}

	statusCode, body, sendErr := h.sendDelivery(ctx, wh, ev)
	if sendErr != nil {
		h.log.Warn("webhook delivery failed",
			"event_id", ledgerID, "webhook_id", wh.ID, "attempts", item.Attempts, "error", redact.WrapError(sendErr))
		return h.handleRetryableFailure(ctx, item, ledgerID, wh.OrganizationID, 0,
			"delivery error: "+redact.SanitizeText(sendErr.Error()))
	}
	if statusCode >= 200 && statusCode < 300 {
		if err := h.markDelivered(ctx, ledgerID, statusCode, body); err != nil {
			// 台账落库失败按可重试处理：接收方以 X-GuliangIAM-Delivery 去重
			//（ADR-018），重复投递可被幂等吸收，丢台账不可接受。
			return apperrors.WrapInternal(err, "webhook delivery: mark delivered")
		}
		h.log.Info("webhook delivered successfully", "event_id", ledgerID, "webhook_id", wh.ID, "status_code", statusCode)
		return nil
	}
	if !webhookStatusRetryable(statusCode) {
		reason := fmt.Sprintf("webhook returned non-retryable status %d", statusCode)
		h.finalizeFailed(ctx, ledgerID, wh.OrganizationID, statusCode, reason)
		return platformoutbox.Permanent(apperrors.NewInternal(reason))
	}
	return h.handleRetryableFailure(ctx, item, ledgerID, wh.OrganizationID, statusCode, body)
}

// handleRetryableFailure 维护台账并返回可重试错误。本次尝试已是预算内最后
// 一次（attempts+1 >= max_attempts）时终结为 failed —— Worker 的 MarkRetry 在
// attempts 达到上限后转死信，不会再有下一轮 Handle 来终结台账。
func (h *DeliveryOutboxHandler) handleRetryableFailure(ctx context.Context, item *platformoutbox.OutboxItem, ledgerID, orgID string, statusCode int, failBody string) error {
	if item.Attempts+1 >= item.MaxAttempts {
		h.finalizeFailed(ctx, ledgerID, orgID, statusCode, failBody)
		h.log.Warn("webhook delivery exhausted retries",
			"event_id", ledgerID, "webhook_id", item.AggregateID, "max_attempts", item.MaxAttempts)
	} else {
		h.recordAttempt(ctx, item, ledgerID, statusCode, failBody)
	}
	if statusCode > 0 {
		return apperrors.NewInternalf("webhook delivery attempt failed with status %d", statusCode)
	}
	return apperrors.NewInternal("webhook delivery attempt failed")
}

func (h *DeliveryOutboxHandler) sendDelivery(ctx context.Context, wh *webhookdomain.Webhook, ev *eventdomain.WebhookEvent) (int, string, error) {
	body := ev.Payload
	if wh.CustomBody != "" {
		body = []byte(wh.CustomBody)
	}
	headers := h.buildDeliveryHeaders(ctx, wh, ev)

	// BE-B-19：webhook.TimeoutMs 必须实际约束本次投递；Worker 的单项超时
	// 更早到期时以更早者为准。
	timeoutMs := wh.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = defaultDeliveryTimeoutMs
	}
	sendCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	result := h.sender.Send(sendCtx, wh.URL, headers, body, wh.Secret)
	if result.Error != nil {
		return 0, "", result.Error
	}
	return result.StatusCode, result.ResponseBody, nil
}

func (h *DeliveryOutboxHandler) buildDeliveryHeaders(ctx context.Context, wh *webhookdomain.Webhook, ev *eventdomain.WebhookEvent) map[string]string {
	headers := map[string]string{
		"X-GuliangIAM-Event":    ev.EventType,
		"X-GuliangIAM-Delivery": ev.ID,
	}
	// HMAC 签名由 DeliverySender 统一生成（GuliangIAM-Signature:
	// t=<unix_sec>,v1=<hmac_sha256(t + '.' + body)>），此处不重复签名。
	contentType := wh.ContentType
	if contentType == "" {
		contentType = "application/json"
	}
	headers["Content-Type"] = contentType
	if wh.Headers != "" {
		var customHeaders map[string]string
		if jsonErr := json.Unmarshal([]byte(wh.Headers), &customHeaders); jsonErr == nil {
			for k, v := range customHeaders {
				headers[k] = v
			}
		} else {
			// 存储的 header blob 可能含凭据材料，解码错误可能回显 offending
			// 字节；只记录脱敏后的消息。
			h.log.WarnContext(ctx, "failed to parse webhook custom headers", "webhook_id", wh.ID, "error", redact.WrapError(jsonErr))
		}
	}
	return headers
}

func (h *DeliveryOutboxHandler) markDelivered(ctx context.Context, ledgerID string, statusCode int, body string) error {
	return h.withSystemTx(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		e, err := tx.WebhookEvents().GetByID(ctx, ledgerID)
		if err != nil {
			return err
		}
		e.ResponseCode = statusCode
		e.ResponseBody = body
		e.Status = "delivered"
		e.NextRetryAt = nil
		e.UpdatedAt = time.Now()
		return tx.WebhookEvents().Update(ctx, e)
	})
}

// recordAttempt 把失败尝试写进台账：状态保持 pending，retry_count 对齐
// outbox attempts（本次 = item.Attempts+1）；next_retry_at 留空，退避调度
// 唯一事实源是 outbox.available_at。落库失败只记日志，不改投递结论。
func (h *DeliveryOutboxHandler) recordAttempt(ctx context.Context, item *platformoutbox.OutboxItem, ledgerID string, statusCode int, failBody string) {
	if err := h.withSystemTx(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		e, err := tx.WebhookEvents().GetByID(ctx, ledgerID)
		if err != nil {
			if isNotFound(err) {
				return nil
			}
			return err
		}
		e.Status = "pending"
		e.RetryCount = item.Attempts + 1
		e.ResponseCode = statusCode
		e.ResponseBody = failBody
		e.NextRetryAt = nil
		e.UpdatedAt = time.Now()
		return tx.WebhookEvents().Update(ctx, e)
	}); err != nil {
		h.log.Error("failed to record webhook delivery attempt", "event_id", ledgerID, "error", err)
	}
}

// finalizeFailed 终结台账为 failed，并在同一事务写 events.yaml 登记的
// webhook_event.failed 审计；租户优先取已加载的父配置，缺失时系统作用域
// 事务内尽力解析。落库失败只记日志，不改投递结论（Permanent 仍生效）。
func (h *DeliveryOutboxHandler) finalizeFailed(ctx context.Context, ledgerID, orgID string, statusCode int, failBody string) {
	if err := h.withSystemTx(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		e, err := tx.WebhookEvents().GetByID(ctx, ledgerID)
		if err != nil {
			if isNotFound(err) {
				return nil
			}
			return err
		}
		e.ResponseCode = statusCode
		e.ResponseBody = failBody
		e.Status = "failed"
		e.NextRetryAt = nil
		e.UpdatedAt = time.Now()
		if err := tx.WebhookEvents().Update(ctx, e); err != nil {
			return err
		}
		auditOrgID := orgID
		if auditOrgID == "" && e.WebhookID != "" {
			if wh, whErr := tx.Webhooks().GetByID(ctx, e.WebhookID); whErr == nil && wh != nil {
				auditOrgID = wh.OrganizationID
			}
		}
		return audit.EmbedAuditEvent(
			clock.SystemClock{}, tx, ctx,
			"", "", "", "", "",
			auditOrgID, "",
			"webhook_event.failed", "WebhookEvent", e.ID,
			"WebhookEvent", "webhook_event.failed",
			map[string]any{"responseCode": statusCode},
		)
	}); err != nil {
		h.log.Error("failed to finalize webhook event as failed", "event_id", ledgerID, "error", err)
	}
}

func isNotFound(err error) bool {
	var appErr *apperrors.Error
	return errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound
}

// webhookStatusRetryable 区分可重试状态：408/425/429 与 5xx 可重试；
// 其余 4xx（含 410）为永久失败，不重试（DEF-28）。
func webhookStatusRetryable(statusCode int) bool {
	switch statusCode {
	case http.StatusRequestTimeout, http.StatusTooEarly, http.StatusTooManyRequests:
		return true
	}
	return statusCode >= 500
}

var _ platformoutbox.DeliveryHandler = (*DeliveryOutboxHandler)(nil)
