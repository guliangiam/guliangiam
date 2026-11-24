package payment

import (
	"context"
	"net/url"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ChannelAdapter wraps the legacy infra/payment.PaymentGateway (4 methods)
// and exposes it as the new domain.ChannelGateway SPI (8 methods).
//
// Architecture: this adapter lives *inside* the payment infra package so it
// can reference the legacy PaymentGateway without creating an import cycle.
// The app layer only depends on domain.ChannelGateway — it never sees this
// file directly. See ADR-035.
type ChannelAdapter struct {
	inner PaymentGateway
	name  string
}

// NewChannelAdapter wraps a PaymentGateway under a stable channel name.
func NewChannelAdapter(name string, inner PaymentGateway) *ChannelAdapter {
	return &ChannelAdapter{inner: inner, name: name}
}

var _ domain.ChannelGateway = (*ChannelAdapter)(nil)

// Gateways whose refund idempotency depends on a business refund number must
// implement KeyedRefunder, otherwise ChannelAdapter silently downgrades them
// to the legacy order+amount key (P8 review: same-amount partial refunds
// would collide and be replayed/swallowed channel-side).
var (
	_ KeyedRefunder = (*StripeGateway)(nil)
	_ KeyedRefunder = (*SquareGateway)(nil)
	_ KeyedRefunder = (*CheckoutGateway)(nil)
	_ KeyedRefunder = (*WeChatPayGateway)(nil)
	_ KeyedRefunder = (*AlipayGateway)(nil)
)

// refundUnsupportedChannels lists adapters whose Refund returns
// apperrors.NewUnimplemented rather than calling the vendor API. Advertising
// SupportRefund for them made the orchestration layer promise a capability it
// could not deliver.
var refundUnsupportedChannels = map[string]bool{
	"gc":           true,
	"fastspring":   true,
	"lemonsqueezy": true,
}

func (a *ChannelAdapter) Metadata() domain.ChannelMetadata {
	return metadataFor(a.name)
}

func (a *ChannelAdapter) Charge(ctx context.Context, req domain.ChargeRequest) (*domain.ChargeResult, *domain.ChannelError) {
	res, err := a.inner.CreatePayment(
		ctx, req.OutTradeNo, req.Amount, req.Currency, req.Subject,
		req.NotifyURL, req.ReturnURL,
	)
	if err != nil {
		return nil, newChannelError("channel_create_failed", err)
	}
	return &domain.ChargeResult{
		State:          domain.AttemptStatePending,
		ChannelOrderNo: res.TradeNo,
		PayURL:         res.PaymentURL,
	}, nil
}

func (a *ChannelAdapter) Query(ctx context.Context, req domain.QueryRequest) (*domain.QueryResult, *domain.ChannelError) {
	res, err := a.inner.QueryPayment(ctx, req.OutTradeNo)
	if err != nil {
		return nil, newChannelError("channel_query_failed", err)
	}
	state := mapLegacyStatus(res.Status)
	return &domain.QueryResult{State: state, ChannelOrderNo: res.TradeNo, Amount: &res.Amount}, nil
}

func (a *ChannelAdapter) Refund(ctx context.Context, req domain.RefundRequest) (*domain.RefundResult, *domain.ChannelError) {
	var res *RefundResult
	var err error
	if kg, ok := a.inner.(KeyedRefunder); ok {
		// Forward the unique business refund number so channel-side
		// idempotency keys are per-refund, not per-order+amount.
		res, err = kg.RefundWithOutRefundNo(ctx, req.ChannelOrderNo, req.Amount, req.Reason, req.OutRefundNo)
	} else {
		res, err = a.inner.Refund(ctx, req.ChannelOrderNo, req.Amount, req.Reason)
	}
	if err != nil {
		return nil, newChannelError("channel_refund_failed", err)
	}
	state := mapLegacyStatus(res.Status)
	return &domain.RefundResult{State: state, ChannelRefundNo: res.RefundNo}, nil
}

func (a *ChannelAdapter) QueryRefund(ctx context.Context, req domain.QueryRefundRequest) (*domain.QueryRefundResult, *domain.ChannelError) {
	return nil, &domain.ChannelError{
		Code:     "not_implemented",
		Message:  "legacy gateway QueryRefund not supported",
		Category: domain.ErrorCategoryPermanent,
	}
}

func (a *ChannelAdapter) Close(ctx context.Context, req domain.CloseRequest) (*domain.CloseResult, *domain.ChannelError) {
	return nil, &domain.ChannelError{
		Code:     "not_implemented",
		Message:  "legacy gateway Close not supported",
		Category: domain.ErrorCategoryPermanent,
	}
}

func (a *ChannelAdapter) ParseCallback(ctx context.Context, req domain.CallbackRequest) (*domain.CallbackEvent, error) {
	if req.Channel == channelAlipay || req.Channel == channelWeChatPay {
		return nil, &channelUnsupportedError{channel: req.Channel, reason: "body-only callback needs dedicated adapter"}
	}
	params := map[string]string{}
	for k, vs := range req.Headers {
		if len(vs) > 0 {
			params[k] = vs[0]
		}
	}
	res, err := a.inner.VerifyNotify(ctx, params)
	if err != nil {
		return nil, err
	}
	state := mapLegacyStatus(res.Status)
	return &domain.CallbackEvent{OutTradeNo: res.OutTradeNo, State: state, Amount: &res.Amount, RawStatus: res.Status}, nil
}

// VerifyConfig performs structural validation of channel credentials
// without making outbound network calls. Network-level connectivity checks
// belong in an explicit TestConnectivity endpoint; this method only guards
// against obviously-invalid configs at save time.
//
// Required field matrix by channel:
//
//	alipay        — AppID + PrivateKey + NotifyURL
//	wechat_pay    — AppID + MerchantID + PrivateKey (mch.p12 / apiclient_key.pem) + NotifyURL
//	stripe        — WebhookSecret (whsec_...)
//	paypal        — AppID (client_id) + PrivateKey (client_secret)
//	braintree     — AppID + PrivateKey (public_key + private_key pair)
//	sandbox       — anything goes (test channel)
//	custom        — Extra map must exist; no structural check on fields
func (a *ChannelAdapter) VerifyConfig(_ context.Context, cfg domain.ChannelConfig) error {
	return configFailure(a.verifyConfig(cfg))
}

func (a *ChannelAdapter) verifyConfig(cfg domain.ChannelConfig) error {
	channel := strings.ToLower(a.name)

	switch channel {
	case "sandbox":
		return nil
	case channelAlipay:
		var missing []string
		if strings.TrimSpace(cfg.AppID) == "" {
			missing = append(missing, "AppID")
		}
		if strings.TrimSpace(cfg.PrivateKey) == "" {
			missing = append(missing, "PrivateKey")
		}
		if strings.TrimSpace(cfg.NotifyURL) == "" {
			missing = append(missing, "NotifyURL")
		}
		if cfg.NotifyURL != "" {
			if err := validateWebhookURL(cfg.NotifyURL); err != nil {
				return apperrors.WrapInvalidArgumentf(err, "alipay.NotifyURL validation failed")
			}
		}
		if len(missing) > 0 {
			return apperrors.NewInvalidArgumentf("alipay: missing required fields: %s", strings.Join(missing, ", "))
		}

	case channelWeChatPay, "wechat":
		var missing []string
		if strings.TrimSpace(cfg.AppID) == "" {
			missing = append(missing, "AppID")
		}
		if strings.TrimSpace(cfg.MerchantID) == "" {
			missing = append(missing, "MerchantID")
		}
		if strings.TrimSpace(cfg.PrivateKey) == "" {
			missing = append(missing, "PrivateKey (mch.p12 or apiclient_key.pem)")
		}
		if strings.TrimSpace(cfg.NotifyURL) == "" {
			missing = append(missing, "NotifyURL")
		}
		if cfg.NotifyURL != "" {
			if err := validateWebhookURL(cfg.NotifyURL); err != nil {
				return apperrors.WrapInvalidArgumentf(err, "wechat_pay.NotifyURL validation failed")
			}
		}
		if len(missing) > 0 {
			return apperrors.NewInvalidArgumentf("wechat_pay: missing required fields: %s", strings.Join(missing, ", "))
		}

	case "stripe":
		if strings.TrimSpace(cfg.WebhookSecret) == "" {
			return apperrors.NewInvalidArgument("stripe: missing WebhookSecret (whsec_...)")
		}
		if !strings.HasPrefix(cfg.WebhookSecret, "whsec_") {
			return apperrors.NewInvalidArgument("stripe: WebhookSecret must start with whsec_")
		}

	case "paypal":
		var missing []string
		if strings.TrimSpace(cfg.AppID) == "" {
			missing = append(missing, "AppID (client_id)")
		}
		if strings.TrimSpace(cfg.PrivateKey) == "" {
			missing = append(missing, "PrivateKey (client_secret)")
		}
		if len(missing) > 0 {
			return apperrors.NewInvalidArgumentf("paypal: missing required fields: %s", strings.Join(missing, ", "))
		}

	case "braintree":
		var missing []string
		if strings.TrimSpace(cfg.AppID) == "" {
			missing = append(missing, "AppID (merchant_id)")
		}
		if strings.TrimSpace(cfg.PrivateKey) == "" {
			missing = append(missing, "PrivateKey (private_key)")
		}
		if len(missing) > 0 {
			return apperrors.NewInvalidArgumentf("braintree: missing required fields: %s", strings.Join(missing, ", "))
		}

	case "custom":
		if cfg.Extra == nil {
			return apperrors.NewInvalidArgument("custom channel: Extra map is required")
		}

	default:
		return apperrors.NewInvalidArgumentf("unknown channel %q — cannot verify config", a.name)
	}

	return nil
}

// validateWebhookURL rejects http:// (cleartext) and malformed URLs.
func validateWebhookURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return apperrors.WrapInvalidArgumentf(err, "invalid webhook URL")
	}
	if u.Scheme != "https" {
		return apperrors.NewInvalidArgumentf("webhook URL must use https://, got %q", u.Scheme)
	}
	if u.Host == "" {
		return apperrors.NewInvalidArgument("webhook URL missing host")
	}
	return nil
}

func (a *ChannelAdapter) DownloadStatement(ctx context.Context, req domain.StatementRequest) ([]domain.StatementLine, *domain.ChannelError) {
	return nil, &domain.ChannelError{
		Code:     "not_supported",
		Message:  "legacy PaymentGateway does not support statement download",
		Category: domain.ErrorCategoryPermanent,
	}
}

func mapLegacyStatus(s string) domain.AttemptState {
	switch s {
	case statusPaid, statusSuccess, gatewaySucceededLower:
		return domain.AttemptStateSucceeded
	case statusFailed, "error":
		return domain.AttemptStateFailed
	case statusRefunded:
		return domain.AttemptStateUnknown
	default:
		return domain.AttemptStatePending
	}
}

type channelUnsupportedError struct {
	channel string
	reason  string
}

func (e *channelUnsupportedError) Error() string { return e.channel + ": " + e.reason }
