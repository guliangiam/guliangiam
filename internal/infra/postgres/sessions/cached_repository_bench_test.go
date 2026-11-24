package sessions

import (
	"context"
	"sync"
	"testing"
	"time"

	domainsessions "github.com/guliangiam/guliangiam/internal/domain/sessions"
	cacheinfra "github.com/guliangiam/guliangiam/internal/infra/cache"
)

// ─────────────────────────────────────────────────────────────────────
// A4. Session Redis Cache — Benchmark
//
// Measures CachedRepository.GetByID vs Postgres Repository.GetByID
// (simulated by in-memory map repo — same as existing benchmark pattern).
//
// Goal: cache hit < 1µs (vs DB 13ms = 13000x faster)
// ─────────────────────────────────────────────────────────────────────

type benchRepo struct {
	mu       sync.RWMutex
	sessions map[string]*domainsessions.Session
}

func newBenchRepo(count int) *benchRepo {
	r := &benchRepo{sessions: make(map[string]*domainsessions.Session, count)}
	now := time.Now()
	for i := 0; i < count; i++ {
		id := "sess-" + string(rune('a'+i%26)) + "-" + itoa(i)
		r.sessions[id] = &domainsessions.Session{
			ID:               id,
			UserID:           "user-" + itoa(i%100),
			Username:         "user" + itoa(i%100),
			OrganizationID:   "org-1",
			OrganizationSlug: "org1",
			ApplicationID:    "app-1",
			ApplicationSlug:  "app1",
			SessionState:     "active",
			Sid:              "oidc-sid-" + itoa(i),
			DeviceInfo:       map[string]any{"ua": "chrome/120"},
			IPAddress:        "192.168.1." + itoa(i%254),
			UserAgent:        "Mozilla/5.0",
			ExpiresAt:        now.Add(24 * time.Hour),
			CreatedAt:        now,
			LastSeenAt:       now,
			UpdatedAt:        now,
			Revision:         1,
		}
	}
	return r
}

func (r *benchRepo) GetByID(_ context.Context, id string) (*domainsessions.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[id]
	if !ok {
		return nil, errNotFound
	}
	return s, nil
}

func (r *benchRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domainsessions.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[id]
	if !ok || s.OrganizationID != organizationID {
		return nil, errNotFound
	}
	return s, nil
}

// stub out other methods (not used in benchmark)
func (r *benchRepo) List(context.Context, domainsessions.ListQuery) ([]*domainsessions.Session, int64, error) {
	return nil, 0, nil
}
func (r *benchRepo) Create(context.Context, *domainsessions.Session) error { return nil }
func (r *benchRepo) Update(context.Context, *domainsessions.Session) error { return nil }
func (r *benchRepo) Touch(context.Context, string, time.Time) error        { return nil }
func (r *benchRepo) Revoke(context.Context, string, time.Time) error       { return nil }
func (r *benchRepo) RevokeForOrganization(context.Context, string, string, time.Time) error {
	return nil
}
func (r *benchRepo) RevokeByUserExcept(context.Context, string, string, time.Time) (int64, error) {
	return 0, nil
}
func (r *benchRepo) RevokeByUserAndApp(context.Context, string, string, string, time.Time) (int64, error) {
	return 0, nil
}
func (r *benchRepo) RevokeByUser(context.Context, string, time.Time) (int64, error) { return 0, nil }
func (r *benchRepo) RevokeByOrganization(context.Context, string, time.Time) (int64, error) {
	return 0, nil
}
func (r *benchRepo) Delete(context.Context, string) error                        { return nil }
func (r *benchRepo) DeleteForOrganization(context.Context, string, string) error { return nil }
func (r *benchRepo) CleanupExpired(context.Context, time.Time) (int64, error)    { return 0, nil }

var errNotFound = &testError{}

type testError struct{}

func (*testError) Error() string { return "not found" }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// countingSessionCache wraps a SessionCache and records Get outcomes so a
// benchmark can prove the workload it actually measured. Benchmarks are
// single-goroutine, so plain counters are enough (no atomics in the hot path).
type countingSessionCache struct {
	inner cacheinfra.SessionCache
	gets  int
	hits  int
}

func (c *countingSessionCache) Get(ctx context.Context, id string) (*domainsessions.Session, bool) {
	c.gets++
	s, ok := c.inner.Get(ctx, id)
	if ok {
		c.hits++
	}
	return s, ok
}

func (c *countingSessionCache) Set(ctx context.Context, s *domainsessions.Session) {
	c.inner.Set(ctx, s)
}

func (c *countingSessionCache) Delete(ctx context.Context, id string) {
	c.inner.Delete(ctx, id)
}

func (c *countingSessionCache) InvalidateAll(ctx context.Context) {
	c.inner.InvalidateAll(ctx)
}

func reportCacheHitRate(b *testing.B, c *countingSessionCache) {
	if c.gets == 0 {
		return
	}
	b.ReportMetric(float64(c.hits)*100/float64(c.gets), "%hit")
}

// sessionIDs returns up to limit ids from the fake repo's key set.
func sessionIDs(r *benchRepo, limit int) []string {
	ids := make([]string, 0, limit)
	for id := range r.sessions {
		ids = append(ids, id)
		if len(ids) >= limit {
			break
		}
	}
	return ids
}

// ───── Benchmarks ─────

// BenchmarkSessionCacheThroughput — single key hot path (cache hit after warmup)
func BenchmarkSessionCacheThroughput(b *testing.B) {
	repo := newBenchRepo(1000)
	cache := cacheinfra.NewMemorySessionCache(24 * time.Hour)
	cached := NewCachedRepository(repo, cache)
	ctx := context.Background()

	// Warmup: populate cache
	for id := range repo.sessions {
		_, _ = cached.GetByID(ctx, id)
	}

	// Benchmark: same id every iteration (hot cache hit)
	hotID := "sess-a-0"
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = cached.GetByID(ctx, hotID)
	}
}

// missKeyPoolSize bounds the pre-generated key pool so the benchmark does not
// allocate b.N strings. The cache is cleared before every pool pass, outside
// the timed region, so each lookup really is a cold-cache miss.
const missKeyPoolSize = 4096

// BenchmarkSessionCacheMiss — cold cache: Get miss → repo read → cache fill.
//
// The pre-fix version cycled a fixed 5,000-key set in an endless loop; after the
// first pass every key was cached, so it reported a cache-hit number under a
// "miss" name (audit §3.3.3 / §5.3). This version resets the cache between
// bounded passes and reports %hit so the workload cannot silently drift back to
// hits: the expected value here is 0.
func BenchmarkSessionCacheMiss(b *testing.B) {
	repo := newBenchRepo(10000)
	inner := cacheinfra.NewMemorySessionCache(24 * time.Hour)
	cache := &countingSessionCache{inner: inner}
	cached := NewCachedRepository(repo, cache)
	ctx := context.Background()
	ids := sessionIDs(repo, missKeyPoolSize)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%len(ids) == 0 {
			// Reset outside the timed window: the map rebuild is setup, not
			// part of the measured GetByID miss path.
			b.StopTimer()
			cache.InvalidateAll(ctx)
			b.StartTimer()
		}
		_, _ = cached.GetByID(ctx, ids[i%len(ids)])
	}
	b.StopTimer()
	reportCacheHitRate(b, cache)
}

// BenchmarkSessionCacheMissLegacyCycle reproduces the pre-fix construction
// (5,000 ids cycled forever, no cache reset) and reports %hit. It is kept only
// as the "before" control for the E3-d comparison; it is not a miss benchmark.
func BenchmarkSessionCacheMissLegacyCycle(b *testing.B) {
	repo := newBenchRepo(10000)
	cache := &countingSessionCache{inner: cacheinfra.NewMemorySessionCache(24 * time.Hour)}
	cached := NewCachedRepository(repo, cache)
	ctx := context.Background()
	ids := sessionIDs(repo, 5000)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cached.GetByID(ctx, ids[i%len(ids)])
	}
	b.StopTimer()
	reportCacheHitRate(b, cache)
}

// BenchmarkSessionCacheRevoke — cache Delete after DB Revoke
func BenchmarkSessionCacheRevoke(b *testing.B) {
	repo := newBenchRepo(100)
	cache := cacheinfra.NewMemorySessionCache(24 * time.Hour)
	cached := NewCachedRepository(repo, cache)
	ctx := context.Background()
	now := time.Now()

	// Warmup
	for id := range repo.sessions {
		_, _ = cached.GetByID(ctx, id)
	}

	ids := make([]string, 0, 50)
	for id := range repo.sessions {
		ids = append(ids, id)
		if len(ids) >= 50 {
			break
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		id := ids[i%len(ids)]
		_ = cached.Revoke(ctx, id, now)
	}
}
