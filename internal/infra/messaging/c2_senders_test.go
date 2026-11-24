package messaging

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func responseWithBody(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}
}

func TestSmsBaoSender_Send(t *testing.T) {
	sender := NewSmsBaoSender("user", "key", "Sign", "Code: %s", "1")
	sender.APIURL = "https://smsbao.example.com/sms"
	var request *http.Request
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		request = req
		return responseWithBody(http.StatusOK, "0")
	})

	require.NoError(t, sender.Send(context.Background(), "+8613800000000", "123456"))
	require.NotNil(t, request)
	assert.Equal(t, "GET", request.Method)
	assert.Equal(t, "13800000000", request.URL.Query().Get("m"))
	assert.Contains(t, request.URL.Query().Get("c"), "【Sign】Code: 123456")
}

func TestSmsBaoSender_StatusErrors(t *testing.T) {
	for body, want := range map[string]string{
		"30": "credentials",
		"40": "does not exist",
		"41": "overdue",
		"43": "IP address",
		"50": "forbidden",
		"51": "phone number",
		"99": "status 99",
	} {
		sender := NewSmsBaoSender("user", "key", "Sign", "%s", "")
		sender.APIURL = "https://smsbao.example.com/sms"
		sender.client = newTestClient(func(req *http.Request) *http.Response {
			return responseWithBody(http.StatusOK, body)
		})
		err := sender.Send(context.Background(), "13800000000", "123456")
		require.Error(t, err, "body %s", body)
		assert.Contains(t, err.Error(), want, "body %s", body)
	}
}

func TestSmsBaoSender_RejectsInternationalNumbers(t *testing.T) {
	sender := NewSmsBaoSender("user", "key", "Sign", "%s", "")
	err := sender.Send(context.Background(), "+15551234567", "123456")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-mainland")
}

func TestMsg91Sender_Send(t *testing.T) {
	sender := NewMsg91Sender("auth-key", "SENDER", "template-id")
	sender.APIURL = "https://msg91.example.com/api/v5/flow/"
	var body string
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		assert.Equal(t, "auth-key", req.Header.Get("authkey"))
		assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
		read, _ := io.ReadAll(req.Body)
		body = string(read)
		return responseWithBody(http.StatusOK, `{"type":"success"}`)
	})

	require.NoError(t, sender.Send(context.Background(), "+8613800000000", "123456"))
	assert.Contains(t, body, `"mobiles":"8613800000000"`)
	assert.Contains(t, body, `"code":"123456"`)
}

func TestMsg91Sender_RequiresTemplate(t *testing.T) {
	sender := NewMsg91Sender("auth-key", "SENDER", "")
	err := sender.Send(context.Background(), "13800000000", "123456")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "templateId")
}

func TestTwilioVerifySender_SendAndCheck(t *testing.T) {
	sender := NewTwilioVerifySender("AC123", "token", "VA456")
	sender.APIURL = "https://verify.example.com/v2/Services/"
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		assert.Equal(t, "/v2/Services/VA456/Verifications", req.URL.Path)
		assert.True(t, strings.HasPrefix(req.Header.Get("Authorization"), "Basic "))
		form, _ := url.ParseQuery(readBody(req))
		assert.Equal(t, "sms", form.Get("Channel"))
		return responseWithBody(http.StatusCreated, `{"status":"pending"}`)
	})

	require.NoError(t, sender.Send(context.Background(), "+15551234567", "unused"))

	sender.client = newTestClient(func(req *http.Request) *http.Response {
		assert.Equal(t, "/v2/Services/VA456/VerificationCheck", req.URL.Path)
		return responseWithBody(http.StatusOK, `{"status":"approved"}`)
	})
	approved, err := sender.CheckCode(context.Background(), "+15551234567", "123456")
	require.NoError(t, err)
	assert.True(t, approved)
}

func TestTwilioVerifySender_CheckRejectsWrongCode(t *testing.T) {
	sender := NewTwilioVerifySender("AC123", "token", "VA456")
	sender.APIURL = "https://verify.example.com/v2/Services/"

	sender.client = newTestClient(func(req *http.Request) *http.Response {
		return responseWithBody(http.StatusOK, `{"status":"pending"}`)
	})
	approved, err := sender.CheckCode(context.Background(), "+15551234567", "000000")
	require.NoError(t, err)
	assert.False(t, approved)

	sender.client = newTestClient(func(req *http.Request) *http.Response {
		return responseWithBody(http.StatusNotFound, `{"code":20404,"message":"not found"}`)
	})
	approved, err = sender.CheckCode(context.Background(), "+15551234567", "000000")
	require.NoError(t, err)
	assert.False(t, approved)

	sender.client = newTestClient(func(req *http.Request) *http.Response {
		return responseWithBody(http.StatusInternalServerError, `{"code":500,"message":"boom"}`)
	})
	_, err = sender.CheckCode(context.Background(), "+15551234567", "000000")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestAlibabaPNVSSender_SendSmsVerifyCode(t *testing.T) {
	sender := NewAlibabaPNVSSenderWithTemplate("access", "secret", "Sign", "SMS_123")
	sender.endpoint = "dypnsapi.example.com"
	var query url.Values
	sender.HTTPClient = newTestClient(func(req *http.Request) *http.Response {
		query = req.URL.Query()
		return responseWithBody(http.StatusOK, `{"Code":"OK"}`)
	})

	require.NoError(t, sender.Send(context.Background(), "+8613800000000", "123456"))
	assert.Equal(t, "SendSmsVerifyCode", query.Get("Action"))
	assert.Equal(t, "13800000000", query.Get("PhoneNumber"))
	assert.Equal(t, "SMS_123", query.Get("TemplateCode"))
	assert.Contains(t, query.Get("TemplateParam"), "123456")
	assert.NotEmpty(t, query.Get("Signature"))
}

func TestSubmailEmailSender_Send(t *testing.T) {
	sender := NewSubmailEmailSender("app", "key", "from@example.com")
	sender.APIURL = "https://submail.example.com/mail/send.json"
	var form url.Values
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		_ = req.ParseForm()
		form = req.PostForm
		return responseWithBody(http.StatusOK, `{"status":"success"}`)
	})

	require.NoError(t, sender.Send(context.Background(), "to@example.com", "Subject", "Body"))
	assert.Equal(t, "app", form.Get("appid"))
	assert.Equal(t, "to@example.com", form.Get("to"))
	assert.Equal(t, "Subject", form.Get("subject"))
	assert.Equal(t, "Body", form.Get("text"))
	assert.Equal(t, "from@example.com", form.Get("from"))
}

func TestSubmailEmailSender_RejectsErrorStatus(t *testing.T) {
	sender := NewSubmailEmailSender("app", "key", "from@example.com")
	sender.APIURL = "https://submail.example.com/mail/send.json"
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		return responseWithBody(http.StatusOK, `{"status":"error","code":101,"msg":"bad appid"}`)
	})

	err := sender.Send(context.Background(), "to@example.com", "Subject", "Body")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad appid")
}

func TestMailtrapEmailSender_Send(t *testing.T) {
	sender := NewMailtrapEmailSender("token", "from@example.com")
	sender.APIURL = "https://mailtrap.example.com/api/send"
	var body string
	sender.client = newTestClient(func(req *http.Request) *http.Response {
		assert.Equal(t, "token", req.Header.Get("Api-Token"))
		read, _ := io.ReadAll(req.Body)
		body = string(read)
		return responseWithBody(http.StatusOK, `{"success":true}`)
	})

	require.NoError(t, sender.Send(context.Background(), "to@example.com", "Subject", "Body"))
	assert.Contains(t, body, `"email":"from@example.com"`)
	assert.Contains(t, body, `"email":"to@example.com"`)
	assert.Contains(t, body, `"subject":"Subject"`)
}

func TestMailtrapEmailSender_RequiresFrom(t *testing.T) {
	sender := NewMailtrapEmailSender("token", "")
	err := sender.Send(context.Background(), "to@example.com", "Subject", "Body")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "from address")
}

func readBody(req *http.Request) string {
	read, _ := io.ReadAll(req.Body)
	return string(read)
}
