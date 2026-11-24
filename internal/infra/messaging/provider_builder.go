package messaging

import (
	"context"
	"encoding/json"
	"strconv"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	domainverifications "github.com/guliangiam/guliangiam/internal/domain/verifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

var (
	_ domain.MessagingSenderBuilder       = ProviderSenderBuilder{}
	_ domainverifications.SmsCodeVerifier = (*TwilioVerifySender)(nil)
)

// ProviderSenderBuilder constructs the concrete sender from a resolved
// provider config. It is the infra half of the messaging resolution loop.
type ProviderSenderBuilder struct {
	// DefaultFrom is the deployment-level From address used when the provider
	// row has no sender value. The email form does not expose a From field yet,
	// so this keeps SMTP/SendGrid/Resend rows usable without inventing a column.
	DefaultFrom string
}

func (b ProviderSenderBuilder) BuildSmsSender(_ context.Context, cfg *domain.MessagingProviderConfig) (domainverifications.SmsSender, error) {
	if cfg == nil {
		return nil, apperrors.NewInvalidArgument("messaging provider config is required")
	}

	switch string(cfg.Kind) {
	case "twilio_sms":
		return NewTwilioSmsSender(
			value(cfg, "accountSid", "admin"),
			value(cfg, "authToken", "password"),
			value(cfg, "from", "sender"),
		), nil
	case "alibaba_sms":
		return NewAlibabaSmsSender(
			value(cfg, "accessKeyId", "admin"),
			value(cfg, "accessKeySecret", "password"),
			value(cfg, "signName", "sender"),
			value(cfg, "templateCode", "template"),
		), nil
	case "alibaba_pnvs":
		return NewAlibabaPNVSSenderWithTemplate(
			value(cfg, "accessKeyId", "admin"),
			value(cfg, "accessKeySecret", "password"),
			value(cfg, "signName", "sender"),
			value(cfg, "templateCode", "template"),
		), nil
	case "tencent_sms":
		return NewTencentSmsSender(
			value(cfg, "secretId", "admin"),
			value(cfg, "secretKey", "password"),
			value(cfg, "sdkAppId", "appId"),
			value(cfg, "signName", "sender"),
			value(cfg, "templateId", "template"),
			value(cfg, "region"),
		), nil
	case "volcengine_sms":
		return NewVolcengineSmsSender(
			value(cfg, "accessKeyId", "admin"),
			value(cfg, "accessKeySecret", "password"),
			value(cfg, "account", "appId"),
			value(cfg, "signName", "sender"),
			value(cfg, "templateId", "template"),
		), nil
	case "huawei_sms":
		return NewHuaweiSmsSender(
			value(cfg, "endpoint", "host", "customApi"),
			value(cfg, "appKey", "admin"),
			value(cfg, "appSecret", "password"),
			value(cfg, "signature", "sender"),
			value(cfg, "templateId", "template"),
			value(cfg, "senderId", "sender"),
		), nil
	case "amazon_sns":
		return NewAmazonSNSSender(
			value(cfg, "accessKeyId", "admin"),
			value(cfg, "secretKey", "password"),
			value(cfg, "region"),
			value(cfg, "senderId", "sender"),
		), nil
	case "azure_acs_sms":
		return NewAzureACSsmsSender(
			value(cfg, "connectionString", "password"),
			value(cfg, "fromNumber", "sender"),
		), nil
	case "infobip":
		return NewInfobipSmsSender(
			value(cfg, "baseURL", "customApi", "host"),
			value(cfg, "apiKey", "password"),
			value(cfg, "senderName", "sender"),
		), nil
	case "ucloud_sms":
		return NewUCloudSmsSender(
			value(cfg, "publicKey", "admin"),
			value(cfg, "privateKey", "password"),
			value(cfg, "projectId", "appId"),
			value(cfg, "signName", "sender"),
			value(cfg, "templateId", "template"),
		), nil
	case "emay_sms":
		return NewEmaySmsSender(
			value(cfg, "apiURL", "customApi", "host"),
			value(cfg, "serialNumber", "admin"),
			value(cfg, "password"),
			value(cfg, "signName", "sender"),
		), nil
	case "baidu_sms":
		return NewBaiduSmsSender(
			value(cfg, "accessKey", "admin"),
			value(cfg, "secretKey", "password"),
			value(cfg, "signId", "sender"),
			value(cfg, "templateId", "template"),
			value(cfg, "invokeId", "customHeader"),
		), nil
	case "submail":
		return NewSubmailSmsSender(
			value(cfg, "appId"),
			value(cfg, "appKey", "password"),
			value(cfg, "signName", "sender"),
			value(cfg, "projectId", "template"),
		), nil
	case "mailtrap_sms":
		return NewMailtrapSmsSender(
			value(cfg, "apiKey", "password"),
			value(cfg, "fromNumber", "sender"),
		), nil
	case "oson_sms":
		return NewOsonSmsSender(
			value(cfg, "apiURL", "customApi", "host"),
			value(cfg, "apiKey", "password"),
			value(cfg, "senderName", "sender"),
		), nil
	case "mock_sms":
		return NewMockSmsSender(), nil
	case "twilio_verify":
		return NewTwilioVerifySender(
			value(cfg, "accountSid", "admin"),
			value(cfg, "authToken", "password"),
			value(cfg, "serviceSid", "appId"),
		), nil
	case "smsbao":
		return NewSmsBaoSender(
			value(cfg, "username", "admin"),
			value(cfg, "apiKey", "password"),
			value(cfg, "signName", "sender"),
			value(cfg, "template"),
			value(cfg, "goodsId", "customHeader"),
		), nil
	case "msg91":
		return NewMsg91Sender(
			value(cfg, "authKey", "password"),
			value(cfg, "senderId", "sender"),
			value(cfg, "templateId", "template"),
		), nil
	case "unisms_sms":
		return NewUnismsSmsSender(
			value(cfg, "mchId", "admin"),
			value(cfg, "appId"),
			value(cfg, "appKey", "password"),
			value(cfg, "templateId", "template"),
		), nil
	case "yunpian_sms":
		return NewYunpianSmsSender(
			value(cfg, "apiKey", "password"),
			value(cfg, "signName", "sender"),
			value(cfg, "template"),
			value(cfg, "baseURL", "customApi", "host"),
		), nil
	case "jdcloud_sms":
		return NewJDCloudSmsSender(
			value(cfg, "accessKey", "admin"),
			value(cfg, "secretKey", "password"),
			value(cfg, "signId", "sender"),
			value(cfg, "templateId", "template"),
			value(cfg, "region"),
		), nil
	case "vonage_sms":
		return NewVonageSmsSender(
			value(cfg, "apiKey", "admin"),
			value(cfg, "apiSecret", "password"),
			value(cfg, "from", "sender"),
		), nil
	case "plivo_sms":
		return NewPlivoSmsSender(
			value(cfg, "authId", "admin"),
			value(cfg, "authToken", "password"),
			value(cfg, "src", "sender"),
		), nil
	case "telnyx_sms":
		return NewTelnyxSmsSender(
			value(cfg, "apiKey", "password"),
			value(cfg, "from", "sender"),
		), nil
	case "sinch_sms":
		return NewSinchSmsSender(
			value(cfg, "keyId", "admin"),
			value(cfg, "keySecret", "password"),
			value(cfg, "projectId", "appId"),
			value(cfg, "from", "sender"),
			value(cfg, "baseURL", "customApi", "host"),
		), nil
	case "bird_sms":
		return NewBirdSmsSender(
			value(cfg, "accessKey", "password"),
			value(cfg, "origin", "sender"),
		), nil
	case "clickatell_sms":
		return NewClickatellSmsSender(
			value(cfg, "apiKey", "password"),
			value(cfg, "from", "sender"),
		), nil
	case "africastalking_sms":
		return NewAfricastalkingSmsSender(
			value(cfg, "username", "admin"),
			value(cfg, "apiKey", "password"),
			value(cfg, "from", "sender"),
		), nil
	case "custom_http_sms":
		endpoint := value(cfg, "endpoint", "customApi", "host")
		if endpoint == "" {
			return nil, apperrors.NewInvalidArgument("custom http sms endpoint is required")
		}
		headers, err := parseHeaders(value(cfg, "headers", "customHeader"))
		if err != nil {
			return nil, err
		}
		return NewCustomHttpSmsSender(endpoint, value(cfg, "method"), headers, value(cfg, "bodyTemplate", "customBody")), nil
	default:
		return nil, apperrors.NewInvalidArgumentf("unsupported sms provider kind: %s", cfg.Kind)
	}
}

func (b ProviderSenderBuilder) BuildEmailSender(ctx context.Context, cfg *domain.MessagingProviderConfig) (domainverifications.EmailSender, error) {
	if cfg == nil {
		return nil, apperrors.NewInvalidArgument("messaging provider config is required")
	}

	switch string(cfg.Kind) {
	case "smtp":
		host := value(cfg, "host")
		if host == "" {
			return nil, apperrors.NewInvalidArgument("smtp host is required")
		}
		port, err := smtpPort(value(cfg, "port"))
		if err != nil {
			return nil, err
		}
		return NewSMTPSender(
			host,
			port,
			value(cfg, "admin", "username"),
			value(cfg, "password"),
			b.emailFrom(cfg),
		), nil
	case "sendgrid":
		apiKey := value(cfg, "apiKey", "password")
		if apiKey == "" {
			return nil, apperrors.NewInvalidArgument("sendgrid apiKey is required")
		}
		return NewSendGridSender(apiKey, b.emailFrom(cfg)), nil
	case "resend":
		apiKey := value(cfg, "apiKey", "password")
		if apiKey == "" {
			return nil, apperrors.NewInvalidArgument("resend apiKey is required")
		}
		return NewResendSender(apiKey, b.emailFrom(cfg)), nil
	case "azure_acs":
		endpoint := value(cfg, "endpoint", "host", "customApi")
		if endpoint == "" {
			return nil, apperrors.NewInvalidArgument("azure acs endpoint is required")
		}
		apiKey := value(cfg, "apiKey", "password")
		if apiKey == "" {
			return nil, apperrors.NewInvalidArgument("azure acs apiKey is required")
		}
		return NewAzureACSSender(endpoint, apiKey, b.emailFrom(cfg)), nil
	case "custom_http":
		endpoint := value(cfg, "endpoint", "customApi", "host")
		if endpoint == "" {
			return nil, apperrors.NewInvalidArgument("custom_http endpoint is required")
		}
		headers, err := parseHeaders(value(cfg, "headers", "customHeader"))
		if err != nil {
			return nil, err
		}
		return NewCustomHTTPSender(endpoint, value(cfg, "method"), headers), nil
	case "submail_email":
		appID := value(cfg, "appId")
		if appID == "" {
			return nil, apperrors.NewInvalidArgument("submail email appId is required")
		}
		appKey := value(cfg, "appKey", "password")
		if appKey == "" {
			return nil, apperrors.NewInvalidArgument("submail email appKey is required")
		}
		return NewSubmailEmailSender(appID, appKey, b.emailFrom(cfg)), nil
	case "mailtrap_email":
		apiToken := value(cfg, "apiKey", "password")
		if apiToken == "" {
			return nil, apperrors.NewInvalidArgument("mailtrap email apiToken is required")
		}
		return NewMailtrapEmailSender(apiToken, b.emailFrom(cfg)), nil
	case "directmail":
		accessKeyID := value(cfg, "accessKeyId", "admin")
		accessKeySecret := value(cfg, "accessKeySecret", "password")
		if accessKeyID == "" || accessKeySecret == "" {
			return nil, apperrors.NewInvalidArgument("directmail accessKeyId/accessKeySecret are required")
		}
		return NewDirectMailEmailSender(accessKeyID, accessKeySecret, b.emailFrom(cfg), value(cfg, "region"), value(cfg, "endpoint", "host", "customApi")), nil
	case "tencent_ses":
		secretID := value(cfg, "secretId", "admin")
		secretKey := value(cfg, "secretKey", "password")
		if secretID == "" || secretKey == "" {
			return nil, apperrors.NewInvalidArgument("tencent ses SecretId/SecretKey are required")
		}
		return NewTencentSESEmailSender(secretID, secretKey, b.emailFrom(cfg), value(cfg, "region"), value(cfg, "endpoint", "host", "customApi")), nil
	case "sendcloud":
		apiUser := value(cfg, "apiUser", "admin")
		apiKey := value(cfg, "apiKey", "password")
		if apiUser == "" || apiKey == "" {
			return nil, apperrors.NewInvalidArgument("sendcloud apiUser/apiKey are required")
		}
		return NewSendCloudEmailSender(apiUser, apiKey, b.emailFrom(cfg), "", value(cfg, "endpoint", "host", "customApi")), nil
	case "mailgun":
		apiKey := value(cfg, "apiKey", "password")
		if apiKey == "" {
			return nil, apperrors.NewInvalidArgument("mailgun apiKey is required")
		}
		sendingDomain := value(cfg, "domain", "host")
		if sendingDomain == "" {
			return nil, apperrors.NewInvalidArgument("mailgun sending domain is required")
		}
		return NewMailgunEmailSender(apiKey, sendingDomain, b.emailFrom(cfg), value(cfg, "endpoint", "customApi")), nil
	case "amazon_ses":
		accessKeyID := value(cfg, "accessKeyId", "admin")
		secretAccessKey := value(cfg, "secretAccessKey", "password")
		if accessKeyID == "" || secretAccessKey == "" {
			return nil, apperrors.NewInvalidArgument("amazon ses accessKeyId/secretAccessKey are required")
		}
		region := value(cfg, "region")
		if region == "" {
			return nil, apperrors.NewInvalidArgument("amazon ses region is required")
		}
		return NewAmazonSESEmailSender(ctx, accessKeyID, secretAccessKey, region, b.emailFrom(cfg), value(cfg, "endpoint", "customApi"))
	case "postmark":
		serverToken := value(cfg, "serverToken", "password")
		if serverToken == "" {
			return nil, apperrors.NewInvalidArgument("postmark serverToken is required")
		}
		return NewPostmarkEmailSender(serverToken, b.emailFrom(cfg), value(cfg, "endpoint", "customApi")), nil
	case "mailersend":
		apiToken := value(cfg, "apiToken", "password")
		if apiToken == "" {
			return nil, apperrors.NewInvalidArgument("mailersend apiToken is required")
		}
		return NewMailerSendEmailSender(apiToken, b.emailFrom(cfg), "", value(cfg, "endpoint", "customApi")), nil
	case "brevo":
		apiKey := value(cfg, "apiKey", "password")
		if apiKey == "" {
			return nil, apperrors.NewInvalidArgument("brevo apiKey is required")
		}
		return NewBrevoEmailSender(apiKey, b.emailFrom(cfg), value(cfg, "senderName")), nil
	case "zoho_mail":
		accessToken := value(cfg, "accessToken", "password")
		if accessToken == "" {
			return nil, apperrors.NewInvalidArgument("zoho mail accessToken is required")
		}
		accountID := value(cfg, "accountId", "appId")
		if accountID == "" {
			return nil, apperrors.NewInvalidArgument("zoho mail accountId is required")
		}
		return NewZohoMailEmailSender(accessToken, accountID, b.emailFrom(cfg), value(cfg, "endpoint", "host", "customApi")), nil
	default:
		return nil, apperrors.NewInvalidArgumentf("unsupported email provider kind: %s", cfg.Kind)
	}
}

// emailFrom prefers the provider row's sender column and falls back to the
// deployment default. A blank From is left to the send call to reject rather
// than turning provider resolution into a configuration error.
func (b ProviderSenderBuilder) emailFrom(cfg *domain.MessagingProviderConfig) string {
	if from := value(cfg, "from", "sender"); from != "" {
		return from
	}
	return b.DefaultFrom
}

func smtpPort(raw string) (int, error) {
	if raw == "" {
		return 587, nil
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port <= 0 || port > 65535 {
		return 0, apperrors.NewInvalidArgumentf("invalid smtp port: %s", raw)
	}
	return port, nil
}

func value(cfg *domain.MessagingProviderConfig, keys ...string) string {
	for _, key := range keys {
		if v := cfg.Values[key]; v != "" {
			return v
		}
	}
	return ""
}

func parseHeaders(raw string) (map[string]string, error) {
	if raw == "" {
		return map[string]string{}, nil
	}
	headers := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &headers); err != nil {
		return nil, apperrors.WrapInvalidArgumentf(err, "custom_http headers must be a JSON object")
	}
	return headers, nil
}
