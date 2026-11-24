package payments

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
)

// ChannelMetadata is the read-only capability view exposed to the console.
// It is derived from the adapter metadata; displayName is presentation data
// owned by the app layer, never a second capability table.
type ChannelMetadata struct {
	Channel          string `json:"channel"`
	DisplayName      string `json:"displayName"`
	SupportsCharge   bool   `json:"supportsCharge"`
	SupportsRefund   bool   `json:"supportsRefund"`
	SupportsQuery    bool   `json:"supportsQuery"`
	SupportsClose    bool   `json:"supportsClose"`
	SupportsCallback bool   `json:"supportsCallback"`
}

type ChannelCatalogProvider interface {
	ChannelMetadata() []domain.ChannelMetadata
}

type ChannelCatalogService interface {
	List(ctx context.Context) []ChannelMetadata
}

type channelCatalogService struct {
	provider ChannelCatalogProvider
}

func NewChannelCatalogService(provider ChannelCatalogProvider) ChannelCatalogService {
	return &channelCatalogService{provider: provider}
}

func (s *channelCatalogService) List(_ context.Context) []ChannelMetadata {
	metadata := s.provider.ChannelMetadata()
	out := make([]ChannelMetadata, 0, len(metadata))
	for _, m := range metadata {
		out = append(out, ChannelMetadata{
			Channel:          m.Channel,
			DisplayName:      channelDisplayName(m.Channel),
			SupportsCharge:   m.SupportsCharge,
			SupportsRefund:   m.SupportsRefund,
			SupportsQuery:    m.SupportsQuery,
			SupportsClose:    m.SupportsClose,
			SupportsCallback: m.SupportsCallback,
		})
	}
	return out
}

var channelDisplayNames = map[string]string{
	"alipay":       "Alipay 支付宝",
	"wechat_pay":   "WeChat Pay 微信支付",
	"stripe":       "Stripe",
	"paypal":       "PayPal",
	"adyen":        "Adyen",
	"airwallex":    "Airwallex",
	"square":       "Square",
	"checkout":     "Checkout.com",
	"paddle":       "Paddle",
	"fastspring":   "FastSpring",
	"lemonsqueezy": "Lemon Squeezy",
	"polar":        "Polar",
	"balance":      "Balance (USDC)",
	"gc":           "Google Pay",
	"dummy":        "Dummy / Sandbox",
}

func channelDisplayName(channel string) string {
	if name, ok := channelDisplayNames[channel]; ok {
		return name
	}
	return channel
}
