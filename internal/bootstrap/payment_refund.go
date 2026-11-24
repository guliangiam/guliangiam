package bootstrap

import (
	"context"

	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainpayments "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// refundExecutor 是 app/payments.RefundExecutor 的组合根实现（BE-C-09）。
// RefundOrchestrator 假设整个流程在一个事务里：这里开启 UoW 事务（RLS GUC
// 随事务注入），把 tx 绑定的 payments/refunds 仓储与 SELECT FOR UPDATE 锁
// 注入编排器。能力断言失败一律 fail-closed，不允许退化为无锁退款。
type refundExecutor struct {
	uow      appuow.UnitOfWork
	registry *apppayments.GatewayRegistry
}

var _ apppayments.RefundExecutor = refundExecutor{}

func (e refundExecutor) ExecuteRefund(ctx context.Context, paymentID, organizationID string, amount float64, reason string) (*domainpayments.Refund, error) {
	var out *domainpayments.Refund
	err := e.uow.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		paymentsProvider, ok := tx.(interface {
			Payments() domainpayments.Repository
		})
		if !ok {
			return apperrors.NewInternal("refund unavailable: transaction lacks payments repository")
		}
		refundsProvider, ok := tx.(interface {
			Refunds() domainpayments.RefundsRepo
		})
		if !ok {
			return apperrors.NewInternal("refund unavailable: transaction lacks refunds repository")
		}
		refunds := refundsProvider.Refunds()
		locker, ok := refunds.(interface {
			LockPaymentForRefund(ctx context.Context, paymentID string) (*domainpayments.LockedPayment, error)
		})
		if !ok {
			return apperrors.NewInternal("refund unavailable: refunds repository lacks row lock")
		}
		orchestrator := &apppayments.RefundOrchestrator{
			Clock:       clock.SystemClock{},
			UUID:        platformuuid.V7Generator{},
			Payments:    paymentsProvider.Payments(),
			Refunds:     refunds,
			Registry:    e.registry,
			LockPayment: locker.LockPaymentForRefund,
		}
		refund, err := orchestrator.Execute(ctx, paymentID, organizationID, amount, reason)
		if err != nil {
			return err
		}
		out = refund
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
