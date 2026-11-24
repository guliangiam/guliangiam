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

// AirwallexGateway implements PaymentGateway for Airwallex.
type AirwallexGateway struct {
	ClientID      string
	APIKey        string
	BaseURL       string // https://api-demo.airwallex.com or https://api.airwallex.com
	WebhookSecret string
	client        *http.Client
	accessToken   string
	tokenExpiry   time.Time
}

var _ PaymentGateway = (*AirwallexGateway)(nil)

// NewAirwallexGateway creates a new Airwallex payment gateway.
func NewAirwallexGateway(clientID, apiKey, webhookSecret string, isSandbox bool) *AirwallexGateway {
	baseURL := "https://api.airwallex.com"
	if isSandbox {
		baseURL = "https://api-demo.airwallex.com"
	}
	return &AirwallexGateway{
		ClientID:      clientID,
		APIKey:        apiKey,
		BaseURL:       baseURL,
		WebhookSecret: webhookSecret,
		client:        ssrf.SafeHTTPClient(30 * time.Second),
	}
}

// getAccessToken obtains or returns a cached Airwallex API access token.
func (g *AirwallexGateway) getAccessToken(ctx context.Context) (string, error) {
	if g.accessToken != "" && time.Now().Before(g.tokenExpiry) {
		return g.accessToken, nil
	}

	payload := map[string]string{
		"client_id": g.ClientID,
		"api_key":   g.APIKey,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "airwallex: marshal auth")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.BaseURL+"/api/v1/authentication/login", bytes.NewReader(body))
	if err != nil {
		return "", apperrors.WrapInternalf(err, "airwallex: create auth request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return "", newTransportErrorf(err, "airwallex: auth request")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", newTransportErrorf(err, "airwallex: read auth response")
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", apperrors.WrapInternalf(err, "airwallex: parse auth response")
	}

	token, ok := result["access_token"].(string)
	if !ok {
		return "", apperrors.NewNotFound("airwallex: no access_token in response")
	}

	g.accessToken = token
	g.tokenExpiry = time.Now().Add(25 * time.Minute)
	return token, nil
}

func (g *AirwallexGateway) doRequest(ctx context.Context, method, path string, payload any) (map[string]any, error) {
	token, err := g.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "airwallex: marshal")
		}
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, g.BaseURL+path, bodyReader)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "airwallex: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, newTransportErrorf(err, "airwallex: request")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, newTransportErrorf(err, "airwallex: read response")
	}

	if resp.StatusCode >= 400 {
		return nil, newUpstreamStatusErrorf(resp.StatusCode, resp.Header, "airwallex: API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, apperrors.WrapInternalf(err, "airwallex: parse response")
	}
	return result, nil
}

func (g *AirwallexGateway) CreatePayment(ctx context.Context, orderID string, amount float64, currency, subject, notifyURL, returnUrl string) (*PaymentURLResult, error) {
	payload := map[string]any{
		"request_id":        orderID,
		fieldAmount:         amount,
		fieldCurrency:       strings.ToUpper(currency),
		"descriptor":        subject,
		"merchant_order_id": orderID,
		"return_url":        returnUrl,
		"webhook_url":       notifyURL,
	}

	result, err := g.doRequest(ctx, http.MethodPost, "/api/v1/pa/payment_intents/create", payload)
	if err != nil {
		return nil, err
	}

	var paymentID, redirectURL string
	if id, ok := result["id"].(string); ok {
		paymentID = id
	}
	if rd, ok := result["next_action"].(map[string]any); ok {
		if url, ok := rd["url"].(string); ok {
			redirectURL = url
		}
	}

	return &PaymentURLResult{
		PaymentURL: redirectURL,
		TradeNo:    paymentID,
	}, nil
}

func (g *AirwallexGateway) QueryPayment(ctx context.Context, outTradeNo string) (*PaymentStatusResult, error) {
	result, err := g.doRequest(ctx, http.MethodGet, "/api/v1/pa/payment_intents/"+outTradeNo, nil)
	if err != nil {
		return nil, err
	}

	status := statusPending
	if s, ok := result["status"].(string); ok {
		status = mapAirwallexStatus(s)
	}

	var amt float64
	if a, ok := result[fieldAmount].(float64); ok {
		amt = a
	}

	return &PaymentStatusResult{
		TradeNo: outTradeNo,
		Status:  status,
		Amount:  amt,
	}, nil
}

func (g *AirwallexGateway) Refund(ctx context.Context, outTradeNo string, refundAmount float64, refundReason string) (*RefundResult, error) {
	payload := map[string]any{
		"payment_intent_id": outTradeNo,
		fieldAmount:         refundAmount,
		fieldReason:         refundReason,
	}

	result, err := g.doRequest(ctx, http.MethodPost, "/api/v1/pa/refunds/create", payload)
	if err != nil {
		return nil, err
	}

	refundStatus := statusProcessing
	if s, ok := result["status"].(string); ok {
		switch s {
		case gatewaySucceededUpper:
			refundStatus = statusSuccess
		case "FAILED", gatewayCancelledUpper:
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

func (g *AirwallexGateway) VerifyNotify(ctx context.Context, params map[string]string) (*NotifyResult, error) {
	// Airwallex webhook signature verification via HMAC-SHA256
	sig, ok := params["x-signature"]
	if !ok {
		return nil, apperrors.NewInvalidArgument("airwallex: missing signature in webhook")
	}

	// Build signing string from sorted keys
	var keys []string
	for k := range params {
		if k != "x-signature" {
			keys = append(keys, k)
		}
	}
	// Sort keys
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(params[k])
		b.WriteString("&")
	}
	data := b.String()

	mac := hmac.New(sha256.New, []byte(g.WebhookSecret))
	mac.Write([]byte(data))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return nil, apperrors.NewUnauthorized("airwallex: webhook signature verification failed")
	}

	status := statusFailed
	if params["status"] == gatewaySucceededUpper || params["status"] == "AUTHENTICATED" {
		status = statusPaid
	}

	var amt float64
	if val, ok := params[fieldAmount]; ok {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			amt = f
		}
	}

	return &NotifyResult{
		OutTradeNo: params["payment_intent_id"],
		Status:     status,
		Amount:     amt,
	}, nil
}

func mapAirwallexStatus(s string) string {
	switch strings.ToUpper(s) {
	case gatewaySucceededUpper, "AUTHENTICATED":
		return statusPaid
	case "FAILED", gatewayCancelledUpper:
		return statusFailed
	case gatewayRefundedUpper:
		return statusRefunded
	default:
		return statusPending
	}
}
