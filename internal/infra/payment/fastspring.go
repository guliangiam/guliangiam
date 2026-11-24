package payment

import (
	"bytes"
	"context"
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

// FastSpringGateway implements PaymentGateway for FastSpring.
type FastSpringGateway struct {
	APIUsername    string
	APIPassword    string
	StorefrontPath string
	client         *http.Client
}

// NewFastSpringGateway creates a new FastSpring payment gateway.
func NewFastSpringGateway(apiUsername, apiPassword, storefrontPath string) *FastSpringGateway {
	return &FastSpringGateway{
		APIUsername:    apiUsername,
		APIPassword:    apiPassword,
		StorefrontPath: storefrontPath,
		client:         ssrf.SafeHTTPClient(30 * time.Second),
	}
}

type fastSpringSessionRequest struct {
	Items []fastSpringItem  `json:"items"`
	Tags  map[string]string `json:"tags,omitempty"`
}

type fastSpringItem struct {
	Product  string             `json:"product"`
	Quantity int                `json:"quantity"`
	Pricing  *fastSpringPricing `json:"pricing,omitempty"`
}

type fastSpringPricing struct {
	Price map[string]float64 `json:"price,omitempty"`
}

type fastSpringSessionResponse struct {
	ID string `json:"id"`
}

type fastSpringOrderResponse struct {
	Reference string            `json:"reference"`
	Total     float64           `json:"total"`
	Currency  string            `json:"currency"`
	Completed bool              `json:"completed"`
	Tags      map[string]string `json:"tags"`
}

// CreatePayment creates a FastSpring checkout session.
func (g *FastSpringGateway) CreatePayment(ctx context.Context, orderID string, amount float64, currency, subject, notifyURL, returnUrl string) (*PaymentURLResult, error) {
	sessionReq := fastSpringSessionRequest{
		Items: []fastSpringItem{
			{
				Product:  orderID,
				Quantity: 1,
				Pricing: &fastSpringPricing{
					Price: map[string]float64{
						strings.ToUpper(currency): amount,
					},
				},
			},
		},
		Tags: map[string]string{
			fieldOrderID: orderID,
		},
	}

	jsonData, err := json.Marshal(sessionReq)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "fastspring: marshal request")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.fastspring.com/sessions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "fastspring: create request")
	}
	req.SetBasicAuth(g.APIUsername, g.APIPassword)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, newTransportErrorf(err, "fastspring: request")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, newTransportErrorf(err, "fastspring: read response")
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, newUpstreamStatusErrorf(resp.StatusCode, resp.Header, "fastspring: API error (status %d): %s", resp.StatusCode, string(body))
	}

	var sessionResp fastSpringSessionResponse
	if err := json.Unmarshal(body, &sessionResp); err != nil {
		return nil, apperrors.WrapInternalf(err, "fastspring: parse response")
	}

	checkoutURL := fmt.Sprintf("https://%s/session/%s", g.StorefrontPath, sessionResp.ID)

	return &PaymentURLResult{
		PaymentURL: checkoutURL,
		TradeNo:    sessionResp.ID,
	}, nil
}

// QueryPayment queries the FastSpring order status.
func (g *FastSpringGateway) QueryPayment(ctx context.Context, outTradeNo string) (*PaymentStatusResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://api.fastspring.com/orders/%s", outTradeNo), http.NoBody)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "fastspring: create request")
	}
	req.SetBasicAuth(g.APIUsername, g.APIPassword)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, newTransportErrorf(err, "fastspring: request")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &PaymentStatusResult{TradeNo: outTradeNo, Status: statusPending}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, newTransportErrorf(err, "fastspring: read response")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, newUpstreamStatusErrorf(resp.StatusCode, resp.Header, "fastspring: API error (status %d): %s", resp.StatusCode, string(body))
	}

	var orderResp fastSpringOrderResponse
	if err := json.Unmarshal(body, &orderResp); err != nil {
		return nil, apperrors.WrapInternalf(err, "fastspring: parse response")
	}

	status := statusPending
	if orderResp.Completed {
		status = statusPaid
	}

	return &PaymentStatusResult{
		TradeNo: outTradeNo,
		Status:  status,
		Amount:  orderResp.Total,
	}, nil
}

// Refund is not directly supported by FastSpring API.
func (g *FastSpringGateway) Refund(ctx context.Context, outTradeNo string, refundAmount float64, refundReason string) (*RefundResult, error) {
	return nil, apperrors.NewUnimplemented("fastspring: refund not supported via API")
}

// VerifyNotify verifies a FastSpring payment notification by fetching order status.
func (g *FastSpringGateway) VerifyNotify(ctx context.Context, params map[string]string) (*NotifyResult, error) {
	orderID := params[fieldOrderID]
	if orderID == "" {
		orderID = params["reference"]
	}

	status := statusFailed
	if params["status"] == statusPaid || params["event"] == "order.completed" {
		status = statusPaid
	}

	var amt float64
	if val, ok := params["total"]; ok {
		parsed, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return nil, apperrors.NewInvalidArgumentf("fastspring: invalid total %q", val)
		}
		amt = parsed
	}

	return &NotifyResult{
		OutTradeNo: orderID,
		Status:     status,
		Amount:     amt,
	}, nil
}

// Ensure FastSpringGateway implements PaymentGateway.
var _ PaymentGateway = (*FastSpringGateway)(nil)
