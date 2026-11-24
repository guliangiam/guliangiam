package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testHTTPSSRFClient 返回一个允许 loopback 的 HTTP client。
// 生产代码 NewHTTPDeliverySender 走严格 SSRF 防护（禁 127.0.0.1），
// 这里单独放行供 httptest.NewServer 使用。
func testHTTPSSRFClient(timeout time.Duration) *http.Client {
	return ssrf.SafeHTTPClientWithAllowlist(timeout, []string{"127.0.0.1/32", "::1/128"})
}

func TestNewHTTPDeliverySender_DefaultClient(t *testing.T) {
	sender := NewHTTPDeliverySender(nil, 10*time.Second)
	assert.NotNil(t, sender)
	assert.NotNil(t, sender.client)
	assert.Equal(t, 10*time.Second, sender.timeout)
}

func TestNewHTTPDeliverySender_CustomClient(t *testing.T) {
	client := testHTTPSSRFClient(5 * time.Second)
	sender := NewHTTPDeliverySender(client, 10*time.Second)
	assert.Equal(t, client, sender.client)
}

func TestNewHTTPDeliverySender_DefaultTimeout(t *testing.T) {
	sender := NewHTTPDeliverySender(nil, 0)
	assert.Equal(t, 30*time.Second, sender.timeout)
}

func TestHTTPDeliverySender_Send_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := testHTTPSSRFClient(10 * time.Second)
	sender := NewHTTPDeliverySender(client, 10*time.Second)
	result := sender.Send(context.Background(), server.URL, map[string]string{"Content-Type": "application/json"}, []byte(`{"event":"test"}`), "")

	assert.NoError(t, result.Error)
	assert.Equal(t, http.StatusOK, result.StatusCode)
	assert.Contains(t, result.ResponseBody, "ok")
}

func TestHTTPDeliverySender_Send_WithHeaders(t *testing.T) {
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := testHTTPSSRFClient(10 * time.Second)
	sender := NewHTTPDeliverySender(client, 10*time.Second)
	result := sender.Send(context.Background(), server.URL, map[string]string{
		"X-Custom": "custom-value",
		"X-Auth":   "bearer-token",
	}, []byte(`test`), "")

	require.NoError(t, result.Error)
	assert.Equal(t, "custom-value", receivedHeaders.Get("X-Custom"))
	assert.Equal(t, "bearer-token", receivedHeaders.Get("X-Auth"))
}

func TestHTTPDeliverySender_Send_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := testHTTPSSRFClient(10 * time.Second)
	sender := NewHTTPDeliverySender(client, 10*time.Second)
	result := sender.Send(context.Background(), server.URL, nil, nil, "")

	assert.NoError(t, result.Error)
	assert.Equal(t, http.StatusInternalServerError, result.StatusCode)
}

func TestHTTPDeliverySender_Send_InvalidURL(t *testing.T) {
	// ValidateURL scheme 检查在 http_sender.Send 入口生效
	sender := NewHTTPDeliverySender(nil, 10*time.Second)
	result := sender.Send(context.Background(), "ftp://example.com/webhook", nil, nil, "")
	assert.Error(t, result.Error)
}

func TestHTTPDeliverySender_Send_LoopbackBlocked(t *testing.T) {
	// 生产代码（allowlist=nil）应该拦截 loopback
	sender := NewHTTPDeliverySender(nil, 10*time.Second)
	result := sender.Send(context.Background(), "http://127.0.0.1:8080/webhook", nil, nil, "")
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "ssrf")
}

func TestHTTPDeliverySender_Send_MetadataBlocked(t *testing.T) {
	// AWS/GCP metadata endpoint 必须被拦截
	sender := NewHTTPDeliverySender(nil, 10*time.Second)
	result := sender.Send(context.Background(), "http://169.254.169.254/latest/meta-data/", nil, nil, "")
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "ssrf")
}

func TestHTTPDeliverySender_SignatureContract(t *testing.T) {
	const secret = "webhook-secret"
	payload := []byte(`{"event":"test"}`)
	fixedTime := time.Unix(1_700_000_000, 0)

	var signature, timestamp string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signature = r.Header.Get(SignatureHeader)
		timestamp = r.Header.Get(TimestampHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewHTTPDeliverySender(testHTTPSSRFClient(10*time.Second), 10*time.Second)
	sender.nowFunc = func() time.Time { return fixedTime }
	result := sender.Send(context.Background(), server.URL, nil, payload, secret)
	require.NoError(t, result.Error)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("1700000000." + string(payload)))
	want := "t=1700000000,v1=" + hex.EncodeToString(mac.Sum(nil))
	assert.Equal(t, want, signature)
	assert.Equal(t, "1700000000", timestamp)
}
