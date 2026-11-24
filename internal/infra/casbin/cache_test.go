package casbin

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewEnforceCache_DefaultMaxSize(t *testing.T) {
	cache := NewEnforceCache(0)
	assert.Equal(t, defaultEnforceCacheMaxSize, cache.maxSize)
}

func TestNewEnforceCache_CustomMaxSize(t *testing.T) {
	cache := NewEnforceCache(500)
	assert.Equal(t, 500, cache.maxSize)
}

func TestEnforceCache_SetGet(t *testing.T) {
	cache := NewEnforceCache(100)

	cache.Set("user:admin", "users", "list", true)
	allowed, found := cache.Get("user:admin", "users", "list")
	assert.True(t, found)
	assert.True(t, allowed)

	_, found = cache.Get("user:viewer", "users", "list")
	assert.False(t, found)
}

func TestEnforceCache_EvictionOnCapacity(t *testing.T) {
	// Create a small cache to trigger eviction quickly.
	maxSize := 10
	cache := NewEnforceCache(maxSize)

	// Fill cache beyond capacity.
	for i := 0; i < maxSize+5; i++ {
		cache.Set(fmt.Sprintf("user:%d", i), "resource", "action", true)
	}

	// Cache should have evicted down to target (80% of maxSize).
	cache.mu.RLock()
	count := len(cache.entries)
	cache.mu.RUnlock()
	target := int(float64(maxSize) * evictionTargetRatio)
	assert.LessOrEqual(t, count, maxSize, "cache should not exceed maxSize after eviction")
	// After eviction + 1 new insertion, entries should be at or below target+1.
	// This is standard LRU behavior: evict to target, then insert 1 new entry.
	assert.LessOrEqual(t, count, target+1, "cache should be at or below eviction target+1 after post-eviction insertion")
}

func TestEnforceCache_ExpiredEntriesEvicted(t *testing.T) {
	cache := NewEnforceCache(100)

	// Set an entry and let it expire.
	cache.Set("user:admin", "users", "list", true)

	// Manually expire the entry.
	cache.mu.Lock()
	for k, v := range cache.entries {
		v.expiresAt = time.Now().Add(-1 * time.Second)
		v.accessAt = time.Now().Add(-1 * time.Second)
		cache.entries[k] = v
	}
	cache.mu.Unlock()

	// Get should return not found and delete the expired entry.
	allowed, found := cache.Get("user:admin", "users", "list")
	assert.False(t, found)
	assert.False(t, allowed)

	// Verify the entry was deleted.
	cache.mu.RLock()
	_, exists := cache.entries[cache.cacheKey("user:admin", "users", "list")]
	cache.mu.RUnlock()
	assert.False(t, exists)
}

func TestEnforceCache_LRU_Eviction(t *testing.T) {
	maxSize := 5
	cache := NewEnforceCache(maxSize)

	// Insert 5 entries.
	for i := 0; i < maxSize; i++ {
		cache.Set(fmt.Sprintf("user:%d", i), "res", "act", true)
	}

	// Access entry 0 to make it recently used.
	cache.Get("user:0", "res", "act")

	// Insert 2 more entries to trigger eviction.
	cache.Set("user:5", "res", "act", true)
	cache.Set("user:6", "res", "act", true)

	// After eviction, entry 0 should still exist (recently accessed),
	// while some older entries should have been evicted.
	_, found := cache.Get("user:0", "res", "act")
	assert.True(t, found, "recently accessed entry should survive eviction")

	// New entries should exist.
	_, found = cache.Get("user:6", "res", "act")
	assert.True(t, found, "newly inserted entry should exist")
}

func TestEnforceCache_Invalidate(t *testing.T) {
	cache := NewEnforceCache(100)
	cache.Set("user:admin", "users", "list", true)
	cache.Invalidate()

	_, found := cache.Get("user:admin", "users", "list")
	assert.False(t, found)

	cache.mu.RLock()
	count := len(cache.entries)
	cache.mu.RUnlock()
	assert.Equal(t, 0, count)
}

func TestEnforceCache_UpdateExistingKey(t *testing.T) {
	cache := NewEnforceCache(100)

	cache.Set("user:admin", "users", "list", true)
	cache.Set("user:admin", "users", "list", false) // update

	allowed, found := cache.Get("user:admin", "users", "list")
	assert.True(t, found)
	assert.False(t, allowed, "value should be updated")
}
