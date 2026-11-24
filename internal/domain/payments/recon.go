package payments

import (
	"fmt"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type ReconBatchStatus string

const (
	ReconBatchStatusPending    ReconBatchStatus = "pending"
	ReconBatchStatusProcessing ReconBatchStatus = "processing"
	ReconBatchStatusCompleted  ReconBatchStatus = "completed"
	ReconBatchStatusFailed     ReconBatchStatus = "failed"
)

type DiffType string

const (
	DiffMissingLocal   DiffType = "missing_local"
	DiffMissingChannel DiffType = "missing_channel"
	DiffAmountMismatch DiffType = "amount_mismatch"
	DiffStateMismatch  DiffType = "state_mismatch"
	DiffRefundMismatch DiffType = "refund_mismatch"
)

// reconBatchStatuses / reconDiffTypes are the closed sets the checks and the
// 422 `allowed` metadata share.
var reconBatchStatuses = []string{
	string(ReconBatchStatusPending), string(ReconBatchStatusProcessing),
	string(ReconBatchStatusCompleted), string(ReconBatchStatusFailed),
}

var reconDiffTypes = []string{
	string(DiffMissingLocal), string(DiffMissingChannel), string(DiffAmountMismatch),
	string(DiffStateMismatch), string(DiffRefundMismatch),
}

type ReconBatch struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	Channel        string     `json:"channel"`
	BatchDate      time.Time  `json:"batchDate"`
	Status         string     `json:"status"`
	DiffCount      int        `json:"diffCount"`
	TotalLocal     int        `json:"totalLocal"`
	TotalChannel   int        `json:"totalChannel"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	DeletedAt      *time.Time `json:"deletedAt,omitempty"`
	Revision       int64      `json:"revision"`
}

func (b *ReconBatch) Validate() error {
	if b.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if b.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if b.Channel == "" {
		return apperrors.NewInvalidArgument("channel is required")
	}
	if b.BatchDate.IsZero() {
		return apperrors.NewInvalidArgument("batchDate is required")
	}
	if b.Status == "" {
		b.Status = string(ReconBatchStatusPending)
	}
	switch ReconBatchStatus(b.Status) {
	case ReconBatchStatusPending, ReconBatchStatusProcessing,
		ReconBatchStatusCompleted, ReconBatchStatusFailed:
	default:
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid recon batch status: %s", b.Status),
			map[string]any{"field": "status", "value": b.Status, "allowed": append([]string{}, reconBatchStatuses...)},
		)
	}
	return nil
}

type ReconItem struct {
	ID              string     `json:"id"`
	BatchID         string     `json:"batchId"`
	DiffType        string     `json:"diffType"`
	ChannelOrderNo  *string    `json:"channelOrderNo,omitempty"`
	LocalPaymentID  *string    `json:"localPaymentId,omitempty"`
	ChannelAmount   *float64   `json:"channelAmount,omitempty"`
	LocalAmount     *float64   `json:"localAmount,omitempty"`
	ChannelState    *string    `json:"channelState,omitempty"`
	LocalState      *string    `json:"localState,omitempty"`
	Resolved        bool       `json:"resolved"`
	ResolutionState string     `json:"resolutionState"`
	ApprovedBy      *string    `json:"approvedBy,omitempty"`
	ApprovedAt      *time.Time `json:"approvedAt,omitempty"`
	ResolvedBy      *string    `json:"resolvedBy,omitempty"`
	ResolvedAt      *time.Time `json:"resolvedAt,omitempty"`
	Note            *string    `json:"note,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
}

func (i *ReconItem) Validate() error {
	if i.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if i.BatchID == "" {
		return apperrors.NewInvalidArgument("batchId is required")
	}
	switch DiffType(i.DiffType) {
	case DiffMissingLocal, DiffMissingChannel, DiffAmountMismatch,
		DiffStateMismatch, DiffRefundMismatch:
	default:
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid diffType: %s", i.DiffType),
			map[string]any{"field": "diffType", "value": i.DiffType, "allowed": append([]string{}, reconDiffTypes...)},
		)
	}
	if i.ResolutionState == "" {
		i.ResolutionState = "pending"
	}
	switch i.ResolutionState {
	case "pending", "resolved", "approved", "rejected":
	default:
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid resolutionState: %s", i.ResolutionState),
			map[string]any{"field": "resolutionState", "value": i.ResolutionState})
	}
	return nil
}
