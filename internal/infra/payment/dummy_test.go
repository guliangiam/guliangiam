package payment

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDummyGateway_CreatePayment(t *testing.T) {
	gw := NewDummyGateway()

	result, err := gw.CreatePayment(
		context.Background(),
		"order-001",
		99.99,
		"USD",
		"Test Product",
		"https://example.com/notify",
		"https://example.com/return",
	)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/return", result.PaymentURL)
	assert.NotEmpty(t, result.TradeNo)

	// Verify the TradeNo contains valid JSON with payment info
	var info dummyOrderInfo
	err = json.Unmarshal([]byte(result.TradeNo), &info)
	require.NoError(t, err)
	assert.Equal(t, 99.99, info.Price)
	assert.Equal(t, "USD", info.Currency)
	assert.Equal(t, "Test Product", info.Subject)
	assert.Equal(t, "order-001", info.OrderID)
}

func TestDummyGateway_QueryPayment(t *testing.T) {
	gw := NewDummyGateway()

	// First create a payment
	result, err := gw.CreatePayment(
		context.Background(),
		"order-002",
		50.00,
		"CNY",
		"Query Test",
		"https://notify.example.com",
		"https://return.example.com",
	)
	require.NoError(t, err)

	// Query by trade number
	status, err := gw.QueryPayment(context.Background(), result.TradeNo)
	require.NoError(t, err)
	assert.Equal(t, "paid", status.Status)
	assert.Equal(t, result.TradeNo, status.TradeNo)
	assert.Equal(t, 50.00, status.Amount)
}

func TestDummyGateway_QueryPayment_InvalidTradeNo(t *testing.T) {
	gw := NewDummyGateway()

	// Query with an invalid trade number
	status, err := gw.QueryPayment(context.Background(), "invalid-trade-no")
	require.NoError(t, err)
	assert.Equal(t, "paid", status.Status)
	assert.Equal(t, "invalid-trade-no", status.TradeNo)
}

func TestDummyGateway_Refund(t *testing.T) {
	gw := NewDummyGateway()

	result, err := gw.Refund(context.Background(), "trade-123", 99.99, "customer request")
	require.NoError(t, err)
	assert.Equal(t, "success", result.Status)
	assert.Contains(t, result.RefundNo, "DUMMY-REF-")
}

func TestDummyGateway_VerifyNotify(t *testing.T) {
	gw := NewDummyGateway()

	params := map[string]string{
		"order_id": "order-003",
		"status":   "paid",
	}

	result, err := gw.VerifyNotify(context.Background(), params)
	require.NoError(t, err)
	assert.Equal(t, "order-003", result.OutTradeNo)
	assert.Equal(t, "paid", result.Status)
}

func TestDummyGateway_VerifyNotify_EmptyParams(t *testing.T) {
	gw := NewDummyGateway()

	result, err := gw.VerifyNotify(context.Background(), map[string]string{})
	require.NoError(t, err)
	assert.Equal(t, "", result.OutTradeNo)
	assert.Equal(t, "paid", result.Status)
}

func TestDummyGateway_ImplementsInterface(t *testing.T) {
	// Compile-time check
	var _ PaymentGateway = (*DummyGateway)(nil)
}
