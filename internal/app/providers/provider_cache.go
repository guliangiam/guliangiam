package providers

import (
	"container/list"
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

const (
	defaultMessagingCacheTTL        = 30 * time.Second
	defaultMessagingCacheMaxEntries = 2048
)

const (
	cacheEvictionExpired    = "expired"
	cacheEvictionCapacity   = "capacity"
	cacheEvictionInvalidate = "invalidate"

	cacheBypassNoTrustedScope       = "no_trusted_scope"
	cacheBypassSystemScope          = "system_scope"
	cacheBypassOrganizationMismatch = "organization_mismatch"
)

// messagingResolutionSource tells the resolver whether a lookup was served by
// the process cache or went to the database.
type messagingResolutionSource string

const (
	messagingSourceDatabase messagingResolutionSource = "database"
	messagingSourceCache    messagingResolutionSource = "cache"
)

// sourcedMessagingLookup is implemented by MessagingProviderCache. Lookups
// without it are assumed to read the database.
type sourcedMessagingLookup interface {
	FindActiveMessagingProviderByChannelWithSource(ctx context.Context, organizationID string, channel domain.MessagingChannel) (*domain.MessagingProviderConfig, messagingResolutionSource, error)
}

type messagingCacheKey struct {
	organizationID string
	channel        domain.MessagingChannel
}

type messagingCacheEntry struct {
	key       messagingCacheKey
	config    *domain.MessagingProviderConfig
	err       error
	expiresAt time.Time
}

// MessagingProviderCache absorbs the per-send lookup of an organization's
// messaging provider. Provider rows change rarely, so a short TTL covers
// verification bursts; a provider write on this process invalidates the
// organization's entries immediately. Other instances converge within the TTL.
//
// Cache entries are only written when the caller's trusted tenant scope
// (tenantctx) proves the requested organization belongs to that tenant. An
// unauthenticated or cross-tenant caller therefore reads the database and
// never seeds a process-wide entry for another organization. The cache holds
// decrypted credentials, so entries carry the same process-memory exposure as
// the resolved config itself.
type MessagingProviderCache struct {
	lookup     domain.MessagingProviderLookup
	ttl        time.Duration
	maxEntries int
	now        func() time.Time

	mu         sync.Mutex
	generation uint64 // bumped on every Invalidate; stale fills are discarded
	order      *list.List
	entries    map[messagingCacheKey]*list.Element
}

// ProviderCacheInvalidator is the narrow surface the provider write path needs.
type ProviderCacheInvalidator interface {
	Invalidate(organizationID string)
}

var _ domain.MessagingProviderLookup = (*MessagingProviderCache)(nil)

func NewMessagingProviderCache(lookup domain.MessagingProviderLookup) *MessagingProviderCache {
	return newMessagingProviderCache(lookup, defaultMessagingCacheTTL, defaultMessagingCacheMaxEntries)
}

func newMessagingProviderCache(lookup domain.MessagingProviderLookup, ttl time.Duration, maxEntries int) *MessagingProviderCache {
	sizeHint := maxEntries
	if sizeHint < 0 {
		sizeHint = 0
	}
	return &MessagingProviderCache{
		lookup:     lookup,
		ttl:        ttl,
		maxEntries: maxEntries,
		now:        time.Now,
		order:      list.New(),
		entries:    make(map[messagingCacheKey]*list.Element, sizeHint),
	}
}

func (c *MessagingProviderCache) FindActiveMessagingProviderByChannel(ctx context.Context, organizationID string, channel domain.MessagingChannel) (*domain.MessagingProviderConfig, error) {
	config, _, err := c.FindActiveMessagingProviderByChannelWithSource(ctx, organizationID, channel)
	return config, err
}

// FindActiveMessagingProviderByChannelWithSource also reports whether the
// answer came from the cache, so metrics can distinguish a hit from a read.
func (c *MessagingProviderCache) FindActiveMessagingProviderByChannelWithSource(ctx context.Context, organizationID string, channel domain.MessagingChannel) (*domain.MessagingProviderConfig, messagingResolutionSource, error) {
	if c == nil || c.lookup == nil {
		return nil, messagingSourceDatabase, apperrors.NewInternal("messaging provider cache is not configured")
	}

	key, trusted, bypassReason := c.trustedCacheKey(ctx, organizationID, channel)
	if !trusted {
		// No authority to cache this organization: read through without storing.
		recordProviderCacheBypass(string(channel), bypassReason)
		config, err := c.lookup.FindActiveMessagingProviderByChannel(ctx, organizationID, channel)
		return config, messagingSourceDatabase, err
	}

	var expired bool
	c.mu.Lock()
	if element, ok := c.entries[key]; ok {
		entry := element.Value.(*messagingCacheEntry)
		if c.now().Before(entry.expiresAt) {
			c.order.MoveToFront(element)
			config, err := entry.config, entry.err
			c.mu.Unlock()
			recordProviderCacheHit(string(channel))
			return config, messagingSourceCache, err
		}
		c.remove(element)
		expired = true
	}
	generation := c.generation
	c.mu.Unlock()
	if expired {
		recordProviderCacheEviction(string(channel), cacheEvictionExpired)
	}
	recordProviderCacheMiss(string(channel))

	// The lookup runs outside the lock: a slow database must not serialize
	// resolutions for other organizations.
	config, err := c.lookup.FindActiveMessagingProviderByChannel(ctx, organizationID, channel)
	if err != nil && !cacheableProviderLookupError(err) {
		return config, messagingSourceDatabase, err
	}
	c.storeIfCurrent(key, generation, config, err)
	return config, messagingSourceDatabase, err
}

// trustedCacheKey returns the canonical cache key only when the request's
// trusted tenant scope is the requested organization. Slug/UUID case variance
// is normalized; the returned key never contains a tenant other than the
// caller's own. Platform-admin/system scopes are deliberately uncached because
// they do not identify one tenant.
func (c *MessagingProviderCache) trustedCacheKey(ctx context.Context, organizationID string, channel domain.MessagingChannel) (messagingCacheKey, bool, string) {
	requested := normalizeOrganizationID(organizationID)
	if requested == "" {
		return messagingCacheKey{}, false, cacheBypassNoTrustedScope
	}

	scope := tenantctx.TenantScopeFromContext(ctx)
	if scope.Mode == tenantctx.TenantFilterSuperAdmin || scope.Mode == tenantctx.TenantFilterSystem {
		return messagingCacheKey{}, false, cacheBypassSystemScope
	}

	scopeOrganization := normalizeOrganizationID(scope.OrganizationID)
	scopeName := normalizeOrganizationID(scope.OrganizationName)
	if scopeOrganization == "" && scopeName == "" {
		return messagingCacheKey{}, false, cacheBypassNoTrustedScope
	}
	if requested == scopeOrganization || requested == scopeName {
		return messagingCacheKey{organizationID: requested, channel: channel}, true, ""
	}
	return messagingCacheKey{}, false, cacheBypassOrganizationMismatch
}

// Invalidate drops every cached channel for one organization and advances the
// generation so fills already in flight are discarded instead of replacing
// the post-write value. It is called after a provider create/update/status
// change/secret rotation/delete commits.
func (c *MessagingProviderCache) Invalidate(organizationID string) {
	if c == nil {
		return
	}
	organizationID = normalizeOrganizationID(organizationID)
	if organizationID == "" {
		return
	}

	channels := make([]string, 0, 2)
	c.mu.Lock()
	c.generation++
	for key, element := range c.entries {
		if key.organizationID == organizationID {
			delete(c.entries, key)
			c.order.Remove(element)
			channels = append(channels, string(key.channel))
		}
	}
	c.mu.Unlock()
	for _, channel := range channels {
		recordProviderCacheEviction(channel, cacheEvictionInvalidate)
	}
}

// storeIfCurrent writes a lookup result only when no Invalidate happened while
// the lookup was in flight. The generation is process-wide: an unrelated
// invalidation may discard a valid fill, trading a redundant read for never
// resurrecting a pre-write value.
func (c *MessagingProviderCache) storeIfCurrent(key messagingCacheKey, generation uint64, config *domain.MessagingProviderConfig, err error) bool {
	evictedChannels := make([]string, 0, 1)
	c.mu.Lock()
	if generation != c.generation || c.maxEntries <= 0 {
		c.mu.Unlock()
		return false
	}
	if element, ok := c.entries[key]; ok {
		c.remove(element)
	}
	for c.order.Len() >= c.maxEntries {
		element := c.order.Back()
		if element == nil {
			break
		}
		evictedChannels = append(evictedChannels, string(element.Value.(*messagingCacheEntry).key.channel))
		c.remove(element)
	}
	c.entries[key] = c.order.PushFront(&messagingCacheEntry{
		key:       key,
		config:    config,
		err:       err,
		expiresAt: c.now().Add(c.ttl),
	})
	c.mu.Unlock()

	for _, channel := range evictedChannels {
		recordProviderCacheEviction(channel, cacheEvictionCapacity)
	}
	return true
}

func (c *MessagingProviderCache) remove(element *list.Element) {
	if element == nil {
		return
	}
	entry := element.Value.(*messagingCacheEntry)
	delete(c.entries, entry.key)
	c.order.Remove(element)
}

// cacheableProviderLookupError keeps a stable disabled configuration (503) in
// the cache. ErrMessagingProviderUnconfigured is never cached: a lookup
// restricted by another tenant's RLS returns it when the row is invisible, and
// caching that answer would poison the victim organization. Transient errors
// also stay uncached so the next send retries.
func cacheableProviderLookupError(err error) bool {
	var appErr *apperrors.Error
	return errors.As(err, &appErr) && appErr.HTTPStatus() == http.StatusServiceUnavailable
}

func normalizeOrganizationID(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
