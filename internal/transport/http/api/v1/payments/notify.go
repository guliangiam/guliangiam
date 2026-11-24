package payments

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/stripe/stripe-go/v82/webhook"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	wechatnotify "github.com/wechatpay-apiv3/wechatpay-go/core/notify"
)

// errNotifyNotConfigured marks a channel whose verification material is
// missing. Callbacks are rejected (fail-closed): a payment callback must never
// be accepted without a cryptographic check, because anyone who can reach the
// endpoint could otherwise forge a "payment succeeded" notification.
var errNotifyNotConfigured = errors.New("payment callback verification material not configured")

// OrderNotifier is the slice of the order service the notify handler drives.
// The business fields it receives (trade no, status, amount, currency) all come
// from a signature-verified payload, never from the URL or query string.
type OrderNotifier interface {
	NotifyOrderPayment(ctx context.Context, tradeNo, providerType, tradeStatus string, amount float64, currency string) error
}

// NotifyConfig carries the signature-verification material for each channel.
// An empty entry means that channel's callbacks are rejected with 503, never
// accepted unverified.
type NotifyConfig struct {
	// AlipayPublicKey is the Alipay RSA2 public key (PEM or raw base64 DER).
	AlipayPublicKey string
	// WeChatAPIV3Key is the WeChat Pay V3 APIv3 key used for AEAD_AES_256_GCM
	// decryption of the notification resource.
	WeChatAPIV3Key string
	// WeChatPlatformCertPEM is one or more WeChat Pay platform certificates
	// (PEM bundle) used for WeChat Pay V3 RSA signature verification.
	WeChatPlatformCertPEM string
	// StripeWebhookSecret is the Stripe webhook signing secret.
	StripeWebhookSecret string
}

// verifiedNotify is the business payload extracted from a callback *after* its
// signature has been verified. Every field that drives the order state machine
// comes from here — the query string and form values of the outer request are
// never trusted.
type verifiedNotify struct {
	TradeNo     string
	TradeStatus string
	Amount      float64
	Currency    string
	EventID     string
}

// NotifyHandler handles payment gateway callback notifications.
// It is separate from the main Handler because it needs access to the OrderService
// for updating order status based on payment callbacks.
type NotifyHandler struct {
	OrderService OrderNotifier
	// CallbackPipeline 可选 — 注入后把原始回调写入 pay_callback_receipts。
	// nil 时跳过，保留原有行为（向后兼容）。
	CallbackPipeline interface {
		Process(ctx context.Context, channel string, payload []byte, headers map[string][]string, clientIP string, organizationID string) (receiptID string, advanced bool, err error)
	}
	// AlipayPublicKey is used for Alipay RSA2 signature verification.
	AlipayPublicKey string
	// WeChatAPIV3Key is used to decrypt WeChat Pay V3 notification resources.
	WeChatAPIV3Key string
	// WeChatPlatformCerts are the WeChat Pay platform certificates used to
	// verify the WECHATPAY2-SHA256-RSA2048 notification signature.
	WeChatPlatformCerts []*x509.Certificate
	// StripeWebhookSecret is used for Stripe webhook signature verification.
	StripeWebhookSecret string
}

// NewNotifyHandler creates a NotifyHandler with the given verification
// material. Channels left unconfigured reject their callbacks.
func NewNotifyHandler(orderService OrderNotifier, cfg NotifyConfig) NotifyHandler {
	h := NotifyHandler{
		OrderService:        orderService,
		AlipayPublicKey:     cfg.AlipayPublicKey,
		WeChatAPIV3Key:      cfg.WeChatAPIV3Key,
		StripeWebhookSecret: cfg.StripeWebhookSecret,
	}
	if cfg.WeChatPlatformCertPEM != "" {
		certs, err := parsePlatformCertificates(cfg.WeChatPlatformCertPEM)
		if err != nil {
			// Fail closed: an unparsable certificate leaves the channel
			// unconfigured, so its callbacks are rejected.
			slog.Error("failed to parse wechat pay platform certificate, wechat callbacks will be rejected",
				"error", err)
		}
		h.WeChatPlatformCerts = certs
	}
	return h
}

// Notify handles incoming payment gateway callback notifications.
func (h NotifyHandler) Notify(w http.ResponseWriter, r *http.Request) {
	providerType := chi.URLParam(r, "providerType")
	if providerType == "" {
		route.WriteError(w, r, apperrors.NewInvalidArgument("providerType is required"))
		return
	}

	// Read the raw body exactly once. Signature verification and business
	// parsing both operate on these same bytes: re-reading r.Body would verify
	// an empty body while processing attacker-controlled leftovers.
	rawBody, bodyErr := io.ReadAll(r.Body)
	if bodyErr != nil {
		slog.Error("failed to read payment notify body",
			"provider_type", providerType, "error", bodyErr)
		// Every supported gateway (Alipay, WeChat Pay, Stripe) treats any non-2xx
		// callback answer as a delivery failure and retries; 413 therefore keeps
		// the retry semantics of the old 400 while naming the real cause. The
		// unified envelope also carries the request/trace ids for support.
		var maxErr *http.MaxBytesError
		if errors.As(bodyErr, &maxErr) {
			route.WriteError(w, r, bodyErr)
			return
		}
		route.WriteError(w, r, apperrors.NewInvalidArgument("invalid body"))
		return
	}

	// Two vocabularies meet at this endpoint: the protocol face historically
	// says "wechat", while the channel registry and DB CHECK use "wechat_pay".
	// Verify with the protocol key, persist with the registry key.
	protocolChannel := protocolNotifyChannel(providerType)
	registryChannel := canonicalNotifyChannel(providerType)

	// Verify the callback signature and extract the business payload. All
	// fields below come from the verified payload, never from query/form
	// values of the outer request.
	verified, err := h.verifyNotify(protocolChannel, r, rawBody)
	if err != nil {
		if errors.Is(err, errNotifyNotConfigured) {
			slog.Error("payment notify channel not configured, rejecting notification (fail-closed)",
				"provider_type", providerType)
			writeNotifyUnavailable(w, r, providerType)
			return
		}
		slog.Error("payment notify signature verification failed, rejecting",
			"provider_type", providerType, "error", err)
		writeNotifyUnauthorized(w, r, providerType)
		return
	}

	// 把原始 payload + headers 交给 App 层 CallbackPipeline 做五阶段归档。
	// signature 已在本 handler 验证过，pipeline 不再重复验签。
	if h.CallbackPipeline != nil && bodyErr == nil && len(rawBody) > 0 {
		headers := map[string][]string{}
		for k, v := range r.Header {
			headers[k] = v
		}
		clientIP, _ := r.Context().Value("client_ip").(string)
		orgID, _ := r.Context().Value("organization_id").(string)
		if _, _, err := h.CallbackPipeline.Process(
			r.Context(), registryChannel, rawBody, headers, clientIP, orgID,
		); err != nil {
			slog.Warn("callback pipeline persist failed, continuing",
				"error", err)
		}
	}

	// A verified event that references no local order (for example an
	// unrelated Stripe event type) is acknowledged but drives nothing.
	if verified.TradeNo == "" {
		slog.Info("payment notify verified but carries no order reference, acking without action",
			"provider_type", protocolChannel, "event_id", verified.EventID)
		writeNotifySuccess(w, protocolChannel)
		return
	}

	// Hand the channel's answer to the app layer, which owns the translation
	// from a provider status vocabulary to the registry's state vocabulary.
	// The app layer re-checks the amount/currency binding against the local
	// order and treats repeat deliveries as no-ops.
	err = h.OrderService.NotifyOrderPayment(r.Context(),
		verified.TradeNo, protocolChannel, verified.TradeStatus, verified.Amount, verified.Currency)
	if err != nil {
		slog.Error("failed to update order from payment notify",
			"provider_type", protocolChannel, "trade_no", verified.TradeNo, "error", err)
		writeNotifyFailure(w, r, protocolChannel)
		return
	}

	slog.Info("payment notify processed successfully",
		"provider_type", protocolChannel, "registry_channel", registryChannel,
		"trade_no", verified.TradeNo, "trade_status", verified.TradeStatus)

	writeNotifySuccess(w, protocolChannel)
}

// canonicalNotifyChannel maps inbound callback spellings to the registry key
// used by pay_callback_receipts and payment attempts. "wechat_pay" is the
// canonical key; "wechat" and "wxpay" are accepted as legacy aliases.
func canonicalNotifyChannel(providerType string) string {
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case "wechat", "wxpay":
		return "wechat_pay"
	default:
		return strings.ToLower(strings.TrimSpace(providerType))
	}
}

// protocolNotifyChannel maps registry keys back to the vocabulary that
// signature verification and trade-status translation understand.
func protocolNotifyChannel(providerType string) string {
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case "wechat_pay", "wxpay":
		return "wechat"
	default:
		return strings.ToLower(strings.TrimSpace(providerType))
	}
}

// verifyNotify verifies the callback signature over rawBody and extracts the
// business payload from the verified bytes. Each provider uses a different
// signature method:
//   - Alipay: RSA2 signature verification using the Alipay public key
//   - WeChat: WeChat Pay V3 platform-certificate RSA verification + official
//     AEAD_AES_256_GCM decryption (wechatpay-go SDK)
//   - Stripe: official stripe-go webhook.ConstructEvent with timestamp
//     tolerance
func (h NotifyHandler) verifyNotify(providerType string, r *http.Request, rawBody []byte) (*verifiedNotify, error) {
	providerType = protocolNotifyChannel(providerType)
	switch providerType {
	case "alipay":
		return h.verifyAlipayNotify(rawBody)
	case "wechat":
		return h.verifyWeChatNotify(r, rawBody)
	case "stripe":
		return h.verifyStripeNotify(r, rawBody)
	default:
		slog.Warn("unknown payment provider for signature verification",
			"provider_type", providerType)
		return nil, apperrors.NewInvalidArgumentf("unsupported payment provider: %s", providerType)
	}
}

// verifyAlipayNotify verifies the Alipay callback using RSA2 over the raw
// form-encoded body. The parameters are parsed from rawBody only — the query
// string of the request is never consulted, so it cannot inject out_trade_no
// or trade_status.
func (h NotifyHandler) verifyAlipayNotify(rawBody []byte) (*verifiedNotify, error) {
	if h.AlipayPublicKey == "" {
		return nil, errNotifyNotConfigured
	}

	values, err := url.ParseQuery(string(rawBody))
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "parse alipay notify body")
	}

	sign := values.Get("sign")
	if sign == "" {
		slog.Error("alipay notify missing sign field")
		return nil, apperrors.NewInvalidArgument("alipay notify missing sign")
	}

	signData := buildAlipaySignData(values)
	verified, err := verifyRSA2(signData, sign, h.AlipayPublicKey)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "verify alipay signature")
	}
	if !verified {
		return nil, apperrors.NewInvalidArgument("alipay signature mismatch")
	}

	return &verifiedNotify{
		TradeNo:     values.Get("out_trade_no"),
		TradeStatus: values.Get("trade_status"),
		Amount:      parseAmountYuan(values.Get("total_amount")),
		Currency:    values.Get("currency"),
		EventID:     values.Get("notify_id"),
	}, nil
}

// buildAlipaySignData builds the Alipay RSA2 signing source: the request
// parameters except "sign" and "sign_type", sorted by byte order and joined
// as key=value pairs with "&".
//
// 校准（OPEN-API-75 P8，https://opendocs.alipay.com/common/02mse3）：待验签串
// 使用收到的**已解码参数值原样**拼接，不做 URL 编码——官方 Java/PHP 验签示例
// 均以 request.getParameter 解码值直接参与签名。原实现对 key/value 做
// url.QueryEscape，真实通知的 gmt_payment（含空格）与 subject（中文）必然验签
// 失败（fail-closed 误拒合法回调），已修复。空值参数不参与签名（官方规则）。
func buildAlipaySignData(values url.Values) string {
	var keys []string
	for k := range values {
		if k == "sign" || k == "sign_type" {
			continue
		}
		if values.Get(k) == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf strings.Builder
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte('&')
		}
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(values.Get(k))
	}
	return buf.String()
}

// wechatTransaction is the decrypted WeChat Pay V3 transaction notification.
type wechatTransaction struct {
	OutTradeNo    string `json:"out_trade_no"`
	TradeState    string `json:"trade_state"`
	TransactionID string `json:"transaction_id"`
	Amount        struct {
		Total    int64  `json:"total"`
		Currency string `json:"currency"`
	} `json:"amount"`
}

// verifyWeChatNotify verifies a WeChat Pay V3 notification with the official
// wechatpay-go SDK: platform-certificate RSA signature verification (including
// the 5-minute timestamp window) followed by official AEAD_AES_256_GCM
// decryption. The custom HMAC scheme is gone: WeChat Pay V3 signatures are
// RSA over the platform certificate and can never be checked with a shared
// secret.
func (h NotifyHandler) verifyWeChatNotify(r *http.Request, rawBody []byte) (*verifiedNotify, error) {
	if h.WeChatAPIV3Key == "" || len(h.WeChatPlatformCerts) == 0 {
		return nil, errNotifyNotConfigured
	}

	verifier := verifiers.NewSHA256WithRSAVerifier(core.NewCertificateMapWithList(h.WeChatPlatformCerts))
	handler, err := wechatnotify.NewRSANotifyHandler(h.WeChatAPIV3Key, verifier)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "init wechat pay notify handler")
	}

	// The SDK parser reads the request body to verify the signature; feed it
	// the one copy of the raw bytes we read above.
	r.Body = io.NopCloser(bytes.NewReader(rawBody))

	var tx wechatTransaction
	req, err := handler.ParseNotifyRequest(r.Context(), r, &tx)
	if err != nil {
		return nil, apperrors.NewInvalidArgumentf("wechat pay notification rejected: %v", err)
	}

	eventID := ""
	if req != nil {
		eventID = req.ID
	}
	currency := tx.Amount.Currency
	if currency == "" {
		// WeChat Pay transacts in CNY; the notification always settles in CNY.
		currency = "CNY"
	}
	return &verifiedNotify{
		TradeNo:     tx.OutTradeNo,
		TradeStatus: tx.TradeState,
		Amount:      float64(tx.Amount.Total) / 100.0,
		Currency:    currency,
		EventID:     eventID,
	}, nil
}

// stripeCallbackEvent is the subset of a Stripe event needed to drive the
// order state machine. It is decoded from the same raw bytes that
// webhook.ConstructEvent verified.
type stripeCallbackEvent struct {
	Type string `json:"type"`
	Data struct {
		Object struct {
			Metadata          map[string]string `json:"metadata"`
			ClientReferenceID string            `json:"client_reference_id"`
			AmountTotal       *int64            `json:"amount_total"`
			AmountReceived    *int64            `json:"amount_received"`
			Amount            *int64            `json:"amount"`
			Currency          string            `json:"currency"`
			PaymentStatus     string            `json:"payment_status"`
			Status            string            `json:"status"`
		} `json:"object"`
	} `json:"data"`
}

// verifyStripeNotify verifies the Stripe webhook with the official
// stripe-go webhook.ConstructEvent, which checks the Stripe-Signature HMAC and
// the timestamp tolerance (stale events are rejected, closing the replay
// window). The business fields are then taken from the verified payload.
func (h NotifyHandler) verifyStripeNotify(r *http.Request, rawBody []byte) (*verifiedNotify, error) {
	if h.StripeWebhookSecret == "" {
		return nil, errNotifyNotConfigured
	}

	sigHeader := r.Header.Get("Stripe-Signature")
	if sigHeader == "" {
		return nil, apperrors.NewInvalidArgument("stripe notify missing Stripe-Signature header")
	}

	// Official verification: HMAC over timestamp.payload plus timestamp
	// tolerance (default 5 minutes) — ConstructEventWithOptions keeps both and
	// only relaxes the SDK's API-version pinning, which would reject otherwise
	// valid events created under a different account API version.
	event, err := webhook.ConstructEventWithOptions(rawBody, sigHeader, h.StripeWebhookSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
	if err != nil {
		return nil, apperrors.NewInvalidArgumentf("stripe signature rejected: %v", err)
	}

	var payload stripeCallbackEvent
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil, apperrors.WrapInternalf(err, "parse stripe notify payload")
	}

	obj := payload.Data.Object
	tradeNo := obj.Metadata["out_trade_no"]
	if tradeNo == "" {
		tradeNo = obj.ClientReferenceID
	}

	var amountCents int64
	switch {
	case obj.AmountTotal != nil:
		amountCents = *obj.AmountTotal
	case obj.AmountReceived != nil:
		amountCents = *obj.AmountReceived
	case obj.Amount != nil:
		amountCents = *obj.Amount
	}

	return &verifiedNotify{
		TradeNo:     tradeNo,
		TradeStatus: stripeTradeStatus(obj.PaymentStatus, obj.Status),
		Amount:      float64(amountCents) / 100.0,
		Currency:    strings.ToLower(obj.Currency),
		EventID:     event.ID,
	}, nil
}

// stripeTradeStatus maps the verified Stripe object status onto the channel
// status vocabulary the domain understands. An unrecognized status becomes
// empty, which the domain treats as unknown and leaves the order untouched.
func stripeTradeStatus(paymentStatus, status string) string {
	s := strings.ToLower(strings.TrimSpace(paymentStatus))
	if s == "" {
		s = strings.ToLower(strings.TrimSpace(status))
	}
	switch s {
	case "paid", "succeeded", "no_payment_required":
		return "PAID"
	case "unpaid", "pending", "processing", "requires_action", "requires_confirmation", "requires_payment_method":
		return "PENDING"
	case "canceled", "cancelled", "expired":
		return "CANCELED"
	case "refunded", "partially_refunded":
		return "REFUNDED"
	default:
		return ""
	}
}

// parseAmountYuan parses a decimal major-unit amount string ("88.88").
func parseAmountYuan(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// verifyRSA2 verifies an RSA2 (SHA256withRSA/PKCS1v15) signature using the Alipay public key.
// The publicKeyPEM should be a PEM-encoded RSA public key (or raw base64 DER).
func verifyRSA2(signData, sign, publicKeyPEM string) (bool, error) {
	if publicKeyPEM == "" {
		return false, apperrors.NewNotFound("alipay public key is empty")
	}

	// Decode the base64-encoded signature from Alipay.
	signBytes, err := base64.StdEncoding.DecodeString(sign)
	if err != nil {
		return false, apperrors.WrapInternalf(err, "base64 decode signature")
	}

	// Parse the public key.
	pubKey, err := parseAlipayPublicKey(publicKeyPEM)
	if err != nil {
		return false, apperrors.WrapInternalf(err, "parse public key")
	}

	// Verify the SHA256withRSA signature.
	hashed := sha256.Sum256([]byte(signData))
	err = rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hashed[:], signBytes)
	if err != nil {
		return false, nil // verification failed, but not an internal error
	}

	return true, nil
}

// parseAlipayPublicKey parses an Alipay public key string.
// Alipay provides the public key either as a raw base64 DER string or as a PEM block.
func parseAlipayPublicKey(keyStr string) (*rsa.PublicKey, error) {
	// Try PEM format first
	block, _ := pem.Decode([]byte(keyStr))
	if block != nil {
		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "parse PKIX public key from PEM")
		}
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, apperrors.NewInternalf("public key is not RSA, got %T", pub)
		}
		return rsaPub, nil
	}

	// Try raw base64 DER format (Alipay default)
	derBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(keyStr))
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "base64 decode public key")
	}

	pub, err := x509.ParsePKIXPublicKey(derBytes)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "parse PKIX public key from DER")
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, apperrors.NewInternalf("public key is not RSA, got %T", pub)
	}
	return rsaPub, nil
}

// parsePlatformCertificates parses a PEM bundle of WeChat Pay platform
// certificates.
func parsePlatformCertificates(pemBundle string) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	rest := []byte(pemBundle)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "parse wechat pay platform certificate")
		}
		certs = append(certs, cert)
	}
	if len(certs) == 0 {
		return nil, apperrors.NewInvalidArgument("no certificate found in wechat pay platform certificate bundle")
	}
	return certs, nil
}

// writeNotifySuccess writes a success response that the payment gateway expects.
func writeNotifySuccess(w http.ResponseWriter, providerType string) {
	switch providerType {
	case "alipay", "wechat":
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	default:
		route.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// writeNotifyFailure writes a failure response so the gateway retries.
func writeNotifyFailure(w http.ResponseWriter, r *http.Request, providerType string) {
	switch providerType {
	case "alipay", "wechat":
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("fail"))
	default:
		route.WriteError(w, r, apperrors.NewInternal("processing failed"))
	}
}

// writeNotifyUnauthorized rejects a callback whose signature failed to verify.
func writeNotifyUnauthorized(w http.ResponseWriter, r *http.Request, providerType string) {
	switch providerType {
	case "alipay", "wechat":
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("fail"))
	default:
		route.WriteError(w, r, apperrors.NewUnauthorized("signature verification failed"))
	}
}

// writeNotifyUnavailable rejects a callback for a channel whose verification
// material is not configured (fail-closed).
func writeNotifyUnavailable(w http.ResponseWriter, r *http.Request, providerType string) {
	switch providerType {
	case "alipay", "wechat":
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("fail"))
	default:
		route.WriteError(w, r, apperrors.NewUnavailable("callback verification not configured"))
	}
}
