package messaging

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSenderFactory_NewSenderFactory(t *testing.T) {
	factory := NewSenderFactory()
	assert.NotNil(t, factory)
	assert.NotNil(t, factory.emailSenders)
	assert.NotNil(t, factory.smsSenders)
}

func TestSenderFactory_RegisterSmsSender_CaseInsensitive(t *testing.T) {
	factory := NewSenderFactory()
	sender := &mockSmsSender{}
	factory.RegisterSmsSender("TWILIO", sender)

	got, err := factory.GetSmsSender("twilio")
	require.NoError(t, err)
	assert.Equal(t, sender, got)
}

func TestSenderFactory_MultipleEmailSenders(t *testing.T) {
	factory := NewSenderFactory()
	sender1 := &mockEmailSender{}
	sender2 := &mockEmailSender{}

	factory.RegisterEmailSender("smtp", sender1)
	factory.RegisterEmailSender("resend", sender2)

	got1, err := factory.GetEmailSender("smtp")
	require.NoError(t, err)
	assert.Equal(t, sender1, got1)

	got2, err := factory.GetEmailSender("resend")
	require.NoError(t, err)
	assert.Equal(t, sender2, got2)
}

func TestSenderFactory_OverwriteSender(t *testing.T) {
	factory := NewSenderFactory()
	sender1 := &mockEmailSender{}
	sender2 := &mockEmailSender{}

	factory.RegisterEmailSender("smtp", sender1)
	factory.RegisterEmailSender("smtp", sender2)

	got, err := factory.GetEmailSender("smtp")
	require.NoError(t, err)
	assert.Equal(t, sender2, got)
}

func TestSenderFactory_SmsCaseInsensitiveLookup(t *testing.T) {
	factory := NewSenderFactory()
	sender := &mockSmsSender{}
	factory.RegisterSmsSender("Huawei", sender)

	got, err := factory.GetSmsSender("huawei")
	require.NoError(t, err)
	assert.Equal(t, sender, got)

	got, err = factory.GetSmsSender("HUAWEI")
	require.NoError(t, err)
	assert.Equal(t, sender, got)
}

func TestMockSmsSender_Send_WithContext(t *testing.T) {
	sender := NewMockSmsSender()
	err := sender.Send(context.Background(), "+8613800138000", "654321")
	require.NoError(t, err)
}

func TestNewAmazonSNSSender(t *testing.T) {
	sender := NewAmazonSNSSender("access", "secret", "us-east-1", "sender")
	assert.Equal(t, "access", sender.AccessKeyID)
	assert.Equal(t, "secret", sender.SecretKey)
	assert.Equal(t, "us-east-1", sender.Region)
	assert.Equal(t, "sender", sender.SenderID)
}

func TestNewAzureACSsmsSender(t *testing.T) {
	sender := NewAzureACSsmsSender("conn-string", "+1234567890")
	assert.Equal(t, "conn-string", sender.ConnectionString)
	assert.Equal(t, "+1234567890", sender.FromNumber)
	assert.NotNil(t, sender.client)
}

func TestNewInfobipSmsSender(t *testing.T) {
	sender := NewInfobipSmsSender("https://api.infobip.com", "api-key", "SenderName")
	assert.Equal(t, "https://api.infobip.com", sender.BaseURL)
	assert.Equal(t, "api-key", sender.APIKey)
	assert.Equal(t, "SenderName", sender.SenderName)
}

func TestNewUCloudSmsSender(t *testing.T) {
	sender := NewUCloudSmsSender("pub", "priv", "proj", "sign", "tmpl")
	assert.Equal(t, "pub", sender.PublicKey)
	assert.Equal(t, "priv", sender.PrivateKey)
	assert.Equal(t, "proj", sender.ProjectID)
}

func TestNewEmaySmsSender(t *testing.T) {
	sender := NewEmaySmsSender("http://api.emay.com", "serial", "pass", "sign")
	assert.Equal(t, "http://api.emay.com", sender.APIURL)
	assert.Equal(t, "serial", sender.SerialNumber)
}

func TestNewOsonSmsSender(t *testing.T) {
	sender := NewOsonSmsSender("http://api.oson.com", "key", "sender")
	assert.Equal(t, "http://api.oson.com", sender.APIURL)
	assert.Equal(t, "key", sender.APIKey)
}

func TestNewBaiduSmsSender(t *testing.T) {
	sender := NewBaiduSmsSender("ak", "sk", "sign", "tmpl", "invoke")
	assert.Equal(t, "ak", sender.AccessKey)
	assert.Equal(t, "sk", sender.SecretKey)
}

func TestNewSubmailSmsSender(t *testing.T) {
	sender := NewSubmailSmsSender("app", "key", "sign", "proj")
	assert.Equal(t, "app", sender.AppID)
	assert.Equal(t, "key", sender.AppKey)
}

func TestNewMailtrapSmsSender(t *testing.T) {
	sender := NewMailtrapSmsSender("key", "+1234567890")
	assert.Equal(t, "key", sender.APIKey)
	assert.Equal(t, "+1234567890", sender.FromNumber)
}

func TestNewCustomHttpSmsSender(t *testing.T) {
	headers := map[string]string{"X-Auth": "token"}
	sender := NewCustomHttpSmsSender("http://api.example.com", "POST", headers, `{"phone":"{{phone}}"}`)
	assert.Equal(t, "http://api.example.com", sender.APIURL)
	assert.Equal(t, "POST", sender.HTTPMethod)
	assert.Equal(t, "token", sender.Headers["X-Auth"])
}

func TestAzureACSsmsSender_Send_Error(t *testing.T) {
	sender := NewAzureACSsmsSender("conn-string", "+1234567890")
	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.Error(t, err)
}
