package messaging

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderSenderBuilder_BuildsEveryMessagingKind(t *testing.T) {
	builder := ProviderSenderBuilder{}
	kinds := []domain.Kind{
		"twilio_sms", "twilio_verify", "alibaba_sms", "alibaba_pnvs",
		"tencent_sms", "volcengine_sms", "huawei_sms",
		"amazon_sns", "azure_acs_sms", "infobip", "ucloud_sms", "emay_sms",
		"baidu_sms", "submail", "mailtrap_sms", "oson_sms", "mock_sms",
		"smsbao", "msg91", "custom_http_sms",
		"unisms_sms", "yunpian_sms", "jdcloud_sms", "vonage_sms",
		"plivo_sms", "telnyx_sms", "sinch_sms", "bird_sms",
		"clickatell_sms", "africastalking_sms",
	}
	for _, kind := range kinds {
		cfg := &domain.MessagingProviderConfig{
			Kind: kind,
			Values: map[string]string{
				"admin": "admin", "password": "secret", "sender": "sender",
				"appId": "app", "appSecret": "app-secret", "region": "cn",
				"template": "template", "templateId": "template-id",
				"templateCode": "template-code", "signName": "sign",
				"serviceSid": "service", "authKey": "auth", "authToken": "token",
				"username": "user", "goodsId": "goods",
				"host":         "https://api.example.com",
				"customApi":    "https://api.example.com",
				"customHeader": `{"X-Key":"v"}`,
			},
		}
		sender, err := builder.BuildSmsSender(context.Background(), cfg)
		require.NoError(t, err, "kind %s", kind)
		assert.NotNil(t, sender, "kind %s", kind)
	}

	for _, kind := range []domain.Kind{
		"smtp", "sendgrid", "resend", "azure_acs", "custom_http", "submail_email", "mailtrap_email",
		"directmail", "tencent_ses", "sendcloud", "mailgun", "amazon_ses", "postmark", "mailersend",
		"brevo", "zoho_mail",
	} {
		cfg := &domain.MessagingProviderConfig{
			Kind: kind,
			Values: map[string]string{
				"host": "smtp.example.com", "port": "587",
				"admin": "user@example.com", "password": "secret",
				"appId": "app", "appKey": "key", "apiKey": "key",
				"sender":   "from@example.com",
				"region":   "cn-north-1",
				"endpoint": "https://api.example.com", "method": "POST",
				"customHeader": `{"X-Key":"v"}`,
			},
		}
		sender, err := builder.BuildEmailSender(context.Background(), cfg)
		require.NoError(t, err, "kind %s", kind)
		assert.NotNil(t, sender, "kind %s", kind)
	}
}

func TestProviderSenderBuilder_UsesDefaultFromAndRejectsIncompleteEmailRows(t *testing.T) {
	builder := ProviderSenderBuilder{DefaultFrom: "default@example.com"}

	sender, err := builder.BuildEmailSender(context.Background(), &domain.MessagingProviderConfig{
		Kind:   "smtp",
		Values: map[string]string{"host": "smtp.example.com"},
	})
	require.NoError(t, err)
	smtpSender, ok := sender.(*SMTPSender)
	require.True(t, ok)
	assert.Equal(t, 587, smtpSender.Port)
	assert.Equal(t, "default@example.com", smtpSender.From)

	for _, cfg := range []*domain.MessagingProviderConfig{
		{Kind: "smtp"},
		{Kind: "smtp", Values: map[string]string{"host": "smtp.example.com", "port": "not-a-port"}},
		{Kind: "sendgrid", Values: map[string]string{"sender": "from@example.com"}},
		{Kind: "resend"},
		{Kind: "azure_acs", Values: map[string]string{"endpoint": "https://acs.example.com"}},
		{Kind: "custom_http", Values: map[string]string{"customHeader": `{"X-Key":"v"}`}},
		{Kind: "submail_email", Values: map[string]string{"appKey": "key"}},
		{Kind: "mailtrap_email"},
		{Kind: "directmail", Values: map[string]string{"admin": "ak"}},
		{Kind: "tencent_ses", Values: map[string]string{"password": "sk"}},
		{Kind: "sendcloud", Values: map[string]string{"admin": "user"}},
		{Kind: "mailgun", Values: map[string]string{"host": "example.com"}},
		{Kind: "amazon_ses", Values: map[string]string{"admin": "ak", "password": "sk"}},
		{Kind: "postmark"},
		{Kind: "mailersend"},
		{Kind: "brevo"},
		{Kind: "zoho_mail", Values: map[string]string{"password": "token"}},
	} {
		_, err := builder.BuildEmailSender(context.Background(), cfg)
		require.Error(t, err, "kind %s", cfg.Kind)
	}
}

func TestProviderSenderBuilder_RejectsUnknownKind(t *testing.T) {
	_, err := (ProviderSenderBuilder{}).BuildSmsSender(context.Background(), &domain.MessagingProviderConfig{Kind: "github"})
	require.Error(t, err)
}

func TestProviderSenderBuilder_RejectsInvalidCustomHeaders(t *testing.T) {
	_, err := (ProviderSenderBuilder{}).BuildEmailSender(context.Background(), &domain.MessagingProviderConfig{Kind: "custom_http", Values: map[string]string{"customHeader": "not-json"}})
	require.Error(t, err)
}
