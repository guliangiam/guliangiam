package sessions

import (
	"context"
	"time"

	domainsessions "github.com/guliangiam/guliangiam/internal/domain/sessions"
	cacheinfra "github.com/guliangiam/guliangiam/internal/infra/cache"
)

// CachedRepository wraps a sessions.Repository with a SessionCache decorator.
//
// Strategy (per experience 100033611: trace every read/write/delete):
//
//	Read  (GetByID):   Cache-Aside — Get cache → miss → Get wrapped → Set cache
//	Write (Create):    DB Create → Set cache (session just created, cache immediately fresh)
//	Write (Update):    DB Update → Delete cache (avoid CAS mismatch on concurrent read)
//	Write (Touch):     DB Touch  → Delete cache (LastSeenAt updated)
//	Write (Revoke):    DB Revoke → Delete cache (critical: revoked session must not resurface)
//	Write (Delete):    DB Delete → Delete cache
//	Bulk Revokes:      DB Revoke* → InvalidateAll (bulk invalidation, simpler and correct)
//	CleanupExpired:    DB Cleanup → InvalidateAll (caches will naturally expire too, but be safe)
//
// Cache-Aside is NOT Write-Through because we want the DB to be the source of truth
// (ADR-002 PostgreSQL). Delete-on-Write ensures we never cache stale revocation/state.
type CachedRepository struct {
	// next is the underlying repository (typically postgres.Repository).
	next domainsessions.Repository
	// cache is the SessionCache implementation (memory or redis).
	cache cacheinfra.SessionCache
}

// NewCachedRepository creates a decorator wrapping `next` with cache.
// The returned object fully satisfies domainsessions.Repository.
func NewCachedRepository(next domainsessions.Repository, c cacheinfra.SessionCache) *CachedRepository {
	return &CachedRepository{next: next, cache: c}
}

// compile-time assertion
var _ domainsessions.Repository = (*CachedRepository)(nil)

// ──────────────────────────────────────────────────────────────────────
// READ PATH — Cache-Aside
// ──────────────────────────────────────────────────────────────────────

func (c *CachedRepository) GetByID(ctx context.Context, id string) (*domainsessions.Session, error) {
	// 1. Try cache
	if s, ok := c.cache.Get(ctx, id); ok {
		// Session in cache; double-check TTL vs ExpiresAt for safety.
		// (Redis handles server-side TTL, but memory cache may have longer default.)
		if !s.IsExpired(time.Now()) && !s.IsRevoked() {
			return s, nil
		}
		// Expired or revoked entry — delete and fall through to DB.
		c.cache.Delete(ctx, id)
	}

	// 2. Cache miss → DB
	s, err := c.next.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 3. Fill cache (only active sessions worth caching; expired/revoked will expire quickly anyway)
	if !s.IsExpired(time.Now()) && !s.IsRevoked() {
		c.cache.Set(ctx, s)
	}

	return s, nil
}

// GetByIDForOrganization is the tenant-scoped read used by management by-ID
// routes. A cached row is served only when it belongs to the requested tenant;
// anything else falls through to the scoped repository, which reports a
// foreign or missing row as not found.
func (c *CachedRepository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domainsessions.Session, error) {
	if s, ok := c.cache.Get(ctx, id); ok && s.OrganizationID == organizationID {
		if !s.IsExpired(time.Now()) && !s.IsRevoked() {
			return s, nil
		}
		c.cache.Delete(ctx, id)
	}

	s, err := c.next.GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}

	if !s.IsExpired(time.Now()) && !s.IsRevoked() {
		c.cache.Set(ctx, s)
	}

	return s, nil
}

// ──────────────────────────────────────────────────────────────────────
// WRITE PATHS — DB first, then Delete cache
// ──────────────────────────────────────────────────────────────────────

func (c *CachedRepository) Create(ctx context.Context, s *domainsessions.Session) error {
	// Keep the cached copy consistent with the column value the repository
	// derives when the domain leaves the assurance level unset.
	s.AssuranceLevel = normalizeAssuranceLevel(s.AssuranceLevel, s.AuthenticationLevel)
	if err := c.next.Create(ctx, s); err != nil {
		return err
	}
	// New session — cache it immediately (fresh, will be hot on next auth check).
	c.cache.Set(ctx, s)
	return nil
}

func (c *CachedRepository) Update(ctx context.Context, s *domainsessions.Session) error {
	if err := c.next.Update(ctx, s); err != nil {
		return err
	}
	// Delete, not Set — avoids CAS mismatch if another goroutine read between DB write and cache set.
	c.cache.Delete(ctx, s.ID)
	return nil
}

func (c *CachedRepository) Touch(ctx context.Context, id string, touchedAt time.Time) error {
	if err := c.next.Touch(ctx, id, touchedAt); err != nil {
		return err
	}
	c.cache.Delete(ctx, id)
	return nil
}

func (c *CachedRepository) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	if err := c.next.Revoke(ctx, id, revokedAt); err != nil {
		return err
	}
	// CRITICAL: delete revoked session from cache immediately.
	// Never cache a revoked session — that would allow a revoked user to continue.
	c.cache.Delete(ctx, id)
	return nil
}

func (c *CachedRepository) Delete(ctx context.Context, id string) error {
	if err := c.next.Delete(ctx, id); err != nil {
		return err
	}
	c.cache.Delete(ctx, id)
	return nil
}

// RevokeForOrganization revokes through the tenant-scoped predicate and then
// evicts the cached entry, so a revoked foreign probe cannot leave stale state.
func (c *CachedRepository) RevokeForOrganization(ctx context.Context, id, organizationID string, revokedAt time.Time) error {
	if err := c.next.RevokeForOrganization(ctx, id, organizationID, revokedAt); err != nil {
		return err
	}
	c.cache.Delete(ctx, id)
	return nil
}

// DeleteForOrganization deletes through the tenant-scoped predicate and then
// evicts the cached entry.
func (c *CachedRepository) DeleteForOrganization(ctx context.Context, id, organizationID string) error {
	if err := c.next.DeleteForOrganization(ctx, id, organizationID); err != nil {
		return err
	}
	c.cache.Delete(ctx, id)
	return nil
}

// ──────────────────────────────────────────────────────────────────────
// BULK PATHS — InvalidateAll (simpler and correct for bulk ops)
// ──────────────────────────────────────────────────────────────────────

func (c *CachedRepository) RevokeByUserExcept(ctx context.Context, userID, excludeSessionID string, revokedAt time.Time) (int64, error) {
	n, err := c.next.RevokeByUserExcept(ctx, userID, excludeSessionID, revokedAt)
	if err != nil {
		return 0, err
	}
	c.cache.InvalidateAll(ctx)
	return n, nil
}

func (c *CachedRepository) RevokeByUserAndApp(ctx context.Context, userID, applicationID, excludeSessionID string, revokedAt time.Time) (int64, error) {
	n, err := c.next.RevokeByUserAndApp(ctx, userID, applicationID, excludeSessionID, revokedAt)
	if err != nil {
		return 0, err
	}
	c.cache.InvalidateAll(ctx)
	return n, nil
}

func (c *CachedRepository) RevokeByUser(ctx context.Context, userID string, revokedAt time.Time) (int64, error) {
	n, err := c.next.RevokeByUser(ctx, userID, revokedAt)
	if err != nil {
		return 0, err
	}
	c.cache.InvalidateAll(ctx)
	return n, nil
}

func (c *CachedRepository) RevokeByOrganization(ctx context.Context, organizationID string, revokedAt time.Time) (int64, error) {
	n, err := c.next.RevokeByOrganization(ctx, organizationID, revokedAt)
	if err != nil {
		return 0, err
	}
	c.cache.InvalidateAll(ctx)
	return n, nil
}

// ──────────────────────────────────────────────────────────────────────
// LIST + CLEANUP — pass through; List is not hot path
// ──────────────────────────────────────────────────────────────────────

func (c *CachedRepository) List(ctx context.Context, query domainsessions.ListQuery) ([]*domainsessions.Session, int64, error) {
	return c.next.List(ctx, query)
}

func (c *CachedRepository) CleanupExpired(ctx context.Context, cutoff time.Time) (int64, error) {
	n, err := c.next.CleanupExpired(ctx, cutoff)
	if err != nil {
		return 0, err
	}
	c.cache.InvalidateAll(ctx)
	return n, nil
}
