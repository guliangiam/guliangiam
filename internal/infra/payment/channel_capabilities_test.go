package payment

import "testing"

// TestChannelAdapterMetadata_RefundMatchesImplementation keeps the advertised
// capability in step with the adapters that actually return unimplemented.
func TestChannelAdapterMetadata_RefundMatchesImplementation(t *testing.T) {
	unsupported := []string{"gc", "fastspring", "lemonsqueezy"}
	supported := []string{"alipay", "wechat_pay", "stripe", "paypal", "balance", "dummy"}

	for _, name := range unsupported {
		meta := NewChannelAdapter(name, nil).Metadata()
		if meta.SupportsRefund {
			t.Errorf("%s: SupportsRefund = true, want false (Refund returns unimplemented)", name)
		}
	}
	for _, name := range supported {
		meta := NewChannelAdapter(name, nil).Metadata()
		if !meta.SupportsRefund {
			t.Errorf("%s: SupportsRefund = false, want true", name)
		}
	}

	// No legacy gateway implements ClosePayment, so close must stay false even
	// for the sandbox channel that used to advertise it.
	for _, name := range append(append([]string{}, unsupported...), supported...) {
		meta := NewChannelAdapter(name, nil).Metadata()
		if meta.SupportsClose {
			t.Errorf("%s: SupportsClose = true, want false", name)
		}
		if !meta.SupportsCharge || !meta.SupportsQuery || !meta.SupportsCallback {
			t.Errorf("%s: charge/query/callback must stay supported", name)
		}
	}
}
