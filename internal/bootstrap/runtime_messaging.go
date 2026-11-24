package bootstrap

import (
	"context"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	appproviders "github.com/guliangiam/guliangiam/internal/app/providers"
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	infrapg "github.com/guliangiam/guliangiam/internal/infra/postgres"
	infrapgproviders "github.com/guliangiam/guliangiam/internal/infra/postgres/providers"

	"github.com/guliangiam/guliangiam/internal/infra/messaging"
	"github.com/guliangiam/guliangiam/internal/platform/config"
)

// registerMessagingSenders wires all email (SMTP/SendGrid/Resend/AzureACS) and SMS
// (Twilio/Alibaba/Tencent/Huawei/Volcengine) senders into a shared factory.
// When no production senders are configured (no env vars), falls back to mock
// senders so verification code flows work out of the box in development.
func registerMessagingSenders() (factory *messaging.SenderFactory, email, sms string) {
	senderFactory := messaging.NewSenderFactory()
	emailRegistered := false
	smsRegistered := false

	if v := config.SMTPHost(); v != "" {
		senderFactory.RegisterEmailSender("smtp", messaging.NewSMTPSender(v, config.SMTPPort(), config.SMTPUsername(), config.SMTPPassword(), config.SMTPFrom()))
		emailRegistered = true
	}
	if v := config.SendGridAPIKey(); v != "" {
		senderFactory.RegisterEmailSender("sendgrid", messaging.NewSendGridSender(v, config.SendGridFrom()))
		emailRegistered = true
	}
	if v := config.ResendAPIKey(); v != "" {
		senderFactory.RegisterEmailSender("resend", messaging.NewResendSender(v, config.ResendFrom()))
		emailRegistered = true
	}
	if v := envOrEmpty("AZURE_ACS_ENDPOINT"); v != "" {
		senderFactory.RegisterEmailSender("azureacs", messaging.NewAzureACSSender(v, envOrEmpty("AZURE_ACS_API_KEY"), envOrEmpty("AZURE_ACS_FROM")))
		emailRegistered = true
	}
	if v := envOrEmpty("TWILIO_ACCOUNT_SID"); v != "" {
		senderFactory.RegisterSmsSender("twilio", messaging.NewTwilioSmsSender(v, envOrEmpty("TWILIO_AUTH_TOKEN"), envOrEmpty("TWILIO_FROM_NUMBER")))
		smsRegistered = true
	}
	if v := envOrEmpty("ALIBABA_SMS_ACCESS_KEY_ID"); v != "" {
		senderFactory.RegisterSmsSender("alibaba", messaging.NewAlibabaSmsSender(v, envOrEmpty("ALIBABA_SMS_ACCESS_KEY_SECRET"), envOrEmpty("ALIBABA_SMS_SIGN_NAME"), envOrEmpty("ALIBABA_SMS_TEMPLATE_CODE")))
		smsRegistered = true
	}
	if v := envOrEmpty("TENCENT_SMS_SECRET_ID"); v != "" {
		senderFactory.RegisterSmsSender("tencent", messaging.NewTencentSmsSender(v, envOrEmpty("TENCENT_SMS_SECRET_KEY"), envOrEmpty("TENCENT_SMS_SDK_APP_ID"), envOrEmpty("TENCENT_SMS_SIGN_NAME"), envOrEmpty("TENCENT_SMS_TEMPLATE_ID"), ""))
		smsRegistered = true
	}
	if v := envOrEmpty("HUAWEI_SMS_APP_KEY"); v != "" {
		senderFactory.RegisterSmsSender("huawei", messaging.NewHuaweiSmsSender(envOrEmpty("HUAWEI_SMS_ENDPOINT"), v, envOrEmpty("HUAWEI_SMS_APP_SECRET"), envOrEmpty("HUAWEI_SMS_SIGNATURE"), envOrEmpty("HUAWEI_SMS_TEMPLATE_ID"), envOrEmpty("HUAWEI_SMS_SENDER")))
		smsRegistered = true
	}
	if v := envOrEmpty("VOLCENGINE_SMS_ACCESS_KEY_ID"); v != "" {
		senderFactory.RegisterSmsSender("volcengine", messaging.NewVolcengineSmsSender(v, envOrEmpty("VOLCENGINE_SMS_ACCESS_KEY_SECRET"), envOrEmpty("VOLCENGINE_SMS_ACCOUNT"), envOrEmpty("VOLCENGINE_SMS_SIGN_NAME"), envOrEmpty("VOLCENGINE_SMS_TEMPLATE_ID")))
		smsRegistered = true
	}

	// Dev fallback: register mock senders when no env-based senders configured,
	// so verification code flows (email/phone reset, MFA) work in local dev.
	// Production must never fall back to a mock: a verification code that is only
	// logged to stdout looks "sent" to the caller while never reaching the user,
	// which silently breaks every login/reset/MFA loop. Without a configured
	// provider the resolver then returns unavailable instead.
	if !config.IsProduction() {
		if !emailRegistered {
			senderFactory.RegisterEmailSender("mock", messaging.NewMockEmailSender())
			slog.Warn("No email provider env vars configured — using mock email sender (codes logged to stdout)")
		}
		if !smsRegistered {
			senderFactory.RegisterSmsSender("mock", messaging.NewMockSmsSender())
			slog.Warn("No SMS provider env vars configured — using mock SMS sender (codes logged to stdout)")
		}
	}

	emailProvider := normalizeEmailProviderName(envOrEmpty("EMAIL_PROVIDER"))
	if emailProvider == "" {
		if emailRegistered {
			emailProvider = "smtp"
		} else if !config.IsProduction() {
			emailProvider = "mock"
		}
	}
	smsProvider := envOrEmpty("SMS_PROVIDER")
	if smsProvider == "" {
		if smsRegistered {
			smsProvider = "twilio"
		} else if !config.IsProduction() {
			smsProvider = "mock"
		}
	}

	return senderFactory, emailProvider, smsProvider
}

// normalizeEmailProviderName aligns the env spelling (the registry documents
// `azure-acs`) with the name the factory registers (`azureacs`).
func normalizeEmailProviderName(value string) string {
	name := strings.ToLower(strings.TrimSpace(value))
	switch name {
	case "azure-acs", "azure_acs":
		return "azureacs"
	default:
		return name
	}
}

// newMessagingResolver builds the channel resolver. PostgreSQL reads the
// organization's provider rows; MySQL has no messaging repository yet
// (OPEN-API-37), so it reports "unconfigured" and keeps the deployment fallback.
func newMessagingResolver(driver string, pool *pgxpool.Pool, senderFactory *messaging.SenderFactory, emailProvider, smsProvider string) (appproviders.MessagingResolver, *appproviders.MessagingProviderCache) {
	resolver := appproviders.MessagingResolver{
		Builder:               messaging.ProviderSenderBuilder{DefaultFrom: defaultEmailFrom(emailProvider)},
		Fallback:              senderFactory,
		FallbackEmailProvider: emailProvider,
		FallbackSmsProvider:   smsProvider,
	}
	if driver == driverMySQL || pool == nil {
		resolver.Lookup = unconfiguredMessagingLookup{}
		return resolver, nil
	}
	cache := appproviders.NewMessagingProviderCache(infrapgproviders.NewRepository(infrapg.NewInstrumentedDBTX(pool)))
	resolver.Lookup = cache
	// Application-level bindings are read straight from the repository: they
	// change at human speed, so a per-request cache is unnecessary, and the
	// org-level cache must not be keyed by application.
	resolver.ApplicationLookup = infrapgproviders.NewRepository(infrapg.NewInstrumentedDBTX(pool))
	return resolver, cache
}

// defaultEmailFrom supplies the From address for provider rows created from the
// console, which does not expose a From field yet.
func defaultEmailFrom(emailProvider string) string {
	switch emailProvider {
	case "sendgrid":
		return config.SendGridFrom()
	case "resend":
		return config.ResendFrom()
	case "azureacs":
		return envOrEmpty("AZURE_ACS_FROM")
	default:
		return config.SMTPFrom()
	}
}

// unconfiguredMessagingLookup keeps the deployment fallback available on
// drivers that cannot read organization provider rows.
type unconfiguredMessagingLookup struct{}

func (unconfiguredMessagingLookup) FindActiveMessagingProviderByChannel(context.Context, string, domain.MessagingChannel) (*domain.MessagingProviderConfig, error) {
	return nil, domain.ErrMessagingProviderUnconfigured
}
