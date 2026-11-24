// audit.go — CallbackReceipts admin read service.
//
// 设计文档 G7: 渠道回调审计
//
// 与 history.go 分离 —— attempts/refunds 是 payment-scoped (ListByPayment)，
// callback receipts 是 organization-scoped (List by org) + GetByPayloadHash/EventKey。
// 不同查询形状 → 独立 app 层端口。
package payments

import (
	"context"

	domainpayments "github.com/guliangiam/guliangiam/internal/domain/payments"
)

// PaymentAuditService 提供 payment 管理面的审计查询（callback receipts）。
type PaymentAuditService interface {
	ListCallbackReceipts(ctx context.Context, orgID string, limit, offset int) ([]*domainpayments.CallbackReceipt, int64, error)
}

type paymentAuditService struct {
	receipts domainpayments.CallbackReceiptsRepo
}

func NewPaymentAuditService(receipts domainpayments.CallbackReceiptsRepo) PaymentAuditService {
	return &paymentAuditService{receipts: receipts}
}

func (s *paymentAuditService) ListCallbackReceipts(ctx context.Context, orgID string, limit, offset int) ([]*domainpayments.CallbackReceipt, int64, error) {
	return s.receipts.List(ctx, orgID, limit, offset)
}
