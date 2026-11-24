package payments

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

// mockOrderNotifier records the calls the notify handler makes to the order
// service so tests can assert that only verified payload fields drive it.
type mockOrderNotifier struct {
	calls []mockOrderNotifyCall
	err   error
}

type mockOrderNotifyCall struct {
	TradeNo      string
	ProviderType string
	TradeStatus  string
	Amount       float64
	Currency     string
}

func (m *mockOrderNotifier) NotifyOrderPayment(_ context.Context, tradeNo, providerType, tradeStatus string, amount float64, currency string) error {
	m.calls = append(m.calls, mockOrderNotifyCall{
		TradeNo: tradeNo, ProviderType: providerType, TradeStatus: tradeStatus,
		Amount: amount, Currency: currency,
	})
	return m.err
}

func TestWriteNotifySuccess_Alipay(t *testing.T) {
	w := httptest.NewRecorder()
	writeNotifySuccess(w, "alipay")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "success", w.Body.String())
}

func TestWriteNotifySuccess_WeChat(t *testing.T) {
	w := httptest.NewRecorder()
	writeNotifySuccess(w, "wechat")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "success", w.Body.String())
}

func TestWriteNotifySuccess_Stripe(t *testing.T) {
	w := httptest.NewRecorder()
	writeNotifySuccess(w, "stripe")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWriteNotifyFailure_Alipay(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/payments/notify/alipay", nil)
	writeNotifyFailure(w, r, "alipay")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "fail", w.Body.String())
}

func TestWriteNotifyFailure_WeChat(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/payments/notify/wechat", nil)
	writeNotifyFailure(w, r, "wechat")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "fail", w.Body.String())
}

func TestWriteNotifyFailure_Stripe(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/payments/notify/stripe", nil)
	writeNotifyFailure(w, r, "stripe")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestWriteNotifyUnauthorized_Stripe(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/payments/notify/stripe", nil)
	writeNotifyUnauthorized(w, r, "stripe")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestWriteNotifyUnavailable_Stripe(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/payments/notify/stripe", nil)
	writeNotifyUnavailable(w, r, "stripe")
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestVerifyNotify_UnknownProvider(t *testing.T) {
	h := NotifyHandler{}
	r := httptest.NewRequest("POST", "/payments/notify/unknown", nil)
	_, err := h.verifyNotify("unknown", r, nil)
	assert.Error(t, err)
}

func TestVerifyNotify_WeChatPayAliasRoutesToWeChat(t *testing.T) {
	// The channel registry calls it "wechat_pay"; an unconfigured handler must
	// reach the WeChat branch (not-configured) instead of answering "unsupported".
	for _, alias := range []string{"wechat_pay", "wxpay", "wechat"} {
		h := NotifyHandler{}
		r := httptest.NewRequest("POST", "/payments/notify/"+alias, nil)
		_, err := h.verifyNotify(alias, r, nil)
		assert.True(t, errors.Is(err, errNotifyNotConfigured), "alias %s", alias)
	}
}

func TestCanonicalNotifyChannel_MapsLegacyAliases(t *testing.T) {
	for _, alias := range []string{"wechat", "wxpay"} {
		if got := canonicalNotifyChannel(alias); got != "wechat_pay" {
			t.Errorf("canonicalNotifyChannel(%q) = %q, want wechat_pay", alias, got)
		}
	}
	if got := canonicalNotifyChannel("alipay"); got != "alipay" {
		t.Errorf("canonicalNotifyChannel(alipay) = %q", got)
	}
}

func TestProtocolNotifyChannel_MapsRegistryKey(t *testing.T) {
	for _, alias := range []string{"wechat_pay", "wxpay"} {
		if got := protocolNotifyChannel(alias); got != "wechat" {
			t.Errorf("protocolNotifyChannel(%q) = %q, want wechat", alias, got)
		}
	}
}

func TestVerifyAlipayNotify_NoPublicKey(t *testing.T) {
	// Fail-closed: no public key means the callback is rejected, never accepted.
	h := NotifyHandler{AlipayPublicKey: ""}
	_, err := h.verifyAlipayNotify([]byte("out_trade_no=123&trade_status=TRADE_SUCCESS"))
	assert.True(t, errors.Is(err, errNotifyNotConfigured))
}

func TestVerifyAlipayNotify_MissingSign(t *testing.T) {
	h := NotifyHandler{AlipayPublicKey: "some-key"}
	_, err := h.verifyAlipayNotify([]byte("trade_no=123"))
	assert.Error(t, err)
}

func TestVerifyWeChatNotify_NotConfigured(t *testing.T) {
	// Fail-closed: empty APIv3 key or empty platform certificate set means the
	// callback is rejected, never accepted unverified.
	r := httptest.NewRequest("POST", "/payments/notify/wechat", strings.NewReader("{}"))

	h := NotifyHandler{WeChatAPIV3Key: ""}
	_, err := h.verifyWeChatNotify(r, []byte("{}"))
	assert.True(t, errors.Is(err, errNotifyNotConfigured))

	h = NotifyHandler{WeChatAPIV3Key: "0123456789abcdef0123456789abcdef"}
	_, err = h.verifyWeChatNotify(r, []byte("{}"))
	assert.True(t, errors.Is(err, errNotifyNotConfigured))
}

func TestVerifyWeChatNotify_MissingHeaders(t *testing.T) {
	cert, _, _ := newSelfSignedCert(t)
	h := NotifyHandler{
		WeChatAPIV3Key:      "0123456789abcdef0123456789abcdef",
		WeChatPlatformCerts: []*x509.Certificate{cert},
	}
	r := httptest.NewRequest("POST", "/payments/notify/wechat", strings.NewReader("{}"))
	_, err := h.verifyWeChatNotify(r, []byte("{}"))
	assert.Error(t, err)
}

func TestVerifyStripeNotify_NoSecret(t *testing.T) {
	// Fail-closed: no webhook secret means the callback is rejected, never accepted.
	h := NotifyHandler{StripeWebhookSecret: ""}
	r := httptest.NewRequest("POST", "/payments/notify/stripe", nil)
	_, err := h.verifyStripeNotify(r, []byte("{}"))
	assert.True(t, errors.Is(err, errNotifyNotConfigured))
}

func TestVerifyStripeNotify_MissingHeader(t *testing.T) {
	h := NotifyHandler{StripeWebhookSecret: "test-secret"}
	r := httptest.NewRequest("POST", "/payments/notify/stripe", strings.NewReader("{}"))
	_, err := h.verifyStripeNotify(r, []byte("{}"))
	assert.Error(t, err)
}

func TestVerifyStripeNotify_InvalidSignatureHeader(t *testing.T) {
	h := NotifyHandler{StripeWebhookSecret: "test-secret"}
	r := httptest.NewRequest("POST", "/payments/notify/stripe", strings.NewReader("{}"))
	r.Header.Set("Stripe-Signature", "invalid-header")
	_, err := h.verifyStripeNotify(r, []byte("{}"))
	assert.Error(t, err)
}

func TestVerifyRSA2_EmptyPublicKey(t *testing.T) {
	verified, err := verifyRSA2("data", "sign", "")
	assert.Error(t, err)
	assert.False(t, verified)
}

func TestVerifyRSA2_InvalidBase64Sign(t *testing.T) {
	verified, err := verifyRSA2("data", "not-base64!!!", "some-key")
	assert.Error(t, err)
	assert.False(t, verified)
}

func TestParseAlipayPublicKey_InvalidPEM(t *testing.T) {
	_, err := parseAlipayPublicKey("not-a-valid-key")
	assert.Error(t, err)
}

func TestParseAlipayPublicKey_ValidPEM(t *testing.T) {
	// Generate a test RSA key
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	pubKeyDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	assert.NoError(t, err)

	pubKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyDER,
	})

	parsed, err := parseAlipayPublicKey(string(pubKeyPEM))
	assert.NoError(t, err)
	assert.NotNil(t, parsed)
}

func TestParseAlipayPublicKey_Base64DER(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	derBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	assert.NoError(t, err)
	base64Key := base64.StdEncoding.EncodeToString(derBytes)

	parsed, err := parseAlipayPublicKey(base64Key)
	assert.NoError(t, err)
	assert.NotNil(t, parsed)
}

func TestParsePlatformCertificates_RejectsEmptyBundle(t *testing.T) {
	_, err := parsePlatformCertificates("not-a-pem-bundle")
	assert.Error(t, err)
}

func TestNewNotifyHandler(t *testing.T) {
	h := NewNotifyHandler(&mockOrderNotifier{}, NotifyConfig{})
	assert.NotNil(t, h)
}

func TestNewNotifyHandler_ParsesWeChatCerts(t *testing.T) {
	_, _, pemBundle := newSelfSignedCert(t)
	h := NewNotifyHandler(&mockOrderNotifier{}, NotifyConfig{
		WeChatAPIV3Key:        "0123456789abcdef0123456789abcdef",
		WeChatPlatformCertPEM: pemBundle,
	})
	assert.Len(t, h.WeChatPlatformCerts, 1)
}

func TestNotify_MissingProviderType(t *testing.T) {
	h := NewNotifyHandler(&mockOrderNotifier{}, NotifyConfig{})
	r := httptest.NewRequest("POST", "/payments/notify/", nil)
	w := httptest.NewRecorder()

	// Set up chi route context with empty providerType
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("providerType", "")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	h.Notify(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBuildAlipaySignData_SortsAndSkipsSignFields(t *testing.T) {
	values := url.Values{
		"out_trade_no": {"order-1"},
		"trade_status": {"TRADE_SUCCESS"},
		"sign":         {"sig"},
		"sign_type":    {"RSA2"},
		"empty":        {""},
	}
	got := buildAlipaySignData(values)
	assert.Equal(t, "out_trade_no=order-1&trade_status=TRADE_SUCCESS", got)
}

func TestStripeTradeStatus(t *testing.T) {
	assert.Equal(t, "PAID", stripeTradeStatus("paid", ""))
	assert.Equal(t, "PAID", stripeTradeStatus("", "succeeded"))
	assert.Equal(t, "PENDING", stripeTradeStatus("unpaid", ""))
	assert.Equal(t, "CANCELED", stripeTradeStatus("", "expired"))
	assert.Equal(t, "", stripeTradeStatus("nonsense", ""))
}
