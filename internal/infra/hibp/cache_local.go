package hibp

import (
	"context"
	"sync"
	"time"
)

// localCacheMaxEntries bounds the process-local range cache. Each body is
// ~45KB, so 4096 entries ≈ 180MB worst case; eviction keeps it below that.
const localCacheMaxEntries = 4096

// LocalRangeCache is the process-local RangeCache fallback when no Redis
// session cache is configured. Entries expire after RangeCacheTTL; eviction
// runs on access, so no background goroutine is needed.
type LocalRangeCache struct {
	mu      sync.Mutex
	entries map[string]localCacheEntry
}

type localCacheEntry struct {
	body      string
	expiresAt time.Time
}

func NewLocalRangeCache() *LocalRangeCache {
	return &LocalRangeCache{entries: make(map[string]localCacheEntry)}
}

func (c *LocalRangeCache) Get(_ context.Context, prefix string) (string, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[prefix]
	if !ok {
		return "", false, nil
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.entries, prefix)
		return "", false, nil
	}
	return entry.body, true, nil
}

func (c *LocalRangeCache) Set(_ context.Context, prefix, body string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if len(c.entries) >= localCacheMaxEntries {
		for k, e := range c.entries {
			if now.After(e.expiresAt) {
				delete(c.entries, k)
			}
		}
		for k := range c.entries {
			if len(c.entries) < localCacheMaxEntries {
				break
			}
			delete(c.entries, k)
		}
	}
	c.entries[prefix] = localCacheEntry{body: body, expiresAt: now.Add(RangeCacheTTL)}
	return nil
}
