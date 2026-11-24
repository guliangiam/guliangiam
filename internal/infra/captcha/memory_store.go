package captcha

import (
	"context"
	"sync"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/recover"
)

// MemoryStore is an in-memory CAPTCHA code store with TTL-based expiration.
type MemoryStore struct {
	mu    sync.RWMutex
	items map[string]*item
	ttl   time.Duration
}

type item struct {
	code      string
	expiresAt time.Time
}

// NewMemoryStore creates a new in-memory CAPTCHA store.
func NewMemoryStore(ttl time.Duration) *MemoryStore {
	s := &MemoryStore{
		items: make(map[string]*item),
		ttl:   ttl,
	}
	recover.SafeGo("captcha-memory-cleanup", s.cleanup)
	return s
}

func (s *MemoryStore) Set(ctx context.Context, id, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[id] = &item{code: code, expiresAt: time.Now().Add(s.ttl)}
	return nil
}

func (s *MemoryStore) Get(ctx context.Context, id string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	it, ok := s.items[id]
	if !ok || time.Now().After(it.expiresAt) {
		return "", errNotFound
	}
	return it.code, nil
}

func (s *MemoryStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, id)
	return nil
}

func (s *MemoryStore) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for k, v := range s.items {
			if now.After(v.expiresAt) {
				delete(s.items, k)
			}
		}
		s.mu.Unlock()
	}
}

var errNotFound = &notFoundError{}

type notFoundError struct{}

func (e *notFoundError) Error() string { return "captcha not found" }
