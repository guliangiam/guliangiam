package payment

import (
	"context"
	"testing"
)

func TestBalanceGateway_CreatePayment(t *testing.T) {
	gw := NewBalanceGateway()
	result, err := gw.CreatePayment(
		context.Background(),
		"order-001",
		100.0,
		"USD",
		"Test Product",
		"https://notify.example.com",
		"https://return.example.com",
	)
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}
	if result.PaymentURL != "https://return.example.com" {
		t.Errorf("PaymentURL = %q, want https://return.example.com", result.PaymentURL)
	}
	if result.TradeNo != "BAL-order-001" {
		t.Errorf("TradeNo = %q, want BAL-order-001", result.TradeNo)
	}
}

func TestBalanceGateway_QueryPayment_BalancePrefix(t *testing.T) {
	gw := NewBalanceGateway()
	status, err := gw.QueryPayment(context.Background(), "BAL-order-001")
	if err != nil {
		t.Fatalf("QueryPayment() error = %v", err)
	}
	if status.Status != "paid" {
		t.Errorf("Status = %q, want paid", status.Status)
	}
	if status.TradeNo != "BAL-order-001" {
		t.Errorf("TradeNo = %q, want BAL-order-001", status.TradeNo)
	}
}

func TestBalanceGateway_QueryPayment_NonBalancePrefix(t *testing.T) {
	gw := NewBalanceGateway()
	status, err := gw.QueryPayment(context.Background(), "EXT-order-001")
	if err != nil {
		t.Fatalf("QueryPayment() error = %v", err)
	}
	if status.Status != "pending" {
		t.Errorf("Status = %q, want pending", status.Status)
	}
}

func TestBalanceGateway_Refund(t *testing.T) {
	gw := NewBalanceGateway()
	result, err := gw.Refund(context.Background(), "BAL-order-001", 50.0, "partial refund")
	if err != nil {
		t.Fatalf("Refund() error = %v", err)
	}
	if result.Status != "success" {
		t.Errorf("Status = %q, want success", result.Status)
	}
	if result.RefundNo != "REF-BAL-order-001" {
		t.Errorf("RefundNo = %q, want REF-BAL-order-001", result.RefundNo)
	}
}

func TestBalanceGateway_VerifyNotify(t *testing.T) {
	gw := NewBalanceGateway()
	params := map[string]string{
		"order_id": "order-001",
	}
	result, err := gw.VerifyNotify(context.Background(), params)
	if err != nil {
		t.Fatalf("VerifyNotify() error = %v", err)
	}
	if result.OutTradeNo != "order-001" {
		t.Errorf("OutTradeNo = %q, want order-001", result.OutTradeNo)
	}
	if result.Status != "paid" {
		t.Errorf("Status = %q, want paid", result.Status)
	}
}

func TestBalanceGateway_VerifyNotify_EmptyParams(t *testing.T) {
	gw := NewBalanceGateway()
	result, err := gw.VerifyNotify(context.Background(), map[string]string{})
	if err != nil {
		t.Fatalf("VerifyNotify() error = %v", err)
	}
	if result.OutTradeNo != "" {
		t.Errorf("OutTradeNo = %q, want empty", result.OutTradeNo)
	}
}

func TestNewGateway_BalanceProvider(t *testing.T) {
	gw, err := NewGateway("balance", nil)
	if err != nil {
		t.Fatalf("NewGateway(balance) error = %v", err)
	}
	if gw == nil {
		t.Error("expected non-nil gateway")
	}
}

func TestNewGateway_DummyProvider(t *testing.T) {
	gw, err := NewGateway("dummy", nil)
	if err != nil {
		t.Fatalf("NewGateway(dummy) error = %v", err)
	}
	if gw == nil {
		t.Error("expected non-nil gateway")
	}
}

func TestNewGateway_StripeProvider(t *testing.T) {
	gw, err := NewGateway("stripe", map[string]string{"apiKey": "sk_test_123"})
	if err != nil {
		t.Fatalf("NewGateway(stripe) error = %v", err)
	}
	if gw == nil {
		t.Error("expected non-nil gateway")
	}
}

func TestNewGateway_WechatPayProvider(t *testing.T) {
	_, err := NewGateway("wechat_pay", nil)
	if err == nil {
		t.Error("expected error for wechat_pay (requires cert-based init)")
	}
}

func TestNewGateway_UnsupportedProvider(t *testing.T) {
	_, err := NewGateway("unsupported_provider", nil)
	if err == nil {
		t.Error("expected error for unsupported provider")
	}
}

func TestNewGateway_AdYenProvider(t *testing.T) {
	gw, err := NewGateway("adyen", map[string]string{
		"apiKey":     "key",
		"merchantId": "merch",
		"hmacKey":    "hmac",
	})
	if err != nil {
		t.Fatalf("NewGateway(adyen) error = %v", err)
	}
	if gw == nil {
		t.Error("expected non-nil gateway")
	}
}

func TestNewGateway_PaddleProvider(t *testing.T) {
	gw, err := NewGateway("paddle", map[string]string{
		"apiToken":      "token",
		"webhookSecret": "secret",
	})
	if err != nil {
		t.Fatalf("NewGateway(paddle) error = %v", err)
	}
	if gw == nil {
		t.Error("expected non-nil gateway")
	}
}
