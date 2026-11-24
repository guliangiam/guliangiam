package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// CheckoutGateway implements PaymentGateway via Checkout.com.
//
// 依据（OPEN-API-75 P8，https://www.checkout.com/docs/payments/accept-payments/hosted-payments）：
//   - 鉴权 Authorization: Bearer <secret key sk_...>；沙箱基址
//     https://api.sandbox.checkout.com，生产 https://api.checkout.com；
//   - Hosted Payments：POST /hosted-payments 返回 _links.redirect.href 收银台
//     链接与 id；查询 GET /payments/{id}；退款 POST /payments/{id}/refunds；
//   - 幂等：POST 请求带 Idempotency-Key 头（下单=orderID、退款=payID+金额分，
//     https://www.checkout.com/docs/api-tokens-and-permissions/idempotency）；
//   - 回调验签：Checkout-Signature 头 = HMAC-SHA256(raw body, whsec webhook
//     secret) 的 hex 小写（https://www.checkout.com/docs/online-payments/webhooks）；
//   - reference 字段承载商户订单号（orderID），回调→订单映射闭环；
//   - 金额一律整数分（minor units），元→分 math.Round。
//
// 未验证边界（L4）：真实 sk/whsec 端到端联调；DSR 签名（RSA）回调形态未接入，
// 仅接入默认 HMAC 形态。
type CheckoutGateway struct {
	SecretKey     string
	WebhookSecret string
	IsSandbox     bool
	// BaseURL overrides the vendor base (httptest contract tests).
	BaseURL    string
	HTTPClient *http.Client
}

// NewCheckoutGateway creates a Checkout.com payment gateway.
func NewCheckoutGateway(secretKey, webhookSecret string, isSandbox bool) *CheckoutGateway {
	return &CheckoutGateway{
		SecretKey:     secretKey,
		WebhookSecret: webhookSecret,
		IsSandbox:     isSandbox,
		HTTPClient:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (g *CheckoutGateway) baseURL() string {
	if g.BaseURL != "" {
		return g.BaseURL
	}
	if g.IsSandbox {
		return "https://api.sandbox.checkout.com"
	}
	return "https://api.checkout.com"
}

type checkoutHostedPaymentRequest struct {
	Currency   string `json:"currency"`
	Amount     int64  `json:"amount"`
	Reference  string `json:"reference"`
	SuccessURL string `json:"success_url"`
	FailureURL string `json:"failure_url"`
	Billing    *struct {
		Address *struct {
			Country string `json:"country,omitempty"`
		} `json:"address,omitempty"`
	} `json:"billing,omitempty"`
}

type checkoutHostedPaymentResponse struct {
	ID    string `json:"id"`
	Links struct {
		Redirect *struct {
			HRef string `json:"href"`
		} `json:"redirect,omitempty"`
	} `json:"_links"`
}

type checkoutPaymentResponse struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	RequestedAmount int64  `json:"requested_amount"`
	Currency        string `json:"currency"`
	Reference       string `json:"reference"`
}

type checkoutRefundRequest struct {
	Amount    int64  `json:"amount,omitempty"`
	Reference string `json:"reference,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type checkoutRefundResponse struct {
	ActionID string `json:"action_id"`
	Status   string `json:"status"`
}

// CreatePayment creates a Checkout.com Hosted Payment page.
func (g *CheckoutGateway) CreatePayment(ctx context.Context, orderID string, amount float64, currency, subject, notifyURL, returnUrl string) (*PaymentURLResult, error) {
	if strings.TrimSpace(g.SecretKey) == "" {
		return nil, apperrors.NewUnavailable("checkout: secret key is required")
	}

	reqBody := checkoutHostedPaymentRequest{
		Currency:   strings.ToUpper(currency),
		Amount:     int64(math.Round(amount * 100)),
		Reference:  orderID,
		SuccessURL: returnUrl,
		FailureURL: returnUrl,
	}

	var resp checkoutHostedPaymentResponse
	if err := g.doJSON(ctx, http.MethodPost, "/hosted-payments", reqBody, orderID, &resp); err != nil {
		return nil, err
	}
	if resp.ID == "" || resp.Links.Redirect == nil {
		return nil, apperrors.NewInternal("checkout: hosted payment response missing id/redirect link")
	}

	return &PaymentURLResult{
		PaymentURL: resp.Links.Redirect.HRef,
		TradeNo:    resp.ID,
	}, nil
}

// QueryPayment retrieves a Checkout.com payment status (TradeNo = payment id).
func (g *CheckoutGateway) QueryPayment(ctx context.Context, outTradeNo string) (*PaymentStatusResult, error) {
	var resp checkoutPaymentResponse
	if err := g.doJSON(ctx, http.MethodGet, "/payments/"+outTradeNo, nil, "", &resp); err != nil {
		return nil, err
	}

	return &PaymentStatusResult{
		TradeNo: outTradeNo,
		Status:  mapCheckoutStatus(resp.Status),
		Amount:  float64(resp.RequestedAmount) / 100.0,
	}, nil
}

// Refund refunds a Checkout.com payment (legacy 3-arg path; production
// refunds go through ChannelAdapter → RefundWithOutRefundNo).
func (g *CheckoutGateway) Refund(ctx context.Context, outTradeNo string, refundAmount float64, refundReason string) (*RefundResult, error) {
	return g.RefundWithOutRefundNo(ctx, outTradeNo, refundAmount, refundReason, "")
}

// RefundWithOutRefundNo keys the Checkout idempotency request with the
// business refund number (unique per Refund aggregate), so a second
// same-amount partial refund on one payment is not silently replayed.
func (g *CheckoutGateway) RefundWithOutRefundNo(ctx context.Context, outTradeNo string, refundAmount float64, refundReason, outRefundNo string) (*RefundResult, error) {
	amountCents := int64(math.Round(refundAmount * 100))
	reqBody := checkoutRefundRequest{
		Amount:    amountCents,
		Reference: outTradeNo,
		Reason:    refundReason,
	}

	idempotencyKey := strings.TrimSpace(outRefundNo)
	if idempotencyKey == "" {
		idempotencyKey = outTradeNo + "-refund-" + strconv.FormatInt(amountCents, 10)
	}

	var resp checkoutRefundResponse
	if err := g.doJSON(ctx, http.MethodPost, "/payments/"+outTradeNo+"/refunds", reqBody, idempotencyKey, &resp); err != nil {
		return nil, err
	}

	return &RefundResult{
		RefundNo: resp.ActionID,
		Status:   mapCheckoutRefundStatus(resp.Status),
	}, nil
}

// VerifyNotify verifies a Checkout.com webhook signature (hex HMAC-SHA256 of
// the raw body keyed by the whsec shared secret).
// params keys: "signature" (Checkout-Signature header), "body" (raw JSON).
func (g *CheckoutGateway) VerifyNotify(ctx context.Context, params map[string]string) (*NotifyResult, error) {
	if strings.TrimSpace(g.WebhookSecret) == "" {
		return nil, apperrors.NewUnavailable("checkout: webhook secret is not configured")
	}
	signature := params["signature"]
	body := params["body"]
	if signature == "" || body == "" {
		return nil, apperrors.NewInvalidArgument("checkout notify: missing signature/body")
	}

	mac := hmac.New(sha256.New, []byte(g.WebhookSecret))
	mac.Write([]byte(body))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(strings.ToLower(signature))) {
		return nil, apperrors.NewUnauthorized("checkout: webhook signature verification failed")
	}

	var payload struct {
		Reference string `json:"reference"`
		Status    string `json:"status"`
		Amount    int64  `json:"amount"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return nil, apperrors.NewInvalidArgumentf("checkout notify: malformed body: %v", err)
	}
	if payload.Reference == "" {
		return nil, apperrors.NewInvalidArgument("checkout notify: missing reference")
	}

	return &NotifyResult{
		OutTradeNo: payload.Reference,
		Status:     mapCheckoutStatus(payload.Status),
		Amount:     float64(payload.Amount) / 100.0,
	}, nil
}

func (g *CheckoutGateway) doJSON(ctx context.Context, method, path string, body any, idempotencyKey string, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return apperrors.WrapInternalf(err, "checkout marshal request")
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, g.baseURL()+path, reader)
	if err != nil {
		return apperrors.WrapInternalf(err, "checkout build request")
	}
	req.Header.Set("Authorization", "Bearer "+g.SecretKey)
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		return newTransportErrorf(err, "checkout %s %s", method, path)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return newTransportErrorf(err, "checkout read body")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newUpstreamStatusErrorf(resp.StatusCode, resp.Header, "checkout %s %s", method, path)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return apperrors.WrapInternalf(err, "checkout decode response")
		}
	}
	return nil
}

func mapCheckoutStatus(status string) string {
	switch strings.ToUpper(status) {
	case "CAPTURED", "PAID":
		return statusPaid
	case "DECLINED", "EXPIRED", "CANCELED", "VOIDED":
		return statusFailed
	default: // PENDING, AUTHORIZED, CARD VERIFIED, PARTIALLY APPROVED...
		return statusPending
	}
}

func mapCheckoutRefundStatus(status string) string {
	switch strings.ToUpper(status) {
	case "SUCCESS", "REFUNDED", "CAPTURED":
		return statusSuccess
	case "FAILED", "DECLINED":
		return statusFailed
	default:
		return statusProcessing
	}
}

var _ PaymentGateway = (*CheckoutGateway)(nil)
