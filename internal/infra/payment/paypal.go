package payment

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/plutov/paypal/v4"
)

// PayPalGateway implements PaymentGateway using the PayPal REST API.
type PayPalGateway struct {
	client       *paypal.Client
	clientID     string
	clientSecret string
	isSandbox    bool
}

// NewPayPalGateway creates a new PayPal payment gateway.
func NewPayPalGateway(clientID, clientSecret string, isSandbox bool) (*PayPalGateway, error) {
	if strings.TrimSpace(clientID) == "" {
		return nil, apperrors.NewInvalidArgument("paypal clientID is required")
	}
	if strings.TrimSpace(clientSecret) == "" {
		return nil, apperrors.NewInvalidArgument("paypal clientSecret is required")
	}

	baseURL := paypal.APIBaseLive
	if isSandbox {
		baseURL = paypal.APIBaseSandBox
	}

	client, err := paypal.NewClient(clientID, clientSecret, baseURL)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "create paypal client")
	}

	// Obtain an access token.
	_, err = client.GetAccessToken(context.Background())
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "paypal get access token")
	}

	return &PayPalGateway{
		client:       client,
		clientID:     clientID,
		clientSecret: clientSecret,
		isSandbox:    isSandbox,
	}, nil
}

// CreatePayment creates a PayPal order and returns the approval URL.
func (g *PayPalGateway) CreatePayment(ctx context.Context, orderID string, amount float64, currency, subject, notifyURL, returnUrl string) (*PaymentURLResult, error) {
	// Ensure we have a fresh access token.
	if _, err := g.client.GetAccessToken(ctx); err != nil {
		return nil, apperrors.WrapInternalf(err, "paypal refresh access token")
	}

	purchaseUnits := []paypal.PurchaseUnitRequest{
		{
			ReferenceID: orderID,
			Amount: &paypal.PurchaseUnitAmount{
				Currency: strings.ToUpper(currency),
				Value:    fmt.Sprintf("%.2f", amount),
			},
			Description: subject,
		},
	}

	appContext := &paypal.ApplicationContext{
		ReturnURL: returnUrl,
		CancelURL: returnUrl,
	}

	order, err := g.client.CreateOrder(ctx, "CAPTURE", purchaseUnits, nil, appContext)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "paypal create order")
	}

	// Extract the approval URL from the links.
	for _, link := range order.Links {
		if link.Rel == "approve" {
			return &PaymentURLResult{
				PaymentURL: link.Href,
				TradeNo:    order.ID,
			}, nil
		}
	}

	return nil, apperrors.NewNotFound("paypal order created but no approval link found")
}

// QueryPayment queries a PayPal order status.
func (g *PayPalGateway) QueryPayment(ctx context.Context, outTradeNo string) (*PaymentStatusResult, error) {
	if _, err := g.client.GetAccessToken(ctx); err != nil {
		return nil, apperrors.WrapInternalf(err, "paypal refresh access token")
	}

	order, err := g.client.GetOrder(ctx, outTradeNo)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "paypal get order")
	}

	status := statusPending
	switch order.Status {
	case gatewayCompletedUpper:
		status = statusPaid
	case "VOIDED", gatewayCancelledUpper:
		status = statusFailed
	case gatewayRefundedUpper:
		status = statusRefunded
	}

	var amount float64
	if len(order.PurchaseUnits) > 0 && order.PurchaseUnits[0].Amount != nil {
		value := order.PurchaseUnits[0].Amount.Value
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, apperrors.NewInvalidArgumentf("paypal: invalid amount %q", value)
		}
		amount = parsed
	}

	return &PaymentStatusResult{
		TradeNo: order.ID,
		Status:  status,
		Amount:  amount,
	}, nil
}

// Refund processes a refund for a captured PayPal order.
func (g *PayPalGateway) Refund(ctx context.Context, outTradeNo string, refundAmount float64, refundReason string) (*RefundResult, error) {
	if _, err := g.client.GetAccessToken(ctx); err != nil {
		return nil, apperrors.WrapInternalf(err, "paypal refresh access token")
	}

	// PayPal refunds are done on the capture, not the order directly.
	// First, get the order to find the capture ID.
	order, err := g.client.GetOrder(ctx, outTradeNo)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "paypal get order for refund")
	}

	if len(order.PurchaseUnits) == 0 || order.PurchaseUnits[0].Payments == nil || len(order.PurchaseUnits[0].Payments.Captures) == 0 {
		return nil, apperrors.NewNotFound("paypal order has no captures to refund")
	}

	captureID := order.PurchaseUnits[0].Payments.Captures[0].ID
	refundReq := paypal.RefundCaptureRequest{
		Amount: &paypal.Money{
			Currency: order.PurchaseUnits[0].Amount.Currency,
			Value:    fmt.Sprintf("%.2f", refundAmount),
		},
		NoteToPayer: refundReason,
	}

	refund, err := g.client.RefundCapture(ctx, captureID, refundReq)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "paypal refund capture")
	}

	status := statusProcessing
	switch refund.Status {
	case gatewayCompletedUpper:
		status = statusSuccess
	case gatewayCancelledUpper, "DENIED":
		status = statusFailed
	}

	return &RefundResult{
		RefundNo: refund.ID,
		Status:   status,
	}, nil
}

// VerifyNotify verifies a PayPal webhook notification.
// PayPal webhook verification requires the raw request body and headers.
// This simplified implementation extracts the order ID and status from params.
func (g *PayPalGateway) VerifyNotify(ctx context.Context, params map[string]string) (*NotifyResult, error) {
	outTradeNo := params[fieldOrderID]
	if strings.TrimSpace(outTradeNo) == "" {
		return nil, apperrors.NewInvalidArgument("paypal notify: missing order_id")
	}

	status := statusPending
	if params["status"] == gatewayCompletedUpper {
		status = statusPaid
	}

	var amount float64
	if v, ok := params[fieldAmount]; ok {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, apperrors.NewInvalidArgumentf("paypal: invalid amount %q", v)
		}
		amount = parsed
	}

	return &NotifyResult{
		OutTradeNo: outTradeNo,
		Status:     status,
		Amount:     amount,
	}, nil
}
