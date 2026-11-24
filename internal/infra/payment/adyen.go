package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// AdyenGateway implements PaymentGateway for Adyen.
type AdyenGateway struct {
	APIKey     string
	MerchantID string
	BaseURL    string // https://checkout-test.adyen.com or https://checkout-live.adyen.com
	HMACKey    string // for webhook signature verification
	client     *http.Client
}

var _ PaymentGateway = (*AdyenGateway)(nil)

// NewAdyenGateway creates a new Adyen payment gateway.
func NewAdyenGateway(apiKey, merchantID, hmacKey string, isSandbox bool) *AdyenGateway {
	baseURL := "https://checkout-live.adyen.com/v71"
	if isSandbox {
		baseURL = "https://checkout-test.adyen.com/v71"
	}
	return &AdyenGateway{
		APIKey:     apiKey,
		MerchantID: merchantID,
		BaseURL:    baseURL,
		HMACKey:    hmacKey,
		client:     ssrf.SafeHTTPClient(30 * time.Second),
	}
}

func (g *AdyenGateway) CreatePayment(ctx context.Context, orderID string, amount float64, currency, subject, notifyURL, returnUrl string) (*PaymentURLResult, error) {
	// Adyen requires amount in minor units (cents)
	amountVal := int64(amount * 100)

	payload := map[string]any{
		"merchantAccount": g.MerchantID,
		"reference":       orderID,
		fieldAmount:       map[string]any{fieldValue: amountVal, fieldCurrency: strings.ToUpper(currency)},
		"description":     subject,
		"returnUrl":       returnUrl,
		"notificationUrl": notifyURL,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "adyen: marshal payment")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.BaseURL+"/paymentMethods", bytes.NewReader(body))
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "adyen: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-API-key", g.APIKey)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, newTransportErrorf(err, "adyen: send request")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, newTransportErrorf(err, "adyen: read response")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, newUpstreamStatusErrorf(resp.StatusCode, resp.Header, "adyen: payment failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, apperrors.WrapInternalf(err, "adyen: parse response")
	}

	var pspRef, redirectURL string
	if ref, ok := result["pspReference"].(string); ok {
		pspRef = ref
	}
	if rd, ok := result["redirect"].(map[string]any); ok {
		if url, ok := rd["url"].(string); ok {
			redirectURL = url
		}
	}

	return &PaymentURLResult{
		PaymentURL: redirectURL,
		TradeNo:    pspRef,
	}, nil
}

func (g *AdyenGateway) QueryPayment(ctx context.Context, outTradeNo string) (*PaymentStatusResult, error) {
	url := fmt.Sprintf("%s/payments/%s", g.BaseURL, outTradeNo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "adyen: create query request")
	}
	req.Header.Set("x-API-key", g.APIKey)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, newTransportErrorf(err, "adyen: query request")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, newTransportErrorf(err, "adyen: read query response")
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, apperrors.WrapInternalf(err, "adyen: parse query response")
	}

	status := statusPending
	if rc, ok := result["resultCode"].(string); ok {
		status = mapAdyenStatus(rc)
	}

	var amt float64
	if amount, ok := result[fieldAmount].(map[string]any); ok {
		if val, ok := amount[fieldValue].(float64); ok {
			amt = val / 100
		}
	}

	return &PaymentStatusResult{
		TradeNo: outTradeNo,
		Status:  status,
		Amount:  amt,
	}, nil
}

func (g *AdyenGateway) Refund(ctx context.Context, outTradeNo string, refundAmount float64, refundReason string) (*RefundResult, error) {
	amountVal := int64(refundAmount * 100)

	payload := map[string]any{
		"merchantAccount":     g.MerchantID,
		"paymentPspReference": outTradeNo,
		fieldAmount:           map[string]any{fieldValue: amountVal, fieldCurrency: "USD"},
		"reference":           refundReason,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "adyen: marshal refund")
	}

	url := fmt.Sprintf("%s/payments/%s/refunds", g.BaseURL, outTradeNo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "adyen: create refund request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-API-key", g.APIKey)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, newTransportErrorf(err, "adyen: refund request")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, newTransportErrorf(err, "adyen: read refund response")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, newUpstreamStatusErrorf(resp.StatusCode, resp.Header, "adyen: refund failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, apperrors.WrapInternalf(err, "adyen: parse refund response")
	}

	refundStatus := statusProcessing
	if rc, ok := result["resultCode"].(string); ok {
		if rc == "received" {
			refundStatus = statusSuccess
		}
	}

	var refundNo string
	if ref, ok := result["pspReference"].(string); ok {
		refundNo = ref
	}

	return &RefundResult{
		RefundNo: refundNo,
		Status:   refundStatus,
	}, nil
}

func (g *AdyenGateway) VerifyNotify(ctx context.Context, params map[string]string) (*NotifyResult, error) {
	// Adyen webhook verification uses HMAC-SHA256 signature
	// The signature is in the params["signature"] field
	signature, ok := params["signature"]
	if !ok {
		return nil, apperrors.NewInvalidArgument("adyen: missing signature in notification")
	}

	// Build the signing data string from key fields in order
	signData := buildAdyenSignData(params)

	if !verifyAdyenHMAC(signData, signature, g.HMACKey) {
		return nil, apperrors.NewUnauthorized("adyen: webhook signature verification failed")
	}

	status := statusFailed
	if params["eventCode"] == "AUTHORISATION" && params["success"] == "true" {
		status = statusPaid
	}

	var amt float64
	if val, ok := params[fieldValue]; ok {
		if cents, err := strconv.ParseFloat(val, 64); err == nil {
			amt = cents / 100
		}
	}

	return &NotifyResult{
		OutTradeNo: params["pspReference"],
		Status:     status,
		Amount:     amt,
	}, nil
}

func mapAdyenStatus(rc string) string {
	switch strings.ToUpper(rc) {
	case "AUTHORISED":
		return statusPaid
	case "REFUSED", "ERROR", gatewayCancelledUpper:
		return statusFailed
	case gatewayRefundedUpper:
		return statusRefunded
	default:
		return statusPending
	}
}

// buildAdyenSignData constructs the signing data string from notification parameters.
func buildAdyenSignData(params map[string]string) string {
	keys := []string{"pspReference", "originalReference", "merchantAccountCode",
		"merchantReference", fieldValue, fieldCurrency, "eventCode", "success"}
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(params[k])
	}
	return b.String()
}

// verifyAdyenHMAC verifies the HMAC-SHA256 signature.
func verifyAdyenHMAC(data, signature, key string) bool {
	decodedKey, err := hex.DecodeString(key)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, decodedKey)
	mac.Write([]byte(data))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expected))
}
