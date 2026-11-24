package casbin

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"sort"
	"sync"
	"time"
)

type cacheEntry struct {
	allowed   bool
	expiresAt time.Time
	accessAt  time.Time // last access time for LRU eviction
}

type EnforceCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
	ttl     time.Duration
	hmacKey []byte
	maxSize int // maximum number of entries before eviction
}

const (
	// defaultEnforceCacheMaxSize is the default maximum cache capacity.
	defaultEnforceCacheMaxSize = 1_000_000

	// evictionTargetRatio is the target fill ratio after eviction (80% of maxSize).
	evictionTargetRatio = 0.8
)

func NewEnforceCache(maxSize int) *EnforceCache {
	if maxSize <= 0 {
		maxSize = defaultEnforceCacheMaxSize
	}
	return &EnforceCache{
		entries: make(map[string]cacheEntry),
		ttl:     5 * time.Second,
		hmacKey: []byte("guliangiam-casbin-cache-v1"),
		maxSize: maxSize,
	}
}

func (c *EnforceCache) cacheKey(sub, obj, act string) string {
	mac := hmac.New(sha256.New, c.hmacKey)
	mac.Write([]byte(sub + "|" + obj + "|" + act))
	return hex.EncodeToString(mac.Sum(nil))
}

func (c *EnforceCache) Get(sub, obj, act string) (allowed, found bool) {
	key := c.cacheKey(sub, obj, act)
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		return false, false
	}
	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return false, false
	}
	// Update access time for LRU tracking.
	c.mu.Lock()
	entry.accessAt = time.Now()
	c.entries[key] = entry
	c.mu.Unlock()
	return entry.allowed, true
}

func (c *EnforceCache) Set(sub, obj, act string, allowed bool) {
	key := c.cacheKey(sub, obj, act)
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	// If key already exists, just update in place — no capacity concern.
	if _, exists := c.entries[key]; exists {
		c.entries[key] = cacheEntry{
			allowed:   allowed,
			expiresAt: now.Add(c.ttl),
			accessAt:  now,
		}
		return
	}

	// Capacity check: evict if at or above maxSize.
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	c.entries[key] = cacheEntry{
		allowed:   allowed,
		expiresAt: now.Add(c.ttl),
		accessAt:  now,
	}
}

// evictToSize evicts entries (expired first, then LRU) until the cache
// size is at or below the specified target.
// Must be called with c.mu held for writing.
func (c *EnforceCache) evictToSize(target int) {
	if target < 0 {
		target = 0
	}
	now := time.Now()

	// Phase 1: Remove all expired entries.
	for k, v := range c.entries {
		if now.After(v.expiresAt) {
			delete(c.entries, k)
		}
	}

	// Phase 2: If still over target, evict LRU entries.
	if len(c.entries) <= target {
		return
	}

	// Build a sorted slice of keys by accessAt (oldest first).
	type kv struct {
		key      string
		accessAt time.Time
	}
	sorted := make([]kv, 0, len(c.entries))
	for k, v := range c.entries {
		sorted = append(sorted, kv{key: k, accessAt: v.accessAt})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].accessAt.Before(sorted[j].accessAt)
	})

	// Evict oldest entries until we reach the target.
	toRemove := len(c.entries) - target
	for i := 0; i < toRemove && i < len(sorted); i++ {
		delete(c.entries, sorted[i].key)
	}

	slog.Debug("enforce cache eviction completed",
		"remaining", len(c.entries),
		"target", target,
		"max_size", c.maxSize,
	)
}

// evictOldest removes expired entries and, if still over capacity,
// evicts the least-recently-used entries until the cache is at 80% of maxSize.
// Must be called with c.mu held for writing.
func (c *EnforceCache) evictOldest() {
	target := int(float64(c.maxSize) * evictionTargetRatio)
	c.evictToSize(target)
}

func (c *EnforceCache) Invalidate() {
	c.mu.Lock()
	clear(c.entries)
	c.mu.Unlock()
}
