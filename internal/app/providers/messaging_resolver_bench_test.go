package providers

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// latentMessagingLookup models a database roundtrip. It is a simulation for
// benchmark comparison only; the real query cost depends on the deployment.
type latentMessagingLookup struct {
	config *domain.MessagingProviderConfig
	delay  time.Duration
}

func (l latentMessagingLookup) FindActiveMessagingProviderByChannel(_ context.Context, _ string, _ domain.MessagingChannel) (*domain.MessagingProviderConfig, error) {
	time.Sleep(l.delay)
	return l.config, nil
}

func benchmarkSmsResolver(b *testing.B, lookup domain.MessagingProviderLookup) {
	resolver := MessagingResolver{
		Lookup:  lookup,
		Builder: &fakeSenderBuilder{sms: fakeSmsSender{}},
	}
	ctx := tenantctx.WithTenantScope(context.Background(), "org-1", "org-1", tenantctx.TenantFilterOptional)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := resolver.ResolveSmsSender(ctx, "org-1"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMessagingResolverResolveSms(b *testing.B) {
	benchmarkSmsResolver(b, &fakeMessagingLookup{config: &domain.MessagingProviderConfig{
		ProviderID: "provider-1", OrganizationID: "org-1", Kind: "twilio_sms", Channel: domain.MessagingChannelSMS,
		Values: map[string]string{"admin": "sid", "password": "token", "sender": "+10086"},
	}})
}

func BenchmarkMessagingResolverResolveSmsCached(b *testing.B) {
	lookup := &fakeMessagingLookup{config: &domain.MessagingProviderConfig{
		ProviderID: "provider-1", OrganizationID: "org-1", Kind: "twilio_sms", Channel: domain.MessagingChannelSMS,
		Values: map[string]string{"admin": "sid", "password": "token", "sender": "+10086"},
	}}
	benchmarkSmsResolver(b, NewMessagingProviderCache(lookup))
}

func BenchmarkMessagingResolverResolveSmsWithDatabaseRoundtrip(b *testing.B) {
	benchmarkSmsResolver(b, latentMessagingLookup{
		config: &domain.MessagingProviderConfig{
			ProviderID: "provider-1", OrganizationID: "org-1", Kind: "twilio_sms", Channel: domain.MessagingChannelSMS,
		},
		delay: 100 * time.Microsecond,
	})
}

func BenchmarkMessagingResolverResolveSmsWithDatabaseRoundtripCached(b *testing.B) {
	benchmarkSmsResolver(b, NewMessagingProviderCache(latentMessagingLookup{
		config: &domain.MessagingProviderConfig{
			ProviderID: "provider-1", OrganizationID: "org-1", Kind: "twilio_sms", Channel: domain.MessagingChannelSMS,
		},
		delay: 100 * time.Microsecond,
	}))
}
