package transactions

import (
	"context"
	"encoding/json"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/transactions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// RecordTransaction creates a transaction record linked to an order/payment event.
func (s Service) RecordTransaction(ctx context.Context, orgID, userID, orderID, paymentID, transactionType string, amount float64, currency, providerType, providerID string, metadata json.RawMessage) (*TransactionDetail, error) {
	if strings.TrimSpace(orgID) == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(transactionType) == "" {
		return nil, apperrors.NewInvalidArgument("transactionType is required")
	}
	// 资金金额守卫（BE-C-15）：账本行只记真实发生的正金额，
	// 负数/零金额会污染对账口径。
	if amount <= 0 {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "amount must be positive", map[string]any{
			"field": "amount", "value": amount, "min": 0,
		})
	}
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}
	now := s.Clock.Now()
	var meta json.RawMessage
	if metadata != nil {
		meta = metadata
	} else {
		meta = json.RawMessage(`{}`)
	}
	t := &domain.Transaction{
		ID: id, OrganizationID: orgID, UserID: userID,
		OrderID: orderID, PaymentID: paymentID,
		TransactionType: transactionType, Amount: amount,
		Currency: currency, Status: domain.StatusCompleted,
		ProviderType: providerType, ProviderID: providerID,
		Metadata: meta, CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	// 落库前重跑领域不变量（类型/状态闭集校验，BE-C-15）：
	// Record 路径此前跳过 Validate，非法 transactionType 可直接落库。
	if err := t.Validate(); err != nil {
		return nil, err
	}
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {

		if err := tx.Transactions().Create(ctx, t); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			t.OrganizationID,
			t.UserID,
			"transaction.created",
			t.ID,
			"transaction.created",
			map[string]any{

				keyTransactionID: t.ID,

				keyName: t.TransactionType,

				keyOrgID: t.OrganizationID,
			},
		); err != nil {

			return err

		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	d := toDetail(t)
	return &d, nil
}

// refundedTotal sums the completed refund rows that reverse the same payment.
// 关联键优先 PaymentID（一笔订单可能有多笔支付），无 PaymentID 时退化为
// OrderID。分页遍历全部退款行，避免单页截断导致上限被低估（BE-C-15）。
func (s Service) refundedTotal(ctx context.Context, original *domain.Transaction) (float64, error) {
	refundType := domain.TypeRefund
	const pageSize = 100
	total := 0.0
	for page := 1; ; page++ {
		items, count, err := s.UOW.Transactions().List(ctx, domain.ListQuery{
			Page: page, PageSize: pageSize,
			OrganizationID:  original.OrganizationID,
			OrderID:         original.OrderID,
			TransactionType: &refundType,
		})
		if err != nil {
			return 0, err
		}
		for _, item := range items {
			if item.Status != domain.StatusCompleted {
				continue
			}
			if original.PaymentID != "" {
				if item.PaymentID != original.PaymentID {
					continue
				}
			} else if item.OrderID != original.OrderID {
				continue
			}
			total += item.Amount
		}
		if int64(page*pageSize) >= count || len(items) == 0 {
			break
		}
	}
	return total, nil
}

// GetOrderTransactions lists all transactions for a specific order.
func (s Service) GetOrderTransactions(ctx context.Context, orgID, orderID string, page, pageSize int) ([]TransactionListItem, int64, error) {
	items, total, err := s.UOW.Transactions().List(ctx, domain.ListQuery{
		Page: page, PageSize: pageSize, OrganizationID: orgID, OrderID: orderID,
	})
	if err != nil {
		return nil, 0, err
	}
	r := make([]TransactionListItem, 0, len(items))
	for _, i := range items {
		r = append(r, toListItem(i))
	}
	return r, total, nil
}

// RefundTransaction creates a refund transaction linked to an original payment
// transaction.
//
// organizationID 是可信租户断言（BE-C-15）：原始交易必须属于该组织，退款行
// 也落在同一组织；金额必须为正且累计退款不得超过原交易金额。
func (s Service) RefundTransaction(ctx context.Context, orgID, originalTxID string, amount float64, metadata json.RawMessage) (*TransactionDetail, error) {
	if strings.TrimSpace(orgID) == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	if amount <= 0 {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "refund amount must be positive", map[string]any{
			"field": "amount", "value": amount, "min": 0,
		})
	}
	original, err := s.UOW.Transactions().GetByIDForOrganization(ctx, originalTxID, orgID)
	if err != nil {
		return nil, err
	}
	if original.TransactionType != "payment" {
		return nil, apperrors.NewInvalidArgument("only payment transactions can be refunded")
	}
	if original.Status != domain.StatusCompleted {
		return nil, apperrors.NewInvalidArgument("only completed transactions can be refunded")
	}
	// 累计退款上限（BE-C-15）：同一笔支付的已完成退款之和 + 本次金额
	// 不得超过原交易金额，否则连续 Refund 可以退出超过收取的钱。
	refunded, err := s.refundedTotal(ctx, original)
	if err != nil {
		return nil, err
	}
	if refunded+amount > original.Amount {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "refund exceeds original transaction amount", map[string]any{
			"field": "amount", "value": amount, "refunded": refunded, "max": original.Amount - refunded,
		})
	}
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}
	now := s.Clock.Now()
	var meta json.RawMessage
	if metadata != nil {
		meta = metadata
	} else {
		meta = json.RawMessage(`{}`)
	}
	t := &domain.Transaction{
		ID: id, OrganizationID: original.OrganizationID, UserID: original.UserID,
		OrderID: original.OrderID, PaymentID: original.PaymentID,
		TransactionType: "refund", Amount: amount,
		Currency: original.Currency, Status: domain.StatusCompleted,
		ProviderType: original.ProviderType, ProviderID: original.ProviderID,
		Metadata: meta, CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	if err := t.Validate(); err != nil {
		return nil, err
	}
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {

		if err := tx.Transactions().Create(ctx, t); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			t.OrganizationID,
			t.UserID,
			"transaction.refunded",
			t.ID,
			"transaction.refunded",
			map[string]any{

				keyTransactionID: t.ID,

				keyName: t.TransactionType,

				keyOrgID: t.OrganizationID,
			},
		); err != nil {

			return err

		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	d := toDetail(t)
	return &d, nil
}
