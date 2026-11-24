package messaging

import (
	"context"
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // test-only MD5 for SmsBao p parameter
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturedRequest records the outgoing request (including its body) for contract assertions.
type capturedRequest struct {
	method string
	url    *url.URL
	header http.Header
	body   []byte
}

// captureTransport returns a test client and a pointer that receives the captured request.
func captureTransport(status int, respBody string) (*http.Client, **capturedRequest) {
	target := new(*capturedRequest)
	client := newTestClient(func(req *http.Request) *http.Response {
		var body []byte
		if req.Body != nil {
			body, _ = io.ReadAll(req.Body)
		}
		*target = &capturedRequest{method: req.Method, url: req.URL, header: req.Header.Clone(), body: body}
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(respBody)),
			Header:     make(http.Header),
		}
	})
	return client, target
}

// ---------- Alibaba Cloud SMS (RPC HMAC-SHA1 signature) ----------

func TestAlibabaSmsSender_Contract(t *testing.T) {
	sender := NewAlibabaSmsSender("AKID-test", "secret-test", "签名", "SMS_123")
	client, captured := captureTransport(http.StatusOK, `{"Code":"OK","Message":"OK","RequestId":"r1"}`)
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "13800000000", "123456"))

	req := *captured
	assert.Equal(t, http.MethodPost, req.method)
	assert.Equal(t, "dysmsapi.aliyuncs.com", req.url.Host)
	assert.Equal(t, "/", req.url.Path)

	params := req.url.Query()
	assert.Equal(t, "SendSms", params.Get("Action"))
	assert.Equal(t, "2017-05-25", params.Get("Version"))
	assert.Equal(t, "13800000000", params.Get("PhoneNumbers"))
	assert.Equal(t, "SMS_123", params.Get("TemplateCode"))
	assert.Equal(t, `{"code":"123456"}`, params.Get("TemplateParam"))
	assert.Equal(t, "HMAC-SHA1", params.Get("SignatureMethod"))

	// Recompute the RPC signature from the transmitted parameters.
	signature := params.Get("Signature")
	require.NotEmpty(t, signature)
	signMap := make(map[string]string, len(params))
	for k, v := range params {
		if k != "Signature" {
			signMap[k] = v[0]
		}
	}
	assert.Equal(t, signRequest(signMap, "secret-test"), signature, "signature must match Alibaba RPC spec")
}

func TestAlibabaSmsSender_APIError(t *testing.T) {
	sender := NewAlibabaSmsSender("AKID-test", "secret-test", "签名", "SMS_123")
	client, _ := captureTransport(http.StatusOK, `{"Code":"isv.MOBILE_NUMBER_ILLEGAL","Message":"手机号码格式错误"}`)
	sender.client = client

	err := sender.Send(context.Background(), "bad-phone", "123456")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "手机号码格式错误")
}

// ---------- Tencent Cloud SMS (TC3-HMAC-SHA256) ----------

func TestTencentSmsSender_Contract(t *testing.T) {
	sender := NewTencentSmsSender("SecretID-test", "SecretKey-test", "1400000000", "腾讯云", "20001", "")
	client, captured := captureTransport(http.StatusOK, `{"Response":{"SendStatusSet":[{"Code":"Ok","Message":"success","PhoneNumber":"+8613800000000","SmsRequestId":"sr-1"}],"RequestId":"req-1"}}`)
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "13800000000", "123456"))

	req := *captured
	assert.Equal(t, http.MethodPost, req.method)
	assert.Equal(t, "sms.tencentcloudapi.com", req.url.Host)
	assert.Equal(t, "/", req.url.Path)
	assert.Empty(t, req.url.RawQuery, "TC3 carries parameters in headers/body, not query")

	assert.Equal(t, "SendSms", req.header.Get("X-TC-Action"))
	assert.Equal(t, "2021-01-11", req.header.Get("X-TC-Version"))
	assert.Equal(t, tencentSMSDefaultRegion, req.header.Get("X-TC-Region"), "required public header defaults to ap-guangzhou")
	assert.Equal(t, "application/json", req.header.Get("Content-Type"))
	timestamp := req.header.Get("X-TC-Timestamp")
	require.NotEmpty(t, timestamp)

	var body map[string]any
	require.NoError(t, json.Unmarshal(req.body, &body))
	assert.Equal(t, []any{"+8613800000000"}, body["PhoneNumberSet"])
	assert.Equal(t, "1400000000", body["SmsSdkAppId"])
	assert.Equal(t, "腾讯云", body["SignName"])
	assert.Equal(t, "20001", body["TemplateId"])
	assert.Equal(t, []any{"123456"}, body["TemplateParamSet"])

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	require.NoError(t, err)
	expected, err := tencentTC3Authorization("SecretID-test", "SecretKey-test", "sms", req.body, ts)
	require.NoError(t, err)
	assert.Equal(t, expected, req.header.Get("Authorization"))
	assert.True(t, strings.HasPrefix(req.header.Get("Authorization"), "TC3-HMAC-SHA256 Credential=SecretID-test/"))
	assert.Contains(t, req.header.Get("Authorization"), "SignedHeaders=content-type;host")
}

// TestTencentTC3Authorization_KnownAnswer 独立按官方派生链
// （SecretDate = HMAC("TC3"+SecretKey, Date)，
// https://cloud.tencent.com/document/api/382/55981）复算签名，
// 钉死「TC3+ 前缀缺失」回归：生产实现若偏离官方派生链，本测试必须失败。
func TestTencentTC3Authorization_KnownAnswer(t *testing.T) {
	const (
		secretID  = "SecretID-test"
		secretKey = "SecretKey-test"
		service   = "sms"
		host      = "sms.tencentcloudapi.com"
	)
	ts := int64(1700000000)
	date := time.Unix(ts, 0).UTC().Format("20060102")
	payload := []byte(`{"PhoneNumberSet":["+8613800000000"]}`)

	hmacSum := func(key []byte, msg string) []byte {
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(msg))
		return mac.Sum(nil)
	}
	hexSHA := func(b []byte) string {
		sum := sha256.Sum256(b)
		return hex.EncodeToString(sum[:])
	}

	canonicalRequest := fmt.Sprintf("POST\n/\n\ncontent-type:application/json\nhost:%s\n\ncontent-type;host\n%s",
		host, hexSHA(payload))
	stringToSign := fmt.Sprintf("TC3-HMAC-SHA256\n%d\n%s/%s/tc3_request\n%s",
		ts, date, service, hexSHA([]byte(canonicalRequest)))

	secretDate := hmacSum([]byte("TC3"+secretKey), date)
	secretService := hmacSum(secretDate, service)
	secretSigning := hmacSum(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSum(secretSigning, stringToSign))

	want := fmt.Sprintf("TC3-HMAC-SHA256 Credential=%s/%s/%s/tc3_request, SignedHeaders=content-type;host, Signature=%s",
		secretID, date, service, signature)

	got, err := tencentTC3Authorization(secretID, secretKey, service, payload, ts)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestTencentSmsSender_ErrorEnvelope(t *testing.T) {
	sender := NewTencentSmsSender("id", "key", "app", "sign", "tmpl", "ap-shanghai")
	client, captured := captureTransport(http.StatusOK, `{"Response":{"Error":{"Code":"FailedOperation.PhoneNumberInBlacklist","Message":"号码在黑名单中"},"RequestId":"req-2"}}`)
	sender.client = client

	err := sender.Send(context.Background(), "13800000000", "123456")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FailedOperation.PhoneNumberInBlacklist")
	assert.Contains(t, err.Error(), "号码在黑名单中")
	assert.Equal(t, "ap-shanghai", (*captured).header.Get("X-TC-Region"), "configured region overrides the default")
}

func TestTencentSmsSender_SendStatusFailure(t *testing.T) {
	sender := NewTencentSmsSender("id", "key", "app", "sign", "tmpl", "")
	client, _ := captureTransport(http.StatusOK, `{"Response":{"SendStatusSet":[{"Code":"LimitsExceeded","Message":"more requests than the limit","PhoneNumber":"+8613800000000"}],"RequestId":"req-3"}}`)
	sender.client = client

	err := sender.Send(context.Background(), "13800000000", "123456")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LimitsExceeded")
}

// ---------- Volcengine SMS (HMAC-SHA256 V4) ----------

func TestVolcengineSmsSender_Contract(t *testing.T) {
	sender := NewVolcengineSmsSender("AKLT-test", "secret-test", "A100000", "火山短信", "ST_100")
	client, captured := captureTransport(http.StatusOK, `{"ResponseMetadata":{"RequestId":"r-1","Action":"SendSms","Version":"2020-01-01","Service":"volcSMS","Region":"cn-north-1"},"Result":{"MessageID":["m-1"]}}`)
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "18800000000", "1234"))

	req := *captured
	assert.Equal(t, http.MethodPost, req.method)
	assert.Equal(t, "sms.volcengineapi.com", req.url.Host)

	q := req.url.Query()
	assert.Equal(t, "SendSms", q.Get("Action"))
	assert.Equal(t, "2020-01-01", q.Get("Version"), "SendSms uses API version 2020-01-01")

	var body map[string]any
	require.NoError(t, json.Unmarshal(req.body, &body))
	assert.Equal(t, "A100000", body["SmsAccount"])
	assert.Equal(t, "火山短信", body["Sign"])
	assert.Equal(t, "ST_100", body["TemplateID"])
	assert.Equal(t, `{"code":"1234"}`, body["TemplateParam"])
	assert.Equal(t, "18800000000", body["PhoneNumbers"], "PhoneNumbers must be a comma-separated string")

	xDate := req.header.Get("X-Date")
	require.Len(t, xDate, len("20060102T150405Z"))
	sum := sha256.Sum256(req.body)
	assert.Equal(t, hexLower(sum), req.header.Get("X-Content-Sha256"))

	expected, err := volcSign(&http.Request{
		Method: http.MethodPost,
		URL:    req.url,
		Host:   req.url.Host,
		Header: req.header,
	}, req.body, "AKLT-test", "secret-test", "cn-north-1", "volcSMS", xDate)
	require.NoError(t, err)
	assert.Equal(t, expected, req.header.Get("Authorization"))
	assert.Contains(t, req.header.Get("Authorization"), "HMAC-SHA256 Credential=AKLT-test/")
	assert.Contains(t, req.header.Get("Authorization"), "/cn-north-1/volcSMS/request")
	assert.Contains(t, req.header.Get("Authorization"), "SignedHeaders=content-type;host;x-content-sha256;x-date")
}

func TestVolcengineSmsSender_ErrorInMetadata(t *testing.T) {
	sender := NewVolcengineSmsSender("ak", "sk", "acct", "sign", "tmpl")
	client, _ := captureTransport(http.StatusOK, `{"ResponseMetadata":{"RequestId":"r-2","Error":{"Code":"RE:0004","Message":"签名错误"}}}`)
	sender.client = client

	err := sender.Send(context.Background(), "18800000000", "1234")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RE:0004")
	assert.Contains(t, err.Error(), "签名错误")
}

// ---------- Huawei Cloud SMS (X-WSSE) ----------

func TestHuaweiSmsSender_Contract(t *testing.T) {
	sender := NewHuaweiSmsSender("https://smsapi.cn-north-4.myhuaweicloud.com:443", "appkey-1", "appsecret-1", "华为云", "SMS.001", "hwSender")
	client, captured := captureTransport(http.StatusOK, `{"code":"000000","description":"success","msgId":"m-1","total":1}`)
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "13800000000", "123456"))

	req := *captured
	assert.Equal(t, http.MethodPost, req.method)
	assert.Equal(t, "/sms/batchSendSms/v1", req.url.Path)
	assert.Equal(t, "application/x-www-form-urlencoded", req.header.Get("Content-Type"))
	assert.Equal(t, `WSSE realm="SDP",profile="UsernameToken",type="Appkey"`, req.header.Get("Authorization"))

	form, err := url.ParseQuery(string(req.body))
	require.NoError(t, err)
	assert.Equal(t, "hwSender", form.Get("from"))
	assert.Equal(t, "+8613800000000", form.Get("to"))
	assert.Equal(t, "SMS.001", form.Get("templateId"))
	assert.Equal(t, `["123456"]`, form.Get("templateParas"))
	assert.Equal(t, "华为云", form.Get("signature"))

	re := regexp.MustCompile(`UsernameToken Username="([^"]+)",PasswordDigest="([^"]+)",Nonce="([^"]+)",Created="([^"]+)"`)
	m := re.FindStringSubmatch(req.header.Get("X-WSSE"))
	require.Len(t, m, 5)
	assert.Equal(t, "appkey-1", m[1])
	assert.NotContains(t, req.header.Get("X-WSSE"), "appsecret-1")
	digest := sha256.Sum256([]byte(m[3] + m[4] + "appsecret-1"))
	assert.Equal(t, base64.StdEncoding.EncodeToString(digest[:]), m[2])
}

func TestToE164CN(t *testing.T) {
	assert.Equal(t, "+8613800000000", toE164CN("13800000000"))
	assert.Equal(t, "+8613800000000", toE164CN("+8613800000000"))
	assert.Equal(t, "+15017122661", toE164CN("+15017122661"))
}

func TestHuaweiSmsSender_APIError(t *testing.T) {
	sender := NewHuaweiSmsSender("https://smsapi.cn-north-4.myhuaweicloud.com:443", "k", "s", "sign", "tmpl", "from")
	client, _ := captureTransport(http.StatusOK, `{"code":"E000114","description":"Invalid templateId"}`)
	sender.client = client

	err := sender.Send(context.Background(), "13800000000", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid templateId")
}

func TestHuaweiSmsSender_NonSuccessStatusIsError(t *testing.T) {
	sender := NewHuaweiSmsSender("https://smsapi.cn-north-4.myhuaweicloud.com:443", "k", "s", "sign", "tmpl", "from")
	client, _ := captureTransport(http.StatusForbidden, `{"error_code":"AUTH.0001","error_msg":"signature mismatch"}`)
	sender.client = client

	err := sender.Send(context.Background(), "13800000000", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 403")
}

func TestHuaweiSmsSender_MissingCodeIsError(t *testing.T) {
	sender := NewHuaweiSmsSender("https://smsapi.cn-north-4.myhuaweicloud.com:443", "k", "s", "sign", "tmpl", "from")
	client, _ := captureTransport(http.StatusOK, `{"description":"gateway accepted","total":1}`)
	sender.client = client

	err := sender.Send(context.Background(), "13800000000", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "huawei sms send failed")
}

// ---------- Twilio Messages ----------

func TestTwilioSmsSender_Contract(t *testing.T) {
	sender := NewTwilioSmsSender("ACtest", "token-test", "+15017122661")
	client, captured := captureTransport(http.StatusCreated, `{"sid":"SM123","status":"queued"}`)
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "+15558675310", "123456"))

	req := *captured
	assert.Equal(t, http.MethodPost, req.method)
	assert.Equal(t, "api.twilio.com", req.url.Host)
	assert.Equal(t, "/2010-04-01/Accounts/ACtest/Messages.json", req.url.Path)
	assert.Empty(t, req.url.RawQuery, "Twilio parameters must be in the form body, not the query string")
	assert.Equal(t, "application/x-www-form-urlencoded", req.header.Get("Content-Type"))

	authz := req.header.Get("Authorization")
	assert.True(t, strings.HasPrefix(authz, "Basic "), "basic auth expected")
	assert.Equal(t, "ACtest", mustBasicUser(t, authz))
	assert.Equal(t, "token-test", mustBasicPass(t, authz))

	form, err := url.ParseQuery(string(req.body))
	require.NoError(t, err)
	assert.Equal(t, "+15558675310", form.Get("To"))
	assert.Equal(t, "+15017122661", form.Get("From"))
	assert.Contains(t, form.Get("Body"), "123456")
}

func TestTwilioSmsSender_APIError(t *testing.T) {
	sender := NewTwilioSmsSender("ACtest", "token", "+15017122661")
	client, _ := captureTransport(http.StatusBadRequest, `{"code":21211,"message":"'From' must be a Twilio phone number"}`)
	sender.client = client

	err := sender.Send(context.Background(), "+15558675310", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a Twilio phone number")
	assert.Contains(t, err.Error(), "21211")
}

// ---------- Azure Communication Services SMS ----------

func TestAzureACSsmsSender_Contract(t *testing.T) {
	sender := NewAzureACSsmsSender("endpoint=https://contoso.communication.azure.com/;accesskey=ZmFrZS1rZXk=", "+18001110000")
	client, captured := captureTransport(http.StatusAccepted, `{"value":[{"to":"+8613800000000","messageId":"id-1","httpStatusCode":202,"successful":true}]}`)
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "13800000000", "123456"))

	req := *captured
	assert.Equal(t, http.MethodPost, req.method)
	assert.Equal(t, "contoso.communication.azure.com", req.url.Host)
	assert.Equal(t, "/sms", req.url.Path)
	assert.Equal(t, "2021-03-07", req.url.Query().Get("api-version"))
	assert.Equal(t, "Basic ZmFrZS1rZXk=", req.header.Get("Authorization"))
	assert.Equal(t, "application/json", req.header.Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(req.body, &body))
	assert.Equal(t, "+18001110000", body["from"])
	assert.Contains(t, body["message"], "123456")
	recipients, ok := body["smsRecipients"].([]any)
	require.True(t, ok)
	require.Len(t, recipients, 1)
	assert.Equal(t, "+8613800000000", recipients[0].(map[string]any)["to"])
}

func TestAzureACSsmsSender_PartialFailure(t *testing.T) {
	sender := NewAzureACSsmsSender("endpoint=https://contoso.communication.azure.com/;accesskey=k", "+18001110000")
	client, _ := captureTransport(http.StatusAccepted, `{"value":[{"to":"+8613800000000","httpStatusCode":400,"errorMessage":"Invalid To phone number format.","successful":false}]}`)
	sender.client = client

	err := sender.Send(context.Background(), "bad", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid To phone number format.")
}

func TestAzureACSsmsSender_BadConnectionString(t *testing.T) {
	sender := NewAzureACSsmsSender("accesskey=only", "+1")
	err := sender.Send(context.Background(), "13800000000", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint")
}

// ---------- Baidu Cloud SMS v3 (BCE v1 signature) ----------

func TestBaiduSmsSender_Contract(t *testing.T) {
	sender := NewBaiduSmsSender("AK-test", "SK-test", "sms-sign-1", "sms-tmpl-1", "")
	client, captured := captureTransport(http.StatusOK, `{"code":1000,"message":"OK"}`)
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "13800000000", "123456"))

	req := *captured
	assert.Equal(t, http.MethodPost, req.method)
	assert.Equal(t, "smsv3.bj.baidubce.com", req.url.Host)
	assert.Equal(t, "/api/v3/sendSms", req.url.Path)
	assert.NotEmpty(t, req.header.Get("x-bce-date"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(req.body, &body))
	assert.Equal(t, "13800000000", body["mobile"])
	assert.Equal(t, "sms-tmpl-1", body["template"])
	assert.Equal(t, "sms-sign-1", body["signatureId"])
	assert.Equal(t, map[string]any{"code": "123456"}, body["contentVar"])

	parts := strings.Split(req.header.Get("Authorization"), "/")
	require.Len(t, parts, 6, "bce-auth-v1/AK/timestamp/expiration/signedHeaders/signature")
	assert.Equal(t, "bce-auth-v1", parts[0])
	assert.Equal(t, "AK-test", parts[1])
	assert.Equal(t, "host;x-bce-date", parts[4])

	prefix := strings.Join(parts[:4], "/")
	signingKey := hexHMACSHA256([]byte("SK-test"), prefix)
	canonical := "POST\n" + uriEncodePath("/api/v3/sendSms") + "\n\nhost:" + uriEncodeRFC3986("smsv3.bj.baidubce.com") +
		"\nx-bce-date:" + uriEncodeRFC3986(req.header.Get("x-bce-date"))
	assert.Equal(t, hexHMACSHA256([]byte(signingKey), canonical), parts[5])
}

func TestBaiduSmsSender_APIError(t *testing.T) {
	sender := NewBaiduSmsSender("ak", "sk", "sign", "tmpl", "")
	client, _ := captureTransport(http.StatusOK, `{"code":1002,"message":"template not found"}`)
	sender.client = client

	err := sender.Send(context.Background(), "13800000000", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "template not found")
}

func TestBaiduSmsSender_HTTPErrorBeforeDecode(t *testing.T) {
	sender := NewBaiduSmsSender("ak", "sk", "sign", "tmpl", "")
	client, _ := captureTransport(http.StatusForbidden, `<html>403 Forbidden</html>`)
	sender.client = client

	err := sender.Send(context.Background(), "13800000000", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 403")
}

// ---------- UCloud USMS (SHA256 signature) ----------

func TestUCloudSmsSender_Contract(t *testing.T) {
	sender := NewUCloudSmsSender("pub-key", "priv-key", "org-1", "UCloud", "UTA2021")
	client, captured := captureTransport(http.StatusOK, `{"Action":"SendUSMSMessageResponse","RetCode":0,"Message":"提交发送成功","SessionNo":"s-1"}`)
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "18516230000", "123456"))

	req := *captured
	assert.Equal(t, http.MethodPost, req.method)
	assert.Equal(t, "api.ucloud.cn", req.url.Host)

	q := req.url.Query()
	assert.Equal(t, "SendUSMSMessage", q.Get("Action"))
	assert.Equal(t, "pub-key", q.Get("PublicKey"))
	assert.Equal(t, "org-1", q.Get("ProjectId"))
	assert.Equal(t, "18516230000", q.Get("PhoneNumbers.0"))
	assert.Equal(t, "UTA2021", q.Get("TemplateId"))
	assert.Equal(t, "123456", q.Get("TemplateParams.0"))

	signature := q.Get("Signature")
	require.NotEmpty(t, signature, "UCloud requests must carry the Signature parameter")
	q.Del("Signature")
	assert.Equal(t, ucloudSignature(q, "priv-key"), signature)
}

func TestUCloudSmsSender_APIError(t *testing.T) {
	sender := NewUCloudSmsSender("pub", "priv", "proj", "sign", "tmpl")
	client, _ := captureTransport(http.StatusOK, `{"RetCode":161,"Message":"invalid signature"}`)
	sender.client = client

	err := sender.Send(context.Background(), "18516230000", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signature")
}

// ---------- SUBMAIL xsend ----------

func TestSubmailSmsSender_Contract(t *testing.T) {
	sender := NewSubmailSmsSender("appid-1", "appkey-1", "赛邮", "ThJBE4")
	client, captured := captureTransport(http.StatusOK, `{"status":"success","send_id":"893c91","fee":1}`)
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "13800000000", "123456"))

	req := *captured
	assert.Equal(t, http.MethodPost, req.method)
	assert.Equal(t, "api-v4.mysubmail.com", req.url.Host)
	assert.Equal(t, "/sms/xsend.json", req.url.Path)

	var body map[string]any
	require.NoError(t, json.Unmarshal(req.body, &body))
	assert.Equal(t, "appid-1", body["appid"])
	assert.Equal(t, "13800000000", body["to"])
	assert.Equal(t, "ThJBE4", body["project"])
	assert.Equal(t, "appkey-1", body["signature"], "normal mode sends the app key directly")
	assert.NotContains(t, body, "sign_type")
	assert.Equal(t, `{"code":"123456"}`, body["vars"], "vars must be a JSON string")
}

func TestSubmailSmsSender_ErrorBodyWithHTTP200(t *testing.T) {
	sender := NewSubmailSmsSender("appid", "appkey", "sign", "proj")
	client, _ := captureTransport(http.StatusOK, `{"status":"error","code":"15555","msg":"wrong captcha"}`)
	sender.client = client

	err := sender.Send(context.Background(), "13800000000", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "15555")
	assert.Contains(t, err.Error(), "wrong captcha")
}

// ---------- SmsBao ----------

func TestSmsBaoSender_PasswordIsMD5(t *testing.T) {
	sender := NewSmsBaoSender("user1", "plain-password", "测试", "验证码：%s", "")
	client, captured := captureTransport(http.StatusOK, "0")
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "+8613800000000", "123456"))

	q := (*captured).url.Query()
	assert.Equal(t, "13800000000", q.Get("m"), "the +86 prefix must be stripped")
	assert.Equal(t, md5Hex("plain-password"), q.Get("p"), "p must be the MD5 of the password")
	assert.Equal(t, "【测试】验证码：123456", q.Get("c"))
}

func TestSmsBaoSender_Hex32KeyPassThrough(t *testing.T) {
	key := "9b11127a9701975c734b8aee81ee3526"
	sender := NewSmsBaoSender("user1", key, "测试", "%s", "")
	client, captured := captureTransport(http.StatusOK, "0")
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "13800000000", "1"))
	assert.Equal(t, key, (*captured).url.Query().Get("p"), "a 32-hex key (MD5/ApiKey) is sent unchanged")
}

func TestSmsBaoSender_StatusCodes(t *testing.T) {
	sender := NewSmsBaoSender("user1", "key", "测试", "%s", "")
	client, _ := captureTransport(http.StatusOK, "30")
	sender.client = client

	err := sender.Send(context.Background(), "13800000000", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credentials")
}

// ---------- MSG91 flow v5 ----------

func TestMsg91Sender_Contract(t *testing.T) {
	sender := NewMsg91Sender("authkey-1", "MSG91", "6466865")
	client, captured := captureTransport(http.StatusOK, `{"messageId":"m-1"}`)
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "+919999999999", "1234"))

	req := *captured
	assert.Equal(t, http.MethodPost, req.method)
	assert.Equal(t, "control.msg91.com", req.url.Host)
	assert.Equal(t, "/api/v5/flow/", req.url.Path)
	assert.Equal(t, "authkey-1", req.header.Get("authkey"))
	assert.Equal(t, "application/json", req.header.Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(req.body, &body))
	assert.Equal(t, "6466865", body["template_id"])
	assert.Equal(t, "MSG91", body["sender"])
	assert.Equal(t, "919999999999", body["mobiles"], "the + prefix must be stripped")
	assert.Equal(t, "1234", body["code"])
}

func TestMsg91Sender_HTTPError(t *testing.T) {
	sender := NewMsg91Sender("authkey", "sender", "tmpl")
	client, _ := captureTransport(http.StatusUnauthorized, `{"error":"invalid authkey"}`)
	sender.client = client

	err := sender.Send(context.Background(), "+919999999999", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

// ---------- Infobip ----------

func TestInfobipSmsSender_Contract(t *testing.T) {
	sender := NewInfobipSmsSender("https://api.infobip.com", "key-1", "InfoSMS")
	client, captured := captureTransport(http.StatusOK, `{"messages":[{"to":"41788676375","status":"ACCEPTED"}]}`)
	sender.client = client

	require.NoError(t, sender.Send(context.Background(), "+41788676375", "1234"))

	req := *captured
	assert.Equal(t, http.MethodPost, req.method)
	assert.Equal(t, "/sms/2/text/advanced", req.url.Path)
	assert.Equal(t, "App key-1", req.header.Get("Authorization"))

	var body struct {
		Messages []struct {
			From         string `json:"from"`
			Destinations []struct {
				To string `json:"to"`
			} `json:"destinations"`
			Text string `json:"text"`
		} `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(req.body, &body))
	require.Len(t, body.Messages, 1)
	assert.Equal(t, "InfoSMS", body.Messages[0].From)
	assert.Equal(t, "+41788676375", body.Messages[0].Destinations[0].To)
	assert.Contains(t, body.Messages[0].Text, "1234")
}

// ---------- helpers ----------

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s)) //nolint:gosec // test-only MD5 for SmsBao p parameter
	return hex.EncodeToString(sum[:])
}

func hexLower(sum [sha256.Size]byte) string {
	const hexDigits = "0123456789abcdef"
	out := make([]byte, 0, len(sum)*2)
	for _, b := range sum {
		out = append(out, hexDigits[b>>4], hexDigits[b&0xf])
	}
	return string(out)
}

func mustBasicUser(t *testing.T, header string) string {
	t.Helper()
	user, _, ok := parseBasicAuthForTest(t, header)
	require.True(t, ok)
	return user
}

func mustBasicPass(t *testing.T, header string) string {
	t.Helper()
	_, pass, ok := parseBasicAuthForTest(t, header)
	require.True(t, ok)
	return pass
}

func parseBasicAuthForTest(t *testing.T, header string) (string, string, bool) {
	t.Helper()
	encoded := strings.TrimPrefix(header, "Basic ")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	user, pass, ok := strings.Cut(string(decoded), ":")
	return user, pass, ok
}
