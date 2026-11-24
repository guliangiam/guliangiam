package payment

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/smartwalle/alipay/v3"
)

// AlipayGateway implements PaymentGateway via Alipay.
//
// 校准（OPEN-API-75 P8，https://opendocs.alipay.com/common/02km9f）：
//   - RSA2（SHA256withRSA）请求签名与同步/异步验签委托官方 SDK 形态
//     smartwalle/alipay/v3（alipay.New + LoadAliPayPublicKey 公钥模式）；
//     证书模式未接入——凭据模型只承载应用公钥/支付宝公钥两段（L4 边界）；
//   - 正式网关 openapi.alipay.com/gateway.do、沙箱 openapi-sandbox 由 SDK
//     production 标志切换（alipay.New 第三参为 production，注意取反）；
//   - 金额字符串 %.2f 格式化（四舍五入），无 float 截断偏差；
//   - 异步通知验签（剔除 sign/sign_type、字典序、已解码值原样拼接）在
//     transport 层完成，向量测试钉契约（notify.go/notify_security_test.go）。
type AlipayGateway struct {
	Client    *alipay.Client
	IsSandbox bool
}

// NewAlipayGateway creates a new Alipay payment gateway.
// The third parameter of alipay.New is "production" (not isSandbox), so we invert it.
func NewAlipayGateway(appID, privateKey, publicKey string, isSandbox bool) (*AlipayGateway, error) {
	client, err := alipay.New(appID, privateKey, !isSandbox)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "init alipay client")
	}

	// Load the Alipay public key for signature verification.
	if err := client.LoadAliPayPublicKey(publicKey); err != nil {
		return nil, apperrors.WrapInternalf(err, "load alipay public key")
	}

	return &AlipayGateway{
		Client:    client,
		IsSandbox: isSandbox,
	}, nil
}

// CreatePayment creates a payment via Alipay TradePagePay API.
func (g *AlipayGateway) CreatePayment(_ context.Context, orderID string, amount float64, _, subject, notifyURL, returnUrl string) (*PaymentURLResult, error) {
	p := alipay.TradePagePay{}
	p.OutTradeNo = orderID
	p.TotalAmount = fmt.Sprintf("%.2f", amount)
	p.Subject = subject
	p.NotifyURL = notifyURL
	p.ReturnURL = returnUrl
	p.ProductCode = "FAST_INSTANT_TRADE_PAY"

	// TradePagePay does not take context; it builds a signed URL.
	result, err := g.Client.TradePagePay(p)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "alipay trade page pay")
	}

	return &PaymentURLResult{
		PaymentURL: result.String(),
		TradeNo:    orderID,
	}, nil
}

// QueryPayment queries the payment status via Alipay TradeQuery API.
func (g *AlipayGateway) QueryPayment(ctx context.Context, outTradeNo string) (*PaymentStatusResult, error) {
	p := alipay.TradeQuery{}
	p.OutTradeNo = outTradeNo

	rsp, err := g.Client.TradeQuery(ctx, p)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "alipay trade query")
	}

	status := mapAlipayTradeStatus(rsp.TradeStatus)

	amount, err := parseAmount(rsp.TotalAmount)
	if err != nil {
		return nil, err
	}

	return &PaymentStatusResult{
		TradeNo: rsp.TradeNo,
		Status:  status,
		Amount:  amount,
	}, nil
}

// Refund processes a refund via Alipay TradeRefund API (legacy 3-arg path;
// production refunds go through ChannelAdapter → RefundWithOutRefundNo).
func (g *AlipayGateway) Refund(ctx context.Context, outTradeNo string, refundAmount float64, refundReason string) (*RefundResult, error) {
	return g.RefundWithOutRefundNo(ctx, outTradeNo, refundAmount, refundReason, "")
}

// RefundWithOutRefundNo sets Alipay's out_request_no — mandatory for partial
// refunds and required to be unique across multiple refunds of one trade
// (https://opendocs.alipay.com/open/02ekfk). Without it a second same-amount
// partial refund is deduplicated against the first.
func (g *AlipayGateway) RefundWithOutRefundNo(ctx context.Context, outTradeNo string, refundAmount float64, refundReason, outRefundNo string) (*RefundResult, error) {
	p := alipay.TradeRefund{}
	p.OutTradeNo = outTradeNo
	p.RefundAmount = fmt.Sprintf("%.2f", refundAmount)
	p.RefundReason = refundReason
	if rno := strings.TrimSpace(outRefundNo); rno != "" {
		p.OutRequestNo = rno
	}

	rsp, err := g.Client.TradeRefund(ctx, p)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "alipay trade refund")
	}

	status := statusSuccess
	if !rsp.IsSuccess() {
		status = statusFailed
	}

	return &RefundResult{
		RefundNo: rsp.TradeNo,
		Status:   status,
	}, nil
}

// VerifyNotify verifies a payment notification from Alipay using RSA2 signature.
func (g *AlipayGateway) VerifyNotify(ctx context.Context, params map[string]string) (*NotifyResult, error) {
	// Build URL values from the params map for signature verification.
	values := make(url.Values, len(params))
	for k, v := range params {
		values.Set(k, v)
	}

	// Verify the signature.
	if err := g.Client.VerifySign(ctx, values); err != nil {
		return nil, apperrors.WrapInternalf(err, "alipay verify sign")
	}

	// Extract notification fields.
	tradeStatus := params["trade_status"]
	outTradeNo := params[fieldOutTradeNo]
	totalAmount := params["total_amount"]

	status := statusFailed
	if tradeStatus == string(alipay.TradeStatusSuccess) || tradeStatus == string(alipay.TradeStatusFinished) {
		status = statusPaid
	}

	amount, err := parseAmount(totalAmount)
	if err != nil {
		return nil, err
	}

	return &NotifyResult{
		OutTradeNo: outTradeNo,
		Status:     status,
		Amount:     amount,
	}, nil
}

// VerifyNotifyRequest verifies a payment notification from an HTTP request.
// This is the preferred method for Alipay notification handling as it
// parses and verifies the request directly using the SDK.
func (g *AlipayGateway) VerifyNotifyRequest(req *http.Request) (*NotifyResult, error) {
	notification, err := g.Client.GetTradeNotification(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "alipay get trade notification")
	}

	status := statusFailed
	if notification.TradeStatus == alipay.TradeStatusSuccess || notification.TradeStatus == alipay.TradeStatusFinished {
		status = statusPaid
	}

	amount, err := parseAmount(notification.TotalAmount)
	if err != nil {
		return nil, err
	}

	return &NotifyResult{
		OutTradeNo: notification.OutTradeNo,
		Status:     status,
		Amount:     amount,
	}, nil
}

// mapAlipayTradeStatus maps Alipay trade status to our internal status.
func mapAlipayTradeStatus(tradeStatus alipay.TradeStatus) string {
	switch tradeStatus {
	case alipay.TradeStatusSuccess, alipay.TradeStatusFinished:
		return statusPaid
	case alipay.TradeStatusClosed:
		return statusFailed
	case alipay.TradeStatusWaitBuyerPay:
		return statusPending
	case "TRADE_REFUND":
		return statusRefunded
	default:
		return statusPending
	}
}

// parseAmount parses a channel-reported amount string into float64.
// An empty string means the channel did not report an amount → 0 without error;
// a non-empty but unparsable value is a channel contract violation and must not
// silently collapse to 0 (that would make callbacks/reconciliation amounts wrong).
func parseAmount(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, apperrors.NewInvalidArgumentf("alipay: invalid amount %q", s)
	}
	return f, nil
}

// Ensure AlipayGateway implements PaymentGateway.
var _ PaymentGateway = (*AlipayGateway)(nil)
