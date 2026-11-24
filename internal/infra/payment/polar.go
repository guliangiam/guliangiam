package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// PolarGateway implements PaymentGateway for Polar (polar.sh).
type PolarGateway struct {
	AccessToken   string
	BaseURL       string // https://api.polar.sh or https://sandbox-api.polar.sh
	WebhookSecret string
	client        *http.Client
}

var _ PaymentGateway = (*PolarGateway)(nil)

// NewPolarGateway creates a new Polar payment gateway.
func NewPolarGateway(accessToken, webhookSecret string, isSandbox bool) *PolarGateway {
	baseURL := "https://api.polar.sh"
	if isSandbox {
		baseURL = "https://sandbox-api.polar.sh"
	}
	return &PolarGateway{
		AccessToken:   accessToken,
		BaseURL:       baseURL,
		WebhookSecret: webhookSecret,
		client:        ssrf.SafeHTTPClient(30 * time.Second),
	}
}

func (g *PolarGateway) doRequest(ctx context.Context, method, path string, payload any) (map[string]any, error) {
	var bodyReader io.Reader
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "polar: marshal")
		}
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, g.BaseURL+path, bodyReader)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "polar: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.AccessToken)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, newTransportErrorf(err, "polar: request")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, newTransportErrorf(err, "polar: read response")
	}

	if resp.StatusCode >= 400 {
		return nil, newUpstreamStatusErrorf(resp.StatusCode, resp.Header, "polar: API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, apperrors.WrapInternalf(err, "polar: parse response")
	}
	return result, nil
}

func (g *PolarGateway) CreatePayment(ctx context.Context, orderID string, amount float64, currency, subject, notifyURL, returnUrl string) (*PaymentURLResult, error) {
	// Polar uses checkout sessions
	payload := map[string]any{
		"product_price_id": orderID, // Polar maps products to price IDs
		"metadata":         map[string]string{fieldOrderID: orderID},
		"success_url":      returnUrl,
		"cancel_url":       returnUrl,
	}

	result, err := g.doRequest(ctx, http.MethodPost, "/v1/checkouts/", payload)
	if err != nil {
		return nil, err
	}

	var checkoutID, checkoutURL string
	if id, ok := result["id"].(string); ok {
		checkoutID = id
	}
	if url, ok := result["url"].(string); ok {
		checkoutURL = url
	}

	return &PaymentURLResult{
		PaymentURL: checkoutURL,
		TradeNo:    checkoutID,
	}, nil
}

func (g *PolarGateway) QueryPayment(ctx context.Context, outTradeNo string) (*PaymentStatusResult, error) {
	result, err := g.doRequest(ctx, http.MethodGet, "/v1/orders/"+outTradeNo, nil)
	if err != nil {
		return nil, err
	}

	status := statusPending
	if s, ok := result["status"].(string); ok {
		status = mapPolarStatus(s)
	}

	var amt float64
	if a, ok := result["total_amount"].(float64); ok {
		amt = a / 100 // Polar stores amounts in cents
	}

	return &PaymentStatusResult{
		TradeNo: outTradeNo,
		Status:  status,
		Amount:  amt,
	}, nil
}

func (g *PolarGateway) Refund(ctx context.Context, outTradeNo string, refundAmount float64, refundReason string) (*RefundResult, error) {
	amountCents := int64(refundAmount * 100)

	payload := map[string]any{
		fieldOrderID: outTradeNo,
		fieldAmount:  amountCents,
		fieldReason:  refundReason,
	}

	result, err := g.doRequest(ctx, http.MethodPost, "/v1/refunds/", payload)
	if err != nil {
		return nil, err
	}

	refundStatus := statusProcessing
	if s, ok := result["status"].(string); ok {
		switch strings.ToLower(s) {
		case gatewaySucceededLower:
			refundStatus = statusSuccess
		case statusFailed, gatewayCanceledLower:
			refundStatus = statusFailed
		}
	}

	var refundNo string
	if id, ok := result["id"].(string); ok {
		refundNo = id
	}

	return &RefundResult{
		RefundNo: refundNo,
		Status:   refundStatus,
	}, nil
}

func (g *PolarGateway) VerifyNotify(ctx context.Context, params map[string]string) (*NotifyResult, error) {
	// Polar webhook signature verification via HMAC-SHA256
	// The raw body is passed in params["__body__"] for signature verification
	bodyStr, hasBody := params["__body__"]
	sig, hasSig := params["x-polar-signature"]

	if hasBody && hasSig {
		mac := hmac.New(sha256.New, []byte(g.WebhookSecret))
		mac.Write([]byte(bodyStr))
		expected := hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(sig), []byte(expected)) {
			return nil, apperrors.NewUnauthorized("polar: webhook signature verification failed")
		}
	}

	status := statusFailed
	if params["status"] == statusPaid || params["status"] == gatewaySucceededLower {
		status = statusPaid
	}

	var amt float64
	if val, ok := params[fieldAmount]; ok {
		if cents, err := strconv.ParseFloat(val, 64); err == nil {
			amt = cents / 100
		}
	}

	return &NotifyResult{
		OutTradeNo: params[fieldOrderID],
		Status:     status,
		Amount:     amt,
	}, nil
}

func mapPolarStatus(s string) string {
	switch strings.ToLower(s) {
	case statusPaid, gatewaySucceededLower, "delivered":
		return statusPaid
	case statusFailed, gatewayCanceledLower:
		return statusFailed
	case statusRefunded:
		return statusRefunded
	default:
		return statusPending
	}
}
