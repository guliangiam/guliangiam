package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"

	"github.com/stripe/stripe-go/v82"
)

// P8 契约测试（OPEN-API-75）：httptest 钉 Square/Checkout.com/Stripe 的
// 认证头、幂等键、端点与金额换算，以及 webhook HMAC 验签向量。
// 真实商户凭据端到端联调属 L4 边界，不在本批宣称范围。

func readJSONBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	raw, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	return m
}

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

func TestSquareCreatePaymentContract(t *testing.T) {
	var auth, version string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v2/checkout/checkouts", r.URL.Path)
		auth = r.Header.Get("Authorization")
		version = r.Header.Get("Square-Version")
		body = readJSONBody(t, r)
		writeJSON(w, `{"checkout":{"id":"ch_1","page_url":"https://checkout.squareup.com/ch_1"},"order":{"id":"o_1"}}`)
	}))
	defer server.Close()

	g := NewSquareGateway("sq-token", "loc-1", "whkey", "", false)
	g.BaseURL = server.URL

	result, err := g.CreatePayment(context.Background(), "order-1", 88.88, "usd", "会员", "https://api/notify", "https://app/return")
	require.NoError(t, err)
	assert.Equal(t, "https://checkout.squareup.com/ch_1", result.PaymentURL)
	// TradeNo 取 order_id：webhook payment.created 的 data.payment.order_id 同键映射。
	assert.Equal(t, "o_1", result.TradeNo)

	assert.Equal(t, "Bearer sq-token", auth)
	assert.Equal(t, "2024-12-18", version, "Square-Version date header must be pinned")
	assert.Equal(t, "order-1", body["idempotency_key"], "CreateCheckout body idempotency_key")
	order := body["order"].(map[string]any)
	assert.Equal(t, "order-1", order["reference"])
	line := order["line_items"].([]any)[0].(map[string]any)
	money := line["base_price_money"].(map[string]any)
	assert.Equal(t, float64(8888), money["amount"], "88.88 元必须四舍五入为 8888 分")
	assert.Equal(t, "USD", money["currency"])
}

func TestSquareQueryAndRefundContract(t *testing.T) {
	var listBody map[string]any
	var refundAuth string
	var refundIdem string
	var refundBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/orders/o_1":
			writeJSON(w, `{"order":{"id":"o_1","status":"COMPLETED","total_money":{"amount":8888,"currency":"USD"}}}`)
		case "/v2/payments/list":
			listBody = readJSONBody(t, r)
			writeJSON(w, `{"payments":[{"id":"pay_1","order_id":"o_1","status":"COMPLETED","amount_money":{"amount":8888,"currency":"USD"}}]}`)
		case "/v2/refunds":
			refundAuth = r.Header.Get("Authorization")
			refundIdem = r.Header.Get("Square-Idempotency-Key")
			refundBody = readJSONBody(t, r)
			writeJSON(w, `{"refund":{"id":"rf_1","status":"COMPLETED"}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	g := NewSquareGateway("sq-token", "loc-1", "whkey", "", false)
	g.BaseURL = server.URL

	status, err := g.QueryPayment(context.Background(), "o_1")
	require.NoError(t, err)
	assert.Equal(t, statusPaid, status.Status)
	assert.InDelta(t, 88.88, status.Amount, 1e-9)

	refund, err := g.Refund(context.Background(), "o_1", 10.0, "质量问题")
	require.NoError(t, err)
	assert.Equal(t, "rf_1", refund.RefundNo)
	assert.Equal(t, statusSuccess, refund.Status)

	assert.Equal(t, "loc-1", listBody["location_id"])
	assert.Equal(t, "Bearer sq-token", refundAuth)
	assert.Equal(t, "o_1-refund-1000", refundIdem, "CreateRefund uses Square-Idempotency-Key header")
	assert.Equal(t, "pay_1", refundBody["payment_id"], "refund resolves payment_id via ListPayments order match")
}

func TestSquareVerifyNotifyVector(t *testing.T) {
	g := NewSquareGateway("tok", "loc", "webhook-secret-key", "", false)

	body := `{"notification_type":"payment.created","data":{"payment":{"id":"pay_1","order_id":"o_1","status":"COMPLETED","amount_money":{"amount":8888,"currency":"USD"}}}}`
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	mac := hmac.New(sha256.New, []byte("webhook-secret-key"))
	// Square 文档签名串："<x-square-timestamp>.<raw body>"。
	_, _ = mac.Write([]byte(ts + "." + body))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	result, err := g.VerifyNotify(context.Background(), map[string]string{
		"signature": sig, "timestamp": ts, "body": body,
	})
	require.NoError(t, err)
	assert.Equal(t, "o_1", result.OutTradeNo)
	assert.Equal(t, statusPaid, result.Status)
	assert.InDelta(t, 88.88, result.Amount, 1e-9)

	// 篡改金额后验签必须失败（fail-closed）。
	_, _ = mac.Write([]byte(ts + "." + strings.Replace(body, "8888", "1", 1)))
	badSig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	_, err = g.VerifyNotify(context.Background(), map[string]string{
		"signature": badSig, "timestamp": ts, "body": strings.Replace(body, "8888", "1", 1),
	})
	require.Error(t, err)

	// 超出 5 分钟容差（重放防护）。
	old := strconv.FormatInt(time.Now().Add(-10*time.Minute).UnixMilli(), 10)
	_, err = g.VerifyNotify(context.Background(), map[string]string{
		"signature": sig, "timestamp": old, "body": body,
	})
	require.Error(t, err)
}

func TestCheckoutCreateQueryRefundContract(t *testing.T) {
	var created map[string]any
	var auth string
	var idem string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/hosted-payments":
			auth = r.Header.Get("Authorization")
			idem = r.Header.Get("Idempotency-Key")
			created = readJSONBody(t, r)
			writeJSON(w, `{"id":"hp_1","_links":{"redirect":{"href":"https://pay.checkout.com/hp_1"}}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/payments/hp_1":
			writeJSON(w, `{"id":"hp_1","status":"Captured","requested_amount":8888,"currency":"USD","reference":"order-1"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/payments/hp_1/refunds":
			writeJSON(w, `{"action_id":"act_1","status":"Pending"}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	g := NewCheckoutGateway("sk_test_key", "whsec_key", false)
	g.BaseURL = server.URL

	result, err := g.CreatePayment(context.Background(), "order-1", 88.88, "usd", "会员", "https://api/notify", "https://app/return")
	require.NoError(t, err)
	assert.Equal(t, "https://pay.checkout.com/hp_1", result.PaymentURL)
	assert.Equal(t, "hp_1", result.TradeNo)
	assert.Equal(t, "Bearer sk_test_key", auth)
	assert.Equal(t, "order-1", idem, "Checkout.com POST carries Idempotency-Key")
	assert.Equal(t, float64(8888), created["amount"])
	assert.Equal(t, "order-1", created["reference"])
	assert.Equal(t, "USD", created["currency"])

	status, err := g.QueryPayment(context.Background(), "hp_1")
	require.NoError(t, err)
	assert.Equal(t, statusPaid, status.Status)
	assert.InDelta(t, 88.88, status.Amount, 1e-9)

	refund, err := g.Refund(context.Background(), "hp_1", 10.0, "质量问题")
	require.NoError(t, err)
	assert.Equal(t, "act_1", refund.RefundNo)
	assert.Equal(t, statusProcessing, refund.Status)
}

func TestCheckoutVerifyNotifyVector(t *testing.T) {
	g := NewCheckoutGateway("sk", "whsec_shared_secret", false)

	body := `{"reference":"order-1","status":"Captured","amount":8888,"currency":"USD"}`
	mac := hmac.New(sha256.New, []byte("whsec_shared_secret"))
	_, _ = mac.Write([]byte(body))
	sig := hex.EncodeToString(mac.Sum(nil))

	result, err := g.VerifyNotify(context.Background(), map[string]string{"signature": sig, "body": body})
	require.NoError(t, err)
	assert.Equal(t, "order-1", result.OutTradeNo)
	assert.Equal(t, statusPaid, result.Status)
	assert.InDelta(t, 88.88, result.Amount, 1e-9)

	_, err = g.VerifyNotify(context.Background(), map[string]string{
		"signature": sig,
		"body":      strings.Replace(body, "Captured", "Declined", 1),
	})
	require.Error(t, err, "tampered body must fail HMAC verification")
}

func TestStripeCreatePaymentIdempotencyAndRounding(t *testing.T) {
	var auth string
	var idem string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/checkout/sessions", r.URL.Path)
		auth = r.Header.Get("Authorization")
		idem = r.Header.Get("Idempotency-Key")
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		q, err := url.ParseQuery(string(raw))
		require.NoError(t, err)
		body = map[string]any{}
		for k, v := range q {
			if len(v) > 0 {
				body[k] = v[0]
			}
		}
		writeJSON(w, `{"id":"cs_test_1","url":"https://checkout.stripe.com/c/pay/cs_test_1","payment_intent":{"id":"pi_1"}}`)
	}))
	defer server.Close()

	client := stripe.NewClient("sk_test_key", stripe.WithBackends(
		stripe.NewBackendsWithConfig(&stripe.BackendConfig{URL: stripe.String(server.URL)}),
	))
	g := newStripeGatewayWithClient("sk_test_key", client)

	result, err := g.CreatePayment(context.Background(), "order-1", 88.88, "usd", "会员", "https://api/notify", "https://app/return")
	require.NoError(t, err)
	assert.Equal(t, "https://checkout.stripe.com/c/pay/cs_test_1", result.PaymentURL)
	assert.Equal(t, "pi_1", result.TradeNo)

	assert.Equal(t, "Bearer sk_test_key", auth)
	assert.Equal(t, "order-1", idem, "docs.stripe.com/api/idempotent_requests: POST carries Idempotency-Key")
	assert.Equal(t, "8888", body["line_items[0][price_data][unit_amount]"], "88.88 元必须四舍五入为 8888 分")
}

func TestFactoryRoutesP8PaymentKinds(t *testing.T) {
	sq, err := NewGateway("square", map[string]string{
		"accessToken": "tok", "locationId": "loc",
	})
	require.NoError(t, err)
	require.NotNil(t, sq)
	_, ok := sq.(*SquareGateway)
	assert.True(t, ok)

	ck, err := NewGateway("checkout", map[string]string{"apiKey": "sk_test"})
	require.NoError(t, err)
	require.NotNil(t, ck)
	_, ok = ck.(*CheckoutGateway)
	assert.True(t, ok)

	_, err = NewGateway("square", map[string]string{"accessToken": "tok"})
	require.Error(t, err, "missing locationId must fail fast")
}

// P8 审查修复钉测：退款幂等键必须取业务退款单号 outRefundNo（每笔退款唯一），
// 同订单两笔等额部分退款不得共享键；无退款单号的遗留三参调用回退 order+金额分。

func TestStripeRefundIdempotencyKeyUsesOutRefundNo(t *testing.T) {
	var keys []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/refunds", r.URL.Path)
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		writeJSON(w, `{"id":"re_1","status":"succeeded"}`)
	}))
	defer server.Close()

	client := stripe.NewClient("sk_test_key", stripe.WithBackends(
		stripe.NewBackendsWithConfig(&stripe.BackendConfig{URL: stripe.String(server.URL)}),
	))
	g := newStripeGatewayWithClient("sk_test_key", client)

	_, err := g.RefundWithOutRefundNo(context.Background(), "pi_1", 10.0, "first", "rf-uuid-1")
	require.NoError(t, err)
	_, err = g.RefundWithOutRefundNo(context.Background(), "pi_1", 10.0, "second", "rf-uuid-2")
	require.NoError(t, err)
	require.Len(t, keys, 2)
	assert.Equal(t, []string{"rf-uuid-1", "rf-uuid-2"}, keys,
		"same-amount partial refunds must carry distinct per-refund idempotency keys")

	_, err = g.Refund(context.Background(), "pi_1", 10.0, "legacy")
	require.NoError(t, err)
	assert.Equal(t, "pi_1-refund-1000", keys[2], "legacy 3-arg path keeps the synthesized fallback")
}

func TestSquareRefundIdempotencyKeyUsesOutRefundNo(t *testing.T) {
	var keys []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/payments/list":
			writeJSON(w, `{"payments":[{"id":"pay_1","order_id":"o_1","status":"COMPLETED","amount_money":{"amount":8888,"currency":"USD"}}]}`)
		case "/v2/refunds":
			keys = append(keys, r.Header.Get("Square-Idempotency-Key"))
			writeJSON(w, `{"refund":{"id":"rf_1","status":"COMPLETED"}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	g := NewSquareGateway("tok", "loc", "whkey", "", false)
	g.BaseURL = server.URL

	_, err := g.RefundWithOutRefundNo(context.Background(), "o_1", 10.0, "first", "rf-uuid-1")
	require.NoError(t, err)
	_, err = g.RefundWithOutRefundNo(context.Background(), "o_1", 10.0, "second", "rf-uuid-2")
	require.NoError(t, err)
	require.Len(t, keys, 2)
	assert.Equal(t, []string{"rf-uuid-1", "rf-uuid-2"}, keys)
}

func TestCheckoutRefundIdempotencyKeyUsesOutRefundNo(t *testing.T) {
	var keys []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/payments/hp_1/refunds" {
			keys = append(keys, r.Header.Get("Idempotency-Key"))
			writeJSON(w, `{"action_id":"act_1","status":"Pending"}`)
			return
		}
		t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	g := NewCheckoutGateway("sk_test_key", "whsec_key", false)
	g.BaseURL = server.URL

	_, err := g.RefundWithOutRefundNo(context.Background(), "hp_1", 10.0, "first", "rf-uuid-1")
	require.NoError(t, err)
	_, err = g.RefundWithOutRefundNo(context.Background(), "hp_1", 10.0, "second", "rf-uuid-2")
	require.NoError(t, err)
	require.Len(t, keys, 2)
	assert.Equal(t, []string{"rf-uuid-1", "rf-uuid-2"}, keys)
}

// keyedStub implements both the legacy and the keyed refund paths.
type keyedStub struct {
	legacy   int
	keyedNos []string
}

func (s *keyedStub) CreatePayment(context.Context, string, float64, string, string, string, string) (*PaymentURLResult, error) {
	return nil, nil
}
func (s *keyedStub) QueryPayment(context.Context, string) (*PaymentStatusResult, error) {
	return &PaymentStatusResult{Status: statusPaid}, nil
}
func (s *keyedStub) Refund(context.Context, string, float64, string) (*RefundResult, error) {
	s.legacy++
	return &RefundResult{RefundNo: "legacy", Status: statusSuccess}, nil
}
func (s *keyedStub) RefundWithOutRefundNo(_ context.Context, _ string, _ float64, _ string, outRefundNo string) (*RefundResult, error) {
	s.keyedNos = append(s.keyedNos, outRefundNo)
	return &RefundResult{RefundNo: outRefundNo, Status: statusSuccess}, nil
}
func (s *keyedStub) VerifyNotify(context.Context, map[string]string) (*NotifyResult, error) {
	return nil, nil
}

// legacyOnlyStub implements only the legacy PaymentGateway surface — it must
// NOT satisfy KeyedRefunder, so the adapter keeps using the 3-arg path.
type legacyOnlyStub struct{ legacy int }

func (s *legacyOnlyStub) CreatePayment(context.Context, string, float64, string, string, string, string) (*PaymentURLResult, error) {
	return nil, nil
}
func (s *legacyOnlyStub) QueryPayment(context.Context, string) (*PaymentStatusResult, error) {
	return &PaymentStatusResult{Status: statusPaid}, nil
}
func (s *legacyOnlyStub) Refund(context.Context, string, float64, string) (*RefundResult, error) {
	s.legacy++
	return &RefundResult{RefundNo: "legacy", Status: statusSuccess}, nil
}
func (s *legacyOnlyStub) VerifyNotify(context.Context, map[string]string) (*NotifyResult, error) {
	return nil, nil
}

func TestChannelAdapterRefundForwardsOutRefundNo(t *testing.T) {
	stub := &keyedStub{}
	a := NewChannelAdapter("stripe", stub)

	res, chErr := a.Refund(context.Background(), domain.RefundRequest{
		ChannelOrderNo: "pi_1",
		Amount:         10.0,
		Reason:         "质量问题",
		OutRefundNo:    "rf-uuid-9",
	})
	require.Nil(t, chErr)
	assert.Equal(t, "rf-uuid-9", res.ChannelRefundNo)
	assert.Equal(t, 0, stub.legacy, "keyed gateways must not fall back to the legacy path")
	assert.Equal(t, []string{"rf-uuid-9"}, stub.keyedNos)
}

func TestChannelAdapterRefundFallsBackToLegacyForNonKeyed(t *testing.T) {
	stub := &legacyOnlyStub{}
	var inner PaymentGateway = stub
	a := NewChannelAdapter("balance", inner)

	res, chErr := a.Refund(context.Background(), domain.RefundRequest{
		ChannelOrderNo: "BAL-1", Amount: 5.0, OutRefundNo: "rf-uuid-9",
	})
	require.Nil(t, chErr)
	assert.Equal(t, "legacy", res.ChannelRefundNo)
	assert.Equal(t, 1, stub.legacy)
}
