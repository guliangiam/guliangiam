package providers

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type countingMessagingLookup struct {
	calls  int
	config *domain.MessagingProviderConfig
	err    error
}

func (f *countingMessagingLookup) FindActiveMessagingProviderByChannel(_ context.Context, _ string, _ domain.MessagingChannel) (*domain.MessagingProviderConfig, error) {
	f.calls++
	return f.config, f.err
}

func TestMessagingProviderCache_ReusesResolvedConfigPerOrganizationAndChannel(t *testing.T) {
	emailConfig := &domain.MessagingProviderConfig{ProviderID: "p-email", OrganizationID: "org-1", Kind: "smtp", Channel: domain.MessagingChannelEmail}
	smsConfig := &domain.MessagingProviderConfig{ProviderID: "p-sms", OrganizationID: "org-1", Kind: "twilio_sms", Channel: domain.MessagingChannelSMS}
	lookup := &countingMessagingLookup{config: emailConfig}
	cache := newMessagingProviderCache(lookup, time.Minute, 10)

	first, err := cache.FindActiveMessagingProviderByChannel(scopedContext("org-1"), "org-1", domain.MessagingChannelEmail)
	require.NoError(t, err)
	assert.Equal(t, emailConfig, first)

	second, err := cache.FindActiveMessagingProviderByChannel(scopedContext("org-1"), "org-1", domain.MessagingChannelEmail)
	require.NoError(t, err)
	assert.Equal(t, first, second)
	assert.Equal(t, 1, lookup.calls, "a cached channel must not hit the lookup again")

	lookup.config = smsConfig
	_, err = cache.FindActiveMessagingProviderByChannel(scopedContext("org-1"), "org-1", domain.MessagingChannelSMS)
	require.NoError(t, err)
	assert.Equal(t, 2, lookup.calls, "a different channel is a different cache key")

	_, err = cache.FindActiveMessagingProviderByChannel(scopedContext("org-2"), "org-2", domain.MessagingChannelEmail)
	require.NoError(t, err)
	assert.Equal(t, 3, lookup.calls, "a different organization is a different cache key")
}

func TestMessagingProviderCache_InvalidateForcesLookup(t *testing.T) {
	lookup := &countingMessagingLookup{config: &domain.MessagingProviderConfig{
		ProviderID: "p-1", OrganizationID: "org-1", Kind: "smtp", Channel: domain.MessagingChannelEmail,
	}}
	cache := newMessagingProviderCache(lookup, time.Minute, 10)
	_, err := cache.FindActiveMessagingProviderByChannel(scopedContext("org-1"), "org-1", domain.MessagingChannelEmail)
	require.NoError(t, err)
	_, err = cache.FindActiveMessagingProviderByChannel(scopedContext("org-1"), "org-1", domain.MessagingChannelEmail)
	require.NoError(t, err)
	require.Equal(t, 1, lookup.calls)

	// Another organization's entry must survive an unrelated invalidation.
	lookup.config = &domain.MessagingProviderConfig{ProviderID: "p-2", OrganizationID: "org-2", Kind: "smtp", Channel: domain.MessagingChannelEmail}
	_, err = cache.FindActiveMessagingProviderByChannel(scopedContext("org-2"), "org-2", domain.MessagingChannelEmail)
	require.NoError(t, err)
	require.Equal(t, 2, lookup.calls)

	lookup.config = &domain.MessagingProviderConfig{ProviderID: "p-1b", OrganizationID: "org-1", Kind: "smtp", Channel: domain.MessagingChannelEmail}
	cache.Invalidate("org-1")
	_, err = cache.FindActiveMessagingProviderByChannel(scopedContext("org-1"), "org-1", domain.MessagingChannelEmail)
	require.NoError(t, err)
	assert.Equal(t, 3, lookup.calls, "invalidation must drop the organization's entries only")

	_, err = cache.FindActiveMessagingProviderByChannel(scopedContext("org-2"), "org-2", domain.MessagingChannelEmail)
	require.NoError(t, err)
	assert.Equal(t, 3, lookup.calls, "unrelated organizations stay cached")
}

func TestMessagingProviderCache_ExpiresAfterTTL(t *testing.T) {
	lookup := &countingMessagingLookup{config: &domain.MessagingProviderConfig{
		ProviderID: "p-1", OrganizationID: "org-1", Kind: "smtp", Channel: domain.MessagingChannelEmail,
	}}
	cache := newMessagingProviderCache(lookup, 30*time.Second, 10)
	now := time.Unix(1_700_000_000, 0)
	cache.now = func() time.Time { return now }

	_, err := cache.FindActiveMessagingProviderByChannel(scopedContext("org-1"), "org-1", domain.MessagingChannelEmail)
	require.NoError(t, err)
	_, err = cache.FindActiveMessagingProviderByChannel(scopedContext("org-1"), "org-1", domain.MessagingChannelEmail)
	require.NoError(t, err)
	require.Equal(t, 1, lookup.calls)

	now = now.Add(31 * time.Second)
	_, err = cache.FindActiveMessagingProviderByChannel(scopedContext("org-1"), "org-1", domain.MessagingChannelEmail)
	require.NoError(t, err)
	assert.Equal(t, 2, lookup.calls, "an expired entry must refetch")
}

func TestMessagingProviderCache_DoesNotCacheUnconfiguredAnswer(t *testing.T) {
	lookup := &countingMessagingLookup{err: domain.ErrMessagingProviderUnconfigured}
	cache := newMessagingProviderCache(lookup, time.Minute, 10)

	_, err := cache.FindActiveMessagingProviderByChannel(scopedContext("org-1"), "org-1", domain.MessagingChannelSMS)
	require.ErrorIs(t, err, domain.ErrMessagingProviderUnconfigured)
	_, err = cache.FindActiveMessagingProviderByChannel(scopedContext("org-1"), "org-1", domain.MessagingChannelSMS)
	require.ErrorIs(t, err, domain.ErrMessagingProviderUnconfigured)
	assert.Equal(t, 2, lookup.calls, "a no-row answer is revalidated instead of being pinned for the TTL")
	assert.Zero(t, cacheEntryCount(cache))
}

func TestMessagingProviderCache_CachesDisabledAnswer(t *testing.T) {
	disabled := apperrors.NewUnavailable("messaging provider for channel sms is disabled")
	lookup := &countingMessagingLookup{err: disabled}
	cache := newMessagingProviderCache(lookup, time.Minute, 10)

	_, err := cache.FindActiveMessagingProviderByChannel(scopedContext("org-1"), "org-1", domain.MessagingChannelSMS)
	require.Error(t, err)
	_, err = cache.FindActiveMessagingProviderByChannel(scopedContext("org-1"), "org-1", domain.MessagingChannelSMS)
	require.Error(t, err)
	assert.Equal(t, 1, lookup.calls, "a disabled row is a stable configuration state and should be cached")
}

func TestMessagingProviderCache_DoesNotCacheTransientErrors(t *testing.T) {
	lookup := &countingMessagingLookup{err: apperrors.NewInternal("database unavailable")}
	cache := newMessagingProviderCache(lookup, time.Minute, 10)

	_, err := cache.FindActiveMessagingProviderByChannel(scopedContext("org-1"), "org-1", domain.MessagingChannelSMS)
	require.Error(t, err)
	_, err = cache.FindActiveMessagingProviderByChannel(scopedContext("org-1"), "org-1", domain.MessagingChannelSMS)
	require.Error(t, err)
	assert.Equal(t, 2, lookup.calls, "transient failures must retry instead of pinning a failure for the TTL")
}

func TestMessagingProviderCache_EvictsBeyondCapacity(t *testing.T) {
	lookup := &countingMessagingLookup{
		config: &domain.MessagingProviderConfig{OrganizationID: "org-1", Kind: "smtp", Channel: domain.MessagingChannelEmail},
	}
	cache := newMessagingProviderCache(lookup, time.Minute, 2)

	for _, org := range []string{"org-1", "org-2", "org-3"} {
		_, err := cache.FindActiveMessagingProviderByChannel(scopedContext(org), org, domain.MessagingChannelEmail)
		require.NoError(t, err)
	}
	assert.Equal(t, 2, cache.order.Len(), "capacity must bound the cache")
	assert.Equal(t, 3, lookup.calls)

	// org-1 was evicted as the least recently used entry.
	_, err := cache.FindActiveMessagingProviderByChannel(scopedContext("org-1"), "org-1", domain.MessagingChannelEmail)
	require.NoError(t, err)
	assert.Equal(t, 4, lookup.calls)
}

func TestMessagingProviderCache_EmptyOrganizationIsPassedThrough(t *testing.T) {
	lookup := &countingMessagingLookup{err: domain.ErrMessagingProviderUnconfigured}
	cache := newMessagingProviderCache(lookup, time.Minute, 10)

	_, err := cache.FindActiveMessagingProviderByChannel(scopedContext("org-1"), "", domain.MessagingChannelEmail)
	require.ErrorIs(t, err, domain.ErrMessagingProviderUnconfigured)
	assert.Equal(t, 1, lookup.calls)
	assert.Zero(t, cacheEntryCount(cache))
}
