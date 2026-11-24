package cache

import (
	"context"
	"sync"
	"time"

	domainsessions "github.com/guliangiam/guliangiam/internal/domain/sessions"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
)

// MemorySessionCache is a process-local SessionCache backed by TTLCache.
//
// Per ADR-010: used in single-instance deployments, tests, and as graceful
// fallback when Redis is unavailable. TTL capped at defaultMemoryTTL to
// prevent unbounded memory growth from sessions with very long lifetimes.
type MemorySessionCache struct {
	mu     sync.RWMutex
	items  map[string]*memoryCacheItem
	maxTTL time.Duration
}

type memoryCacheItem struct {
	s         *domainsessions.Session
	expiresAt time.Time
}

func NewMemorySessionCache(maxTTL time.Duration) *MemorySessionCache {
	c := &MemorySessionCache{
		items:  make(map[string]*memoryCacheItem),
		maxTTL: maxTTL,
	}
	recover.SafeGo("session-cache-memory-cleanup", c.cleanup)
	return c
}

func (c *MemorySessionCache) Get(_ context.Context, id string) (*domainsessions.Session, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, ok := c.items[sessionKey(id)]
	if !ok || time.Now().After(item.expiresAt) {
		return nil, false
	}
	return item.s, true
}

func (c *MemorySessionCache) Set(_ context.Context, s *domainsessions.Session) {
	if s == nil {
		return
	}
	ttl := time.Until(s.ExpiresAt)
	if ttl <= 0 {
		return // already expired, don't cache
	}
	if ttl > c.maxTTL {
		ttl = c.maxTTL
	}
	c.mu.Lock()
	c.items[sessionKey(s.ID)] = &memoryCacheItem{
		s:         s,
		expiresAt: time.Now().Add(ttl),
	}
	c.mu.Unlock()
}

func (c *MemorySessionCache) Delete(_ context.Context, id string) {
	c.mu.Lock()
	delete(c.items, sessionKey(id))
	c.mu.Unlock()
}

func (c *MemorySessionCache) InvalidateAll(_ context.Context) {
	c.mu.Lock()
	c.items = make(map[string]*memoryCacheItem)
	c.mu.Unlock()
}

func (c *MemorySessionCache) cleanup() {
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
