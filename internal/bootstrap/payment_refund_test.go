package bootstrap

import (
	"context"
	"errors"
	"testing"

	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainpayments "github.com/guliangiam/guliangiam/internal/domain/payments"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRefundUow 只实现 WithinTransaction，把预置 tx 交给回调；
// 其余 UnitOfWork 能力经嵌入 nil 接口满足类型（不会被调用）。
type fakeRefundUow struct {
	appuow.UnitOfWork
	tx appuow.UnitOfWork
}

func (f fakeRefundUow) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, f.tx)
}

type txNoCapabilities struct{ appuow.UnitOfWork }

type txPaymentsOnly struct{ appuow.UnitOfWork }

func (txPaymentsOnly) Payments() domainpayments.Repository { return nil }

type refundsNoLock struct{ domainpayments.RefundsRepo }

type txRefundsNoLock struct{ appuow.UnitOfWork }

func (txRefundsNoLock) Payments() domainpayments.Repository { return nil }
func (txRefundsNoLock) Refunds() domainpayments.RefundsRepo { return refundsNoLock{} }

type refundsWithLock struct {
	domainpayments.RefundsRepo
	lockErr error
}

func (r refundsWithLock) LockPaymentForRefund(_ context.Context, _ string) (*domainpayments.LockedPayment, error) {
	return nil, r.lockErr
}

type txFull struct {
	appuow.UnitOfWork
	lockErr error
}

func (txFull) Payments() domainpayments.Repository { return nil }
func (t txFull) Refunds() domainpayments.RefundsRepo {
	return refundsWithLock{lockErr: t.lockErr}
}

// BE-C-09：能力缺失必须 fail-closed（500），不允许退化为无锁/无归属校验退款。
func TestRefundExecutor_FailClosedOnMissingCapabilities(t *testing.T) {
	cases := []struct {
		name string
		tx   appuow.UnitOfWork
	}{
		{"no capabilities", txNoCapabilities{}},
		{"payments only", txPaymentsOnly{}},
		{"refunds without row lock", txRefundsNoLock{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			executor := refundExecutor{uow: fakeRefundUow{tx: tc.tx}, registry: apppayments.NewGatewayRegistry()}
			_, err := executor.ExecuteRefund(context.Background(), "pay-1", "org-1", 5, "")
			require.Error(t, err)
			var appErr *apperrors.Error
			require.True(t, errors.As(err, &appErr))
			assert.Equal(t, 500, appErr.HTTPStatus())
		})
	}
}

// 金额门禁在编排器内先行：amount<=0 → 422，证明编排器确在事务内被构造执行。
func TestRefundExecutor_OrchestratorRunsInsideTx(t *testing.T) {
	executor := refundExecutor{uow: fakeRefundUow{tx: txFull{}}, registry: apppayments.NewGatewayRegistry()}
	_, err := executor.ExecuteRefund(context.Background(), "pay-1", "org-1", 0, "")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, 422, appErr.HTTPStatus())
}

// 锁错误原样上抛（NotFound → 404），事务回滚由 UoW 负责。
func TestRefundExecutor_PropagatesLockError(t *testing.T) {
	executor := refundExecutor{
		uow:      fakeRefundUow{tx: txFull{lockErr: apperrors.NewNotFound("payment not found")}},
		registry: apppayments.NewGatewayRegistry(),
	}
	_, err := executor.ExecuteRefund(context.Background(), "pay-1", "org-1", 5, "")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, 404, appErr.HTTPStatus())
}
