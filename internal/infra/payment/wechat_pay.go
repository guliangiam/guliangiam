package payment

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"math"
	"strconv"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	refunddomestic "github.com/wechatpay-apiv3/wechatpay-go/services/refunddomestic"
)

// WeChatPayGateway implements PaymentGateway via WeChat Pay.
//
// 校准（OPEN-API-75 P8，https://pay.weixin.qq.com/doc/v3/merchant/4012068212）：
//   - 请求签名 WECHATPAY2-SHA256-RSA2048 与应答验签由官方 wechatpay-go core
//     客户端实现（option.WithMerchantCredential + 平台证书验签），无偏差；
//   - 平台证书采用配置注入的静态证书（option.WithWechatPayCertificate）；官方
//     自动下载轮换形态（WithWechatPayAutoAuthCipher）未接入——证书轮换属运维
//     前置（L4），轮换窗口需更新 PAYMENT_WECHAT_PLATFORM_CERTS；
//   - 回调 Wechatpay-Signature 验签 + AEAD_AES_256_GCM 解密在 transport 层
//     经官方 notify 解析器完成（payments/notify.go，向量测试钉契约）；
//   - 元→分转换修复为 math.Round（原 float64 截断 88.88 元→8887 分，资损偏差）。
type WeChatPayGateway struct {
	Svc       *native.NativeApiService
	RefundSvc *refunddomestic.RefundsApiService
	AppID     string
	MchID     string
	IsSandbox bool
}

// NewWeChatPayGateway creates a new WeChat Pay gateway.
// It requires a merchant ID, certificate serial number, and RSA private key
// for signing requests, plus the WeChat Pay platform X.509 certificates
// for verifying responses.
func NewWeChatPayGateway(appID, mchID, certSerialNo string, privateKey *rsa.PrivateKey, wechatPayCerts []*x509.Certificate, isSandbox bool) (*WeChatPayGateway, error) {
	ctx := context.Background()

	opts := []core.ClientOption{
		option.WithMerchantCredential(mchID, certSerialNo, privateKey),
		option.WithWechatPayCertificate(wechatPayCerts),
	}

	client, err := core.NewClient(ctx, opts...)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "init wechat pay client")
	}

	svc := &native.NativeApiService{Client: client}
	refundSvc := &refunddomestic.RefundsApiService{Client: client}

	return &WeChatPayGateway{
		Svc:       svc,
		RefundSvc: refundSvc,
		AppID:     appID,
		MchID:     mchID,
		IsSandbox: isSandbox,
	}, nil
}

// CreatePayment creates a payment via WeChat Pay Native Order API.
func (g *WeChatPayGateway) CreatePayment(ctx context.Context, orderID string, amount float64, _, subject, notifyURL, _ string) (*PaymentURLResult, error) {
	// WeChat Pay uses amount in fen (cents), not yuan. Round to avoid float truncation.
	amountFen := int64(math.Round(amount * 100))

	prepay := native.PrepayRequest{
		Appid:       &g.AppID,
		Mchid:       &g.MchID,
		Description: &subject,
		OutTradeNo:  &orderID,
		NotifyUrl:   &notifyURL,
		Amount: &native.Amount{
			Total: &amountFen,
		},
	}

	resp, _, err := g.Svc.Prepay(ctx, prepay)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "wechat pay prepay")
	}

	codeURL := ""
	if resp.CodeUrl != nil {
		codeURL = *resp.CodeUrl
	}

	return &PaymentURLResult{
		PaymentURL: codeURL,
		TradeNo:    orderID,
	}, nil
}

// QueryPayment queries the payment status via WeChat Pay Order Query API.
func (g *WeChatPayGateway) QueryPayment(ctx context.Context, outTradeNo string) (*PaymentStatusResult, error) {
	query := native.QueryOrderByOutTradeNoRequest{
		OutTradeNo: &outTradeNo,
		Mchid:      &g.MchID,
	}

	resp, _, err := g.Svc.QueryOrderByOutTradeNo(ctx, query)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "wechat pay query order")
	}

	status := statusPending
	if resp.TradeState != nil {
		status = mapWeChatTradeState(*resp.TradeState)
	}

	amount := 0.0
	if resp.Amount != nil && resp.Amount.Total != nil {
		amount = float64(*resp.Amount.Total) / 100.0
	}

	tradeNo := outTradeNo
	if resp.TransactionId != nil {
		tradeNo = *resp.TransactionId
	}

	return &PaymentStatusResult{
		TradeNo: tradeNo,
		Status:  status,
		Amount:  amount,
	}, nil
}

// Refund processes a refund via WeChat Pay Refund API (legacy 3-arg path;
// production refunds go through ChannelAdapter → RefundWithOutRefundNo).
func (g *WeChatPayGateway) Refund(ctx context.Context, outTradeNo string, refundAmount float64, refundReason string) (*RefundResult, error) {
	return g.RefundWithOutRefundNo(ctx, outTradeNo, refundAmount, refundReason, "")
}

// RefundWithOutRefundNo sets WeChat's out_refund_no to the business refund
// number, which the API requires to be unique per merchant refund — the
// former order-number fallback blocked a second partial refund on the same
// order. Legacy callers without a refund number keep the outTradeNo fallback.
func (g *WeChatPayGateway) RefundWithOutRefundNo(ctx context.Context, outTradeNo string, refundAmount float64, refundReason, outRefundNo string) (*RefundResult, error) {
	refundAmtFen := int64(math.Round(refundAmount * 100))

	// Query the order first to get the total amount.
	orderResult, err := g.QueryPayment(ctx, outTradeNo)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "wechat pay refund: query order")
	}
	totalFen := int64(math.Round(orderResult.Amount * 100))

	merchantRefundNo := strings.TrimSpace(outRefundNo)
	if merchantRefundNo == "" {
		merchantRefundNo = outTradeNo
	}

	req := refunddomestic.CreateRequest{
		OutTradeNo:  &outTradeNo,
		OutRefundNo: &merchantRefundNo,
		Reason:      &refundReason,
		Amount: &refunddomestic.AmountReq{
			Refund: &refundAmtFen,
			Total:  &totalFen,
		},
	}

	r, _, err := g.RefundSvc.Create(ctx, req)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "wechat pay refund")
	}

	refundNo := outTradeNo
	if r.RefundId != nil {
		refundNo = *r.RefundId
	}

	status := statusSuccess
	if r.Status != nil && *r.Status != gatewaySuccessUpper {
		status = statusProcessing
	}

	return &RefundResult{
		RefundNo: refundNo,
		Status:   status,
	}, nil
}

// VerifyNotify verifies a payment notification from WeChat Pay.
func (g *WeChatPayGateway) VerifyNotify(_ context.Context, params map[string]string) (*NotifyResult, error) {
	// WeChat Pay V3 uses HTTP headers for signature verification.
	// The handler layer extracts the notification body and verifies
	// using the certificate-based signature.
	outTradeNo := params[fieldOutTradeNo]
	tradeState := params["trade_state"]
	totalStr := params["total"]

	status := mapWeChatTradeState(tradeState)

	amount := 0.0
	if totalStr != "" {
		fen, err := strconv.ParseInt(totalStr, 10, 64)
		if err != nil {
			return nil, apperrors.NewInvalidArgumentf("wechat pay: invalid total %q", totalStr)
		}
		amount = float64(fen) / 100.0
	}

	return &NotifyResult{
		OutTradeNo: outTradeNo,
		Status:     status,
		Amount:     amount,
	}, nil
}

// mapWeChatTradeState maps WeChat Pay trade state to our internal status.
func mapWeChatTradeState(state string) string {
	switch state {
	case gatewaySuccessUpper:
		return statusPaid
	case "REFUND":
		return statusRefunded
	case "NOTPAY":
		return statusPending
	case "CLOSED":
		return statusFailed
	case "REVOKED":
		return statusFailed
	case "USERPAYING":
		return statusPending
	case "PAYERROR":
		return statusFailed
	default:
		return statusPending
	}
}

// Ensure WeChatPayGateway implements PaymentGateway.
var _ PaymentGateway = (*WeChatPayGateway)(nil)
