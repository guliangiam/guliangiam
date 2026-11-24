package payment

import (
	"context"
	"encoding/json"
	"fmt"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// DummyGateway is a test/mock payment gateway that always succeeds.
// It encodes payment information in the TradeNo for later retrieval.
type DummyGateway struct{}

// NewDummyGateway creates a new dummy payment gateway.
func NewDummyGateway() *DummyGateway {
	return &DummyGateway{}
}

// dummyOrderInfo holds payment info encoded in the TradeNo.
type dummyOrderInfo struct {
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
	Subject  string  `json:"subject"`
	OrderID  string  `json:"orderId"`
}

// CreatePayment encodes payment info into the TradeNo and returns the return URL.
func (g *DummyGateway) CreatePayment(ctx context.Context, orderID string, amount float64, currency, subject, notifyURL, returnUrl string) (*PaymentURLResult, error) {
	info := dummyOrderInfo{
		Price:    amount,
		Currency: currency,
		Subject:  subject,
		OrderID:  orderID,
	}
	infoBytes, err := json.Marshal(info)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "dummy: encode order info")
	}

	return &PaymentURLResult{
		PaymentURL: returnUrl,
		TradeNo:    string(infoBytes),
	}, nil
}

// QueryPayment returns paid status for dummy payments.
func (g *DummyGateway) QueryPayment(ctx context.Context, outTradeNo string) (*PaymentStatusResult, error) {
	// TradeNo 不是可解码的 JSON 时 info 保持零值（Amount 为 0），与历史行为一致：
	// dummy 渠道不做失败判定，任何 TradeNo 都按已支付返回。
	var info dummyOrderInfo
	_ = json.Unmarshal([]byte(outTradeNo), &info)

	return &PaymentStatusResult{
		TradeNo: outTradeNo,
		Status:  statusPaid,
		Amount:  info.Price,
	}, nil
}

// Refund is a no-op for dummy payments.
func (g *DummyGateway) Refund(ctx context.Context, outTradeNo string, refundAmount float64, refundReason string) (*RefundResult, error) {
	return &RefundResult{
		RefundNo: fmt.Sprintf("DUMMY-REF-%s", outTradeNo[:min(16, len(outTradeNo))]),
		Status:   statusSuccess,
	}, nil
}

// VerifyNotify always returns paid for dummy payments.
func (g *DummyGateway) VerifyNotify(ctx context.Context, params map[string]string) (*NotifyResult, error) {
	return &NotifyResult{
		OutTradeNo: params[fieldOrderID],
		Status:     statusPaid,
	}, nil
}

// Ensure DummyGateway implements PaymentGateway.
var _ PaymentGateway = (*DummyGateway)(nil)
