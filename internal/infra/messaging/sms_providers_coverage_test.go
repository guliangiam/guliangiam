package messaging

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// roundTripFunc is a custom RoundTripper for testing HTTP clients.
type roundTripFunc func(req *http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func newTestClient(fn roundTripFunc) *http.Client {
	return &http.Client{Transport: fn}
}

func TestInfobipSmsSender_Send_Success(t *testing.T) {
	sender := NewInfobipSmsSender("https://api.infobip.com", "test-api-key", "TestSender")
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		assert.Equal(t, "POST", req.Method)
		assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
		assert.Equal(t, "App test-api-key", req.Header.Get("Authorization"))
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}
	})

	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.NoError(t, err)
}

func TestInfobipSmsSender_Send_RateLimited(t *testing.T) {
	sender := NewInfobipSmsSender("https://api.infobip.com", "test-api-key", "TestSender")
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusTooManyRequests, Body: http.NoBody}
	})

	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limited")
}

func TestInfobipSmsSender_Send_APIError(t *testing.T) {
	sender := NewInfobipSmsSender("https://api.infobip.com", "test-api-key", "TestSender")
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusInternalServerError, Body: http.NoBody}
	})

	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API error")
}

func TestEmaySmsSender_Send_Success(t *testing.T) {
	sender := NewEmaySmsSender("https://api.emay.com/sms", "serial123", "password", "TestSign")
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		assert.Equal(t, "POST", req.Method)
		assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}
	})

	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.NoError(t, err)
}

func TestEmaySmsSender_Send_APIError(t *testing.T) {
	sender := NewEmaySmsSender("https://api.emay.com/sms", "serial123", "password", "TestSign")
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusInternalServerError, Body: http.NoBody}
	})

	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API error")
}

func TestOsonSmsSender_Send_Success(t *testing.T) {
	sender := NewOsonSmsSender("https://api.oson.com/sms", "test-api-key", "TestSender")
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		assert.Equal(t, "POST", req.Method)
		assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-api-key", req.Header.Get("Authorization"))
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}
	})

	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.NoError(t, err)
}

func TestOsonSmsSender_Send_APIError(t *testing.T) {
	sender := NewOsonSmsSender("https://api.oson.com/sms", "test-api-key", "TestSender")
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusBadRequest, Body: http.NoBody}
	})

	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API error")
}

func TestBaiduSmsSender_Send_Success(t *testing.T) {
	sender := NewBaiduSmsSender("ak", "sk", "sign1", "tmpl1", "invoke1")
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		assert.Equal(t, "POST", req.Method)
		assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"code":1000,"message":"success"}`))}
	})

	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.NoError(t, err)
}

func TestBaiduSmsSender_Send_APIError(t *testing.T) {
	sender := NewBaiduSmsSender("ak", "sk", "sign1", "tmpl1", "invoke1")
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader(`{"code":1001,"message":"internal error"}`))}
	})

	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 500", "HTTP failures surface with the status code before business decoding")
}

func TestSubmailSmsSender_Send_Success(t *testing.T) {
	sender := NewSubmailSmsSender("app1", "key1", "sign1", "proj1")
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		assert.Equal(t, "POST", req.Method)
		assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"status":"success","send_id":"abc123","fee":1}`))}
	})

	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.NoError(t, err)
}

func TestSubmailSmsSender_Send_APIError(t *testing.T) {
	sender := NewSubmailSmsSender("app1", "key1", "sign1", "proj1")
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"status":"error","code":"15555","msg":"wrong sms"}`))}
	})

	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API error")
}

func TestMailtrapSmsSender_Send_Success(t *testing.T) {
	sender := NewMailtrapSmsSender("test-api-key", "+15551234567")
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		assert.Equal(t, "POST", req.Method)
		assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-api-key", req.Header.Get("Authorization"))
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}
	})

	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.NoError(t, err)
}

func TestMailtrapSmsSender_Send_APIError(t *testing.T) {
	sender := NewMailtrapSmsSender("test-api-key", "+15551234567")
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusInternalServerError, Body: http.NoBody}
	})

	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API error")
}

func TestCustomHttpSmsSender_Send_Success(t *testing.T) {
	sender := NewCustomHttpSmsSender(
		"https://custom.api/sms",
		"POST",
		map[string]string{"X-Custom-Auth": "Bearer custom-token"},
		"",
	)
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		assert.Equal(t, "POST", req.Method)
		assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer custom-token", req.Header.Get("X-Custom-Auth"))
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}
	})

	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.NoError(t, err)
}

func TestCustomHttpSmsSender_Send_DefaultMethod(t *testing.T) {
	sender := NewCustomHttpSmsSender("https://custom.api/sms", "", nil, "")
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		assert.Equal(t, "POST", req.Method) // defaults to POST
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}
	})

	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.NoError(t, err)
}

func TestCustomHttpSmsSender_Send_APIError(t *testing.T) {
	sender := NewCustomHttpSmsSender("https://custom.api/sms", "POST", nil, "")
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusInternalServerError, Body: http.NoBody}
	})

	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API error")
}

func TestAmazonSNSSender_Send_WithSenderID(t *testing.T) {
	// Test the code path where SenderID is set (message attributes are added)
	sender := &AmazonSNSSender{
		AccessKeyID: "test-key",
		SecretKey:   "test-secret",
		Region:      "us-east-1",
		SenderID:    "TestSender",
		client:      nil, // Will return error for nil client
	}
	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client not initialized")
}

func TestUCloudSmsSender_Send_APIError(t *testing.T) {
	sender := NewUCloudSmsSender("pub", "priv", "proj", "sign", "tmpl")
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"RetCode":4001,"Message":"invalid public key"}`))}
	})

	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API error")
}
