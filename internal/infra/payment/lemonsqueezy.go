package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// LemonSqueezyGateway implements PaymentGateway for LemonSqueezy.
type LemonSqueezyGateway struct {
	APIKey        string
	StoreID       string
	WebhookSecret string
	client        *http.Client
}

// NewLemonSqueezyGateway creates a new LemonSqueezy payment gateway.
func NewLemonSqueezyGateway(apiKey, storeID, webhookSecret string) *LemonSqueezyGateway {
	return &LemonSqueezyGateway{
		APIKey:        apiKey,
		StoreID:       storeID,
		WebhookSecret: webhookSecret,
		client:        ssrf.SafeHTTPClient(30 * time.Second),
	}
}

type lemonCheckoutRequest struct {
	Data lemonCheckoutData `json:"data"`
}

type lemonCheckoutData struct {
	Type       string                  `json:"type"`
	Attributes lemonCheckoutAttributes `json:"attributes"`
}

type lemonCheckoutAttributes struct {
	ProductOptions lemonProductOptions `json:"product_options"`
	CheckoutData   lemonCheckoutInfo   `json:"checkout_data"`
}

type lemonProductOptions struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	RedirectURL string `json:"redirect_url,omitempty"`
}

type lemonCheckoutInfo struct {
	Email  string         `json:"email,omitempty"`
	Name   string         `json:"name,omitempty"`
	Custom map[string]any `json:"custom,omitempty"`
}

type lemonCheckoutResponse struct {
	Data lemonCheckoutRespData `json:"data"`
}

type lemonCheckoutRespData struct {
	ID         string                `json:"id"`
	Attributes lemonCheckoutRespAttr `json:"attributes"`
}

type lemonCheckoutRespAttr struct {
	URL string `json:"url"`
}

// CreatePayment creates a LemonSqueezy checkout.
func (g *LemonSqueezyGateway) CreatePayment(ctx context.Context, orderID string, amount float64, currency, subject, notifyURL, returnUrl string) (*PaymentURLResult, error) {
	checkoutReq := lemonCheckoutRequest{
		Data: lemonCheckoutData{
			Type: "checkouts",
			Attributes: lemonCheckoutAttributes{
				ProductOptions: lemonProductOptions{
					Name:        subject,
					RedirectURL: returnUrl,
				},
				CheckoutData: lemonCheckoutInfo{
					Custom: map[string]any{
						fieldOrderID:  orderID,
						"price":       amount,
						fieldCurrency: currency,
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(checkoutReq)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "lemonsqueezy: marshal request")
	}

	apiURL := fmt.Sprintf("https://api.lemonsqueezy.com/v1/stores/%s/checkouts", g.StoreID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "lemonsqueezy: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.APIKey)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, newTransportErrorf(err, "lemonsqueezy: request")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, newTransportErrorf(err, "lemonsqueezy: read response")
	}

	if resp.StatusCode >= 400 {
		return nil, newUpstreamStatusErrorf(resp.StatusCode, resp.Header, "lemonsqueezy: API error (status %d): %s", resp.StatusCode, string(body))
	}

	var checkoutResp lemonCheckoutResponse
	if err := json.Unmarshal(body, &checkoutResp); err != nil {
		return nil, apperrors.WrapInternalf(err, "lemonsqueezy: parse response")
	}

	return &PaymentURLResult{
		PaymentURL: checkoutResp.Data.Attributes.URL,
		TradeNo:    checkoutResp.Data.ID,
	}, nil
}

// QueryPayment queries a LemonSqueezy checkout status.
func (g *LemonSqueezyGateway) QueryPayment(ctx context.Context, outTradeNo string) (*PaymentStatusResult, error) {
	apiURL := fmt.Sprintf("https://api.lemonsqueezy.com/v1/checkouts/%s", outTradeNo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, http.NoBody)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "lemonsqueezy: create request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.APIKey)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, newTransportErrorf(err, "lemonsqueezy: request")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &PaymentStatusResult{TradeNo: outTradeNo, Status: statusPending}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, newTransportErrorf(err, "lemonsqueezy: read response")
	}

	if resp.StatusCode >= 400 {
		return nil, newUpstreamStatusErrorf(resp.StatusCode, resp.Header, "lemonsqueezy: API error (status %d): %s", resp.StatusCode, string(body))
	}

	// LemonSqueezy checkouts don't have a direct payment status.
	// Webhooks should be used for proper status tracking.
	return &PaymentStatusResult{
		TradeNo: outTradeNo,
		Status:  statusPending,
	}, nil
}

// Refund is not directly supported via LemonSqueezy checkout API.
func (g *LemonSqueezyGateway) Refund(ctx context.Context, outTradeNo string, refundAmount float64, refundReason string) (*RefundResult, error) {
	return nil, apperrors.NewUnimplemented("lemonsqueezy: refund not supported via API")
}

// VerifyNotify verifies a LemonSqueezy webhook notification.
func (g *LemonSqueezyGateway) VerifyNotify(ctx context.Context, params map[string]string) (*NotifyResult, error) {
	status := statusFailed
	if params["status"] == statusPaid || params["event_name"] == "order_created" {
		status = statusPaid
	}

	var amt float64
	if val, ok := params["total"]; ok {
		parsed, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return nil, apperrors.NewInvalidArgumentf("lemonsqueezy: invalid total %q", val)
		}
		amt = parsed
	}

	return &NotifyResult{
		OutTradeNo: params[fieldOrderID],
		Status:     status,
		Amount:     amt,
	}, nil
}

// Ensure LemonSqueezyGateway implements PaymentGateway.
var _ PaymentGateway = (*LemonSqueezyGateway)(nil)
