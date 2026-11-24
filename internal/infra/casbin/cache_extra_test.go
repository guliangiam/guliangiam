package casbin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewEnforceCache_NegativeMaxSize(t *testing.T) {
	cache := NewEnforceCache(-1)
	assert.Equal(t, defaultEnforceCacheMaxSize, cache.maxSize)
}

func TestEnforceCache_Get_NotFound(t *testing.T) {
	cache := NewEnforceCache(100)
	allowed, found := cache.Get("user-1", "resource-1", "read")
	assert.False(t, found)
	assert.False(t, allowed)
}

func TestEnforceCache_SetAndGet_Denied(t *testing.T) {
	cache := NewEnforceCache(100)
	cache.Set("user-1", "resource-1", "write", false)

	allowed, found := cache.Get("user-1", "resource-1", "write")
	assert.True(t, found)
	assert.False(t, allowed)
}

func TestEnforceCache_CacheKey_Deterministic(t *testing.T) {
	cache := NewEnforceCache(100)
	key1 := cache.cacheKey("sub", "obj", "act")
	key2 := cache.cacheKey("sub", "obj", "act")
	assert.Equal(t, key1, key2)
}

func TestEnforceCache_CacheKey_DifferentInputs(t *testing.T) {
	cache := NewEnforceCache(100)
	key1 := cache.cacheKey("sub1", "obj", "act")
	key2 := cache.cacheKey("sub2", "obj", "act")
	assert.NotEqual(t, key1, key2)
}

func TestEnforceCache_EvictToSize_NegativeTarget(t *testing.T) {
	cache := NewEnforceCache(100)
	cache.ttl = 10 * time.Minute
	cache.Set("sub", "obj", "act", true)

	cache.evictToSize(-1)
	// Should not panic; negative target treated as 0
	cache.mu.RLock()
	size := len(cache.entries)
	cache.mu.RUnlock()
	assert.Equal(t, 0, size)
}

func TestEnforceCache_EvictToSize_ExpiredEntries(t *testing.T) {
	cache := NewEnforceCache(100)
	cache.ttl = 1 * time.Nanosecond // entries expire immediately

	cache.Set("sub", "obj", "act", true)
	time.Sleep(1 * time.Millisecond) // wait for expiration

	cache.evictToSize(100)
	cache.mu.RLock()
	size := len(cache.entries)
	cache.mu.RUnlock()
	assert.Equal(t, 0, size)
}

func TestEnforceCache_EvictOldest(t *testing.T) {
	cache := NewEnforceCache(100)
	cache.ttl = 10 * time.Minute
	cache.Set("sub", "obj", "act", true)

	cache.evictOldest()
	// maxSize=100, target=80, but only 1 entry so nothing to evict
	cache.mu.RLock()
	size := len(cache.entries)
	cache.mu.RUnlock()
	assert.Equal(t, 1, size)
}

func TestEnforceCache_Invalidate_Empty(t *testing.T) {
	cache := NewEnforceCache(100)
	cache.Invalidate() // should not panic
	cache.mu.RLock()
	size := len(cache.entries)
	cache.mu.RUnlock()
	assert.Equal(t, 0, size)
}
