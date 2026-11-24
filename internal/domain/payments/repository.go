package payments

import (
	"context"
	"time"

	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
)

// ListQuery defines the query parameters for listing payments.
type ListQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	User                    string
	State                   *string
	CreatedFrom             *time.Time // batch reconciliation date range
	CreatedTo               *time.Time
}

// Repository defines the interface for payments persistence operations.
type Repository interface {
	// GetByID is the system-facing lookup used by payment callbacks and charge
	// orchestration, where no HTTP tenant scope exists. Management HTTP routes
	// must use GetByIDForOrganization.
	GetByID(ctx context.Context, id string) (*Payment, error)
	// GetByIDForOrganization returns the payment only when it belongs to the
	// trusted tenant; a foreign id is indistinguishable from a missing one.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Payment, error)
	GetByChannelOrderNo(ctx context.Context, channel, orderNo string) (*Payment, error)
	GetByOutTradeNo(ctx context.Context, channel, outTradeNo string) (*Payment, error)
	List(ctx context.Context, q ListQuery) ([]*Payment, int64, error)
	Create(ctx context.Context, payment *Payment) error
	// Update applies organization_id = payment.OrganizationID, so an entity read
	// under one tenant cannot rewrite a row owned by another.
	Update(ctx context.Context, payment *Payment) error
	Delete(ctx context.Context, id, organizationID string) error
}

// CallbackReceiptsRepo persists raw callback receipts (五阶段管线 stage 2).
type CallbackReceiptsRepo interface {
	GetByPayloadHash(ctx context.Context, channel, sha256 string) (*CallbackReceipt, error)
	GetByEventKey(ctx context.Context, channel, eventKey string) (*CallbackReceipt, error)
	Create(ctx context.Context, r *CallbackReceipt) error
	Update(ctx context.Context, r *CallbackReceipt) error
	List(ctx context.Context, organizationID string, limit, offset int) ([]*CallbackReceipt, int64, error)
}

// AttemptsRepo persists each channel interaction (charge/query/refund/close/callback).
// The unscoped reads are kept for system callers; management HTTP routes use the
// ForOrganization variants.
type AttemptsRepo interface {
	GetByID(ctx context.Context, id string) (*PayAttempt, error)
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*PayAttempt, error)
	ListByPayment(ctx context.Context, paymentID string) ([]*PayAttempt, error)
	ListByPaymentForOrganization(ctx context.Context, paymentID, organizationID string) ([]*PayAttempt, error)
	ListByRefund(ctx context.Context, refundID string) ([]*PayAttempt, error)
	Create(ctx context.Context, a *PayAttempt) error
}

// RefundsRepo persists refund aggregates. Management HTTP routes use the
// ForOrganization variants so MySQL (no RLS) cannot cross tenants by id.
type RefundsRepo interface {
	GetByID(ctx context.Context, id string) (*Refund, error)
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Refund, error)
	GetByOutRefundNo(ctx context.Context, channel, outRefundNo string) (*Refund, error)
	ListByPayment(ctx context.Context, paymentID string) ([]*Refund, error)
	ListByPaymentForOrganization(ctx context.Context, paymentID, organizationID string) ([]*Refund, error)
	List(ctx context.Context, organizationID string, limit, offset int) ([]*Refund, int64, error)
	Create(ctx context.Context, r *Refund) error
	Update(ctx context.Context, r *Refund) error
}

// ChannelConfigsRepo persists tenant channel credentials (versioned).
type ChannelConfigsRepo interface {
	GetByID(ctx context.Context, id string) (*PayChannelConfig, error)
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*PayChannelConfig, error)
	GetActive(ctx context.Context, organizationID, channel, environment string) (*PayChannelConfig, error)
	List(ctx context.Context, organizationID string) ([]*PayChannelConfig, error)
	Create(ctx context.Context, c *PayChannelConfig) error
	Update(ctx context.Context, c *PayChannelConfig) error
}

// ReconRepo persists reconciliation batches and items.
// The unscoped reads are used by the reconcile worker; management HTTP routes
// use the ForOrganization variants.
type ReconRepo interface {
	Batches(ctx context.Context, organizationID string, limit, offset int) ([]*ReconBatch, int64, error)
	GetBatch(ctx context.Context, id string) (*ReconBatch, error)
	GetBatchForOrganization(ctx context.Context, id, organizationID string) (*ReconBatch, error)
	CreateBatch(ctx context.Context, b *ReconBatch) error
	UpdateBatch(ctx context.Context, b *ReconBatch) error
	CreateItem(ctx context.Context, item *ReconItem) error
	ListItems(ctx context.Context, batchID string) ([]*ReconItem, error)
	ListItemsForOrganization(ctx context.Context, batchID, organizationID string) ([]*ReconItem, error)
	ResolveItem(ctx context.Context, itemID, operator string) error
	ResolveItemForOrganization(ctx context.Context, itemID, organizationID, operator string) error
	ApproveItemForOrganization(ctx context.Context, itemID, organizationID, approver string) (string, error)
}

// ReconApprovalAuditor 是 ReconRepo 的可选能力（capability interface，先例：
// TenantProvisioningUoW）：复核差异项与审计记录/Outbox 事件必须同事务提交
// （BE-C-18）。buildAudit 在事务内、approve 成功后被调用，入参是原处置人；
// 返回的 record/event 任一为 nil 则跳过对应写入。不支持原子提交的镜像仓储
// （如 MySQL）不实现本接口，调用方回退到旧的两步路径。
type ReconApprovalAuditor interface {
	ApproveItemWithAudit(ctx context.Context, itemID, organizationID, approver string, buildAudit func(originalOperator string) (*domainrecords.Record, *domainoutbox.OutboxEvent, error)) (string, error)
}

// ReconRunner 是对账执行端口（Clean Architecture Port）。
// transport 层通过此端口手动触发对账（POST /api/v1/pay-recon:run）。
// app/payments.ReconcileWorker 实现此接口。
type ReconRunner interface {
	RunManual(ctx context.Context, batchID string) error
}
