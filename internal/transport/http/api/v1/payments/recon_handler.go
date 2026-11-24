package payments

import (
	"context"
	"time"

	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

// ReconHandler 是对账模块的 HTTP handler (design doc §8.2)。
// 依赖 apppayments.ReconService（app 层 Service 接口），
// 不直接 import domain — 保持 Clean Architecture 红线 Rule 2 合规。
//
// 与主 Handler 分离：主 Handler 依赖 apppayments.Service (UOW 模式)，
// ReconHandler 依赖独立 ReconService（ReconRepo 尚未挂到 CommerceUoW 上），
// 沿用 NotifyHandler 分离式注册模式。
type ReconHandler struct {
	Svc apppayments.ReconService
}

func NewReconHandler(svc apppayments.ReconService) ReconHandler {
	return ReconHandler{Svc: svc}
}

// ─── Request / Response structs ────────────────────────────────────

type ListReconBatchesRequest struct {
	OrganizationID string
	Page           int
	PageSize       int
}

type ListReconItemsRequest struct {
	BatchID string
}

type ResolveReconItemRequest struct {
	ItemID   string `json:"itemId"`
	Operator string `json:"operator"`
	Note     string `json:"note,omitempty"`
}

// RunReconRequest 手动触发对账 (design doc §8.2 POST /api/v1/pay-recon:run)
// 支持两种模式：
//   - BatchID 非空 → 触发已存在的 pending batch 立即执行（跳过 cron 等待）
//   - BatchID 空 + Channel + BatchDate → 先 CreateBatch 再立即执行
type ApproveReconItemRequest struct {
	ItemID string `json:"itemId"`
	Reason string `json:"reason,omitempty"`
}

type RunReconRequest struct {
	BatchID        string `json:"batchId,omitempty"`
	OrganizationID string `json:"organizationId"`
	Channel        string `json:"channel,omitempty"`
	BatchDate      string `json:"batchDate,omitempty"` // RFC3339, e.g. "2025-10-01T00:00:00Z"
}

type ReconBatchListItem struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organizationId"`
	Channel        string `json:"channel"`
	BatchDate      string `json:"batchDate"`
	Status         string `json:"status"`
	DiffCount      int    `json:"diffCount"`
	TotalLocal     int    `json:"totalLocal"`
	TotalChannel   int    `json:"totalChannel"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type ReconItemView struct {
	ID              string   `json:"id"`
	BatchID         string   `json:"batchId"`
	DiffType        string   `json:"diffType"`
	ChannelOrderNo  *string  `json:"channelOrderNo,omitempty"`
	LocalPaymentID  *string  `json:"localPaymentId,omitempty"`
	ChannelAmount   *float64 `json:"channelAmount,omitempty"`
	LocalAmount     *float64 `json:"localAmount,omitempty"`
	ChannelState    *string  `json:"channelState,omitempty"`
	LocalState      *string  `json:"localState,omitempty"`
	Resolved        bool     `json:"resolved"`
	ResolutionState string   `json:"resolutionState"`
	ApprovedBy      *string  `json:"approvedBy,omitempty"`
	ResolvedBy      *string  `json:"resolvedBy,omitempty"`
	CreatedAt       string   `json:"createdAt"`
}

type ListReconBatchesResponse = presenter.ListResponse[ReconBatchListItem]

// ─── Handler 方法 ──────────────────────────────────────────────────

// ListBatches 返回指定组织的对账批次列表。
func (h ReconHandler) ListBatches(ctx context.Context, req ListReconBatchesRequest) (ListReconBatchesResponse, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	batches, total, err := h.Svc.ListBatches(ctx, req.OrganizationID, page, pageSize)
	if err != nil {
		return ListReconBatchesResponse{}, err
	}
	r := make([]ReconBatchListItem, 0, len(batches))
	for _, b := range batches {
		r = append(r, ReconBatchListItem{
			ID:             b.ID,
			OrganizationID: b.OrganizationID,
			Channel:        b.Channel,
			BatchDate:      b.BatchDate.Format("2006-01-02"),
			Status:         b.Status,
			DiffCount:      b.DiffCount,
			TotalLocal:     b.TotalLocal,
			TotalChannel:   b.TotalChannel,
			CreatedAt:      b.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:      b.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}
	return presenter.NewListResponse(r, page, pageSize, total), nil
}

// ListItems 返回指定 batch 的所有差异项。
func (h ReconHandler) ListItems(ctx context.Context, organizationID string, req ListReconItemsRequest) ([]ReconItemView, error) {
	items, err := h.Svc.ListItems(ctx, organizationID, req.BatchID)
	if err != nil {
		return nil, err
	}
	r := make([]ReconItemView, 0, len(items))
	for _, it := range items {
		r = append(r, ReconItemView{
			ID:              it.ID,
			BatchID:         it.BatchID,
			DiffType:        it.DiffType,
			ChannelOrderNo:  it.ChannelOrderNo,
			LocalPaymentID:  it.LocalPaymentID,
			ChannelAmount:   it.ChannelAmount,
			LocalAmount:     it.LocalAmount,
			ChannelState:    it.ChannelState,
			LocalState:      it.LocalState,
			Resolved:        it.Resolved,
			ResolutionState: it.ResolutionState,
			ApprovedBy:      it.ApprovedBy,
			ResolvedBy:      it.ResolvedBy,
			CreatedAt:       it.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}
	return r, nil
}

// ResolveItem 人工处置一条差异项 (design doc §8.2 审批流入口)。
// 审批流由上层（审批服务）写入 ResolvedBy + ResolvedAt，此处只调 app 层方法。
func (h ReconHandler) ResolveItem(ctx context.Context, organizationID string, req ResolveReconItemRequest) error {
	if req.ItemID == "" {
		return errInvalidArgument("itemId is required")
	}
	session, ok := authtransport.SessionContextFromContext(ctx)
	if !ok || session.UserID == "" {
		return apperrors.NewUnauthorized("authentication required")
	}
	return h.Svc.ResolveItem(ctx, organizationID, req.ItemID, session.UserID)
}

// ApproveItem is the second-person review step: the approver always comes from
// the authenticated session, never from the request body.
func (h ReconHandler) ApproveItem(ctx context.Context, organizationID string, req ApproveReconItemRequest) error {
	if req.ItemID == "" {
		return errInvalidArgument("itemId is required")
	}
	session, ok := authtransport.SessionContextFromContext(ctx)
	if !ok || session.UserID == "" {
		return apperrors.NewUnauthorized("authentication required")
	}
	return h.Svc.ApproveItem(ctx, organizationID, req.ItemID, session.UserID)
}

// RunRecon 手动触发对账 (design doc §8.2)。两种模式：
//  1. BatchID 指定 → 触发已存在的 pending batch 立即执行
//  2. BatchID 空 + Channel + BatchDate → 先 CreateBatch(pending) 再立即执行
func (h ReconHandler) RunRecon(ctx context.Context, req RunReconRequest) (batchID string, err error) {
	if req.BatchID != "" {
		return req.BatchID, h.Svc.RunManual(ctx, req.OrganizationID, req.BatchID)
	}

	// 模式 2: 新建 batch + 立即执行
	if req.OrganizationID == "" || req.Channel == "" || req.BatchDate == "" {
		return "", errInvalidArgument("batchId or (organizationId + channel + batchDate) required")
	}
	batchDate, err := time.Parse("2006-01-02T15:04:05Z", req.BatchDate)
	if err != nil {
		batchDate, err = time.Parse("2006-01-02", req.BatchDate)
		if err != nil {
			return "", errInvalidArgument("batchDate must be RFC3339 or YYYY-MM-DD")
		}
	}
	return h.Svc.CreateAndRun(ctx, req.OrganizationID, req.Channel, batchDate, time.Now().UTC())
}

// errInvalidArgument 统一走 apperrors：本地 error 类型没有 HTTPStatus()/ProtocolCode()，
// route.WriteError 只能落到 500，参数校验失败会被错误地渲染成 server_error。
func errInvalidArgument(msg string) error {
	return apperrors.NewInvalidArgument(msg)
}
