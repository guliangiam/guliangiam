package payment

import (
	"testing"

	"github.com/smartwalle/alipay/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stripe/stripe-go/v82"
)

func TestMapAlipayTradeStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   alipay.TradeStatus
		expected string
	}{
		{"success", alipay.TradeStatusSuccess, "paid"},
		{"finished", alipay.TradeStatusFinished, "paid"},
		{"closed", alipay.TradeStatusClosed, "failed"},
		{"wait_buyer_pay", alipay.TradeStatusWaitBuyerPay, "pending"},
		{"refund", "TRADE_REFUND", "refunded"},
		{"unknown", "UNKNOWN_STATUS", "pending"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := mapAlipayTradeStatus(tc.status)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestParseAmount(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{"valid_integer", "100", 100.0, false},
		{"valid_decimal", "99.99", 99.99, false},
		{"zero", "0", 0.0, false},
		{"empty", "", 0.0, false},
		{"invalid", "abc", 0.0, true},
		{"negative", "-50.5", -50.5, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseAmount(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestMapStripeSessionStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   stripe.CheckoutSessionPaymentStatus
		expected string
	}{
		{"paid", stripe.CheckoutSessionPaymentStatusPaid, "paid"},
		{"unpaid", stripe.CheckoutSessionPaymentStatusUnpaid, "pending"},
		{"no_payment_required", stripe.CheckoutSessionPaymentStatusNoPaymentRequired, "paid"},
		{"unknown", stripe.CheckoutSessionPaymentStatus("unknown"), "pending"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := mapStripeSessionStatus(tc.status)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestStringsToLower(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"uppercase", "HELLO", "hello"},
		{"mixed", "Hello World", "hello world"},
		{"lowercase", "hello", "hello"},
		{"empty", "", ""},
		{"numbers", "ABC123", "abc123"},
		{"special", "A!@#B", "a!@#b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := stringsToLower(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
