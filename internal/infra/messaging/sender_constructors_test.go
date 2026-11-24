package messaging

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAlibabaPNVSSender_PercentEncode(t *testing.T) {
	// Alibaba RPC percent encoding: space must be %20 (not '+'), '*' -> %2A, '~' stays.
	// https://help.aliyun.com/zh/sdk/product-overview/rpc-mechanism
	assert.Equal(t, "hello%20world", percentEncode("hello world"))
	assert.Equal(t, "test%26key%3Dvalue", percentEncode("test&key=value"))
	assert.Equal(t, "a%2Ab", percentEncode("a*b"))
	assert.Equal(t, "a~b", percentEncode("a~b"))
	assert.Equal(t, "", percentEncode(""))
}

func TestAlibabaPNVSSender_GenerateNonce(t *testing.T) {
	nonce1 := generateNonce()
	assert.Len(t, nonce1, 32)

	nonce2 := generateNonce()
	assert.Len(t, nonce2, 32)
	assert.NotEqual(t, nonce1, nonce2, "two nonces should differ")
}

func TestNewAlibabaPNVSSender(t *testing.T) {
	sender := NewAlibabaPNVSSender("access", "secret")
	assert.NotNil(t, sender)
	assert.Equal(t, "access", sender.accessKeyID)
	assert.Equal(t, "secret", sender.accessKeySecret)
}

func TestNewAlibabaSmsSender(t *testing.T) {
	sender := NewAlibabaSmsSender("access", "secret", "sign", "template")
	assert.NotNil(t, sender)
}

func TestNewSMTPSender(t *testing.T) {
	sender := NewSMTPSender("smtp.example.com", 587, "user", "pass", "from@example.com")
	assert.Equal(t, "smtp.example.com", sender.Host)
	assert.Equal(t, 587, sender.Port)
	assert.Equal(t, "user", sender.Username)
	assert.Equal(t, "from@example.com", sender.From)
}

func TestNewHuaweiSmsSender(t *testing.T) {
	sender := NewHuaweiSmsSender("https://smsapi.cn-north-4.myhuaweicloud.com:443", "key", "secret", "sign", "tmpl", "sender")
	assert.Equal(t, "key", sender.AppKey)
	assert.Equal(t, "secret", sender.AppSecret)
}

func TestBuildWSSEHeader(t *testing.T) {
	header := buildWSSEHeader("appkey123", "secret456")
	assert.Contains(t, header, "UsernameToken")
	assert.Contains(t, header, `Username="appkey123"`)
	// The app secret must never appear in the header; it is folded into the digest.
	assert.NotContains(t, header, "secret456")

	// PasswordDigest = Base64(SHA256(Nonce + Created + AppSecret)) per Huawei spec.
	re := regexp.MustCompile(`PasswordDigest="([^"]+)",Nonce="([^"]+)",Created="([^"]+)"`)
	m := re.FindStringSubmatch(header)
	assert.Len(t, m, 4)
	if len(m) == 4 {
		digest := sha256.Sum256([]byte(m[2] + m[3] + "secret456"))
		assert.Equal(t, base64.StdEncoding.EncodeToString(digest[:]), m[1])
	}
}

func TestNewTwilioSmsSender(t *testing.T) {
	sender := NewTwilioSmsSender("sid", "token", "+1234567890")
	assert.NotNil(t, sender)
}

func TestNewTencentSmsSender(t *testing.T) {
	sender := NewTencentSmsSender("secretId", "secretKey", "appId", "sign", "templateId", "")
	assert.NotNil(t, sender)
}

func TestNewVolcengineSmsSender(t *testing.T) {
	sender := NewVolcengineSmsSender("accessKey", "secretKey", "smsAccount", "sign", "templateId")
	assert.NotNil(t, sender)
}

func TestNewResendSender(t *testing.T) {
	sender := NewResendSender("apiKey", "from@example.com")
	assert.NotNil(t, sender)
}

func TestNewSendGridSender(t *testing.T) {
	sender := NewSendGridSender("apiKey", "from@example.com")
	assert.NotNil(t, sender)
}

func TestNewAzureACSSender(t *testing.T) {
	sender := NewAzureACSSender("https://endpoint.com", "apiKey", "from@example.com")
	assert.NotNil(t, sender)
}

func TestNewCustomHTTPSender(t *testing.T) {
	sender := NewCustomHTTPSender("http://example.com/webhook", "POST", map[string]string{"X-Auth": "token"})
	assert.NotNil(t, sender)
}

func TestJsonMarshal(t *testing.T) {
	data, err := jsonMarshal(map[string]string{"key": "value"})
	assert.NoError(t, err)
	assert.Contains(t, string(data), "key")
}

func TestJsonReader(t *testing.T) {
	reader := jsonReader([]byte(`{"key":"value"}`))
	assert.NotNil(t, reader)
}

func TestSignRequest(t *testing.T) {
	params := map[string]string{
		"Action":  "SendSms",
		"Version": "2017-05-25",
	}
	sig := signRequest(params, "mysecret")
	assert.NotEmpty(t, sig, "signature should not be empty")
}

func TestAlibabaPNVSSender_Send_RequiresTemplate(t *testing.T) {
	sender := NewAlibabaPNVSSender("access", "secret")
	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signName and templateCode are required")
}
