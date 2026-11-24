package cache

import (
	"sync"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/recover"
)

type TTLCache struct {
	mu    sync.RWMutex
	items map[string]*cacheItem
	ttl   time.Duration
}

type cacheItem struct {
	value     interface{}
	expiresAt time.Time
}

func NewTTLCache(ttl time.Duration) *TTLCache {
	c := &TTLCache{
		items: make(map[string]*cacheItem),
		ttl:   ttl,
	}
	recover.SafeGo("ttl-cache-cleanup", c.cleanup)
	return c
}

func (c *TTLCache) Set(key string, value interface{}) {
	c.SetWithTTL(key, value, c.ttl)
}

func (c *TTLCache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = &cacheItem{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

func (c *TTLCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, ok := c.items[key]
	if !ok || time.Now().After(item.expiresAt) {
		return nil, false
	}
	return item.value, true
}

func (c *TTLCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

func (c *TTLCache) Invalidate(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.items {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(c.items, k)
		}
	}
}

func (c *TTLCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*cacheItem)
}

func (c *TTLCache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for k, v := range c.items {
			if now.After(v.expiresAt) {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}
