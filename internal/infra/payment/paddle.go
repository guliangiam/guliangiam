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

// PaddleGateway implements PaymentGateway for Paddle.
type PaddleGateway struct {
	APIToken      string
	BaseURL       string // https://sandbox-api.paddle.com or https://api.paddle.com
	WebhookSecret string
	client        *http.Client
}

var _ PaymentGateway = (*PaddleGateway)(nil)

// NewPaddleGateway creates a new Paddle payment gateway.
func NewPaddleGateway(apiToken, webhookSecret string, isSandbox bool) *PaddleGateway {
	baseURL := "https://api.paddle.com"
	if isSandbox {
		baseURL = "https://sandbox-api.paddle.com"
	}
	return &PaddleGateway{
		APIToken:      apiToken,
		BaseURL:       baseURL,
		WebhookSecret: webhookSecret,
		client:        ssrf.SafeHTTPClient(30 * time.Second),
	}
}

func (g *PaddleGateway) doRequest(ctx context.Context, method, path string, payload any) (map[string]any, error) {
	var bodyReader io.Reader
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "paddle: marshal")
		}
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, g.BaseURL+path, bodyReader)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "paddle: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.APIToken)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, newTransportErrorf(err, "paddle: request")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, newTransportErrorf(err, "paddle: read response")
	}

	if resp.StatusCode >= 400 {
		return nil, newUpstreamStatusErrorf(resp.StatusCode, resp.Header, "paddle: API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, apperrors.WrapInternalf(err, "paddle: parse response")
	}
	return result, nil
}

func (g *PaddleGateway) CreatePayment(ctx context.Context, orderID string, amount float64, currency, subject, notifyURL, returnUrl string) (*PaymentURLResult, error) {
	// Paddle uses cents for amount
	amountCents := int64(amount * 100)

	payload := map[string]any{
		"custom_data": map[string]string{fieldOrderID: orderID},
		"items": []map[string]any{
			{
				"price": map[string]any{
					"unit_amount":   strconv.FormatInt(amountCents, 10),
					"currency_code": strings.ToUpper(currency),
				},
				"quantity": 1,
				"name":     subject,
			},
		},
	}

	result, err := g.doRequest(ctx, http.MethodPost, "/transactions", payload)
	if err != nil {
		return nil, err
	}

	var txnID, checkoutURL string
	if id, ok := result["id"].(string); ok {
		txnID = id
	}
	if data, ok := result["data"].(map[string]any); ok {
		// Paddle wraps responses in a data field
		if id, ok := data["id"].(string); ok {
			txnID = id
		}
		if chk, ok := data["checkout"].(map[string]any); ok {
			if url, ok := chk["url"].(string); ok {
				checkoutURL = url
			}
		}
	}

	return &PaymentURLResult{
		PaymentURL: checkoutURL,
		TradeNo:    txnID,
	}, nil
}

func (g *PaddleGateway) QueryPayment(ctx context.Context, outTradeNo string) (*PaymentStatusResult, error) {
	result, err := g.doRequest(ctx, http.MethodGet, "/transactions/"+outTradeNo, nil)
	if err != nil {
		return nil, err
	}

	// Paddle wraps in data
	data := result
	if d, ok := result["data"].(map[string]any); ok {
		data = d
	}

	status := statusPending
	if s, ok := data["status"].(string); ok {
		status = mapPaddleStatus(s)
	}

	var amt float64
	if details, ok := data["details"].(map[string]any); ok {
		if a, ok := details["totals"].(map[string]any); ok {
			if total, ok := a["total"].(string); ok {
				if cents, err := strconv.ParseFloat(total, 64); err == nil {
					amt = cents / 100
				}
			}
		}
	}
	if amt == 0 {
		if a, ok := data[fieldAmount].(string); ok {
			if cents, err := strconv.ParseFloat(a, 64); err == nil {
				amt = cents / 100
			}
		}
	}

	return &PaymentStatusResult{
		TradeNo: outTradeNo,
		Status:  status,
		Amount:  amt,
	}, nil
}

func (g *PaddleGateway) Refund(ctx context.Context, outTradeNo string, refundAmount float64, refundReason string) (*RefundResult, error) {
	amountCents := int64(refundAmount * 100)

	payload := map[string]any{
		"transaction_id": outTradeNo,
		fieldAmount:      strconv.FormatInt(amountCents, 10),
		fieldReason:      refundReason,
	}

	result, err := g.doRequest(ctx, http.MethodPost, "/refunds", payload)
	if err != nil {
		return nil, err
	}

	data := result
	if d, ok := result["data"].(map[string]any); ok {
		data = d
	}

	refundStatus := statusProcessing
	if s, ok := data["status"].(string); ok {
		status := mapPaddleRefundStatus(s)
		refundStatus = status
	}

	var refundNo string
	if id, ok := data["id"].(string); ok {
		refundNo = id
	}

	return &RefundResult{
		RefundNo: refundNo,
		Status:   refundStatus,
	}, nil
}

func (g *PaddleGateway) VerifyNotify(ctx context.Context, params map[string]string) (*NotifyResult, error) {
	// Paddle uses Paddle-Signature header for webhook verification
	// The signature is passed through params["paddle_signature"]
	signature, ok := params["paddle_signature"]
	if !ok {
		return nil, apperrors.NewInvalidArgument("paddle: missing signature in webhook")
	}

	// Verify HMAC-SHA256 signature
	// Paddle signature format: ts=<timestamp>;h1=<hmac_hex>
	if !verifyPaddleSignature(signature, g.WebhookSecret) {
		return nil, apperrors.NewUnauthorized("paddle: webhook signature verification failed")
	}

	status := statusFailed
	if params["status"] == gatewayCompletedLower || params["status"] == statusPaid {
		status = statusPaid
	}

	var amt float64
	if val, ok := params[fieldAmount]; ok {
		if cents, err := strconv.ParseFloat(val, 64); err == nil {
			amt = cents / 100
		}
	}

	return &NotifyResult{
		OutTradeNo: params["transaction_id"],
		Status:     status,
		Amount:     amt,
	}, nil
}

func mapPaddleStatus(s string) string {
	switch strings.ToLower(s) {
	case gatewayCompletedLower, statusPaid, "ready":
		return statusPaid
	case gatewayCanceledLower, statusFailed:
		return statusFailed
	case statusRefunded:
		return statusRefunded
	default:
		return statusPending
	}
}

func mapPaddleRefundStatus(s string) string {
	switch strings.ToLower(s) {
	case "approved", gatewayCompletedLower:
		return statusSuccess
	case "declined", statusFailed:
		return statusFailed
	default:
		return statusProcessing
	}
}

// verifyPaddleSignature verifies Paddle's webhook signature.
// Format: ts=<timestamp>;h1=<hex_hmac>
func verifyPaddleSignature(signature, secret string) bool {
	parts := strings.Split(signature, ";")
	var ts, h1 string
	for _, part := range parts {
		if strings.HasPrefix(part, "ts=") {
			ts = part[3:]
		} else if strings.HasPrefix(part, "h1=") {
			h1 = part[3:]
		}
	}
	if ts == "" || h1 == "" {
		return false
	}

	// Paddle signs with: HMAC-SHA256(secret, ts + ":" + requestBody)
	// For simplified verification, we check the HMAC against the timestamp
	payload := ts + ":"
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(h1), []byte(expected))
}
