package payment

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
)

// channelNames is the canonical channel registry. It feeds bootstrap
// registration, the read-only capability API and the adapter metadata, so the
// channel set cannot drift between those three surfaces.
var channelNames = []string{
	channelAlipay, channelWeChatPay, channelStripe, channelPayPal, channelAdyen,
	channelAirwallex, channelSquare, channelCheckout, channelPaddle, channelFastSpring,
	channelLemonSqueezy, channelPolar, channelBalance, channelGC, channelDummy,
}

// ChannelNames returns a copy of the canonical channel keys.
func ChannelNames() []string {
	return append([]string(nil), channelNames...)
}

// ChannelCatalog adapts the package-level matrix to the app layer port.
type ChannelCatalog struct{}

func (ChannelCatalog) ChannelMetadata() []domain.ChannelMetadata {
	return ChannelMetadata()
}

// ChannelMetadata returns the code-level capability matrix for every channel.
// It is independent of runtime credentials: a channel that is not configured
// still advertises what its adapter can do.
func ChannelMetadata() []domain.ChannelMetadata {
	out := make([]domain.ChannelMetadata, 0, len(channelNames))
	for _, name := range channelNames {
		out = append(out, metadataFor(name))
	}
	return out
}

func metadataFor(name string) domain.ChannelMetadata {
	return domain.ChannelMetadata{
		Channel:          name,
		SupportsCharge:   true,
		SupportsRefund:   !refundUnsupportedChannels[name],
		SupportsQuery:    true,
		SupportsClose:    false, // no legacy gateway implements ClosePayment
		SupportsCallback: true,
	}
}
