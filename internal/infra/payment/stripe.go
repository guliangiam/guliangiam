package payment

import (
	"context"
	"math"
	"strconv"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/stripe/stripe-go/v82"
)

// StripeGateway implements PaymentGateway via Stripe.
//
// 校准（OPEN-API-75 P8，https://docs.stripe.com）：
//   - 改用 v82 新式 per-gateway *stripe.Client，消除原实现每次调用写全局
//     stripe.Key 的跨租户凭据竞态（多租户并发下单会串用 API key）；
//   - POST 重试携带 Idempotency-Key（https://docs.stripe.com/api/idempotent_requests）：
//     Checkout Session 以 orderID、退款以业务退款单号 outRefundNo 为键
//     （无退款单号的遗留调用回退 outTradeNo+金额分）；
//   - 金额分转换使用 math.Round，修复 float64 直接截断（88.88*100→8887）的资损偏差；
//   - Webhook Stripe-Signature（t=,v1= HMAC-SHA256 + 时间戳容差）验签在 transport
//     层经官方 webhook.ConstructEventWithOptions 完成（payments/notify.go，已复核）。
type StripeGateway struct {
	APIKey string
	client *stripe.Client
}

// NewStripeGateway creates a new Stripe payment gateway.
func NewStripeGateway(apiKey string) *StripeGateway {
	return &StripeGateway{
		APIKey: apiKey,
		client: stripe.NewClient(apiKey),
	}
}

// newStripeGatewayWithClient wires a pre-configured client (httptest backend in tests).
func newStripeGatewayWithClient(apiKey string, client *stripe.Client) *StripeGateway {
	return &StripeGateway{APIKey: apiKey, client: client}
}

// CreatePayment creates a payment via Stripe Checkout Session.
func (g *StripeGateway) CreatePayment(ctx context.Context, orderID string, amount float64, currency, subject, notifyURL, returnUrl string) (*PaymentURLResult, error) {
	// Stripe uses cents for amounts; round to avoid float truncation.
	amountCents := int64(math.Round(amount * 100))

	params := &stripe.CheckoutSessionCreateParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems: []*stripe.CheckoutSessionCreateLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionCreateLineItemPriceDataParams{
					Currency: stripe.String(stringsToLower(currency)),
					ProductData: &stripe.CheckoutSessionCreateLineItemPriceDataProductDataParams{
						Name: stripe.String(subject),
					},
					UnitAmount: stripe.Int64(amountCents),
				},
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(returnUrl),
		CancelURL:  stripe.String(returnUrl),
		PaymentIntentData: &stripe.CheckoutSessionCreatePaymentIntentDataParams{
			Metadata: map[string]string{
				fieldOutTradeNo: orderID,
			},
		},
		Metadata: map[string]string{
			fieldOutTradeNo: orderID,
		},
		Params: stripe.Params{IdempotencyKey: stripe.String(orderID)},
	}

	s, err := g.client.V1CheckoutSessions.Create(ctx, params)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "stripe create session")
	}

	return &PaymentURLResult{
		PaymentURL: s.URL,
		TradeNo:    s.PaymentIntent.ID,
	}, nil
}

// QueryPayment queries the payment status via Stripe Checkout Session.
func (g *StripeGateway) QueryPayment(ctx context.Context, outTradeNo string) (*PaymentStatusResult, error) {
	params := &stripe.CheckoutSessionListParams{
		PaymentIntent: &outTradeNo,
	}

	var s *stripe.CheckoutSession
	for sess, err := range g.client.V1CheckoutSessions.List(ctx, params) {
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "stripe list sessions")
		}
		s = sess
		break
	}
	if s == nil {
		return nil, apperrors.NewNotFoundf("stripe session not found for payment: %s", outTradeNo)
	}

	status := mapStripeSessionStatus(s.PaymentStatus)

	amount := 0.0
	if s.AmountTotal > 0 {
		amount = float64(s.AmountTotal) / 100.0
	}

	return &PaymentStatusResult{
		TradeNo: outTradeNo,
		Status:  status,
		Amount:  amount,
	}, nil
}

// Refund processes a refund via Stripe Refund API (legacy 3-arg path;
// production refunds go through ChannelAdapter → RefundWithOutRefundNo).
func (g *StripeGateway) Refund(ctx context.Context, outTradeNo string, refundAmount float64, refundReason string) (*RefundResult, error) {
	return g.RefundWithOutRefundNo(ctx, outTradeNo, refundAmount, refundReason, "")
}

// RefundWithOutRefundNo keys the Stripe idempotency request with the business
// refund number (unique per Refund aggregate). Falling back to the
// order+amount synthesis only when no refund number is supplied.
func (g *StripeGateway) RefundWithOutRefundNo(ctx context.Context, outTradeNo string, refundAmount float64, refundReason, outRefundNo string) (*RefundResult, error) {
	amountCents := int64(math.Round(refundAmount * 100))

	idempotencyKey := strings.TrimSpace(outRefundNo)
	if idempotencyKey == "" {
		idempotencyKey = outTradeNo + "-refund-" + strconv.FormatInt(amountCents, 10)
	}

	params := &stripe.RefundCreateParams{
		PaymentIntent: &outTradeNo,
		Amount:        &amountCents,
		Reason:        stripe.String(string(stripe.RefundReasonRequestedByCustomer)),
		Metadata: map[string]string{
			fieldOutTradeNo: outTradeNo,
			"refund_reason": refundReason,
		},
		Params: stripe.Params{IdempotencyKey: stripe.String(idempotencyKey)},
	}

	r, err := g.client.V1Refunds.Create(ctx, params)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "stripe refund")
	}

	status := statusProcessing
	switch r.Status {
	case stripe.RefundStatusSucceeded:
		status = statusSuccess
	case stripe.RefundStatusFailed:
		status = statusFailed
	case stripe.RefundStatusCanceled:
		status = statusFailed
	case stripe.RefundStatusPending, stripe.RefundStatusRequiresAction:
		// 非终态：保持初始 processing，等待后续查询/回调收敛
	}

	return &RefundResult{
		RefundNo: r.ID,
		Status:   status,
	}, nil
}

// VerifyNotify verifies a payment notification from Stripe webhook.
func (g *StripeGateway) VerifyNotify(ctx context.Context, params map[string]string) (*NotifyResult, error) {
	// Stripe webhook verification uses the Stripe-Signature header.
	// The handler layer extracts the payload and signature header,
	// then uses stripe.ConstructEvent to verify.
	// This method provides the result after verification.
	outTradeNo := params[fieldOutTradeNo]
	eventType := params["event_type"]

	status := statusPaid
	if eventType == "checkout.session.expired" || eventType == "payment_intent.canceled" {
		status = statusFailed
	}

	amount := 0.0
	if amountStr, ok := params["amount_total"]; ok {
		cents, err := strconv.ParseInt(amountStr, 10, 64)
		if err != nil {
			return nil, apperrors.NewInvalidArgumentf("stripe: invalid amount_total %q", amountStr)
		}
		amount = float64(cents) / 100.0
	}

	return &NotifyResult{
		OutTradeNo: outTradeNo,
		Status:     status,
		Amount:     amount,
	}, nil
}

// mapStripeSessionStatus maps Stripe checkout session payment status to our internal status.
func mapStripeSessionStatus(status stripe.CheckoutSessionPaymentStatus) string {
	switch status {
	case stripe.CheckoutSessionPaymentStatusPaid:
		return statusPaid
	case stripe.CheckoutSessionPaymentStatusUnpaid:
		return statusPending
	case stripe.CheckoutSessionPaymentStatusNoPaymentRequired:
		return statusPaid
	default:
		return statusPending
	}
}

// stringsToLower is a local helper to avoid importing strings just for ToLower.
func stringsToLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// Ensure StripeGateway implements PaymentGateway.
var _ PaymentGateway = (*StripeGateway)(nil)
