// callback_receipts_handler.go — CallbackReceipts admin read surface.
//
// 设计文档 G7: 渠道回调审计 (Callback Receipts)
// pay_callback_receipts 表记录每一笔渠道回调的原始 payload (BYTEA) + headers +
// 处理状态 —— 运营调试 "渠道是否真的回调了 / 回调 payload 是否符合预期" 的唯一权威来源。
//
// Handler 保持薄壳：List(Get) → 调 domain repo → presenter.ListResponse 输出。
// 不 import domain 类型 —— 通过 apppayments.PaymentAuditService 接口拿到数据，
// 保持 Rule 2 (transport → app only) 合规。
package payments

import (
	"context"
	"time"

	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type CallbackReceiptsHandler struct {
	Svc apppayments.PaymentAuditService
}

func NewCallbackReceiptsHandler(svc apppayments.PaymentAuditService) CallbackReceiptsHandler {
	return CallbackReceiptsHandler{Svc: svc}
}

type CallbackReceiptListItem struct {
	ID            string         `json:"id"`
	Channel       string         `json:"channel"`
	PayloadSHA256 string         `json:"payloadSha256"`
	ReceivedAt    string         `json:"receivedAt"`
	ClientIP      *string        `json:"clientIp,omitempty"`
	ProcessState  string         `json:"processState"`
	EventKey      *string        `json:"eventKey,omitempty"`
	OutTradeNo    *string        `json:"outTradeNo,omitempty"`
	ErrorMessage  *string        `json:"errorMessage,omitempty"`
	Headers       map[string]any `json:"headers,omitempty"`
}

type ListCallbackReceiptsResponse = presenter.ListResponse[CallbackReceiptListItem]

func (h CallbackReceiptsHandler) List(
	ctx context.Context,
	orgID string,
	page, pageSize int,
) (ListCallbackReceiptsResponse, error) {
	if orgID == "" {
		return ListCallbackReceiptsResponse{}, errInvalidArgument("organizationId is required")
	}
	pageSize = platformpagination.ClampPageSize(pageSize)
	offset := platformpagination.Offset(page, pageSize)

	items, total, err := h.Svc.ListCallbackReceipts(ctx, orgID, pageSize, offset)
	if err != nil {
		return ListCallbackReceiptsResponse{}, err
	}

	r := make([]CallbackReceiptListItem, 0, len(items))
	for _, it := range items {
		var clientIP *string
		if it.ClientIP != nil {
			s := *it.ClientIP
			clientIP = &s
		}
		r = append(r, CallbackReceiptListItem{
			ID:            it.ID,
			Channel:       it.Channel,
			PayloadSHA256: it.PayloadSHA256,
			ReceivedAt:    it.ReceivedAt.Format(time.RFC3339),
			ClientIP:      clientIP,
			ProcessState:  it.ProcessState,
			EventKey:      it.EventKey,
			OutTradeNo:    it.OutTradeNo,
			ErrorMessage:  it.ErrorMessage,
			Headers:       it.Headers,
		})
	}
	return presenter.NewListResponse(r, page, pageSize, total), nil
}
