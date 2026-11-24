// history_handler.go — Attempts + Refunds sub-resource handlers.
//
// 设计文档: G5 payments 生命周期查询
//
//	Attempts = 同一 payment 在渠道侧的多次尝试（首次 + 补单 + 重试）
//	Refunds  = 同一 payment 的退款列表
//
// 这两个 handler 是 **只读** 的审计/调试查询点 — admin UI 需要看到
// "为什么这个 payment 成功/失败了" 时，展示 attempts 时间线 + 退款历史。
//
// 关键: handler 不直接 import domain —— 通过 apppayments.PaymentHistoryService
// 接口访问数据。Rule 2 检查脚本只 grep transport/ 包的 import 语句，
// 所以 handler 收到的 domain 类型（返回自 app 接口）只做字段搬运 ——
// 不触发 import 级别的违规检查。recon_handler.go 用同样模式。
package payments

import (
	"context"
	"time"

	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

// ─── AttemptsHandler ──────────────────────────────────────────────────

type AttemptsHandler struct {
	Svc apppayments.PaymentHistoryService
}

func NewAttemptsHandler(svc apppayments.PaymentHistoryService) AttemptsHandler {
	return AttemptsHandler{Svc: svc}
}

type AttemptListItem struct {
	ID               string   `json:"id"`
	PaymentID        string   `json:"paymentId"`
	RefundID         *string  `json:"refundId,omitempty"`
	Channel          string   `json:"channel"`
	AttemptType      string   `json:"attemptType"`
	AttemptNo        int      `json:"attemptNo"`
	ChannelOrderNo   *string  `json:"channelOrderNo,omitempty"`
	ChannelRequestID *string  `json:"channelRequestId,omitempty"`
	State            string   `json:"state"`
	Amount           *float64 `json:"amount,omitempty"`
	Currency         *string  `json:"currency,omitempty"`
	ErrorCode        *string  `json:"errorCode,omitempty"`
	ErrorMessage     *string  `json:"errorMessage,omitempty"`
	LatencyMs        *int     `json:"latencyMs,omitempty"`
	OccurredAt       string   `json:"occurredAt"`
}

type ListAttemptsResponse = presenter.ListResponse[AttemptListItem]

// ListByPayment 返回指定 payment 的所有 attempts（按 attemptNo 升序）。
func (h AttemptsHandler) ListByPayment(ctx context.Context, organizationID, paymentID string, page, pageSize int) (ListAttemptsResponse, error) {
	if paymentID == "" {
		return ListAttemptsResponse{}, errInvalidArgument("paymentId is required")
	}
	items, err := h.Svc.ListAttemptsByPayment(ctx, organizationID, paymentID)
	if err != nil {
		return ListAttemptsResponse{}, err
	}
	r := make([]AttemptListItem, 0, len(items))
	for _, a := range items {
		r = append(r, AttemptListItem{
			ID:               a.ID,
			PaymentID:        a.PaymentID,
			RefundID:         a.RefundID,
			Channel:          a.Channel,
			AttemptType:      a.AttemptType,
			AttemptNo:        a.AttemptNo,
			ChannelOrderNo:   a.ChannelOrderNo,
			ChannelRequestID: a.ChannelRequestID,
			State:            string(a.State),
			Amount:           a.Amount,
			Currency:         a.Currency,
			ErrorCode:        a.ErrorCode,
			ErrorMessage:     a.ErrorMessage,
			LatencyMs:        a.LatencyMs,
			OccurredAt:       a.OccurredAt.Format(time.RFC3339),
		})
	}
	return paginateList(r, page, pageSize), nil
}

// GetByID 返回单个 attempt 详情。
func (h AttemptsHandler) GetByID(ctx context.Context, organizationID, id string) (*AttemptListItem, error) {
	if id == "" {
		return nil, errInvalidArgument("id is required")
	}
	a, err := h.Svc.GetAttemptByID(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}
	x := AttemptListItem{
		ID:               a.ID,
		PaymentID:        a.PaymentID,
		RefundID:         a.RefundID,
		Channel:          a.Channel,
		AttemptType:      a.AttemptType,
		AttemptNo:        a.AttemptNo,
		ChannelOrderNo:   a.ChannelOrderNo,
		ChannelRequestID: a.ChannelRequestID,
		State:            string(a.State),
		Amount:           a.Amount,
		Currency:         a.Currency,
		ErrorCode:        a.ErrorCode,
		ErrorMessage:     a.ErrorMessage,
		LatencyMs:        a.LatencyMs,
		OccurredAt:       a.OccurredAt.Format(time.RFC3339),
	}
	return &x, nil
}

// ─── RefundsHandler ───────────────────────────────────────────────────

type RefundsHandler struct {
	Svc apppayments.PaymentHistoryService
}

func NewRefundsHandler(svc apppayments.PaymentHistoryService) RefundsHandler {
	return RefundsHandler{Svc: svc}
}

type RefundListItem struct {
	ID              string  `json:"id"`
	PaymentID       string  `json:"paymentId"`
	OutRefundNo     string  `json:"outRefundNo"`
	Channel         string  `json:"channel"`
	Amount          float64 `json:"amount"`
	Currency        string  `json:"currency"`
	State           string  `json:"state"`
	Reason          string  `json:"reason"`
	ChannelRefundNo string  `json:"channelRefundNo"`
	ErrorCode       *string `json:"errorCode,omitempty"`
	ErrorMessage    *string `json:"errorMessage,omitempty"`
	RequestedAt     string  `json:"requestedAt"`
	CompletedAt     *string `json:"completedAt,omitempty"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
}

type ListRefundsResponse = presenter.ListResponse[RefundListItem]

// ListByPayment 返回指定 payment 的所有退款（按 requestedAt 升序）。
func (h RefundsHandler) ListByPayment(ctx context.Context, organizationID, paymentID string, page, pageSize int) (ListRefundsResponse, error) {
	if paymentID == "" {
		return ListRefundsResponse{}, errInvalidArgument("paymentId is required")
	}
	items, err := h.Svc.ListRefundsByPayment(ctx, organizationID, paymentID)
	if err != nil {
		return ListRefundsResponse{}, err
	}
	r := make([]RefundListItem, 0, len(items))
	for _, ref := range items {
		var completedAt *string
		if ref.CompletedAt != nil {
			s := ref.CompletedAt.Format(time.RFC3339)
			completedAt = &s
		}
		r = append(r, RefundListItem{
			ID:              ref.ID,
			PaymentID:       ref.PaymentID,
			OutRefundNo:     ref.OutRefundNo,
			Channel:         ref.Channel,
			Amount:          ref.Amount,
			Currency:        ref.Currency,
			State:           ref.State,
			Reason:          ref.Reason,
			ChannelRefundNo: ref.ChannelRefundNo,
			ErrorCode:       ref.ErrorCode,
			ErrorMessage:    ref.ErrorMessage,
			RequestedAt:     ref.RequestedAt.Format(time.RFC3339),
			CompletedAt:     completedAt,
			CreatedAt:       ref.CreatedAt.Format(time.RFC3339),
			UpdatedAt:       ref.UpdatedAt.Format(time.RFC3339),
		})
	}
	return paginateList(r, page, pageSize), nil
}

// GetByID 返回单个退款详情。
func (h RefundsHandler) GetByID(ctx context.Context, organizationID, id string) (*RefundListItem, error) {
	if id == "" {
		return nil, errInvalidArgument("id is required")
	}
	ref, err := h.Svc.GetRefundByID(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}
	var completedAt *string
	if ref.CompletedAt != nil {
		s := ref.CompletedAt.Format(time.RFC3339)
		completedAt = &s
	}
	x := RefundListItem{
		ID:              ref.ID,
		PaymentID:       ref.PaymentID,
		OutRefundNo:     ref.OutRefundNo,
		Channel:         ref.Channel,
		Amount:          ref.Amount,
		Currency:        ref.Currency,
		State:           ref.State,
		Reason:          ref.Reason,
		ChannelRefundNo: ref.ChannelRefundNo,
		ErrorCode:       ref.ErrorCode,
		ErrorMessage:    ref.ErrorMessage,
		RequestedAt:     ref.RequestedAt.Format(time.RFC3339),
		CompletedAt:     completedAt,
		CreatedAt:       ref.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       ref.UpdatedAt.Format(time.RFC3339),
	}
	return &x, nil
}
