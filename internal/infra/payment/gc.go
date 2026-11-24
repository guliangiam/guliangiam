package payment

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // G501: GC payment gateway signature spec mandates MD5
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// GCGateway implements PaymentGateway for GC (Government Cloud) payment.
type GCGateway struct {
	Xmpch     string // merchant ID
	SecretKey string
	Host      string // API endpoint
	client    *http.Client
}

// NewGCGateway creates a new GC payment gateway.
func NewGCGateway(xmpch, secretKey, host string) *GCGateway {
	return &GCGateway{
		Xmpch:     xmpch,
		SecretKey: secretKey,
		Host:      host,
		client:    ssrf.SafeHTTPClient(30 * time.Second),
	}
}

// gcRequestBody is the request wrapper for GC API.
type gcRequestBody struct {
	Op          string `json:"op"`
	Xmpch       string `json:"xmpch"`
	Version     string `json:"version"`
	Data        string `json:"data"`
	RequestTime string `json:"requesttime"`
	Sign        string `json:"sign"`
}

// gcPayReqInfo is the payment request payload.
type gcPayReqInfo struct {
	OrderDate string `json:"orderdate"`
	OrderNo   string `json:"orderno"`
	Amount    string `json:"amount"`
	Xmpch     string `json:"xmpch"`
	Body      string `json:"body"`
	ReturnUrl string `json:"return_url"`
	NotifyUrl string `json:"notify_url"`
}

// gcPayRespInfo is the payment response data.
type gcPayRespInfo struct {
	Jylsh   string `json:"jylsh"`
	Amount  string `json:"amount"`
	PayUrl  string `json:"payurl"`
	TradeNo string `json:"tradeno"`
}

// gcResponseBody is the API response wrapper.
type gcResponseBody struct {
	ReturnCode string `json:"return_code"`
	ReturnMsg  string `json:"return_msg"`
	Data       string `json:"data"`
	Sign       string `json:"sign"`
}

// gcNotifyInfo is the notification payload.
type gcNotifyInfo struct {
	OrderNo    string  `json:"orderno"`
	Amount     float64 `json:"amount"`
	Jylsh      string  `json:"jylsh"`
	TradeNo    string  `json:"tradeno"`
	OrderState string  `json:"orderstate"`
}

func (g *GCGateway) doPost(ctx context.Context, bodyBytes []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.Host, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "gc: create request")
	}
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, newTransportErrorf(err, "gc: request")
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, newTransportErrorf(err, "gc: read response")
	}
	return respBytes, nil
}

func (g *GCGateway) sign(data, op, requestTime, version string) string {
	params := fmt.Sprintf("data=%s&op=%s&requesttime=%s&version=%s&xmpch=%s%s",
		data, op, requestTime, version, g.Xmpch, g.SecretKey)
	// 渠道协议固定用 MD5 大写摘要，替换算法会导致验签失败（非本系统可选项）。
	return strings.ToUpper(fmt.Sprintf("%x", md5.Sum([]byte(params)))) //nolint:gosec // G401: gateway signature spec
}

// CreatePayment creates a GC payment order.
func (g *GCGateway) CreatePayment(ctx context.Context, orderID string, amount float64, currency, subject, notifyURL, returnUrl string) (*PaymentURLResult, error) {
	payReq := gcPayReqInfo{
		OrderDate: time.Now().Format("20060102150405"),
		OrderNo:   orderID,
		Amount:    fmt.Sprintf("%.2f", amount),
		Xmpch:     g.Xmpch,
		Body:      subject,
		ReturnUrl: returnUrl,
		NotifyUrl: notifyURL,
	}

	b, err := json.Marshal(payReq)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "gc: marshal pay request")
	}

	requestTime := time.Now().Format("20060102150405")
	data := base64.StdEncoding.EncodeToString(b)
	sign := g.sign(data, "OrderCreate", requestTime, "1.4")

	body := gcRequestBody{
		Op:          "OrderCreate",
		Xmpch:       g.Xmpch,
		Version:     "1.4",
		Data:        data,
		RequestTime: requestTime,
		Sign:        sign,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "gc: marshal body")
	}

	respBytes, err := g.doPost(ctx, bodyBytes)
	if err != nil {
		return nil, err
	}

	var respBody gcResponseBody
	if err := json.Unmarshal(respBytes, &respBody); err != nil {
		return nil, apperrors.WrapInternalf(err, "gc: parse response")
	}

	if respBody.ReturnCode != gatewaySuccessUpper {
		return nil, apperrors.NewInternalf("gc: %s: %s", respBody.ReturnCode, respBody.ReturnMsg)
	}

	payRespBytes, err := base64.StdEncoding.DecodeString(respBody.Data)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "gc: decode response data")
	}

	var payResp gcPayRespInfo
	if err := json.Unmarshal(payRespBytes, &payResp); err != nil {
		return nil, apperrors.WrapInternalf(err, "gc: parse pay response")
	}

	return &PaymentURLResult{
		PaymentURL: payResp.PayUrl,
		TradeNo:    payResp.Jylsh,
	}, nil
}

// QueryPayment queries a GC payment status.
func (g *GCGateway) QueryPayment(ctx context.Context, outTradeNo string) (*PaymentStatusResult, error) {
	// GC does not have a dedicated query API; return pending as fallback.
	return &PaymentStatusResult{
		TradeNo: outTradeNo,
		Status:  statusPending,
	}, nil
}

// Refund is not supported by GC gateway.
func (g *GCGateway) Refund(ctx context.Context, outTradeNo string, refundAmount float64, refundReason string) (*RefundResult, error) {
	return nil, apperrors.NewUnimplemented("gc: refund not supported")
}

// VerifyNotify verifies a GC payment notification.
func (g *GCGateway) VerifyNotify(ctx context.Context, params map[string]string) (*NotifyResult, error) {
	bodyStr, hasBody := params["__body__"]
	if !hasBody {
		return nil, apperrors.NewInvalidArgument("gc: missing notification body")
	}

	m, err := url.ParseQuery(bodyStr)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "gc: parse notification")
	}

	data := m.Get("data")
	sign := m.Get("sign")
	op := m.Get("op")
	requestTime := m.Get("requesttime")
	version := m.Get("version")

	expectedSign := g.sign(data, op, requestTime, version)
	if !strings.EqualFold(sign, expectedSign) {
		return nil, apperrors.NewUnauthorized("gc: signature verification failed")
	}

	notifyBytes, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "gc: decode notification data")
	}

	var notifyInfo gcNotifyInfo
	if err := json.Unmarshal(notifyBytes, &notifyInfo); err != nil {
		return nil, apperrors.WrapInternalf(err, "gc: parse notification")
	}

	if notifyInfo.OrderState != "1" {
		return &NotifyResult{
			OutTradeNo: notifyInfo.OrderNo,
			Status:     statusFailed,
			Amount:     notifyInfo.Amount,
		}, nil
	}

	return &NotifyResult{
		OutTradeNo: notifyInfo.OrderNo,
		Status:     statusPaid,
		Amount:     notifyInfo.Amount,
	}, nil
}

// Ensure GCGateway implements PaymentGateway.
var _ PaymentGateway = (*GCGateway)(nil)
