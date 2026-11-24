package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// SquareGateway implements PaymentGateway via Square Checkout API v2.
//
// 依据（OPEN-API-75 P8，https://developer.squareup.com/docs/checkout-api/what-is-checkout）：
//   - 鉴权 Authorization: Bearer <access token> + Square-Version 日期版本头
//     （https://developer.squareup.com/docs/build-basics/api-versions，可经
//     config["apiVersion"] 覆盖，默认钉 2024-12-18）；
//   - 沙箱基址 https://connect.squareupsandbox.com，生产 https://connect.squareup.com；
//   - 幂等：CreateCheckout 请求体 idempotency_key=orderID，CreateRefund 使用
//     Square-Idempotency-Key 头（https://developer.squareup.com/docs/idempotency）；
//   - TradeNo 取 Square order_id：webhook payment.created 的 data.payment.order_id
//     与之同键，回调→订单映射闭环；
//   - 回调验签 HMAC-SHA256(key=webhook signature key, msg="<timestamp>." + raw body)，
//     x-square-signature 为 base64，时间戳容差 5 分钟
//     （https://developer.squareup.com/docs/webhooks/verify-events）；
//   - 金额一律整数分（minor units），元→分 math.Round。
//
// 未验证边界（L4）：真实商户 token 端到端联调；退款链路需先经
// ListPayments(location_id) 反查 payment_id，超出分页窗口的历史订单退款
// 应改由 webhook 落库 payment_id（留待接入）。
type SquareGateway struct {
	AccessToken string
	LocationID  string
	WebhookKey  string
	APIVersion  string
	IsSandbox   bool
	// BaseURL overrides the vendor base (httptest contract tests).
	BaseURL    string
	HTTPClient *http.Client
}

// NewSquareGateway creates a Square payment gateway.
func NewSquareGateway(accessToken, locationID, webhookKey, apiVersion string, isSandbox bool) *SquareGateway {
	if apiVersion == "" {
		apiVersion = "2024-12-18"
	}
	return &SquareGateway{
		AccessToken: accessToken,
		LocationID:  locationID,
		WebhookKey:  webhookKey,
		APIVersion:  apiVersion,
		IsSandbox:   isSandbox,
		HTTPClient:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (g *SquareGateway) baseURL() string {
	if g.BaseURL != "" {
		return g.BaseURL
	}
	if g.IsSandbox {
		return "https://connect.squareupsandbox.com"
	}
	return "https://connect.squareup.com"
}

type squareMoney struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

type squareLineItem struct {
	Name           string       `json:"name"`
	Quantity       string       `json:"quantity"`
	BasePriceMoney *squareMoney `json:"base_price_money,omitempty"`
}

type squareOrder struct {
	ID         string           `json:"id,omitempty"`
	LocationID string           `json:"location_id,omitempty"`
	Reference  string           `json:"reference,omitempty"`
	Status     string           `json:"status,omitempty"`
	TotalMoney *squareMoney     `json:"total_money,omitempty"`
	LineItems  []squareLineItem `json:"line_items,omitempty"`
}

type squareCheckoutRequest struct {
	IdempotencyKey string      `json:"idempotency_key,omitempty"`
	Order          squareOrder `json:"order"`
	Checkout       struct {
		CheckoutMode string `json:"checkout_mode,omitempty"`
	} `json:"checkout"`
}

type squareCheckoutResponse struct {
	Checkout struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		PageURL string `json:"page_url"`
	} `json:"checkout"`
	Order squareOrder `json:"order"`
}

type squareOrderResponse struct {
	Order squareOrder `json:"order"`
}

type squarePayment struct {
	ID          string       `json:"id"`
	OrderID     string       `json:"order_id"`
	Status      string       `json:"status"`
	AmountMoney *squareMoney `json:"amount_money,omitempty"`
}

type squareListPaymentsResponse struct {
	Payments []squarePayment `json:"payments"`
}

type squareRefundRequest struct {
	PaymentID   string       `json:"payment_id"`
	AmountMoney *squareMoney `json:"amount_money"`
	Reason      string       `json:"reason,omitempty"`
	Timestamp   string       `json:"timestamp,omitempty"`
}

type squareRefundResponse struct {
	Refund struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"refund"`
}

// CreatePayment creates a Square Checkout and returns its payment page URL.
func (g *SquareGateway) CreatePayment(ctx context.Context, orderID string, amount float64, currency, subject, notifyURL, returnUrl string) (*PaymentURLResult, error) {
	if strings.TrimSpace(g.AccessToken) == "" || strings.TrimSpace(g.LocationID) == "" {
		return nil, apperrors.NewUnavailable("square: accessToken and locationId are required")
	}

	reqBody := squareCheckoutRequest{
		IdempotencyKey: orderID,
		Order: squareOrder{
			LocationID: g.LocationID,
			Reference:  orderID,
			LineItems: []squareLineItem{{
				Name:     subject,
				Quantity: "1",
				BasePriceMoney: &squareMoney{
					Amount:   int64(math.Round(amount * 100)),
					Currency: strings.ToUpper(currency),
				},
			}},
		},
	}
	reqBody.Checkout.CheckoutMode = "PROCESS_PAYMENT"

	var resp squareCheckoutResponse
	if err := g.doJSON(ctx, http.MethodPost, "/v2/checkout/checkouts", reqBody, "", &resp); err != nil {
		return nil, err
	}
	if resp.Order.ID == "" {
		return nil, apperrors.NewInternal("square: create checkout returned no order")
	}

	return &PaymentURLResult{
		PaymentURL: resp.Checkout.PageURL,
		TradeNo:    resp.Order.ID,
	}, nil
}

// QueryPayment retrieves the Square order status (TradeNo = order_id).
func (g *SquareGateway) QueryPayment(ctx context.Context, outTradeNo string) (*PaymentStatusResult, error) {
	var resp squareOrderResponse
	if err := g.doJSON(ctx, http.MethodGet, "/v2/orders/"+outTradeNo, nil, "", &resp); err != nil {
		return nil, err
	}

	amount := 0.0
	if resp.Order.TotalMoney != nil {
		amount = float64(resp.Order.TotalMoney.Amount) / 100.0
	}

	return &PaymentStatusResult{
		TradeNo: outTradeNo,
		Status:  mapSquareOrderStatus(resp.Order.Status),
		Amount:  amount,
	}, nil
}

// Refund refunds the completed Square payment for the given order (legacy
// 3-arg path; production refunds go through ChannelAdapter →
// RefundWithOutRefundNo).
func (g *SquareGateway) Refund(ctx context.Context, outTradeNo string, refundAmount float64, refundReason string) (*RefundResult, error) {
	return g.RefundWithOutRefundNo(ctx, outTradeNo, refundAmount, refundReason, "")
}

// RefundWithOutRefundNo keys the Square idempotency request with the business
// refund number (unique per Refund aggregate). Square mandates an
// Idempotency-Key on refunds; reusing order+amount would silently replay the
// first refund for a second same-amount partial refund.
func (g *SquareGateway) RefundWithOutRefundNo(ctx context.Context, outTradeNo string, refundAmount float64, refundReason, outRefundNo string) (*RefundResult, error) {
	paymentID, currency, err := g.findCompletedPayment(ctx, outTradeNo)
	if err != nil {
		return nil, err
	}

	reqBody := squareRefundRequest{
		PaymentID: paymentID,
		AmountMoney: &squareMoney{
			Amount:   int64(math.Round(refundAmount * 100)),
			Currency: currency,
		},
		Reason: refundReason,
	}

	idempotencyKey := strings.TrimSpace(outRefundNo)
	if idempotencyKey == "" {
		idempotencyKey = outTradeNo + "-refund-" + strconv.FormatInt(int64(math.Round(refundAmount*100)), 10)
	}
	var resp squareRefundResponse
	if err := g.doJSON(ctx, http.MethodPost, "/v2/refunds", reqBody, idempotencyKey, &resp); err != nil {
		return nil, err
	}

	return &RefundResult{
		RefundNo: resp.Refund.ID,
		Status:   mapSquareRefundStatus(resp.Refund.Status),
	}, nil
}

// findCompletedPayment lists payments for the configured location and returns
// the first COMPLETED payment bound to the order (TradeNo = order_id).
func (g *SquareGateway) findCompletedPayment(ctx context.Context, orderID string) (paymentID, currency string, err error) {
	var resp squareListPaymentsResponse
	if err := g.doJSON(ctx, http.MethodPost, "/v2/payments/list", map[string]string{"location_id": g.LocationID}, "", &resp); err != nil {
		return "", "", err
	}
	for _, p := range resp.Payments {
		if p.OrderID == orderID && p.Status == "COMPLETED" && p.AmountMoney != nil {
			return p.ID, p.AmountMoney.Currency, nil
		}
	}
	return "", "", apperrors.NewNotFoundf("square: no completed payment for order %s", orderID)
}

// VerifyNotify verifies a Square webhook event (HMAC-SHA256 over
// "<x-square-timestamp>.<raw body>", base64 signature, 5-minute window).
// params keys: "signature", "timestamp", "body" (raw JSON).
func (g *SquareGateway) VerifyNotify(ctx context.Context, params map[string]string) (*NotifyResult, error) {
	if strings.TrimSpace(g.WebhookKey) == "" {
		return nil, apperrors.NewUnavailable("square: webhook signature key is not configured")
	}
	signature := params["signature"]
	timestamp := params["timestamp"]
	body := params["body"]
	if signature == "" || timestamp == "" || body == "" {
		return nil, apperrors.NewInvalidArgument("square notify: missing signature/timestamp/body")
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return nil, apperrors.NewInvalidArgumentf("square notify: invalid timestamp %q", timestamp)
	}
	// Square docs: reject events older than 5 minutes (replay protection).
	if age := time.Now().UnixMilli() - ts; age > 5*60*1000 || age < -5*60*1000 {
		return nil, apperrors.NewUnauthorized("square notify: timestamp outside tolerance")
	}

	mac := hmac.New(sha256.New, []byte(g.WebhookKey))
	mac.Write([]byte(timestamp + "." + body))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return nil, apperrors.NewUnauthorized("square: webhook signature verification failed")
	}

	var payload struct {
		NotificationType string `json:"notification_type"`
		Data             struct {
			Payment *squarePayment `json:"payment"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return nil, apperrors.NewInvalidArgumentf("square notify: malformed body: %v", err)
	}
	if payload.Data.Payment == nil || payload.Data.Payment.OrderID == "" {
		return nil, apperrors.NewInvalidArgument("square notify: missing payment.order_id")
	}

	amount := 0.0
	if payload.Data.Payment.AmountMoney != nil {
		amount = float64(payload.Data.Payment.AmountMoney.Amount) / 100.0
	}

	return &NotifyResult{
		OutTradeNo: payload.Data.Payment.OrderID,
		Status:     mapSquarePaymentStatus(payload.Data.Payment.Status),
		Amount:     amount,
	}, nil
}

func (g *SquareGateway) doJSON(ctx context.Context, method, path string, body any, idempotencyKey string, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return apperrors.WrapInternalf(err, "square marshal request")
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, g.baseURL()+path, reader)
	if err != nil {
		return apperrors.WrapInternalf(err, "square build request")
	}
	req.Header.Set("Authorization", "Bearer "+g.AccessToken)
	req.Header.Set("Square-Version", g.APIVersion)
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Square-Idempotency-Key", idempotencyKey)
	}

	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		return newTransportErrorf(err, "square %s %s", method, path)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return newTransportErrorf(err, "square read body")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newUpstreamStatusErrorf(resp.StatusCode, resp.Header, "square %s %s", method, path)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return apperrors.WrapInternalf(err, "square decode response")
		}
	}
	return nil
}

func mapSquareOrderStatus(status string) string {
	switch strings.ToUpper(status) {
	case "COMPLETED":
		return statusPaid
	case "CANCELED":
		return statusFailed
	default: // OPENED and unknown states stay pending
		return statusPending
	}
}

func mapSquarePaymentStatus(status string) string {
	switch strings.ToUpper(status) {
	case "COMPLETED":
		return statusPaid
	case "FAILED", "CANCELED":
		return statusFailed
	default: // PENDING, AUTHORIZED, QUEUED...
		return statusPending
	}
}

func mapSquareRefundStatus(status string) string {
	switch strings.ToUpper(status) {
	case "COMPLETED":
		return statusSuccess
	case "FAILED", "REJECTED":
		return statusFailed
	default: // PENDING, REQUESTED
		return statusProcessing
	}
}

var _ PaymentGateway = (*SquareGateway)(nil)
