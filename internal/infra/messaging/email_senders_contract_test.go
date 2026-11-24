package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// failingRoundTrip 模拟提供商不可达（传输层失败）。
type failingRoundTrip struct{ err error }

func (f failingRoundTrip) RoundTrip(*nethttp.Request) (*nethttp.Response, error) {
	return nil, f.err
}

func requireAppErrorCode(t *testing.T, err error, want apperrors.Code) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %T: %v", err, err)
	assert.Equal(t, want, appErr.Code)
}

func TestSendGridSender_Contract(t *testing.T) {
	sender := NewSendGridSender("SG.fake-key", "from@example.com")
	var captured *nethttp.Request
	var payload map[string]any
	sender.client = newTestClient(func(req *nethttp.Request) *nethttp.Response {
		captured = req
		raw, _ := io.ReadAll(req.Body)
		require.NoError(t, json.Unmarshal(raw, &payload))
		return responseWithBody(nethttp.StatusAccepted, "")
	})

	require.NoError(t, sender.Send(context.Background(), "to@example.com", "Hello", "code-123"))

	assert.Equal(t, nethttp.MethodPost, captured.Method)
	assert.Equal(t, "https://api.sendgrid.com/v3/mail/send", captured.URL.String())
	assert.Equal(t, "Bearer SG.fake-key", captured.Header.Get("Authorization"))
	assert.Equal(t, "application/json", captured.Header.Get("Content-Type"))

	// 官方 v3 Mail Send 请求体：personalizations[].to[].email / from.email / subject / content[].{type,value}
	personalizations, ok := payload["personalizations"].([]any)
	require.True(t, ok)
	require.Len(t, personalizations, 1)
	first := personalizations[0].(map[string]any)
	to := first["to"].([]any)[0].(map[string]any)
	assert.Equal(t, "to@example.com", to["email"])
	assert.Equal(t, "from@example.com", payload["from"].(map[string]any)["email"])
	assert.Equal(t, "Hello", payload["subject"])
	content := payload["content"].([]any)[0].(map[string]any)
	assert.Equal(t, "text/plain", content["type"])
	assert.Equal(t, "code-123", content["value"])
}

func TestSendGridSender_TransportErrorIsUnavailable(t *testing.T) {
	sender := NewSendGridSender("SG.fake-key", "from@example.com")
	sender.client = &nethttp.Client{Transport: failingRoundTrip{err: errors.New("dial refused")}}

	err := sender.Send(context.Background(), "to@example.com", "Hello", "code-123")
	requireAppErrorCode(t, err, apperrors.CodeUnavailable)
}

func TestSendGridSender_HTTPError(t *testing.T) {
	sender := NewSendGridSender("SG.fake-key", "from@example.com")
	sender.client = newTestClient(func(req *nethttp.Request) *nethttp.Response {
		return responseWithBody(nethttp.StatusBadRequest, `{"errors":[{"message":"invalid api key"}]}`)
	})

	err := sender.Send(context.Background(), "to@example.com", "Hello", "code-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 400")
}

func TestResendSender_Contract(t *testing.T) {
	sender := NewResendSender("re_fake_key", "Acme <onboarding@resend.dev>")
	var captured *nethttp.Request
	var payload map[string]any
	sender.client = newTestClient(func(req *nethttp.Request) *nethttp.Response {
		captured = req
		raw, _ := io.ReadAll(req.Body)
		require.NoError(t, json.Unmarshal(raw, &payload))
		return responseWithBody(nethttp.StatusOK, `{"id":"rm-123"}`)
	})

	require.NoError(t, sender.Send(context.Background(), "to@example.com", "Hello", "code-123"))

	assert.Equal(t, nethttp.MethodPost, captured.Method)
	assert.Equal(t, "https://api.resend.com/emails", captured.URL.String())
	assert.Equal(t, "Bearer re_fake_key", captured.Header.Get("Authorization"))
	assert.Equal(t, "application/json", captured.Header.Get("Content-Type"))

	// 官方 Create Email：from(string) / to(string[]) / subject / text
	assert.Equal(t, "Acme <onboarding@resend.dev>", payload["from"])
	assert.Equal(t, []any{"to@example.com"}, payload["to"])
	assert.Equal(t, "Hello", payload["subject"])
	assert.Equal(t, "code-123", payload["text"])
}

func TestResendSender_TransportErrorIsUnavailable(t *testing.T) {
	sender := NewResendSender("re_fake_key", "from@example.com")
	sender.client = &nethttp.Client{Transport: failingRoundTrip{err: errors.New("connection reset")}}

	err := sender.Send(context.Background(), "to@example.com", "Hello", "code-123")
	requireAppErrorCode(t, err, apperrors.CodeUnavailable)
}

func TestAzureACSSender_Contract(t *testing.T) {
	sender := NewAzureACSSender("https://contoso.communication.azure.com", "fake-access-key", "sender@contoso.com")
	var captured *nethttp.Request
	var payload map[string]any
	sender.client = newTestClient(func(req *nethttp.Request) *nethttp.Response {
		captured = req
		raw, _ := io.ReadAll(req.Body)
		require.NoError(t, json.Unmarshal(raw, &payload))
		return responseWithBody(nethttp.StatusAccepted, `{"id":"op-1","status":"Running"}`)
	})

	require.NoError(t, sender.Send(context.Background(), "to@example.com", "Hello", "code-123"))

	assert.Equal(t, nethttp.MethodPost, captured.Method)
	assert.Equal(t, "/emails:send", captured.URL.Path)
	assert.Equal(t, "2023-03-31", captured.URL.Query().Get("api-version"))
	// ACS REST 约定：accesskey 已是 base64，以 Basic 方案放入 Authorization。
	assert.Equal(t, "Basic fake-access-key", captured.Header.Get("Authorization"))
	assert.Equal(t, "application/json", captured.Header.Get("Content-Type"))

	// 官方 EmailSend：senderAddress / recipients.to[].address / content.{subject,plainText}
	assert.Equal(t, "sender@contoso.com", payload["senderAddress"])
	assert.Equal(t, "to@example.com", payload["recipients"].(map[string]any)["to"].([]any)[0].(map[string]any)["address"])
	content := payload["content"].(map[string]any)
	assert.Equal(t, "Hello", content["subject"])
	assert.Equal(t, "code-123", content["plainText"])
}

func TestAzureACSSender_TransportErrorIsUnavailable(t *testing.T) {
	sender := NewAzureACSSender("https://contoso.communication.azure.com", "fake-access-key", "sender@contoso.com")
	sender.client = &nethttp.Client{Transport: failingRoundTrip{err: errors.New("tls handshake")}}

	err := sender.Send(context.Background(), "to@example.com", "Hello", "code-123")
	requireAppErrorCode(t, err, apperrors.CodeUnavailable)
}

func TestSubmailEmailSender_TransportErrorIsUnavailable(t *testing.T) {
	sender := NewSubmailEmailSender("app", "key", "from@example.com")
	sender.client = &nethttp.Client{Transport: failingRoundTrip{err: errors.New("no route to host")}}

	err := sender.Send(context.Background(), "to@example.com", "Subject", "Body")
	requireAppErrorCode(t, err, apperrors.CodeUnavailable)
}

func TestSubmailEmailSender_HTTPStatusError(t *testing.T) {
	sender := NewSubmailEmailSender("app", "key", "from@example.com")
	sender.APIURL = "https://submail.example.com/mail/send.json"
	sender.client = newTestClient(func(req *nethttp.Request) *nethttp.Response {
		return responseWithBody(nethttp.StatusInternalServerError, "")
	})

	err := sender.Send(context.Background(), "to@example.com", "Subject", "Body")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
}

func TestMailtrapEmailSender_HTTPStatusError(t *testing.T) {
	sender := NewMailtrapEmailSender("token", "from@example.com")
	sender.APIURL = "https://mailtrap.example.com/api/send"
	sender.client = newTestClient(func(req *nethttp.Request) *nethttp.Response {
		return responseWithBody(nethttp.StatusForbidden, "")
	})

	err := sender.Send(context.Background(), "to@example.com", "Subject", "Body")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 403")
}

func TestCustomHTTPSender_Contract(t *testing.T) {
	var capturedMethod, capturedPath, capturedAuth, capturedContentType string
	var payload map[string]any
	server := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("X-Auth")
		capturedContentType = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(raw, &payload))
		w.WriteHeader(nethttp.StatusOK)
	}))
	defer server.Close()

	sender := NewCustomHTTPSender(server.URL+"/mail", "", map[string]string{"X-Auth": "token"})
	// httptest 服务在回环地址，需绕过 ssrf.SafeTransport。
	sender.client = &nethttp.Client{}
	require.NoError(t, sender.Send(context.Background(), "to@example.com", "Hello", "code-123"))

	assert.Equal(t, nethttp.MethodPost, capturedMethod)
	assert.Equal(t, "/mail", capturedPath)
	assert.Equal(t, "token", capturedAuth)
	assert.Equal(t, "application/json", capturedContentType)
	assert.Equal(t, "to@example.com", payload["to"])
	assert.Equal(t, "Hello", payload["subject"])
	assert.Equal(t, "code-123", payload["body"])
}

func TestCustomHTTPSender_MethodOverrideAndStatusError(t *testing.T) {
	server := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		w.WriteHeader(nethttp.StatusBadGateway)
	}))
	defer server.Close()

	sender := NewCustomHTTPSender(server.URL, "PUT", nil)
	sender.client = &nethttp.Client{}
	err := sender.Send(context.Background(), "to@example.com", "Hello", "code-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 502")
}

func TestCustomHTTPSender_RejectsNonHTTPScheme(t *testing.T) {
	sender := NewCustomHTTPSender("ftp://example.com/mail", "", nil)
	err := sender.Send(context.Background(), "to@example.com", "Hello", "code-123")
	requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)
}
