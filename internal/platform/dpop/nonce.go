package dpop

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
)

// ─── Nonce TTL / capacity ───────────────────────────────────────────────────

// NonceTTL is the server-side validity window for a DPoP nonce (RFC 9449 §4.2
// suggests short-lived nonces; 5 minutes keeps the server state small while
// tolerating client-side clock drift and request queuing).
const NonceTTL = 5 * time.Minute

// nonceMaxEntries caps the process-local store so a forgotten cleanup cannot
// leak memory. 10k entries ≈ 50ms of cache pressure; the cleanup loop runs
// every minute.
const nonceMaxEntries = 10000

// ─── NonceStore ─────────────────────────────────────────────────────────────

// NonceStore holds DPoP nonces the server has issued. Each entry is a
// one-time-use token valid for NonceTTL. A nonce is consumed atomically on
// first verification; subsequent uses of the same nonce fail.
//
// LocalNonceStore is the process-local implementation; RedisNonceStore shares
// state across replicas (OPEN-API-86, mirrors the SAML replay store pattern).
// Store failures must fail closed on the verification path.
type NonceStore interface {
	Generate(ctx context.Context) (string, time.Duration, error)
	Verify(ctx context.Context, nonce string) error
}

// LocalNonceStore is the process-local NonceStore. Single-instance only: a
// nonce issued by one replica is not recognized by another.
type LocalNonceStore struct {
	mu     sync.Mutex
	issued map[string]time.Time // nonce → issue time
}

// NewLocalNonceStore constructs a process-local store and starts its cleanup
// loop. Bootstrap wires exactly one instance into the auth Service when no
// shared Redis backend is available.
func NewLocalNonceStore() *LocalNonceStore {
	s := &LocalNonceStore{
		issued: make(map[string]time.Time),
	}
	recover.SafeGo("dpop-nonce-cleanup", s.cleanupLoop)
	return s
}

// Generate produces a new cryptographically random nonce and records it in the
// store so Verify will accept it once. Returns the nonce string + its TTL.
func (s *LocalNonceStore) Generate(_ context.Context) (string, time.Duration, error) {
	buf := make([]byte, 32) // 256 bits → base64url = 43 chars, collision resistance 2^-128
	if _, err := rand.Read(buf); err != nil {
		return "", 0, apperrors.NewInternalf("generate DPoP nonce: %s", err.Error())
	}
	nonce := base64.RawURLEncoding.EncodeToString(buf)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Evict if at capacity (least-recently-issued first) so we never OOM.
	s.evictLocked(time.Now())
	if len(s.issued) >= nonceMaxEntries {
		s.evictOldestLocked(nonceMaxEntries / 10) // drop oldest decile
	}

	s.issued[nonce] = time.Now()
	return nonce, NonceTTL, nil
}

// Verify checks that nonce was issued by this server and has not been used or
// expired. Returns nil on success and consumes the nonce so it cannot be
// replayed.
func (s *LocalNonceStore) Verify(_ context.Context, nonce string) error {
	if nonce == "" {
		return apperrors.NewInvalidArgument("DPoP nonce is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	issuedAt, ok := s.issued[nonce]
	if !ok {
		return apperrors.NewInvalidArgument("DPoP nonce not recognized")
	}
	if time.Since(issuedAt) > NonceTTL {
		delete(s.issued, nonce)
		return apperrors.NewInvalidArgument("DPoP nonce expired")
	}

	delete(s.issued, nonce) // one-time use
	return nil
}

// cleanupLoop runs periodically to drop expired nonces.
func (s *LocalNonceStore) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		s.evictLocked(time.Now())
		s.mu.Unlock()
	}
}

func (s *LocalNonceStore) evictLocked(now time.Time) {
	for nonce, issuedAt := range s.issued {
		if now.Sub(issuedAt) > NonceTTL {
			delete(s.issued, nonce)
		}
	}
}

func (s *LocalNonceStore) evictOldestLocked(budget int) {
	type entry struct {
		nonce    string
		issuedAt time.Time
	}
	entries := make([]entry, 0, len(s.issued))
	for n, t := range s.issued {
		entries = append(entries, entry{n, t})
	}
	// sort oldest first
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].issuedAt.Before(entries[i].issuedAt) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
	if budget > len(entries) {
		budget = len(entries)
	}
	for i := 0; i < budget; i++ {
		delete(s.issued, entries[i].nonce)
	}
}
