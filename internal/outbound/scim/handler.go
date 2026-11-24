package scim

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	connectors "github.com/guliangiam/guliangiam/internal/domain/connectors"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	outboxplatform "github.com/guliangiam/guliangiam/internal/platform/outbox"
)

// Handler 实现 outbox.DeliveryHandler，aggregate_type="scim_outbound"。
//
// 从 OutboxItem.Headers (JSON) 里读：
//
//	{
//	  "connector_id": "...",
//	  "organization_id": "...",
//	  "scim_path": "/Users/abc-123"  // 可选；否则按 eventType 自动推导
//	}
//
// Payload 是完整 SCIM 资源 JSON（由写 outbox 的业务层负责映射）。
type Handler struct {
	connectors connectors.ConnectorRepository
	clock      func() time.Time // 可注入（测试用）
}

// NewHandler 构造 DeliveryHandler。
func NewHandler(connRepo connectors.ConnectorRepository) outboxplatform.DeliveryHandler {
	return &Handler{connectors: connRepo, clock: time.Now}
}

// NewHandlerWithClock 构造 Handler 并注入 clock（测试用）。
func NewHandlerWithClock(connRepo connectors.ConnectorRepository, clock func() time.Time) outboxplatform.DeliveryHandler {
	return &Handler{connectors: connRepo, clock: clock}
}

// scimHeaders 是 OutboxItem.Headers 的解析结构。
type scimHeaders struct {
	ConnectorID    string `json:"connector_id"`
	OrganizationID string `json:"organization_id"`
	SCIMPath       string `json:"scim_path,omitempty"`
}

// Handle 按 Outbox 状态机投递 SCIM 事件。
func (h *Handler) Handle(ctx context.Context, item *outboxplatform.OutboxItem) error {
	// 1. 解析 Headers
	var meta scimHeaders
	if len(item.Headers) > 0 {
		if err := json.Unmarshal(item.Headers, &meta); err != nil {
			return apperrors.WrapInvalidArgument(err, "scim_outbound: parse headers")
		}
	}
	if meta.ConnectorID == "" {
		return apperrors.NewInvalidArgument(
			"scim_outbound: missing connector_id in headers " +
				"(event_type=" + item.EventType + " aggregate_id=" + item.AggregateID + ")")
	}

	// 2. repo.Get 查 connector
	connector, err := h.connectors.Get(ctx, meta.ConnectorID, meta.OrganizationID)
	if err != nil {
		return apperrors.WrapNotFound(err, "scim_outbound: connector not found")
	}
	if connector == nil {
		return apperrors.NewNotFound("scim_outbound: connector not found: " + meta.ConnectorID)
	}

	// 3. connector disabled → 静默成功（上游业务关了，不想误推）
	if !connector.IsEnabled {
		return nil
	}

	// 4. 按 eventType 推导 path 和 HTTP method
	method, path, body, err := h.routeEvent(item, &meta)
	if err != nil {
		return err // 已经是 apperrors 包装好的
	}

	// 5. Client.Do
	client := NewClient(connector.SCIMBaseURL, connector.BearerToken, connector.TimeoutMs, connector.MaxRatePerMin)
	status, respBody, err := client.Do(ctx, method, path, body)
	if err != nil {
		// 连接层 error → INTERNAL，Worker 会 MarkRetry 指数退避
		return apperrors.WrapInternal(err, "scim_outbound: transport error")
	}

	// 6. 结果分类
	switch {
	case status >= 200 && status < 300:
		return nil // Worker MarkSuccess

	case status == http.StatusBadRequest ||
		status == http.StatusUnauthorized ||
		status == http.StatusForbidden ||
		status == http.StatusNotFound ||
		status == http.StatusConflict:
		// 业务拒绝 → 死信
		return apperrors.NewInvalidArgumentf(
			"scim_outbound: downstream rejected HTTP %d: %s", status, truncate(string(respBody), 200))

	case status == http.StatusTooManyRequests ||
		status >= 500:
		// 可重试 → INTERNAL，Worker 退避
		return apperrors.NewInternalf(
			"scim_outbound: downstream temporary HTTP %d: %s", status, truncate(string(respBody), 200))

	default:
		// 其他 3xx/4xx → 保守按重试处理（理论上 3xx 客户端已自动 follow）
		return apperrors.NewInternalf(
			"scim_outbound: unhandled HTTP %d", status)
	}
}

// truncate 截断字符串，用于错误日志。
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// routeEvent 按 eventType 推导 method + path + body。
func (h *Handler) routeEvent(item *outboxplatform.OutboxItem, meta *scimHeaders) (method, path string, body []byte, err error) {
	// 如果 Headers 里已带 scim_path，直接用（优先业务层指定）
	if meta.SCIMPath != "" {
		path = meta.SCIMPath
	}

	switch item.EventType {
	case connectors.EvtUserCreated:
		method = http.MethodPost
		if path == "" {
			path = "/Users"
		}
		body = item.Payload

	case connectors.EvtUserUpdated:
		method = http.MethodPut
		if path == "" {
			id := extractResourceID(item.Payload, item.AggregateID)
			if id == "" {
				return "", "", nil, apperrors.NewInvalidArgument("scim_outbound: user.updated missing id in payload/aggregate_id")
			}
			path = "/Users/" + id
		}
		body = item.Payload

	case connectors.EvtUserDeactivated:
		method = http.MethodPatch
		if path == "" {
			id := extractResourceID(item.Payload, item.AggregateID)
			if id == "" {
				return "", "", nil, apperrors.NewInvalidArgument("scim_outbound: user.deactivated missing id")
			}
			path = "/Users/" + id
		}
		body = []byte(`{"Operations":[{"op":"replace","path":"active","value":false}]}`)

	case connectors.EvtGroupCreated:
		method = http.MethodPost
		if path == "" {
			path = "/Groups"
		}
		body = item.Payload

	case connectors.EvtGroupUpdated:
		method = http.MethodPut
		if path == "" {
			id := extractResourceID(item.Payload, item.AggregateID)
			if id == "" {
				return "", "", nil, apperrors.NewInvalidArgument("scim_outbound: group.updated missing id")
			}
			path = "/Groups/" + id
		}
		body = item.Payload

	case connectors.EvtGroupDeleted:
		method = http.MethodDelete
		if path == "" {
			id := item.AggregateID
			if id == "" {
				return "", "", nil, apperrors.NewInvalidArgument("scim_outbound: group.deleted missing aggregate_id")
			}
			path = "/Groups/" + id
		}
		body = nil

	case connectors.EvtGroupMemberAdded, connectors.EvtGroupMemberRemoved,
		connectors.EvtRoleMemberAdded, connectors.EvtRoleMemberRemoved:
		method = http.MethodPatch
		if path == "" {
			id := item.AggregateID // group uuid
			if id == "" {
				return "", "", nil, apperrors.NewInvalidArgument(
					fmt.Sprintf("scim_outbound: %s missing aggregate_id (group id)", item.EventType))
			}
			path = "/Groups/" + id
		}
		// 如果 Payload 里带了 Operations，直接用；否则用默认 PATCH body
		body = item.Payload
		if len(body) == 0 {
			if strings.HasSuffix(item.EventType, ".added") {
				body = []byte(`{"Operations":[{"op":"add","path":"members","value":{}}]}`)
			} else {
				body = []byte(`{"Operations":[{"op":"remove","path":"members","value":{}}]}`)
			}
		}

	default:
		return "", "", nil, apperrors.NewInvalidArgumentf(
			"scim_outbound: unknown event_type=%q (aggregate_id=%s)", item.EventType, item.AggregateID)
	}

	return method, path, body, nil
}

// extractResourceID 从 SCIM JSON payload 里提取 id 字段；fallback 用 fallbackID。
// 仅解析顶层的 "id"，避免引入 gjson 等新依赖。
func extractResourceID(payload []byte, fallbackID string) string {
	if len(payload) == 0 {
		return fallbackID
	}
	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil {
		return fallbackID
	}
	if id, ok := obj["id"].(string); ok && id != "" {
		return id
	}
	if id, ok := obj["ID"].(string); ok && id != "" {
		return id
	}
	return fallbackID
}

// parseResponseStatus 辅助（预留）。strconv 避免编译警告。
var _ = strconv.Itoa

// 编译期断言
var _ outboxplatform.DeliveryHandler = (*Handler)(nil)
