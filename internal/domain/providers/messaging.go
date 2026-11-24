package providers

import (
	"context"
	"errors"

	domainverifications "github.com/guliangiam/guliangiam/internal/domain/verifications"
)

// MessagingChannel is the delivery channel a provider kind serves. Call sites
// know the channel (email or phone), not the vendor, so a kind is selected from
// the organization's enabled rows for that channel.
type MessagingChannel string

const (
	MessagingChannelEmail MessagingChannel = "email"
	MessagingChannelSMS   MessagingChannel = "sms"
)

// ErrMessagingProviderUnconfigured reports that the organization has no
// provider row for the channel at all. It is deliberately distinct from a
// disabled row: only the former may fall back to the deployment-level sender.
var ErrMessagingProviderUnconfigured = errors.New("organization has no messaging provider for channel")

// MessagingKindRoute maps one identity kind to the channel it delivers on.
type MessagingKindRoute struct {
	Kind    Kind
	Channel MessagingChannel
}

// messagingKindRoutes is the runtime source for kind -> channel. Every kind
// listed here is also in contracts/registries/provider-kinds.yaml with
// subsystem=messaging; check-provider-kinds.py holds the two sets equal.
var messagingKindRoutes = []MessagingKindRoute{
	// Email
	{Kind: "smtp", Channel: MessagingChannelEmail},
	{Kind: "sendgrid", Channel: MessagingChannelEmail},
	{Kind: "resend", Channel: MessagingChannelEmail},
	{Kind: "azure_acs", Channel: MessagingChannelEmail},
	{Kind: "custom_http", Channel: MessagingChannelEmail},
	{Kind: "submail_email", Channel: MessagingChannelEmail},
	{Kind: "mailtrap_email", Channel: MessagingChannelEmail},
	{Kind: "directmail", Channel: MessagingChannelEmail},
	{Kind: "tencent_ses", Channel: MessagingChannelEmail},
	{Kind: "sendcloud", Channel: MessagingChannelEmail},
	{Kind: "mailgun", Channel: MessagingChannelEmail},
	{Kind: "amazon_ses", Channel: MessagingChannelEmail},
	{Kind: "postmark", Channel: MessagingChannelEmail},
	{Kind: "mailersend", Channel: MessagingChannelEmail},
	{Kind: "brevo", Channel: MessagingChannelEmail},
	{Kind: "zoho_mail", Channel: MessagingChannelEmail},
	// SMS
	{Kind: "twilio_sms", Channel: MessagingChannelSMS},
	{Kind: "twilio_verify", Channel: MessagingChannelSMS},
	{Kind: "alibaba_sms", Channel: MessagingChannelSMS},
	{Kind: "alibaba_pnvs", Channel: MessagingChannelSMS},
	{Kind: "tencent_sms", Channel: MessagingChannelSMS},
	{Kind: "volcengine_sms", Channel: MessagingChannelSMS},
	{Kind: "huawei_sms", Channel: MessagingChannelSMS},
	{Kind: "amazon_sns", Channel: MessagingChannelSMS},
	{Kind: "azure_acs_sms", Channel: MessagingChannelSMS},
	{Kind: "infobip", Channel: MessagingChannelSMS},
	{Kind: "ucloud_sms", Channel: MessagingChannelSMS},
	{Kind: "emay_sms", Channel: MessagingChannelSMS},
	{Kind: "baidu_sms", Channel: MessagingChannelSMS},
	{Kind: "submail", Channel: MessagingChannelSMS},
	{Kind: "mailtrap_sms", Channel: MessagingChannelSMS},
	{Kind: "oson_sms", Channel: MessagingChannelSMS},
	{Kind: "mock_sms", Channel: MessagingChannelSMS},
	{Kind: "smsbao", Channel: MessagingChannelSMS},
	{Kind: "msg91", Channel: MessagingChannelSMS},
	{Kind: "custom_http_sms", Channel: MessagingChannelSMS},
	{Kind: "unisms_sms", Channel: MessagingChannelSMS},
	{Kind: "yunpian_sms", Channel: MessagingChannelSMS},
	{Kind: "jdcloud_sms", Channel: MessagingChannelSMS},
	{Kind: "vonage_sms", Channel: MessagingChannelSMS},
	{Kind: "plivo_sms", Channel: MessagingChannelSMS},
	{Kind: "telnyx_sms", Channel: MessagingChannelSMS},
	{Kind: "sinch_sms", Channel: MessagingChannelSMS},
	{Kind: "bird_sms", Channel: MessagingChannelSMS},
	{Kind: "clickatell_sms", Channel: MessagingChannelSMS},
	{Kind: "africastalking_sms", Channel: MessagingChannelSMS},
}

var messagingKindChannelIndex = func() map[Kind]MessagingChannel {
	out := make(map[Kind]MessagingChannel, len(messagingKindRoutes))
	for _, route := range messagingKindRoutes {
		out[route.Kind] = route.Channel
	}
	return out
}()

// MessagingChannelForKind reports which channel a messaging kind serves.
func MessagingChannelForKind(kind Kind) (MessagingChannel, bool) {
	channel, ok := messagingKindChannelIndex[kind]
	return channel, ok
}

// IsMessagingKind reports whether the kind resolves through the messaging
// subsystem at all.
func IsMessagingKind(kind Kind) bool {
	_, ok := messagingKindChannelIndex[kind]
	return ok
}

// MessagingKindsForChannel returns the kinds the channel may resolve to, in
// declaration order. The Postgres lookup passes this list to bound the query.
func MessagingKindsForChannel(channel MessagingChannel) []Kind {
	out := make([]Kind, 0, len(messagingKindRoutes))
	for _, route := range messagingKindRoutes {
		if route.Channel == channel {
			out = append(out, route.Kind)
		}
	}
	return out
}

// MessagingKinds returns every messaging kind, in declaration order.
func MessagingKinds() []Kind {
	out := make([]Kind, 0, len(messagingKindRoutes))
	for _, route := range messagingKindRoutes {
		out = append(out, route.Kind)
	}
	return out
}

// MessagingKindStrings returns the 422 `allowed` list for kind-based callers.
func MessagingKindStrings() []string {
	kinds := MessagingKinds()
	out := make([]string, len(kinds))
	for i, kind := range kinds {
		out[i] = string(kind)
	}
	return out
}

// MessagingProviderConfig is the resolved, organization-scoped configuration a
// messaging sender needs. Values are keyed by the neutral names the builder
// understands; native provider columns and provider_configs JSON both feed it.
type MessagingProviderConfig struct {
	ProviderID     string
	OrganizationID string
	Kind           Kind
	Channel        MessagingChannel
	Values         map[string]string
}

// MessagingProviderLookup resolves the organization's enabled provider for a
// channel. Implementations must filter by organization_id, skip deleted rows,
// and report ErrMessagingProviderUnconfigured when the organization has no row
// for the channel (a disabled row is an unavailable error instead).
type MessagingProviderLookup interface {
	FindActiveMessagingProviderByChannel(ctx context.Context, organizationID string, channel MessagingChannel) (*MessagingProviderConfig, error)
}

// MessagingApplicationLookup resolves the provider bound to a specific
// application. An application binding wins over the organization default so a
// tenant can route one application's SMS/email through a dedicated account
// while the rest of the tenant keeps its own provider.
//
// Implementations must return ErrMessagingProviderUnconfigured when the
// application has no binding for the channel (callers then fall back to the
// organization default), and must filter by organization_id so a forged
// application id cannot borrow another tenant's provider.
type MessagingApplicationLookup interface {
	FindActiveMessagingProviderByApplication(ctx context.Context, organizationID, applicationID string, channel MessagingChannel) (*MessagingProviderConfig, error)
}

// MessagingSenderBuilder constructs the concrete sender for a resolved config.
// It lives in the domain as a port so app orchestration never imports infra/messaging.
type MessagingSenderBuilder interface {
	BuildSmsSender(ctx context.Context, config *MessagingProviderConfig) (domainverifications.SmsSender, error)
	BuildEmailSender(ctx context.Context, config *MessagingProviderConfig) (domainverifications.EmailSender, error)
}
