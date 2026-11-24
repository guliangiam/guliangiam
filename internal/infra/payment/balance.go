package payment

import (
	"context"
	"fmt"
	"strings"
)

// BalanceGateway implements PaymentGateway using the user's account balance.
// This is an internal payment method that deducts from the user's balance
// without involving any external payment provider.
type BalanceGateway struct{}

// NewBalanceGateway creates a new balance payment gateway.
func NewBalanceGateway() *BalanceGateway {
	return &BalanceGateway{}
}

// CreatePayment for balance payments is a no-op — the actual balance deduction
// happens in the order service within a transaction.
// We return a placeholder payment URL that indicates balance payment.
func (g *BalanceGateway) CreatePayment(ctx context.Context, orderID string, amount float64, currency, subject, notifyURL, returnUrl string) (*PaymentURLResult, error) {
	return &PaymentURLResult{
		PaymentURL: returnUrl,
		TradeNo:    fmt.Sprintf("BAL-%s", orderID),
	}, nil
}

// QueryPayment returns a paid status for balance payments since the deduction
// happens synchronously during order creation.
func (g *BalanceGateway) QueryPayment(ctx context.Context, outTradeNo string) (*PaymentStatusResult, error) {
	if strings.HasPrefix(outTradeNo, "BAL-") {
		return &PaymentStatusResult{
			TradeNo: outTradeNo,
			Status:  statusPaid,
			Amount:  0, // amount is tracked in the order
		}, nil
	}
	return &PaymentStatusResult{
		TradeNo: outTradeNo,
		Status:  statusPending,
	}, nil
}

// Refund for balance payments adds the refund amount back to the user's balance.
// The actual balance restoration is handled by the order service within a transaction.
func (g *BalanceGateway) Refund(ctx context.Context, outTradeNo string, refundAmount float64, refundReason string) (*RefundResult, error) {
	return &RefundResult{
		RefundNo: fmt.Sprintf("REF-%s", outTradeNo),
		Status:   statusSuccess,
	}, nil
}

// VerifyNotify for balance payments always returns success since there is no
// external notification — the payment is processed synchronously.
func (g *BalanceGateway) VerifyNotify(ctx context.Context, params map[string]string) (*NotifyResult, error) {
	outTradeNo := params[fieldOrderID]
	return &NotifyResult{
		OutTradeNo: outTradeNo,
		Status:     statusPaid,
	}, nil
}
