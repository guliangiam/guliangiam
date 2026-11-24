package payments

// notify_security_test.go pins the fail-closed callback security baseline:
// empty configuration rejects, only signature-verified payload fields drive the
// order, tampered bodies and stale timestamps are rejected, and amount binding
// plus event idempotency hold at the service boundary. All signatures are
// produced locally (RSA test keys, official Stripe signing method) — no network.

import (
	"bytes"
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// newSelfSignedCert generates a self-signed RSA certificate to act as a WeChat
// Pay platform certificate in tests.
func newSelfSignedCert(t *testing.T) (*x509.Certificate, *rsa.PrivateKey, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(123456789),
		Subject:      pkix.Name{CommonName: "wechatpay-platform-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	bundle := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return cert, key, string(bundle)
}

// newNotifyRequest builds a request carrying the chi route context the handler
// expects.
func newNotifyRequest(providerType, target string, body []byte) *http.Request {
	r := httptest.NewRequest("POST", target, bytes.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("providerType", providerType)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// stripeSigHeader computes a Stripe-Signature header the official way.
func stripeSigHeader(t *testing.T, secret string, payload []byte, ts time.Time) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts.Unix(), 10) + "."))
	mac.Write(payload)
	return fmt.Sprintf("t=%d,v1=%s", ts.Unix(), hex.EncodeToString(mac.Sum(nil)))
}

func stripeEventBody(t *testing.T, outTradeNo string, amountCents int64, currency, paymentStatus string) []byte {
	t.Helper()
	body := fmt.Sprintf(
		`{"id":"evt_test","object":"event","type":"checkout.session.completed","api_version":"2025-01-01.base","data":{"object":{"object":"checkout.session","metadata":{"out_trade_no":%q},"amount_total":%d,"currency":%q,"payment_status":%q,"status":"complete"}}}`,
		outTradeNo, amountCents, currency, paymentStatus)
	return []byte(body)
}

// alipaySignedBody builds an Alipay form-encoded callback signed with RSA2 over
// the same source the verifier reconstructs.
func alipaySignedBody(t *testing.T, priv *rsa.PrivateKey, values map[string]string) []byte {
	t.Helper()
	form := make(url.Values, len(values)+2)
	for k, v := range values {
		form.Set(k, v)
	}

	signData := buildAlipaySignData(form)
	hashed := sha256.Sum256([]byte(signData))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, hashed[:])
	require.NoError(t, err)

	form.Set("sign", base64.StdEncoding.EncodeToString(sig))
	form.Set("sign_type", "RSA2")
	return []byte(form.Encode())
}

// wechatSignedBody builds a WeChat Pay V3 notification: the transaction is
// AEAD_AES_256_GCM-encrypted with the APIv3 key and the envelope is signed with
// the platform certificate's private key, exactly as WeChat Pay does.
func wechatSignedBody(t *testing.T, apiV3Key string, platformCert *x509.Certificate, platformKey *rsa.PrivateKey, tx map[string]any, ts time.Time) ([]byte, map[string]string) {
	t.Helper()
	plaintext, err := json.Marshal(tx)
	require.NoError(t, err)

	block, err := aes.NewCipher([]byte(apiV3Key))
	require.NoError(t, err)
	aead, err := cipher.NewGCM(block)
	require.NoError(t, err)

	// The resource nonce is an ASCII string of exactly one GCM nonce size.
	nonceBytes := make([]byte, aead.NonceSize())
	_, err = rand.Read(nonceBytes)
	require.NoError(t, err)
	nonce := hex.EncodeToString(nonceBytes)[:aead.NonceSize()]
	aad := []byte("transaction")
	ciphertext := aead.Seal(nil, []byte(nonce), plaintext, aad)

	envelope := map[string]any{
		"id":            "evt_wx_test",
		"event_type":    "TRANSACTION.SUCCESS",
		"resource_type": "trade",
		"resource": map[string]any{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      base64.StdEncoding.EncodeToString(ciphertext),
			"nonce":           nonce,
			"associated_data": string(aad),
		},
	}
	body, err := json.Marshal(envelope)
	require.NoError(t, err)

	message := fmt.Sprintf("%d\n%s\n%s\n", ts.Unix(), nonce, string(body))
	hashed := sha256.Sum256([]byte(message))
	sig, err := rsa.SignPKCS1v15(rand.Reader, platformKey, crypto.SHA256, hashed[:])
	require.NoError(t, err)

	headers := map[string]string{
		"Wechatpay-Timestamp": strconv.FormatInt(ts.Unix(), 10),
		"Wechatpay-Nonce":     string(nonce),
		"Wechatpay-Signature": base64.StdEncoding.EncodeToString(sig),
		"Wechatpay-Serial":    utils.GetCertificateSerialNumber(*platformCert),
	}
	return body, headers
}

// ---------------------------------------------------------------------------
// fail-closed: unconfigured channels
// ---------------------------------------------------------------------------

func TestNotify_StripeRejectsWhenSecretMissing(t *testing.T) {
	notifier := &mockOrderNotifier{}
	h := NewNotifyHandler(notifier, NotifyConfig{}) // no secret configured

	body := stripeEventBody(t, "order-1", 1000, "usd", "paid")
	r := newNotifyRequest("stripe", "/api/v1/payments/notify/stripe", body)
	r.Header.Set("Stripe-Signature", stripeSigHeader(t, "whsec_test", body, time.Now()))
	w := httptest.NewRecorder()

	h.Notify(w, r)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Empty(t, notifier.calls, "an unconfigured channel must never reach the order service")
}

func TestNotify_WeChatRejectsWhenNotConfigured(t *testing.T) {
	notifier := &mockOrderNotifier{}
	h := NewNotifyHandler(notifier, NotifyConfig{}) // no APIv3 key / platform cert

	r := newNotifyRequest("wechat", "/api/v1/payments/notify/wechat", []byte(`{}`))
	w := httptest.NewRecorder()

	h.Notify(w, r)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Empty(t, notifier.calls)
}

func TestNotify_AlipayRejectsWhenNotConfigured(t *testing.T) {
	notifier := &mockOrderNotifier{}
	h := NewNotifyHandler(notifier, NotifyConfig{}) // no public key

	r := newNotifyRequest("alipay", "/api/v1/payments/notify/alipay", []byte("out_trade_no=order-1&trade_status=TRADE_SUCCESS"))
	w := httptest.NewRecorder()

	h.Notify(w, r)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Empty(t, notifier.calls)
}

// ---------------------------------------------------------------------------
// Stripe: official verification + verified-payload fields
// ---------------------------------------------------------------------------

func TestNotify_StripeAcceptsValidSignature(t *testing.T) {
	notifier := &mockOrderNotifier{}
	h := NewNotifyHandler(notifier, NotifyConfig{StripeWebhookSecret: "whsec_test"})

	body := stripeEventBody(t, "order-1", 1000, "usd", "paid")
	r := newNotifyRequest("stripe", "/api/v1/payments/notify/stripe", body)
	r.Header.Set("Stripe-Signature", stripeSigHeader(t, "whsec_test", body, time.Now()))
	w := httptest.NewRecorder()

	h.Notify(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, notifier.calls, 1)
	assert.Equal(t, "order-1", notifier.calls[0].TradeNo)
	assert.Equal(t, "stripe", notifier.calls[0].ProviderType)
	assert.Equal(t, "PAID", notifier.calls[0].TradeStatus)
	assert.InDelta(t, 10.0, notifier.calls[0].Amount, 0.001)
	assert.Equal(t, "usd", notifier.calls[0].Currency)
}

func TestNotify_StripeRejectsTamperedBody(t *testing.T) {
	notifier := &mockOrderNotifier{}
	h := NewNotifyHandler(notifier, NotifyConfig{StripeWebhookSecret: "whsec_test"})

	signed := stripeEventBody(t, "order-1", 1000, "usd", "paid")
	tampered := stripeEventBody(t, "order-1", 999900, "usd", "paid")

	r := newNotifyRequest("stripe", "/api/v1/payments/notify/stripe", tampered)
	r.Header.Set("Stripe-Signature", stripeSigHeader(t, "whsec_test", signed, time.Now()))
	w := httptest.NewRecorder()

	h.Notify(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Empty(t, notifier.calls, "a tampered body must not reach the order service")
}

func TestNotify_StripeRejectsExpiredTimestamp(t *testing.T) {
	notifier := &mockOrderNotifier{}
	h := NewNotifyHandler(notifier, NotifyConfig{StripeWebhookSecret: "whsec_test"})

	body := stripeEventBody(t, "order-1", 1000, "usd", "paid")
	stale := time.Now().Add(-time.Hour) // far beyond the 5-minute tolerance
	r := newNotifyRequest("stripe", "/api/v1/payments/notify/stripe", body)
	r.Header.Set("Stripe-Signature", stripeSigHeader(t, "whsec_test", body, stale))
	w := httptest.NewRecorder()

	h.Notify(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Empty(t, notifier.calls, "a replayed stale event must not reach the order service")
}

func TestNotify_IgnoresQueryStringInjection(t *testing.T) {
	notifier := &mockOrderNotifier{}
	h := NewNotifyHandler(notifier, NotifyConfig{StripeWebhookSecret: "whsec_test"})

	body := stripeEventBody(t, "order-real", 1000, "usd", "paid")
	target := "/api/v1/payments/notify/stripe?out_trade_no=order-forged&trade_status=CANCELED&amount_total=1"
	r := newNotifyRequest("stripe", target, body)
	r.Header.Set("Stripe-Signature", stripeSigHeader(t, "whsec_test", body, time.Now()))
	w := httptest.NewRecorder()

	h.Notify(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, notifier.calls, 1)
	assert.Equal(t, "order-real", notifier.calls[0].TradeNo, "business fields must come from the verified payload, not the query string")
	assert.Equal(t, "PAID", notifier.calls[0].TradeStatus)
}

// ---------------------------------------------------------------------------
// Alipay: RSA2 over the raw form body
// ---------------------------------------------------------------------------

func TestNotify_AlipayAcceptsValidSignature(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	require.NoError(t, err)
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))

	notifier := &mockOrderNotifier{}
	h := NewNotifyHandler(notifier, NotifyConfig{AlipayPublicKey: pubPEM})

	body := alipaySignedBody(t, priv, map[string]string{
		"out_trade_no": "order-1",
		"trade_status": "TRADE_SUCCESS",
		"total_amount": "10.00",
		"notify_id":    "evt_alipay",
	})
	r := newNotifyRequest("alipay", "/api/v1/payments/notify/alipay?out_trade_no=order-forged", body)
	w := httptest.NewRecorder()

	h.Notify(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, notifier.calls, 1)
	assert.Equal(t, "order-1", notifier.calls[0].TradeNo)
	assert.Equal(t, "alipay", notifier.calls[0].ProviderType)
	assert.Equal(t, "TRADE_SUCCESS", notifier.calls[0].TradeStatus)
	assert.InDelta(t, 10.0, notifier.calls[0].Amount, 0.001)
}

func TestNotify_AlipayRejectsTamperedBody(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	require.NoError(t, err)
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))

	notifier := &mockOrderNotifier{}
	h := NewNotifyHandler(notifier, NotifyConfig{AlipayPublicKey: pubPEM})

	signed := alipaySignedBody(t, priv, map[string]string{
		"out_trade_no": "order-1",
		"trade_status": "TRADE_SUCCESS",
		"total_amount": "10.00",
	})
	// Flip the amount after signing.
	tampered := []byte(strings.Replace(string(signed), "total_amount=10.00", "total_amount=1.00", 1))

	r := newNotifyRequest("alipay", "/api/v1/payments/notify/alipay", tampered)
	w := httptest.NewRecorder()

	h.Notify(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Empty(t, notifier.calls)
}

// TestNotify_AlipaySignDataOfficialVector pins the signing source against the
// documented contract (OPEN-API-75 P8, https://opendocs.alipay.com/common/02mse3):
// decoded parameter values are concatenated verbatim (no URL encoding), sorted
// by byte order, joined with "&". Values containing spaces or CJK characters —
// present in every real notification (gmt_payment, subject) — must verify.
func TestNotify_AlipaySignDataOfficialVector(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	require.NoError(t, err)
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))

	const wantSignData = "gmt_payment=2026-11-14 10:00:00&notify_id=evt_alipay&out_trade_no=order-1&subject=云量会员&total_amount=88.88&trade_status=TRADE_SUCCESS"
	hashed := sha256.Sum256([]byte(wantSignData))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, hashed[:])
	require.NoError(t, err)

	form := url.Values{}
	form.Set("gmt_payment", "2026-11-14 10:00:00")
	form.Set("notify_id", "evt_alipay")
	form.Set("out_trade_no", "order-1")
	form.Set("subject", "云量会员")
	form.Set("total_amount", "88.88")
	form.Set("trade_status", "TRADE_SUCCESS")
	form.Set("sign", base64.StdEncoding.EncodeToString(sig))
	form.Set("sign_type", "RSA2")

	assert.Equal(t, wantSignData, buildAlipaySignData(form))

	notifier := &mockOrderNotifier{}
	h := NewNotifyHandler(notifier, NotifyConfig{AlipayPublicKey: pubPEM})
	r := newNotifyRequest("alipay", "/api/v1/payments/notify/alipay", []byte(form.Encode()))
	w := httptest.NewRecorder()

	h.Notify(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, notifier.calls, 1)
	assert.Equal(t, "order-1", notifier.calls[0].TradeNo)
}

// ---------------------------------------------------------------------------
// WeChat Pay V3: platform-certificate RSA verification + official decryption
// ---------------------------------------------------------------------------

func wechatConfig(t *testing.T) (NotifyConfig, *x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	cert, key, bundle := newSelfSignedCert(t)
	return NotifyConfig{
		WeChatAPIV3Key:        "0123456789abcdef0123456789abcdef",
		WeChatPlatformCertPEM: bundle,
	}, cert, key
}

func TestNotify_WeChatAcceptsValidNotification(t *testing.T) {
	cfg, cert, platformKey := wechatConfig(t)
	notifier := &mockOrderNotifier{}
	h := NewNotifyHandler(notifier, cfg)

	body, headers := wechatSignedBody(t, cfg.WeChatAPIV3Key, cert, platformKey, map[string]any{
		"out_trade_no":   "order-1",
		"trade_state":    "SUCCESS",
		"transaction_id": "wx-1",
		"amount":         map[string]any{"total": 1000, "currency": "CNY"},
	}, time.Now())

	r := newNotifyRequest("wechat", "/api/v1/payments/notify/wechat", body)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()

	h.Notify(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, notifier.calls, 1)
	assert.Equal(t, "order-1", notifier.calls[0].TradeNo)
	assert.Equal(t, "wechat", notifier.calls[0].ProviderType)
	assert.Equal(t, "SUCCESS", notifier.calls[0].TradeStatus)
	assert.InDelta(t, 10.0, notifier.calls[0].Amount, 0.001, "fen must be converted to major units")
	assert.Equal(t, "CNY", notifier.calls[0].Currency)
}

func TestNotify_WeChatRejectsExpiredTimestamp(t *testing.T) {
	cfg, cert, platformKey := wechatConfig(t)
	notifier := &mockOrderNotifier{}
	h := NewNotifyHandler(notifier, cfg)

	body, headers := wechatSignedBody(t, cfg.WeChatAPIV3Key, cert, platformKey, map[string]any{
		"out_trade_no": "order-1",
		"trade_state":  "SUCCESS",
		"amount":       map[string]any{"total": 1000, "currency": "CNY"},
	}, time.Now().Add(-time.Hour)) // beyond the 5-minute window

	r := newNotifyRequest("wechat", "/api/v1/payments/notify/wechat", body)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()

	h.Notify(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Empty(t, notifier.calls)
}

func TestNotify_WeChatRejectsTamperedBody(t *testing.T) {
	cfg, cert, platformKey := wechatConfig(t)
	notifier := &mockOrderNotifier{}
	h := NewNotifyHandler(notifier, cfg)

	_, headers := wechatSignedBody(t, cfg.WeChatAPIV3Key, cert, platformKey, map[string]any{
		"out_trade_no": "order-1",
		"trade_state":  "SUCCESS",
		"amount":       map[string]any{"total": 1000, "currency": "CNY"},
	}, time.Now())

	// A different body claiming a huge amount, delivered with the signature of
	// the original body.
	tampered, _ := wechatSignedBody(t, cfg.WeChatAPIV3Key, cert, platformKey, map[string]any{
		"out_trade_no": "order-1",
		"trade_state":  "SUCCESS",
		"amount":       map[string]any{"total": 999900, "currency": "CNY"},
	}, time.Now())

	r := newNotifyRequest("wechat", "/api/v1/payments/notify/wechat", tampered)
	for k, v := range headers {
		r.Header.Set(k, v) // signature covers the original body, not the tampered one
	}
	w := httptest.NewRecorder()

	h.Notify(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Empty(t, notifier.calls)
}

// ---------------------------------------------------------------------------
// order binding + idempotency propagation at the handler boundary
// ---------------------------------------------------------------------------

func TestNotify_OrderBindingRejectionReturnsFailure(t *testing.T) {
	// The app layer rejects a callback whose verified amount/currency does not
	// match the local order; the handler must not ack it as processed.
	notifier := &mockOrderNotifier{err: errors.New("payment callback amount 1.00 does not match order amount 10.00")}
	h := NewNotifyHandler(notifier, NotifyConfig{StripeWebhookSecret: "whsec_test"})

	body := stripeEventBody(t, "order-1", 100, "usd", "paid")
	r := newNotifyRequest("stripe", "/api/v1/payments/notify/stripe", body)
	r.Header.Set("Stripe-Signature", stripeSigHeader(t, "whsec_test", body, time.Now()))
	w := httptest.NewRecorder()

	h.Notify(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	require.Len(t, notifier.calls, 1)
}

func TestNotify_RepeatedEventDrivesServiceIdempotently(t *testing.T) {
	// The same signed event delivered twice yields identical verified fields
	// both times; the order service's state machine treats the second delivery
	// as a no-op (covered in internal/app/orders), so repeat deliveries cannot
	// double-apply. Here we pin that the handler derives identical inputs.
	notifier := &mockOrderNotifier{}
	h := NewNotifyHandler(notifier, NotifyConfig{StripeWebhookSecret: "whsec_test"})

	body := stripeEventBody(t, "order-1", 1000, "usd", "paid")
	sig := stripeSigHeader(t, "whsec_test", body, time.Now())

	for i := 0; i < 2; i++ {
		r := newNotifyRequest("stripe", "/api/v1/payments/notify/stripe", body)
		r.Header.Set("Stripe-Signature", sig)
		w := httptest.NewRecorder()
		h.Notify(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
	}
	require.Len(t, notifier.calls, 2)
	assert.Equal(t, notifier.calls[0], notifier.calls[1], "a re-delivered event must produce the exact same verified payload")
}
