package messaging

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureSequence records every outgoing request in order and replies with
// the canned responses (the last one repeats when requests exceed responses).
func captureSequence(statuses []int, bodies []string) (*http.Client, *[]*capturedRequest) {
	var captured []*capturedRequest
	client := newTestClient(func(req *http.Request) *http.Response {
		var body []byte
		if req.Body != nil {
			body, _ = io.ReadAll(req.Body)
		}
		captured = append(captured, &capturedRequest{method: req.Method, url: req.URL, header: req.Header.Clone(), body: body})
		i := len(captured) - 1
		if i >= len(statuses) {
			i = len(statuses) - 1
		}
		return &http.Response{
			StatusCode: statuses[i],
			Body:       io.NopCloser(strings.NewReader(bodies[i])),
			Header:     make(http.Header),
		}
	})
	return client, &captured
}

// ---------- Unisms / 联麓 template SMS ----------

func TestUnismsSmsSender_Contract(t *testing.T) {
	sender := NewUnismsSmsSender("100066", "1000000000000100", "app-key-secret", "70000278")
	client, captured := captureTransport(http.StatusOK, `{"status":"00","taskId":"202203170025179000005","message":"success","timestamp":1647483044178,"count":1}`)
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "13800000000", "123456"))

	req := *captured
	assert.Equal(t, http.MethodPost, req.method)
	assert.Equal(t, "apis.shlianlu.com", req.url.Host)
	assert.Equal(t, "/sms/trade/template/send", req.url.Path)
	assert.Equal(t, "application/json;charset=utf-8", req.header.Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(req.body, &body))
	assert.Equal(t, "3", body["Type"], "template SMS uses fixed Type=3")
	assert.Equal(t, "MD5", body["SignType"])
	assert.Equal(t, "1.2.0", body["Version"])
	assert.Equal(t, "100066", body["MchId"])
	assert.Equal(t, "1000000000000100", body["AppId"])
	assert.Equal(t, "70000278", body["TemplateId"])
	assert.Equal(t, []any{"13800000000"}, body["PhoneNumberSet"])
	assert.Equal(t, []any{"123456"}, body["TemplateParamSet"])
	require.NotEmpty(t, body["TimeStamp"])

	// Recompute the 联麓 MD5 signature from the transmitted signed params.
	params := map[string]string{
		"AppId":      body["AppId"].(string),
		"MchId":      body["MchId"].(string),
		"SignType":   body["SignType"].(string),
		"TemplateId": body["TemplateId"].(string),
		"TimeStamp":  body["TimeStamp"].(string),
		"Type":       body["Type"].(string),
		"Version":    body["Version"].(string),
	}
	assert.Equal(t, unismsSignature(params, "app-key-secret"), body["Signature"])
	assert.Equal(t, strings.ToUpper(unismsSignature(params, "app-key-secret")), body["Signature"], "signature must be uppercase")
}

func TestUnismsSmsSender_APIError(t *testing.T) {
	sender := NewUnismsSmsSender("m", "a", "k", "t")
	client, _ := captureTransport(http.StatusOK, `{"status":"01","message":"invalid signature"}`)
	sender.client = client

	err := sender.Send(context.Background(), "13800000000", "123456")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signature")
}

func TestUnismsSignature_ExcludesUnsignedParams(t *testing.T) {
	params := map[string]string{
		"AppId":            "app",
		"MchId":            "mch",
		"SignType":         "MD5",
		"TemplateId":       "tmpl",
		"TimeStamp":        "1647483066580",
		"Type":             "3",
		"Version":          "1.2.0",
		"PhoneNumberSet":   "ignored",
		"TemplateParamSet": "ignored",
		"Signature":        "ignored",
		"Tag":              "",
	}
	got := unismsSignature(params, "secret-key")
	// Manual expectation: sorted signed params + &key=secret-key, MD5 upper.
	want := strings.ToUpper(md5Hex("AppId=app&MchId=mch&SignType=MD5&TemplateId=tmpl&TimeStamp=1647483066580&Type=3&Version=1.2.0&key=secret-key"))
	assert.Equal(t, want, got)
}

// ---------- Yunpian / 云片 ----------

func TestYunpianSmsSender_Contract(t *testing.T) {
	sender := NewYunpianSmsSender("api-key-1", "我的签名", "", "")
	client, captured := captureTransport(http.StatusOK, `{"code":0,"msg":"发送成功"}`)
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "13800000000", "654321"))

	req := *captured
	assert.Equal(t, http.MethodPost, req.method)
	assert.Equal(t, "sms.yunpian.com", req.url.Host)
	assert.Equal(t, "/v2/sms/single_send.json", req.url.Path)
	assert.Equal(t, "application/x-www-form-urlencoded;charset=utf-8", req.header.Get("Content-Type"))
	assert.Equal(t, "api-key-1", mustBasicUser(t, req.header.Get("Authorization")))

	form, err := url.ParseQuery(string(req.body))
	require.NoError(t, err)
	assert.Equal(t, "13800000000", form.Get("mobile"))
	assert.True(t, strings.HasPrefix(form.Get("text"), "【我的签名】"), "yunpian requires 【签名】 prefix in text")
	assert.Contains(t, form.Get("text"), "654321")
}

func TestYunpianSmsSender_TemplateRendering(t *testing.T) {
	sender := NewYunpianSmsSender("k", "签名", "您的验证码是%s，5分钟内有效。", "")
	text := sender.renderYunpianText("998877")
	assert.Equal(t, "【签名】您的验证码是998877，5分钟内有效。", text)
}

func TestYunpianSmsSender_APIError(t *testing.T) {
	sender := NewYunpianSmsSender("k", "签名", "", "")
	client, _ := captureTransport(http.StatusOK, `{"code":114,"msg":"缺少参数"}`)
	sender.client = client

	err := sender.Send(context.Background(), "13800000000", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "缺少参数")
}

// ---------- JD Cloud SMS ----------

func TestJDCloudSmsSender_Contract(t *testing.T) {
	sender := NewJDCloudSmsSender("AK-test", "SK-test", "qm_sign_id", "bm_tmpl_id", "")
	client, captured := captureTransport(http.StatusOK, `{"requestId":"req-1","result":{"status":true,"code":"200","message":"ok","data":{"sequenceNumber":"seq-1"}}}`)
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "13800000000", "123456"))

	req := *captured
	assert.Equal(t, http.MethodPost, req.method)
	assert.Equal(t, "sms.jdcloud-api.com", req.url.Host)
	assert.Equal(t, "/v1/regions/cn-north-1/batchSend", req.url.Path, "default region cn-north-1")
	assert.Equal(t, "application/json", req.header.Get("Content-Type"))
	assert.NotEmpty(t, req.header.Get("x-jdcloud-date"))
	assert.NotEmpty(t, req.header.Get("x-jdcloud-nonce"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(req.body, &body))
	assert.Equal(t, "bm_tmpl_id", body["templateId"])
	assert.Equal(t, "qm_sign_id", body["signId"])
	assert.Equal(t, []any{"13800000000"}, body["phoneList"])
	assert.Equal(t, []any{"123456"}, body["params"])

	// Recompute the JDCLOUD2-HMAC-SHA256 signature independently.
	dateTime := req.header.Get("x-jdcloud-date")
	auth := jdcloudAuthorization("sms.jdcloud-api.com", "/v1/regions/cn-north-1/batchSend",
		"AK-test", "SK-test", "cn-north-1", "sms", req.body,
		dateTime[:8], dateTime, req.header.Get("x-jdcloud-nonce"))
	assert.Equal(t, auth, req.header.Get("Authorization"))
	assert.True(t, strings.HasPrefix(req.header.Get("Authorization"), "JDCLOUD2-HMAC-SHA256 Credential=AK-test/"))
	assert.Contains(t, req.header.Get("Authorization"), "/cn-north-1/sms/jdcloud2_request")
	assert.Contains(t, req.header.Get("Authorization"), "SignedHeaders=host;x-jdcloud-date;x-jdcloud-nonce")
}

func TestJDCloudSmsSender_ResultFailure(t *testing.T) {
	sender := NewJDCloudSmsSender("ak", "sk", "sign", "tmpl", "cn-east-1")
	client, captured := captureTransport(http.StatusOK, `{"requestId":"r","result":{"status":false,"code":"400","message":"INVALID_ARGUMENT"}}`)
	sender.client = client

	err := sender.Send(context.Background(), "13800000000", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "INVALID_ARGUMENT")
	assert.Equal(t, "/v1/regions/cn-east-1/batchSend", (*captured).url.Path, "configured region overrides the default")
}

// ---------- Vonage ----------

func TestVonageSmsSender_Contract(t *testing.T) {
	sender := NewVonageSmsSender("api-key", "api-secret", "ACME")
	client, captured := captureTransport(http.StatusOK, `{"messages":[{"status":"0","message-id":"m-1"}]}`)
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "+13800000000", "123456"))

	req := *captured
	assert.Equal(t, http.MethodPost, req.method)
	assert.Equal(t, "rest.nexmo.com", req.url.Host)
	assert.Equal(t, "/sms/json", req.url.Path)

	form, err := url.ParseQuery(string(req.body))
	require.NoError(t, err)
	assert.Equal(t, "api-key", form.Get("api_key"))
	assert.Equal(t, "api-secret", form.Get("api_secret"))
	assert.Equal(t, "13800000000", form.Get("to"), "vonage expects the number without the + prefix")
	assert.Equal(t, "ACME", form.Get("from"))
	assert.Contains(t, form.Get("text"), "123456")
}

func TestVonageSmsSender_BusinessFailure(t *testing.T) {
	sender := NewVonageSmsSender("k", "s", "F")
	client, _ := captureTransport(http.StatusOK, `{"messages":[{"status":"5","error-text":"Throttled"}]}`)
	sender.client = client

	err := sender.Send(context.Background(), "13800000000", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Throttled")
}

// ---------- Plivo ----------

func TestPlivoSmsSender_Contract(t *testing.T) {
	sender := NewPlivoSmsSender("mauthid123", "authtoken456", "+14155550123")
	client, captured := captureTransport(http.StatusCreated, `{"error_code":null,"error_message":null,"message_uuid":["uuid-1"]}`)
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "+14155550100", "123456"))

	req := *captured
	assert.Equal(t, http.MethodPost, req.method)
	assert.Equal(t, "api.plivo.com", req.url.Host)
	assert.Equal(t, "/v1/Account/mauthid123/Message/", req.url.Path)
	assert.Equal(t, "mauthid123", mustBasicUser(t, req.header.Get("Authorization")))
	assert.Equal(t, "authtoken456", mustBasicPass(t, req.header.Get("Authorization")))

	var body map[string]any
	require.NoError(t, json.Unmarshal(req.body, &body))
	assert.Equal(t, "+14155550123", body["src"])
	assert.Equal(t, []any{"+14155550100"}, body["dst"])
	assert.Contains(t, body["text"], "123456")
}

// ---------- Telnyx ----------

func TestTelnyxSmsSender_Contract(t *testing.T) {
	sender := NewTelnyxSmsSender("KEY123", "+14155550123")
	client, captured := captureTransport(http.StatusCreated, `{"data":{"id":"mg-1","to":"+14155550100"}}`)
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "+14155550100", "123456"))

	req := *captured
	assert.Equal(t, http.MethodPost, req.method)
	assert.Equal(t, "api.telnyx.com", req.url.Host)
	assert.Equal(t, "/v2/messages", req.url.Path)
	assert.Equal(t, "Bearer KEY123", req.header.Get("Authorization"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(req.body, &body))
	assert.Equal(t, "+14155550123", body["from"])
	assert.Equal(t, "+14155550100", body["to"])
	assert.Contains(t, body["text"], "123456")
}

func TestTelnyxSmsSender_APIError(t *testing.T) {
	sender := NewTelnyxSmsSender("KEY123", "+1")
	client, _ := captureTransport(http.StatusUnprocessableEntity, `{"errors":[{"code":"422","title":"Unprocessable Entity","detail":"invalid number"}]}`)
	sender.client = client

	err := sender.Send(context.Background(), "bad", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid number")
}

// ---------- Sinch (Conversation API, OAuth2) ----------

func TestSinchSmsSender_Contract(t *testing.T) {
	sender := NewSinchSmsSender("key-id-1", "key-secret-1", "project-1", "+14155550123", "")
	client, captured := captureSequence(
		[]int{http.StatusOK, http.StatusOK},
		[]string{`{"access_token":"tok-abc","token_type":"bearer"}`, `{"messages":[{"messageId":"msg-1","status":"QUEUED_FOR_DELIVERY"}]}`},
	)
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "+4915100000001", "123456"))

	reqs := *captured
	require.Len(t, reqs, 2, "sinch must exchange credentials for a bearer token first")

	tokenReq := reqs[0]
	assert.Equal(t, "auth.sinch.com", tokenReq.url.Host)
	assert.Equal(t, "/oauth/token", tokenReq.url.Path)
	form, err := url.ParseQuery(string(tokenReq.body))
	require.NoError(t, err)
	assert.Equal(t, "client_credentials", form.Get("grant_type"))
	assert.Equal(t, "key-id-1", form.Get("client_id"))
	assert.Equal(t, "key-secret-1", form.Get("client_secret"))
	assert.Equal(t, "sms", form.Get("scope"))

	sendReq := reqs[1]
	assert.Equal(t, "conversation.api.sinch.com", sendReq.url.Host)
	assert.Equal(t, "/v1/projects/project-1/messages:send", sendReq.url.Path)
	assert.Equal(t, "Bearer tok-abc", sendReq.header.Get("Authorization"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(sendReq.body, &body))
	assert.Equal(t, "+14155550123", body["from"])
	assert.Equal(t, "+4915100000001", body["to"])
	assert.Equal(t, "sms", body["transport"])
	text, ok := body["text"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, text["text"], "123456")
}

func TestSinchSmsSender_TokenFailure(t *testing.T) {
	sender := NewSinchSmsSender("k", "s", "p", "+1", "")
	client, _ := captureSequence([]int{http.StatusUnauthorized}, []string{`{"error":"invalid_client"}`})
	sender.client = client

	err := sender.Send(context.Background(), "+49", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token error")
}

// ---------- Bird (MessageBird) ----------

func TestBirdSmsSender_Contract(t *testing.T) {
	sender := NewBirdSmsSender("MBKEY123", "ACME")
	client, captured := captureTransport(http.StatusCreated, `{"id":"msg-1","recipients":{"total_count":1}}`)
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "+4915100000001", "123456"))

	req := *captured
	assert.Equal(t, http.MethodPost, req.method)
	assert.Equal(t, "rest.messagebird.com", req.url.Host)
	assert.Equal(t, "/messages", req.url.Path)
	assert.Equal(t, "AccessKey MBKEY123", req.header.Get("Authorization"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(req.body, &body))
	assert.Equal(t, "ACME", body["origin"])
	assert.Equal(t, []any{"+4915100000001"}, body["recipients"])
	assert.Contains(t, body["body"], "123456")
}

// ---------- Clickatell ----------

func TestClickatellSmsSender_Contract(t *testing.T) {
	sender := NewClickatellSmsSender("CAKEY123", "ACME")
	client, captured := captureTransport(http.StatusCreated, `{"data":[{"response":{"url":"https://api.clickatell.com/messages/msg-1"}}],"errors":[]}`)
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "+27111234567", "123456"))

	req := *captured
	assert.Equal(t, http.MethodPost, req.method)
	assert.Equal(t, "api.clickatell.com", req.url.Host)
	assert.Equal(t, "/messages", req.url.Path)
	assert.Equal(t, "Bearer CAKEY123", req.header.Get("Authorization"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(req.body, &body))
	assert.Equal(t, "+27111234567", body["to"])
	assert.Equal(t, "ACME", body["from"])
	assert.Contains(t, body["text"], "123456")
}

func TestClickatellSmsSender_APIError(t *testing.T) {
	sender := NewClickatellSmsSender("k", "F")
	client, _ := captureTransport(http.StatusOK, `{"data":[],"errors":[{"code":"401","message":"Invalid API key"}]}`)
	sender.client = client

	err := sender.Send(context.Background(), "+27", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid API key")
}

// ---------- Africa's Talking ----------

func TestAfricastalkingSmsSender_Contract(t *testing.T) {
	sender := NewAfricastalkingSmsSender("my-app", "ATKEY123", "ACME")
	client, captured := captureTransport(http.StatusOK, `{"SMSMessageData":{"Message":"Sent for queueing","Recipients":[{"statusCode":"Success","messageId":"m-1","number":"+254700000000"}]}}`)
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "+254700000000", "123456"))

	req := *captured
	assert.Equal(t, http.MethodPost, req.method)
	assert.Equal(t, "api.africastalking.com", req.url.Host)
	assert.Equal(t, "/version1/messaging", req.url.Path)
	assert.Equal(t, "my-app", req.header.Get("username"))
	assert.Equal(t, "ATKEY123", req.header.Get("apiKey"))
	assert.Equal(t, "application/json", req.header.Get("Accept"))

	form, err := url.ParseQuery(string(req.body))
	require.NoError(t, err)
	assert.Equal(t, "+254700000000", form.Get("to"))
	assert.Equal(t, "ACME", form.Get("from"))
	assert.Contains(t, form.Get("message"), "123456")
}

func TestAfricastalkingSmsSender_RecipientFailure(t *testing.T) {
	sender := NewAfricastalkingSmsSender("app", "key", "")
	client, _ := captureTransport(http.StatusOK, `{"SMSMessageData":{"Recipients":[{"statusCode":"Failed","message":"Invalid phone number"}]}}`)
	sender.client = client

	err := sender.Send(context.Background(), "bad", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid phone number")
}

// ---------- Builder registration ----------

func TestBuildSmsSender_SMSFamilyBatch_MapsNativeColumns(t *testing.T) {
	builder := ProviderSenderBuilder{}
	cfg := func(kind domain.Kind, values map[string]string) *domain.MessagingProviderConfig {
		return &domain.MessagingProviderConfig{Kind: kind, Values: values}
	}

	sender, err := builder.BuildSmsSender(context.Background(), cfg("unisms_sms", map[string]string{
		"admin": "mch-1", "password": "app-key", "appId": "app-1", "template": "tmpl-1",
	}))
	require.NoError(t, err)
	unisms, ok := sender.(*UnismsSmsSender)
	require.True(t, ok)
	assert.Equal(t, "mch-1", unisms.MchID)
	assert.Equal(t, "app-key", unisms.AppKey)
	assert.Equal(t, "app-1", unisms.AppID)
	assert.Equal(t, "tmpl-1", unisms.TemplateID)

	sender, err = builder.BuildSmsSender(context.Background(), cfg("yunpian_sms", map[string]string{
		"password": "yp-key", "sender": "云片签名",
	}))
	require.NoError(t, err)
	yunpian, ok := sender.(*YunpianSmsSender)
	require.True(t, ok)
	assert.Equal(t, "yp-key", yunpian.APIKey)
	assert.Equal(t, "云片签名", yunpian.SignName)

	sender, err = builder.BuildSmsSender(context.Background(), cfg("jdcloud_sms", map[string]string{
		"admin": "ak", "password": "sk", "sender": "qm-sign", "template": "bm-tmpl", "region": "cn-east-1",
	}))
	require.NoError(t, err)
	jd, ok := sender.(*JDCloudSmsSender)
	require.True(t, ok)
	assert.Equal(t, "ak", jd.AccessKey)
	assert.Equal(t, "sk", jd.SecretKey)
	assert.Equal(t, "qm-sign", jd.SignID)
	assert.Equal(t, "bm-tmpl", jd.TemplateID)
	assert.Equal(t, "cn-east-1", jd.Region)

	sender, err = builder.BuildSmsSender(context.Background(), cfg("vonage_sms", map[string]string{
		"admin": "vk", "password": "vs", "sender": "ACME",
	}))
	require.NoError(t, err)
	vonage, ok := sender.(*VonageSmsSender)
	require.True(t, ok)
	assert.Equal(t, "vk", vonage.APIKey)
	assert.Equal(t, "ACME", vonage.From)

	sender, err = builder.BuildSmsSender(context.Background(), cfg("plivo_sms", map[string]string{
		"admin": "pa", "password": "pt", "sender": "+1",
	}))
	require.NoError(t, err)
	plivo, ok := sender.(*PlivoSmsSender)
	require.True(t, ok)
	assert.Equal(t, "pa", plivo.AuthID)
	assert.Equal(t, "pt", plivo.AuthToken)

	sender, err = builder.BuildSmsSender(context.Background(), cfg("telnyx_sms", map[string]string{
		"password": "tk", "sender": "+1",
	}))
	require.NoError(t, err)
	telnyx, ok := sender.(*TelnyxSmsSender)
	require.True(t, ok)
	assert.Equal(t, "tk", telnyx.APIKey)

	sender, err = builder.BuildSmsSender(context.Background(), cfg("sinch_sms", map[string]string{
		"admin": "ki", "password": "ks", "appId": "proj", "sender": "+1",
	}))
	require.NoError(t, err)
	sinch, ok := sender.(*SinchSmsSender)
	require.True(t, ok)
	assert.Equal(t, "ki", sinch.KeyID)
	assert.Equal(t, "ks", sinch.KeySecret)
	assert.Equal(t, "proj", sinch.ProjectID)

	sender, err = builder.BuildSmsSender(context.Background(), cfg("bird_sms", map[string]string{
		"password": "bk", "sender": "ACME",
	}))
	require.NoError(t, err)
	bird, ok := sender.(*BirdSmsSender)
	require.True(t, ok)
	assert.Equal(t, "bk", bird.AccessKey)
	assert.Equal(t, "ACME", bird.Origin)

	sender, err = builder.BuildSmsSender(context.Background(), cfg("clickatell_sms", map[string]string{
		"password": "ck", "sender": "ACME",
	}))
	require.NoError(t, err)
	clickatell, ok := sender.(*ClickatellSmsSender)
	require.True(t, ok)
	assert.Equal(t, "ck", clickatell.APIKey)

	sender, err = builder.BuildSmsSender(context.Background(), cfg("africastalking_sms", map[string]string{
		"admin": "user", "password": "ak", "sender": "ACME",
	}))
	require.NoError(t, err)
	at, ok := sender.(*AfricastalkingSmsSender)
	require.True(t, ok)
	assert.Equal(t, "user", at.Username)
	assert.Equal(t, "ak", at.APIKey)
}
