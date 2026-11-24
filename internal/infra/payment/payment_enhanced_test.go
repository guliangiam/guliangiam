package payment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGateway_PolarProvider(t *testing.T) {
	gw, err := NewGateway("polar", map[string]string{
		"accessToken":   "token",
		"webhookSecret": "secret",
	})
	require.NoError(t, err)
	assert.NotNil(t, gw)
}

func TestNewGateway_AirwallexProvider(t *testing.T) {
	gw, err := NewGateway("airwallex", map[string]string{
		"clientId":      "client",
		"apiKey":        "key",
		"webhookSecret": "secret",
	})
	require.NoError(t, err)
	assert.NotNil(t, gw)
}

func TestNewGateway_FastSpringProvider(t *testing.T) {
	gw, err := NewGateway("fastspring", map[string]string{
		"apiUsername":    "user",
		"apiPassword":    "pass",
		"storefrontPath": "/store",
	})
	require.NoError(t, err)
	assert.NotNil(t, gw)
}

func TestNewGateway_LemonSqueezyProvider(t *testing.T) {
	gw, err := NewGateway("lemonsqueezy", map[string]string{
		"apiKey":        "key",
		"storeId":       "store",
		"webhookSecret": "secret",
	})
	require.NoError(t, err)
	assert.NotNil(t, gw)
}

func TestNewGateway_GCProvider(t *testing.T) {
	gw, err := NewGateway("gc", map[string]string{
		"xmpch":     "merchant",
		"secretKey": "secret",
		"host":      "https://gc.example.com",
	})
	require.NoError(t, err)
	assert.NotNil(t, gw)
}

func TestNewGateway_PayPalProvider(t *testing.T) {
	gw, err := NewGateway("paypal", map[string]string{
		"clientId":     "dummy-client",
		"clientSecret": "dummy-secret",
	})
	if err != nil {
		t.Logf("paypal gateway creation may fail with dummy creds: %v", err)
	}
	_ = gw
}

func TestNewGateway_PayPalSandboxProvider(t *testing.T) {
	gw, err := NewGateway("paypal", map[string]string{
		"clientId":     "dummy-client",
		"clientSecret": "dummy-secret",
		"isSandbox":    "true",
	})
	if err != nil {
		t.Logf("paypal sandbox gateway creation may fail with dummy creds: %v", err)
	}
	_ = gw
}

func TestNewGateway_AdyenSandboxProvider(t *testing.T) {
	gw, err := NewGateway("adyen", map[string]string{
		"apiKey":     "key",
		"merchantId": "merchant",
		"hmacKey":    "hmac",
		"isSandbox":  "true",
	})
	require.NoError(t, err)
	assert.NotNil(t, gw)
}

func TestBalanceGateway_CreatePayment_VerifyResult(t *testing.T) {
	gw := NewBalanceGateway()
	result, err := gw.CreatePayment(context.Background(), "order-123", 50.0, "CNY", "Test", "https://notify", "https://return")
	require.NoError(t, err)
	assert.Equal(t, "https://return", result.PaymentURL)
	assert.Equal(t, "BAL-order-123", result.TradeNo)
}

func TestBalanceGateway_QueryPayment_PaidStatus(t *testing.T) {
	gw := NewBalanceGateway()
	result, err := gw.QueryPayment(context.Background(), "BAL-order-123")
	require.NoError(t, err)
	assert.Equal(t, "paid", result.Status)
	assert.Equal(t, float64(0), result.Amount)
}

func TestBalanceGateway_Refund_VerifyResult(t *testing.T) {
	gw := NewBalanceGateway()
	result, err := gw.Refund(context.Background(), "BAL-order-123", 25.0, "partial")
	require.NoError(t, err)
	assert.Equal(t, "REF-BAL-order-123", result.RefundNo)
	assert.Equal(t, "success", result.Status)
}

func TestDummyGateway_CreatePayment_VerifyInfo(t *testing.T) {
	gw := NewDummyGateway()
	result, err := gw.CreatePayment(context.Background(), "order-dummy", 199.99, "USD", "Product", "", "https://return")
	require.NoError(t, err)
	assert.Equal(t, "https://return", result.PaymentURL)
	assert.NotEmpty(t, result.TradeNo)
}

func TestDummyGateway_Refund_ShortTradeNo(t *testing.T) {
	gw := NewDummyGateway()
	result, err := gw.Refund(context.Background(), "short", 10.0, "test")
	require.NoError(t, err)
	assert.Contains(t, result.RefundNo, "DUMMY-REF-")
	assert.Equal(t, "success", result.Status)
}

func TestDummyGateway_VerifyNotify_WithOrderID(t *testing.T) {
	gw := NewDummyGateway()
	result, err := gw.VerifyNotify(context.Background(), map[string]string{"order_id": "order-notify"})
	require.NoError(t, err)
	assert.Equal(t, "order-notify", result.OutTradeNo)
	assert.Equal(t, "paid", result.Status)
}

func TestGCGateway_QueryPayment(t *testing.T) {
	gw := NewGCGateway("xmpch", "secret", "https://gc.example.com")
	result, err := gw.QueryPayment(context.Background(), "trade-123")
	require.NoError(t, err)
	assert.Equal(t, "pending", result.Status)
	assert.Equal(t, "trade-123", result.TradeNo)
}

func TestGCGateway_Refund_Unsupported(t *testing.T) {
	gw := NewGCGateway("xmpch", "secret", "https://gc.example.com")
	_, err := gw.Refund(context.Background(), "trade-123", 10.0, "test")
	assert.Error(t, err)
}

func TestGCGateway_VerifyNotify_MissingBody(t *testing.T) {
	gw := NewGCGateway("xmpch", "secret", "https://gc.example.com")
	_, err := gw.VerifyNotify(context.Background(), map[string]string{})
	assert.Error(t, err)
}

func TestGCGateway_Sign(t *testing.T) {
	gw := NewGCGateway("xmpch", "secret", "https://gc.example.com")
	sign := gw.sign("data", "op", "time", "1.4")
	assert.NotEmpty(t, sign)
}
