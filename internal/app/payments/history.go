// history.go — Payments history (attempts timeline + refund list) read-side service.
//
// 设计文档: G5 payments 生命周期查询
//
// 纯粹的查询聚合层 —— 不做写操作，不承担业务决策。
// 存在的唯一原因: Clean Architecture Rule 2 禁止 transport → domain 直接依赖，
// handler 需要通过 app 层接口才能访问 domain repo。
//
// 职责:
//   - AttemptsHandler.GetByPayment → Attempts.ListByPayment → domain.AttemptsRepo
//   - RefundsHandler.GetByPayment  → Refunds.ListByPayment  → domain.RefundsRepo
package payments

import (
	"context"

	domainpayments "github.com/guliangiam/guliangiam/internal/domain/payments"
)

// PaymentHistoryService 是 transport 层 attempts/refunds handler 的 app 层端口。
// 保持为最小 read-only 接口 —— 任何写操作都不应该走这里。
type PaymentHistoryService interface {
	ListAttemptsByPayment(ctx context.Context, organizationID, paymentID string) ([]*domainpayments.PayAttempt, error)
	GetAttemptByID(ctx context.Context, organizationID, id string) (*domainpayments.PayAttempt, error)
	ListRefundsByPayment(ctx context.Context, organizationID, paymentID string) ([]*domainpayments.Refund, error)
	GetRefundByID(ctx context.Context, organizationID, id string) (*domainpayments.Refund, error)
}

// historyService 是一个直接把 domain repo 暴露出来的薄壳。
// 之所以走 app 层 —— 纯粹是为了让 transport 不直接 import domain。
type historyService struct {
	attempts domainpayments.AttemptsRepo
	refunds  domainpayments.RefundsRepo
}

func NewHistoryService(
	attempts domainpayments.AttemptsRepo,
	refunds domainpayments.RefundsRepo,
) PaymentHistoryService {
	return &historyService{attempts: attempts, refunds: refunds}
}

func (s *historyService) ListAttemptsByPayment(ctx context.Context, organizationID, paymentID string) ([]*domainpayments.PayAttempt, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	return s.attempts.ListByPaymentForOrganization(ctx, paymentID, organizationID)
}
func (s *historyService) GetAttemptByID(ctx context.Context, organizationID, id string) (*domainpayments.PayAttempt, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	return s.attempts.GetByIDForOrganization(ctx, id, organizationID)
}
func (s *historyService) ListRefundsByPayment(ctx context.Context, organizationID, paymentID string) ([]*domainpayments.Refund, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	return s.refunds.ListByPaymentForOrganization(ctx, paymentID, organizationID)
}
func (s *historyService) GetRefundByID(ctx context.Context, organizationID, id string) (*domainpayments.Refund, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	return s.refunds.GetByIDForOrganization(ctx, id, organizationID)
}
