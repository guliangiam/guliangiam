package payment

import "context"

// PaymentGateway is the interface for payment gateways.
type PaymentGateway interface {
	// CreatePayment creates a payment and returns the payment URL for the user to complete payment.
	CreatePayment(ctx context.Context, orderID string, amount float64, currency string, subject string, notifyURL string, returnUrl string) (*PaymentURLResult, error)
	// QueryPayment queries the payment status by the out trade number.
	QueryPayment(ctx context.Context, outTradeNo string) (*PaymentStatusResult, error)
	// Refund processes a refund for a payment.
	Refund(ctx context.Context, outTradeNo string, refundAmount float64, refundReason string) (*RefundResult, error)
	// VerifyNotify verifies a payment notification/callback from the provider.
	VerifyNotify(ctx context.Context, params map[string]string) (*NotifyResult, error)
}

// KeyedRefunder is an optional interface for gateways whose refund
// idempotency depends on a business refund number (Stripe/Square
// Idempotency-Key, WeChat out_refund_no, Alipay out_request_no).
// ChannelAdapter forwards RefundRequest.OutRefundNo to these gateways so two
// same-amount partial refunds on one order cannot collide on a synthesized
// order+amount key; gateways without this interface keep the legacy 3-arg path.
type KeyedRefunder interface {
	RefundWithOutRefundNo(ctx context.Context, outTradeNo string, refundAmount float64, refundReason, outRefundNo string) (*RefundResult, error)
}

// PaymentURLResult contains the result of creating a payment.
type PaymentURLResult struct {
	PaymentURL string // URL to redirect user for payment
	TradeNo    string // gateway trade number
}

// PaymentStatusResult contains the status of a payment.
type PaymentStatusResult struct {
	TradeNo string  // gateway trade number
	Status  string  // "pending", "paid", "failed", "refunded"
	Amount  float64 // payment amount
}

// RefundResult contains the result of a refund.
type RefundResult struct {
	RefundNo string // refund transaction number
	Status   string // "success", "failed", "processing"
}

// NotifyResult contains the result of verifying a payment notification.
type NotifyResult struct {
	OutTradeNo string  // merchant order number
	Status     string  // "paid", "failed"
	Amount     float64 // payment amount
}
