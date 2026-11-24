package messaging

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// email_family_batch_test.go — P2 邮件批次契约测试：断言每家新增 sender 的
// 请求形状（method/path/鉴权头/参数）与官方文档一致，以及错误映射。
// 官方文档版本引用见各 sender 文件头部注释。

func TestDirectMailEmailSender_Contract(t *testing.T) {
	sender := NewDirectMailEmailSender("ak-test", "sk-test", "noreply@mail.example.com", "cn-hangzhou", "https://dm.example.com/")
	var captured *nethttp.Request
	var form url.Values
	sender.client = newTestClient(func(req *nethttp.Request) *nethttp.Response {
		captured = req
		raw, _ := io.ReadAll(req.Body)
		form, _ = url.ParseQuery(string(raw))
		return responseWithBody(nethttp.StatusOK, `{"EnvVersion":"2.0","RequestId":"r-1"}`)
	})

	require.NoError(t, sender.Send(context.Background(), "to@example.com", "Hello", "code-123"))

	assert.Equal(t, nethttp.MethodPost, captured.Method)
	assert.Equal(t, "dm.example.com", captured.URL.Host)
	// SingleSendMail（RPC 2015-11-23）公共+业务参数
	assert.Equal(t, "SingleSendMail", form.Get("Action"))
	assert.Equal(t, "2015-11-23", form.Get("Version"))
	assert.Equal(t, "HMAC-SHA1", form.Get("SignatureMethod"))
	assert.Equal(t, "1.0", form.Get("SignatureVersion"))
	assert.Equal(t, "ak-test", form.Get("AccessKeyId"))
	assert.Equal(t, "cn-hangzhou", form.Get("RegionId"))
	assert.Equal(t, "noreply@mail.example.com", form.Get("AccountName"))
	assert.Equal(t, "1", form.Get("AddressType"))
	assert.Equal(t, "false", form.Get("ReplyToAddress"))
	assert.Equal(t, "to@example.com", form.Get("ToAddress"))
	assert.Equal(t, "Hello", form.Get("Subject"))
	assert.Equal(t, "code-123", form.Get("TextBody"))
	assert.NotEmpty(t, form.Get("Signature"))
	assert.NotEmpty(t, form.Get("SignatureNonce"))
	assert.NotEmpty(t, form.Get("Timestamp"))
}

func TestDirectMailEmailSender_APIError(t *testing.T) {
	sender := NewDirectMailEmailSender("ak", "sk", "from@x.com", "", "")
	sender.client = newTestClient(func(req *nethttp.Request) *nethttp.Response {
		return responseWithBody(nethttp.StatusBadRequest, `{"Code":"InvalidAccountName.NotFound","Message":"account not found"}`)
	})
	err := sender.Send(context.Background(), "to@example.com", "S", "B")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "InvalidAccountName.NotFound")
}

func TestDirectMailEmailSender_TransportErrorIsUnavailable(t *testing.T) {
	sender := NewDirectMailEmailSender("ak", "sk", "from@x.com", "", "")
	sender.client = &nethttp.Client{Transport: failingRoundTrip{err: errors.New("dial refused")}}
	requireAppErrorCode(t, sender.Send(context.Background(), "to@example.com", "S", "B"), apperrors.CodeUnavailable)
}

func TestTencentSESEmailSender_Contract(t *testing.T) {
	sender := NewTencentSESEmailSender("id-test", "key-test", "noreply@ses.example.com", "ap-guangzhou", "https://ses.example.com/")
	var captured *nethttp.Request
	var payload map[string]any
	sender.client = newTestClient(func(req *nethttp.Request) *nethttp.Response {
		captured = req
		raw, _ := io.ReadAll(req.Body)
		require.NoError(t, json.Unmarshal(raw, &payload))
		return responseWithBody(nethttp.StatusOK, `{"Response":{"RequestId":"r-1","MessageId":"m-1"}}`)
	})

	require.NoError(t, sender.Send(context.Background(), "to@example.com", "Hello", "code-123"))

	assert.Equal(t, nethttp.MethodPost, captured.Method)
	assert.Equal(t, "ses.example.com", captured.URL.Host)
	// SendEmail（TC3，版本 2020-10-02）
	assert.Equal(t, "SendEmail", captured.Header.Get("X-TC-Action"))
	assert.Equal(t, "2020-10-02", captured.Header.Get("X-TC-Version"))
	assert.Equal(t, "ap-guangzhou", captured.Header.Get("X-TC-Region"))
	assert.NotEmpty(t, captured.Header.Get("X-TC-Timestamp"))
	auth := captured.Header.Get("Authorization")
	assert.True(t, strings.HasPrefix(auth, "TC3-HMAC-SHA256 Credential=id-test/"), "auth header: %s", auth)
	assert.Contains(t, auth, "/ses/tc3_request")
	assert.Contains(t, auth, "SignedHeaders=content-type;host")

	assert.Equal(t, "noreply@ses.example.com", payload["FromEmailAddress"])
	assert.Equal(t, []any{"to@example.com"}, payload["Destination"])
	assert.Equal(t, "Hello", payload["Subject"])
	simple := payload["Simple"].(map[string]any)
	decoded, err := base64.StdEncoding.DecodeString(simple["Text"].(string))
	require.NoError(t, err)
	assert.Equal(t, "code-123", string(decoded))
}

func TestTencentSESEmailSender_APIError(t *testing.T) {
	sender := NewTencentSESEmailSender("id", "key", "from@x.com", "", "")
	sender.client = newTestClient(func(req *nethttp.Request) *nethttp.Response {
		return responseWithBody(nethttp.StatusBadRequest, `{"Response":{"Error":{"Code":"FailedOperation.SendMailError","Message":"bad"}}}`)
	})
	err := sender.Send(context.Background(), "to@example.com", "S", "B")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FailedOperation.SendMailError")
}

func TestTencentSESEmailSender_TransportErrorIsUnavailable(t *testing.T) {
	sender := NewTencentSESEmailSender("id", "key", "from@x.com", "", "")
	sender.client = &nethttp.Client{Transport: failingRoundTrip{err: errors.New("no route")}}
	requireAppErrorCode(t, sender.Send(context.Background(), "to@example.com", "S", "B"), apperrors.CodeUnavailable)
}

func TestSendCloudEmailSender_Contract(t *testing.T) {
	sender := NewSendCloudEmailSender("user-test", "key-test", "noreply@sc.example.com", "", "")
	var captured *nethttp.Request
	var form url.Values
	sender.client = newTestClient(func(req *nethttp.Request) *nethttp.Response {
		captured = req
		raw, _ := io.ReadAll(req.Body)
		form, _ = url.ParseQuery(string(raw))
		return responseWithBody(nethttp.StatusOK, `{"statusCode":200,"message":"success"}`)
	})

	require.NoError(t, sender.Send(context.Background(), "to@example.com", "Hello", "code-123"))

	assert.Equal(t, nethttp.MethodPost, captured.Method)
	assert.Equal(t, "api.sendcloud.net", captured.URL.Host)
	assert.Equal(t, "/apiv2/mail/send", captured.URL.Path)
	assert.Equal(t, "user-test", form.Get("apiUser"))
	assert.Equal(t, "key-test", form.Get("apiKey"))
	assert.Equal(t, "noreply@sc.example.com", form.Get("from"))
	assert.Equal(t, "to@example.com", form.Get("to"))
	assert.Equal(t, "Hello", form.Get("subject"))
	assert.Equal(t, "code-123", form.Get("plain"))
}

func TestSendCloudEmailSender_BusinessError(t *testing.T) {
	sender := NewSendCloudEmailSender("u", "k", "from@x.com", "", "")
	sender.client = newTestClient(func(req *nethttp.Request) *nethttp.Response {
		return responseWithBody(nethttp.StatusOK, `{"statusCode":40863,"message":"email format error"}`)
	})
	err := sender.Send(context.Background(), "to@example.com", "S", "B")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "40863")
}

func TestSendCloudEmailSender_TransportErrorIsUnavailable(t *testing.T) {
	sender := NewSendCloudEmailSender("u", "k", "from@x.com", "", "")
	sender.client = &nethttp.Client{Transport: failingRoundTrip{err: errors.New("dns")}}
	requireAppErrorCode(t, sender.Send(context.Background(), "to@example.com", "S", "B"), apperrors.CodeUnavailable)
}

func TestMailgunEmailSender_Contract(t *testing.T) {
	sender := NewMailgunEmailSender("key-mg", "mg.example.com", "Hello <hi@mg.example.com>", "")
	var captured *nethttp.Request
	var form url.Values
	sender.client = newTestClient(func(req *nethttp.Request) *nethttp.Response {
		captured = req
		raw, _ := io.ReadAll(req.Body)
		form, _ = url.ParseQuery(string(raw))
		return responseWithBody(nethttp.StatusOK, `{"id":"<m-1>","message":"Queued. Thank you."}`)
	})

	require.NoError(t, sender.Send(context.Background(), "to@example.com", "Hello", "code-123"))

	assert.Equal(t, nethttp.MethodPost, captured.Method)
	assert.Equal(t, "api.mailgun.net", captured.URL.Host)
	assert.Equal(t, "/v3/mg.example.com/messages", captured.URL.Path)
	user, pass, ok := captured.BasicAuth()
	require.True(t, ok)
	assert.Equal(t, "api", user)
	assert.Equal(t, "key-mg", pass)
	assert.Equal(t, "Hello <hi@mg.example.com>", form.Get("from"))
	assert.Equal(t, "to@example.com", form.Get("to"))
	assert.Equal(t, "Hello", form.Get("subject"))
	assert.Equal(t, "code-123", form.Get("text"))
}

func TestMailgunEmailSender_DefaultFromAndError(t *testing.T) {
	sender := NewMailgunEmailSender("key", "mg.example.com", "", "")
	sender.client = newTestClient(func(req *nethttp.Request) *nethttp.Response {
		raw, _ := io.ReadAll(req.Body)
		form, _ := url.ParseQuery(string(raw))
		assert.Equal(t, "postmaster@mg.example.com", form.Get("from"))
		return responseWithBody(nethttp.StatusForbidden, `{"message":"Unknown domain"}`)
	})
	err := sender.Send(context.Background(), "to@example.com", "S", "B")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unknown domain")
}

func TestMailgunEmailSender_TransportErrorIsUnavailable(t *testing.T) {
	sender := NewMailgunEmailSender("key", "d.example.com", "from@x.com", "")
	sender.client = &nethttp.Client{Transport: failingRoundTrip{err: errors.New("reset")}}
	requireAppErrorCode(t, sender.Send(context.Background(), "to@example.com", "S", "B"), apperrors.CodeUnavailable)
}

func TestPostmarkEmailSender_Contract(t *testing.T) {
	sender := NewPostmarkEmailSender("pm-token", "from@pm.example.com", "")
	var captured *nethttp.Request
	var payload map[string]any
	sender.client = newTestClient(func(req *nethttp.Request) *nethttp.Response {
		captured = req
		raw, _ := io.ReadAll(req.Body)
		require.NoError(t, json.Unmarshal(raw, &payload))
		return responseWithBody(nethttp.StatusOK, `{"MessageID":"m-1","To":"to@example.com","SubmittedAt":"now"}`)
	})

	require.NoError(t, sender.Send(context.Background(), "to@example.com", "Hello", "code-123"))

	assert.Equal(t, nethttp.MethodPost, captured.Method)
	assert.Equal(t, "api.postmarkapp.com", captured.URL.Host)
	assert.Equal(t, "/email", captured.URL.Path)
	assert.Equal(t, "pm-token", captured.Header.Get("X-Postmark-Server-Token"))
	assert.Equal(t, "from@pm.example.com", payload["From"])
	assert.Equal(t, "to@example.com", payload["To"])
	assert.Equal(t, "Hello", payload["Subject"])
	assert.Equal(t, "code-123", payload["TextBody"])
}

func TestPostmarkEmailSender_ErrorBody(t *testing.T) {
	sender := NewPostmarkEmailSender("pm-token", "from@x.com", "")
	sender.client = newTestClient(func(req *nethttp.Request) *nethttp.Response {
		return responseWithBody(nethttp.StatusUnprocessableEntity, `{"ErrorCode":300,"Message":"Invalid 'From' email address."}`)
	})
	err := sender.Send(context.Background(), "to@example.com", "S", "B")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid 'From' email address.")
}

func TestPostmarkEmailSender_TransportErrorIsUnavailable(t *testing.T) {
	sender := NewPostmarkEmailSender("t", "from@x.com", "")
	sender.client = &nethttp.Client{Transport: failingRoundTrip{err: errors.New("reset")}}
	requireAppErrorCode(t, sender.Send(context.Background(), "to@example.com", "S", "B"), apperrors.CodeUnavailable)
}

func TestMailerSendEmailSender_Contract(t *testing.T) {
	sender := NewMailerSendEmailSender("ms-token", "from@ms.example.com", "", "")
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
	assert.Equal(t, "api.mailersend.com", captured.URL.Host)
	assert.Equal(t, "/v1/email", captured.URL.Path)
	assert.Equal(t, "Bearer ms-token", captured.Header.Get("Authorization"))
	assert.Equal(t, "from@ms.example.com", payload["from"].(map[string]any)["email"])
	assert.Equal(t, "to@example.com", payload["to"].([]any)[0].(map[string]any)["email"])
	assert.Equal(t, "Hello", payload["subject"])
	assert.Equal(t, "code-123", payload["text"])
}

func TestMailerSendEmailSender_TransportErrorIsUnavailable(t *testing.T) {
	sender := NewMailerSendEmailSender("t", "from@x.com", "", "")
	sender.client = &nethttp.Client{Transport: failingRoundTrip{err: errors.New("reset")}}
	requireAppErrorCode(t, sender.Send(context.Background(), "to@example.com", "S", "B"), apperrors.CodeUnavailable)
}

func TestBrevoEmailSender_Contract(t *testing.T) {
	sender := NewBrevoEmailSender("br-key", "from@br.example.com", "Acme")
	var captured *nethttp.Request
	var payload map[string]any
	sender.client = newTestClient(func(req *nethttp.Request) *nethttp.Response {
		captured = req
		raw, _ := io.ReadAll(req.Body)
		require.NoError(t, json.Unmarshal(raw, &payload))
		return responseWithBody(nethttp.StatusOK, `{"messageId":"<m-1>"}`)
	})

	require.NoError(t, sender.Send(context.Background(), "to@example.com", "Hello", "code-123"))

	assert.Equal(t, nethttp.MethodPost, captured.Method)
	assert.Equal(t, "api.brevo.com", captured.URL.Host)
	assert.Equal(t, "/v3/smtp/email", captured.URL.Path)
	assert.Equal(t, "br-key", captured.Header.Get("api-key"))
	senderObj := payload["sender"].(map[string]any)
	assert.Equal(t, "from@br.example.com", senderObj["email"])
	assert.Equal(t, "Acme", senderObj["name"])
	assert.Equal(t, "to@example.com", payload["to"].([]any)[0].(map[string]any)["email"])
	assert.Equal(t, "Hello", payload["subject"])
	assert.Equal(t, "code-123", payload["textContent"])
}

func TestBrevoEmailSender_ErrorBody(t *testing.T) {
	sender := NewBrevoEmailSender("k", "from@x.com", "")
	sender.client = newTestClient(func(req *nethttp.Request) *nethttp.Response {
		return responseWithBody(nethttp.StatusUnauthorized, `{"code":"unauthorized","message":"API key is invalid"}`)
	})
	err := sender.Send(context.Background(), "to@example.com", "S", "B")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key is invalid")
}

func TestBrevoEmailSender_TransportErrorIsUnavailable(t *testing.T) {
	sender := NewBrevoEmailSender("k", "from@x.com", "")
	sender.client = &nethttp.Client{Transport: failingRoundTrip{err: errors.New("reset")}}
	requireAppErrorCode(t, sender.Send(context.Background(), "to@example.com", "S", "B"), apperrors.CodeUnavailable)
}

func TestZohoMailEmailSender_Contract(t *testing.T) {
	sender := NewZohoMailEmailSender("zo-token", "9000000123", "from@zoho.example.com", "")
	var captured *nethttp.Request
	var form url.Values
	sender.client = newTestClient(func(req *nethttp.Request) *nethttp.Response {
		captured = req
		raw, _ := io.ReadAll(req.Body)
		form, _ = url.ParseQuery(string(raw))
		return responseWithBody(nethttp.StatusOK, `{"status":"success","data":{"messageId":"9000000234"}}`)
	})

	require.NoError(t, sender.Send(context.Background(), "to@example.com", "Hello", "code-123"))

	assert.Equal(t, nethttp.MethodPost, captured.Method)
	assert.Equal(t, "mail.zoho.com", captured.URL.Host)
	assert.Equal(t, "/api/accounts/9000000123/messages", captured.URL.Path)
	assert.Equal(t, "Zoho-oauthtoken zo-token", captured.Header.Get("Authorization"))
	assert.Equal(t, "from@zoho.example.com", form.Get("fromAddress"))
	assert.Equal(t, "to@example.com", form.Get("toEmail"))
	assert.Equal(t, "Hello", form.Get("subject"))
	assert.Equal(t, "code-123", form.Get("content"))
}

func TestZohoMailEmailSender_BusinessError(t *testing.T) {
	sender := NewZohoMailEmailSender("t", "1", "from@x.com", "https://mail.zoho.eu")
	var captured *nethttp.Request
	sender.client = newTestClient(func(req *nethttp.Request) *nethttp.Response {
		captured = req
		return responseWithBody(nethttp.StatusOK, `{"status":"error","data":{"errorCode":"100094","message":"invalid"}}`)
	})
	err := sender.Send(context.Background(), "to@example.com", "S", "B")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "100094")
	assert.Equal(t, "mail.zoho.eu", captured.URL.Host)
}

func TestZohoMailEmailSender_TransportErrorIsUnavailable(t *testing.T) {
	sender := NewZohoMailEmailSender("t", "1", "from@x.com", "")
	sender.client = &nethttp.Client{Transport: failingRoundTrip{err: errors.New("reset")}}
	requireAppErrorCode(t, sender.Send(context.Background(), "to@example.com", "S", "B"), apperrors.CodeUnavailable)
}

func TestAmazonSESEmailSender_Contract(t *testing.T) {
	var captured *nethttp.Request
	var payload map[string]any
	server := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		captured = r
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &payload)
		w.WriteHeader(nethttp.StatusOK)
	}))
	defer server.Close()

	sender, err := NewAmazonSESEmailSender(context.Background(), "akid", "ssecret", "cn-north-1", "from@ses.example.com", server.URL)
	require.NoError(t, err)
	require.NoError(t, sender.Send(context.Background(), "to@example.com", "Hello", "code-123"))

	require.NotNil(t, captured)
	assert.Equal(t, nethttp.MethodPost, captured.Method)
	assert.Equal(t, "/v2/email/outbound-emails", captured.URL.Path)
	// SigV4 service=ses
	auth := captured.Header.Get("Authorization")
	assert.Contains(t, auth, "AWS4-HMAC-SHA256")
	assert.Contains(t, auth, "Credential=akid/")
	assert.Contains(t, auth, "/cn-north-1/ses/aws4_request")
	assert.Equal(t, "from@ses.example.com", payload["FromEmailAddress"])
	assert.Equal(t, []any{"to@example.com"}, payload["Destination"].(map[string]any)["ToAddresses"])
	simple := payload["Content"].(map[string]any)["Simple"].(map[string]any)
	assert.Equal(t, "Hello", simple["Subject"].(map[string]any)["Data"])
	assert.Equal(t, "code-123", simple["Body"].(map[string]any)["Text"].(map[string]any)["Data"])
}

func TestAmazonSESEmailSender_RequiresCredentialsAndRegion(t *testing.T) {
	_, err := NewAmazonSESEmailSender(context.Background(), "", "", "us-east-1", "f@x.com", "")
	requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)
	_, err = NewAmazonSESEmailSender(context.Background(), "ak", "sk", "", "f@x.com", "")
	requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)
}
