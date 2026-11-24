package saml

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ReplayStore records one-time protocol identifiers (SAML Response / Assertion
// IDs). SAML 2.0 SP-side replay protection needs a store that is shared by all
// replicas (Security Considerations of SAML 2.0 §4.1.1: identifier reuse must
// be detected across the whole deployment); bootstrap wires the Redis-backed
// implementation when Redis is configured and falls back to the process-local
// store otherwise.
//
// Reserve must be atomic: it returns reserved=true only for the first sighting
// of key within ttl. A false, nil result means the key was already consumed
// (replay). An error is a hard failure and callers must fail closed.
type ReplayStore interface {
	Reserve(ctx context.Context, key string, ttl time.Duration) (reserved bool, err error)
}

// replayStoreMaxEntries bounds the process-local store so a burst of invalid
// assertions cannot grow memory without limit.
const replayStoreMaxEntries = 100_000

// MemoryReplayStore is a process-local ReplayStore. It is correct for a
// single-instance deployment and for tests; multi-replica deployments get the
// shared Redis implementation from bootstrap.
type MemoryReplayStore struct {
	mu         sync.Mutex
	seen       map[string]time.Time
	maxEntries int
	now        func() time.Time
}

// NewMemoryReplayStore creates an empty process-local replay store.
func NewMemoryReplayStore() *MemoryReplayStore {
	return &MemoryReplayStore{
		seen:       make(map[string]time.Time),
		maxEntries: replayStoreMaxEntries,
		now:        time.Now,
	}
}

// Reserve records key until now+ttl and reports whether it was new.
func (s *MemoryReplayStore) Reserve(_ context.Context, key string, ttl time.Duration) (bool, error) {
	if key == "" {
		return false, fmt.Errorf("saml replay store: empty key")
	}
	if ttl <= 0 {
		return false, fmt.Errorf("saml replay store: non-positive ttl")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	if expiresAt, ok := s.seen[key]; ok && now.Before(expiresAt) {
		return false, nil
	}
	if len(s.seen) >= s.maxEntries {
		s.evictExpired(now)
		if len(s.seen) >= s.maxEntries {
			return false, fmt.Errorf("saml replay store: capacity exhausted")
		}
	}
	s.seen[key] = now.Add(ttl)
	return true, nil
}

func (s *MemoryReplayStore) evictExpired(now time.Time) {
	for key, expiresAt := range s.seen {
		if !now.Before(expiresAt) {
			delete(s.seen, key)
		}
	}
}
