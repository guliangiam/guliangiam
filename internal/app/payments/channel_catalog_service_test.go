package payments

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
)

type fakeChannelCatalog struct {
	items []domain.ChannelMetadata
}

func (f fakeChannelCatalog) ChannelMetadata() []domain.ChannelMetadata {
	return f.items
}

func TestChannelCatalogService_ListMapsCapabilitiesAndNames(t *testing.T) {
	svc := NewChannelCatalogService(fakeChannelCatalog{items: []domain.ChannelMetadata{
		{Channel: "alipay", SupportsCharge: true, SupportsRefund: true, SupportsQuery: true, SupportsCallback: true},
		{Channel: "wechat_pay", SupportsCharge: true, SupportsRefund: true, SupportsQuery: true, SupportsCallback: true},
		{Channel: "gc", SupportsCharge: true, SupportsRefund: false, SupportsQuery: true, SupportsCallback: true},
		{Channel: "square", SupportsCharge: true, SupportsRefund: true, SupportsQuery: true, SupportsCallback: true},
		{Channel: "checkout", SupportsCharge: true, SupportsRefund: true, SupportsQuery: true, SupportsCallback: true},
	}})

	got := svc.List(context.Background())
	if len(got) != 5 {
		t.Fatalf("List() returned %d entries, want 5", len(got))
	}
	if got[0].DisplayName != "Alipay 支付宝" {
		t.Errorf("alipay display name = %q", got[0].DisplayName)
	}
	if got[2].Channel != "gc" || got[2].SupportsRefund {
		t.Errorf("gc capability = %+v, want refund unsupported", got[2])
	}
	if got[3].DisplayName != "Square" {
		t.Errorf("square display name = %q, want Square", got[3].DisplayName)
	}
	if got[4].DisplayName != "Checkout.com" {
		t.Errorf("checkout display name = %q, want Checkout.com", got[4].DisplayName)
	}
}
