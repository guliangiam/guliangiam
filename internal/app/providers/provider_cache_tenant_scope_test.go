package providers

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func scopedContext(organizationID string) context.Context {
	return tenantctx.WithTenantScope(context.Background(), organizationID, organizationID, tenantctx.TenantFilterOptional)
}

func cacheEntryCount(cache *MessagingProviderCache) int {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return len(cache.entries)
}

type orgScriptedMessagingLookup struct {
	mu      sync.Mutex
	calls   int
	configs map[string]*domain.MessagingProviderConfig
	errs    map[string]error
}

func (l *orgScriptedMessagingLookup) FindActiveMessagingProviderByChannel(_ context.Context, organizationID string, _ domain.MessagingChannel) (*domain.MessagingProviderConfig, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls++
	if config, ok := l.configs[organizationID]; ok {
		return config, nil
	}
	if err, ok := l.errs[organizationID]; ok {
		return nil, err
	}
	return nil, domain.ErrMessagingProviderUnconfigured
}

func (l *orgScriptedMessagingLookup) callCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.calls
}

// TestMessagingProviderCache_DoesNotCacheAcrossTrustedTenant reproduces the
// cross-tenant negative poisoning: tenant X resolves tenant Y's unconfigured
// channel. The answer must not be stored under Y, and a later resolution made
// with Y's own trusted scope must still reach the lookup.
func TestMessagingProviderCache_DoesNotCacheAcrossTrustedTenant(t *testing.T) {
	lookup := &orgScriptedMessagingLookup{
		errs: map[string]error{"tenant-y": domain.ErrMessagingProviderUnconfigured},
	}
	cache := newMessagingProviderCache(lookup, time.Minute, 10)

	_, err := cache.FindActiveMessagingProviderByChannel(scopedContext("tenant-x"), "tenant-y", domain.MessagingChannelEmail)
	require.ErrorIs(t, err, domain.ErrMessagingProviderUnconfigured)
	assert.Zero(t, cacheEntryCount(cache), "another tenant's negative answer must not be cached")

	_, err = cache.FindActiveMessagingProviderByChannel(scopedContext("tenant-y"), "tenant-y", domain.MessagingChannelEmail)
	require.ErrorIs(t, err, domain.ErrMessagingProviderUnconfigured)
	assert.Equal(t, 2, lookup.callCount(), "tenant Y must still query the database after tenant X's request")
}

// TestMessagingProviderCache_DoesNotCacheAnotherTenantsConfig covers the
// positive-result variant of the same boundary.
func TestMessagingProviderCache_DoesNotCacheAnotherTenantsConfig(t *testing.T) {
	config := &domain.MessagingProviderConfig{
		ProviderID: "provider-y", OrganizationID: "tenant-y", Kind: "smtp", Channel: domain.MessagingChannelEmail,
	}
	lookup := &orgScriptedMessagingLookup{
		configs: map[string]*domain.MessagingProviderConfig{"tenant-y": config},
	}
	cache := newMessagingProviderCache(lookup, time.Minute, 10)

	resolved, err := cache.FindActiveMessagingProviderByChannel(scopedContext("tenant-x"), "tenant-y", domain.MessagingChannelEmail)
	require.NoError(t, err)
	assert.Equal(t, config, resolved)
	assert.Zero(t, cacheEntryCount(cache), "another tenant's config must not be cached")

	_, err = cache.FindActiveMessagingProviderByChannel(scopedContext("tenant-y"), "tenant-y", domain.MessagingChannelEmail)
	require.NoError(t, err)
	assert.Equal(t, 2, lookup.callCount(), "tenant Y must resolve through its own trusted scope")
}

// TestMessagingProviderCache_HitIsNotReportedAsDatabase asserts the metric
// vocabulary: the second resolution in one trusted scope is a cache hit, so
// provider_resolution_total must not count it as a database read.
func TestMessagingProviderCache_HitIsNotReportedAsDatabase(t *testing.T) {
	lookup := &orgScriptedMessagingLookup{
		configs: map[string]*domain.MessagingProviderConfig{
			"org-1": {ProviderID: "provider-1", OrganizationID: "org-1", Kind: "twilio_sms", Channel: domain.MessagingChannelSMS},
		},
	}
	cache := newMessagingProviderCache(lookup, time.Minute, 10)
	resolver := MessagingResolver{Lookup: cache, Builder: &fakeSenderBuilder{sms: fakeSmsSender{}}}
	ctx := scopedContext("org-1")

	dbBefore := counterValue(t, "guliangiam_provider_resolution_total", map[string]string{
		"family": "identity", "channel": "sms", "result": "database",
	})
	cacheBefore := counterValue(t, "guliangiam_provider_resolution_total", map[string]string{
		"family": "identity", "channel": "sms", "result": "cache",
	})
	hitBefore := counterValue(t, "guliangiam_provider_cache_hit_total", map[string]string{"channel": "sms"})
	missBefore := counterValue(t, "guliangiam_provider_cache_miss_total", map[string]string{"channel": "sms"})

	_, err := resolver.ResolveSmsSender(ctx, "org-1")
	require.NoError(t, err)
	_, err = resolver.ResolveSmsSender(ctx, "org-1")
	require.NoError(t, err)

	assert.Equal(t, 1.0, counterValue(t, "guliangiam_provider_resolution_total", map[string]string{
		"family": "identity", "channel": "sms", "result": "database",
	})-dbBefore, "only the first resolution read the database")
	assert.Equal(t, 1.0, counterValue(t, "guliangiam_provider_resolution_total", map[string]string{
		"family": "identity", "channel": "sms", "result": "cache",
	})-cacheBefore, "the cached resolution must be labelled cache")
	assert.Equal(t, 1.0, counterValue(t, "guliangiam_provider_cache_hit_total", map[string]string{
		"channel": "sms",
	})-hitBefore, "one cache hit must be counted")
	assert.Equal(t, 1.0, counterValue(t, "guliangiam_provider_cache_miss_total", map[string]string{
		"channel": "sms",
	})-missBefore, "one cache miss must be counted")
}

// blockingMessagingLookup holds the first database read open until the test
// releases it, so the cache can be invalidated while a fill is in flight.
type blockingMessagingLookup struct {
	started chan struct{}
	release chan struct{}
	calls   atomic.Int64
	first   *domain.MessagingProviderConfig
	next    *domain.MessagingProviderConfig
}

func (l *blockingMessagingLookup) FindActiveMessagingProviderByChannel(context.Context, string, domain.MessagingChannel) (*domain.MessagingProviderConfig, error) {
	if l.calls.Add(1) == 1 {
		close(l.started)
		<-l.release
		return l.first, nil
	}
	return l.next, nil
}

// TestMessagingProviderCache_InvalidateDiscardsInFlightFill reproduces the
// read-through race: a lookup that captured the pre-invalidation snapshot must
// not repopulate the cache after Invalidate returned.
func TestMessagingProviderCache_InvalidateDiscardsInFlightFill(t *testing.T) {
	oldConfig := &domain.MessagingProviderConfig{
		ProviderID: "provider-old", OrganizationID: "org-1", Kind: "smtp", Channel: domain.MessagingChannelEmail,
	}
	newConfig := &domain.MessagingProviderConfig{
		ProviderID: "provider-new", OrganizationID: "org-1", Kind: "smtp", Channel: domain.MessagingChannelEmail,
	}
	lookup := &blockingMessagingLookup{
		started: make(chan struct{}),
		release: make(chan struct{}),
		first:   oldConfig,
		next:    newConfig,
	}
	cache := newMessagingProviderCache(lookup, time.Minute, 10)
	ctx := scopedContext("org-1")

	resolved := make(chan *domain.MessagingProviderConfig, 1)
	go func() {
		config, _ := cache.FindActiveMessagingProviderByChannel(ctx, "org-1", domain.MessagingChannelEmail)
		resolved <- config
	}()

	<-lookup.started
	cache.Invalidate("org-1")
	close(lookup.release)
	<-resolved

	assert.Zero(t, cacheEntryCount(cache), "an in-flight fill started before Invalidate must be discarded")

	config, err := cache.FindActiveMessagingProviderByChannel(ctx, "org-1", domain.MessagingChannelEmail)
	require.NoError(t, err)
	assert.Equal(t, newConfig, config, "the next resolution must observe the post-invalidation value")
	assert.Equal(t, int64(2), lookup.calls.Load())
}

// TestMessagingProviderCache_ConcurrentResolveAndInvalidate exercises the
// mutex/generation path under -race.
func TestMessagingProviderCache_ConcurrentResolveAndInvalidate(t *testing.T) {
	config := &domain.MessagingProviderConfig{
		ProviderID: "provider-1", OrganizationID: "org-1", Kind: "smtp", Channel: domain.MessagingChannelEmail,
	}
	lookup := &orgScriptedMessagingLookup{
		configs: map[string]*domain.MessagingProviderConfig{"org-1": config},
	}
	cache := newMessagingProviderCache(lookup, time.Minute, 10)
	ctx := scopedContext("org-1")

	var waitGroup sync.WaitGroup
	for i := 0; i < 4; i++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for j := 0; j < 50; j++ {
				if _, err := cache.FindActiveMessagingProviderByChannel(ctx, "org-1", domain.MessagingChannelEmail); err != nil {
					t.Errorf("resolve failed: %v", err)
					return
				}
			}
		}()
	}
	for i := 0; i < 2; i++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for j := 0; j < 50; j++ {
				cache.Invalidate("org-1")
			}
		}()
	}
	waitGroup.Wait()
	cache.Invalidate("org-1")
	assert.Zero(t, cacheEntryCount(cache))
}

func counterValue(t *testing.T, familyName string, labels map[string]string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() != familyName {
			continue
		}
		for _, metric := range family.GetMetric() {
			if !hasLabels(metric.GetLabel(), labels) {
				continue
			}
			if metric.Counter != nil {
				return metric.Counter.GetValue()
			}
			if metric.Gauge != nil {
				return metric.Gauge.GetValue()
			}
		}
	}
	return 0
}

func hasLabels(pairs []*dto.LabelPair, want map[string]string) bool {
	for key, value := range want {
		found := false
		for _, pair := range pairs {
			if pair.GetName() == key && pair.GetValue() == value {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
