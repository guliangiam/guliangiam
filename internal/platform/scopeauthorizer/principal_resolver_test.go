package scopeauthorizer

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/require"
)

var errResolverTestUnused = errors.New("fake platform admin repository: method unused by principal resolution")

// mutablePlatformAdminRepo lets the TTL test deactivate an administrator
// between Resolve calls. Only GetByUserID participates in classification.
type mutablePlatformAdminRepo struct {
	mu      sync.Mutex
	admin   *platformadmins.PlatformAdmin
	queries int
}

func (r *mutablePlatformAdminRepo) setActive(active bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.admin.IsActive = active
}

func (r *mutablePlatformAdminRepo) queryCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.queries
}

func (r *mutablePlatformAdminRepo) GetByUserID(context.Context, string) (*platformadmins.PlatformAdmin, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queries++
	if r.admin == nil {
		return nil, apperrors.NewNotFound("platform admin not found")
	}
	admin := *r.admin
	return &admin, nil
}

func (*mutablePlatformAdminRepo) GetByID(context.Context, string) (*platformadmins.PlatformAdmin, error) {
	return nil, errResolverTestUnused
}

func (*mutablePlatformAdminRepo) List(context.Context, platformadmins.ListOptions) ([]*platformadmins.PlatformAdmin, int64, error) {
	return nil, 0, errResolverTestUnused
}

func (*mutablePlatformAdminRepo) Create(context.Context, *platformadmins.PlatformAdmin) error {
	return errResolverTestUnused
}

func (*mutablePlatformAdminRepo) Update(context.Context, *platformadmins.PlatformAdmin) error {
	return errResolverTestUnused
}

func (*mutablePlatformAdminRepo) SetActive(context.Context, string, bool) error {
	return errResolverTestUnused
}

func (*mutablePlatformAdminRepo) Delete(context.Context, string) error {
	return errResolverTestUnused
}

// fakeClock is the injected clock seam for the TTL test. Advancing it moves
// the resolver across cache boundaries deterministically, with no sleep.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// TestCachedDBResolverTTLBoundary proves the 60 second principal cache keeps
// serving stale classifications for a deactivated administrator and re-reads
// platform_admins only after the entry expires.
func TestCachedDBResolverTTLBoundary(t *testing.T) {
	const userID = "11111111-1111-4111-8111-111111111111"

	require.NotNil(t, NewCachedDBResolver(nil, nil).now, "nil now must default to time.Now")

	clk := &fakeClock{now: time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)}
	repo := &mutablePlatformAdminRepo{
		admin: &platformadmins.PlatformAdmin{ID: "pa", UserID: userID, IsActive: true},
	}
	resolver := NewCachedDBResolver(repo, clk.Now)
	require.Equal(t, 60*time.Second, resolver.ttl, "the documented cache window must stay 60s")

	ctx := context.Background()
	require.Equal(t, "platform_admin", resolver.Resolve(ctx, userID))
	require.Equal(t, 1, repo.queryCount())

	// The administrator is deactivated inside the validity window.
	repo.setActive(false)

	// 59s after the first resolve the entry is still valid: the stale
	// platform_admin result wins and the repository is not consulted again.
	clk.Advance(59 * time.Second)
	require.Equal(t, "platform_admin", resolver.Resolve(ctx, userID))
	require.Equal(t, 1, repo.queryCount(),
		"59s is inside the 60s TTL, so the cache must answer without a repository read")

	// 61s after the first resolve the entry has expired: the resolver re-reads
	// platform_admins, sees the deactivated row and drops the admin.
	clk.Advance(2 * time.Second)
	require.Equal(t, "user", resolver.Resolve(ctx, userID))
	require.Equal(t, 2, repo.queryCount(),
		"61s is past the 60s TTL, so the resolver must re-read platform_admins")

	// The refreshed "user" entry starts a new 60s window: 59s later it is
	// still cached.
	clk.Advance(59 * time.Second)
	require.Equal(t, "user", resolver.Resolve(ctx, userID))
	require.Equal(t, 2, repo.queryCount(),
		"the cache miss must restart the 60s window")

	// 61s after the re-resolve the refreshed entry expires and re-reads again.
	clk.Advance(2 * time.Second)
	require.Equal(t, "user", resolver.Resolve(ctx, userID))
	require.Equal(t, 3, repo.queryCount())
}

func TestCachedDBResolverInvalidateDropsCachedClassification(t *testing.T) {
	admin := &platformadmins.PlatformAdmin{ID: "pa-1", UserID: "user-1", IsActive: true}
	repo := &mutablePlatformAdminRepo{admin: admin}
	resolver := NewCachedDBResolver(repo, func() time.Time { return time.Unix(1000, 0) })

	if got := resolver.Resolve(context.Background(), "user-1"); got != "platform_admin" {
		t.Fatalf("Resolve = %q, want platform_admin", got)
	}
	repo.setActive(false)
	if got := resolver.Resolve(context.Background(), "user-1"); got != "platform_admin" {
		t.Fatalf("cached Resolve = %q, want platform_admin until invalidation", got)
	}
	resolver.Invalidate("user-1")
	if got := resolver.Resolve(context.Background(), "user-1"); got != "user" {
		t.Fatalf("Resolve after invalidate = %q, want user", got)
	}
}
