package aiupstream

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newQuotaMeter(t *testing.T) (*RedisQuotaMeter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisQuotaMeter(client), mr
}

// ── Incr: key doesn't exist → INCRBY creates it with delta ────────────

func TestRedisQuotaMeter_Incr_KeyNotExist(t *testing.T) {
	m, mr := newQuotaMeter(t)

	val, err := m.Incr(context.Background(), "quota:rpm:api-key-1", 5, time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(5), val, "INCRBY on non-existent key returns delta")

	// Verify Redis state: count persisted
	got, err := m.Get(context.Background(), "quota:rpm:api-key-1")
	require.NoError(t, err)
	assert.Equal(t, int64(5), got)

	// Verify TTL set (within the window)
	ttl := mr.TTL("quota:rpm:api-key-1")
	assert.True(t, ttl > 0 && ttl <= time.Minute, "TTL must be >0 and ≤1min, got %v", ttl)
}

// ── Incr: subsequent calls accumulate ─────────────────────────────────

func TestRedisQuotaMeter_Incr_Accumulates(t *testing.T) {
	m, _ := newQuotaMeter(t)

	val1, err := m.Incr(context.Background(), "quota:rpm:api-key-2", 3, time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(3), val1)

	val2, err := m.Incr(context.Background(), "quota:rpm:api-key-2", 7, time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(10), val2, "3 + 7 = 10")

	val3, err := m.Incr(context.Background(), "quota:rpm:api-key-2", -2, time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(8), val3, "10 + (-2) = 8")
}

// ── Incr: TTL refreshed on each call ──────────────────────────────────

func TestRedisQuotaMeter_Incr_TTLRefresh(t *testing.T) {
	m, mr := newQuotaMeter(t)

	// First call with 10s window
	_, err := m.Incr(context.Background(), "quota:ttl", 1, 10*time.Second)
	require.NoError(t, err)

	// Advance miniredis clock by 9s (close to expiry)
	mr.FastForward(9 * time.Second)

	// Second call should refresh TTL
	_, err = m.Incr(context.Background(), "quota:ttl", 1, 10*time.Second)
	require.NoError(t, err)

	// After refresh + 9s more, key should still exist (10s total window from last call)
	mr.FastForward(9 * time.Second)
	got, err := m.Get(context.Background(), "quota:ttl")
	require.NoError(t, err)
	assert.True(t, got > 0, "key must survive past original window because TTL was refreshed, got %d", got)

	// After another full window, key expires
	mr.FastForward(10 * time.Second)
	got, err = m.Get(context.Background(), "quota:ttl")
	require.NoError(t, err)
	assert.Equal(t, int64(0), got, "key must be expired after full window since last refresh")
}

// ── Get: key doesn't exist → (0, nil), NOT redis.Nil ──────────────────

func TestRedisQuotaMeter_Get_KeyNotExist(t *testing.T) {
	m, _ := newQuotaMeter(t)

	val, err := m.Get(context.Background(), "quota:nonexistent")
	require.NoError(t, err)
	assert.Equal(t, int64(0), val, "missing key → 0, nil (not redis.Nil)")
}

// ── Get: populated key returns count ───────────────────────────────────

func TestRedisQuotaMeter_Get_Populated(t *testing.T) {
	m, _ := newQuotaMeter(t)

	_, _ = m.Incr(context.Background(), "quota:populated", 42, time.Minute)

	val, err := m.Get(context.Background(), "quota:populated")
	require.NoError(t, err)
	assert.Equal(t, int64(42), val)
}

// ── Client nil guard on Incr ───────────────────────────────────────────

func TestRedisQuotaMeter_Incr_NilClient(t *testing.T) {
	m := NewRedisQuotaMeter(nil)
	_, err := m.Incr(context.Background(), "quota:nil", 1, time.Minute)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

// ── Client nil guard on Get ────────────────────────────────────────────

func TestRedisQuotaMeter_Get_NilClient(t *testing.T) {
	m := NewRedisQuotaMeter(nil)
	_, err := m.Get(context.Background(), "quota:nil")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

// ── Pipeline failure: simulate Redis down mid-flight ────────────────────

func TestRedisQuotaMeter_Incr_PipelineError(t *testing.T) {
	m, mr := newQuotaMeter(t)

	// Close miniredis → connection lost
	mr.Close()

	_, err := m.Incr(context.Background(), "quota:down", 1, time.Minute)
	require.Error(t, err)
	// Should NOT be nil-client error; should be a real Redis error
	assert.NotContains(t, err.Error(), "client is nil")
}

func TestRedisQuotaMeter_Get_DisconnectedRedis(t *testing.T) {
	m, mr := newQuotaMeter(t)
	mr.Close()

	_, err := m.Get(context.Background(), "quota:down")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "client is nil")
}

// ── NewRedisQuotaMeter accepts client (we already test this above) ──────

func TestNewRedisQuotaMeter(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	m := NewRedisQuotaMeter(client)
	require.NotNil(t, m)
	assert.NotNil(t, m.client)
}

// ── Verify Get returns 0,nil not redis.Nil ─────────────────────────────

func TestRedisQuotaMeter_Get_ReturnsZeroNotRedisNil(t *testing.T) {
	m, _ := newQuotaMeter(t)
	_, err := m.Get(context.Background(), "quota:never-created")
	require.NoError(t, err, "must return nil error, not redis.Nil")
	assert.False(t, errors.Is(err, redis.Nil), "must NOT be redis.Nil — callers don't want to handle that")
}

// ── Edge: negative delta (decrement below zero) ───────────────────────

func TestRedisQuotaMeter_Incr_NegativeDelta(t *testing.T) {
	m, _ := newQuotaMeter(t)

	// Key starts at 0
	val, err := m.Incr(context.Background(), "quota:neg", -3, time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(-3), val, "Redis INCRBY allows negatives")
}

// ── TryIncr: first call within limit → allowed, count=delta ──────────

func TestRedisQuotaMeter_TryIncr_WithinLimit(t *testing.T) {
	m, mr := newQuotaMeter(t)

	allowed, count, err := m.TryIncr(context.Background(), "rpm:key-1", 1, 60, time.Minute)
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, int64(1), count, "cur=0+1=1")

	// Redis state persisted
	got, _ := m.Get(context.Background(), "rpm:key-1")
	assert.Equal(t, int64(1), got)

	// TTL set
	ttl := mr.TTL("rpm:key-1")
	assert.True(t, ttl > 0 && ttl <= time.Minute)
}

// ── TryIncr: exactly at limit → allowed (cur+delta == limit) ─────────

func TestRedisQuotaMeter_TryIncr_ExactlyAtLimit(t *testing.T) {
	m, _ := newQuotaMeter(t)

	// First fill up to 59
	for i := 0; i < 59; i++ {
		allowed, _, err := m.TryIncr(context.Background(), "rpm:key-2", 1, 60, time.Minute)
		require.NoError(t, err)
		assert.True(t, allowed, "call %d should be allowed", i+1)
	}
	// 60th call should still be allowed (59+1 == 60)
	allowed, count, err := m.TryIncr(context.Background(), "rpm:key-2", 1, 60, time.Minute)
	require.NoError(t, err)
	assert.True(t, allowed, "exactly at limit should be allowed")
	assert.Equal(t, int64(60), count)

	// 61st call — not allowed
	allowed, count, err = m.TryIncr(context.Background(), "rpm:key-2", 1, 60, time.Minute)
	require.NoError(t, err)
	assert.False(t, allowed, "over limit should be denied")
	assert.Equal(t, int64(60), count, "count should NOT have incremented")

	// Redis state still 60
	got, _ := m.Get(context.Background(), "rpm:key-2")
	assert.Equal(t, int64(60), got)
}

// ── TryIncr: denial does NOT mutate the counter ────────────────────────

func TestRedisQuotaMeter_TryIncr_DeniedDoesNotIncrement(t *testing.T) {
	m, _ := newQuotaMeter(t)

	// Pre-fill to 10
	_, _ = m.Incr(context.Background(), "rpm:key-3", 10, time.Minute)

	// TryIncr with limit=10, delta=1 → denied
	allowed, count, err := m.TryIncr(context.Background(), "rpm:key-3", 1, 10, time.Minute)
	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, int64(10), count, "count unchanged")

	// Verify key still 10
	got, _ := m.Get(context.Background(), "rpm:key-3")
	assert.Equal(t, int64(10), got)
}

// ── TryIncr: larger delta than remaining headroom → denied ────────────

func TestRedisQuotaMeter_TryIncr_LargeDeltaDenied(t *testing.T) {
	m, _ := newQuotaMeter(t)

	allowed, count, err := m.TryIncr(context.Background(), "rpm:key-4", 5, 3, time.Minute)
	require.NoError(t, err)
	assert.False(t, allowed, "5 > 3 should be denied")
	assert.Equal(t, int64(0), count, "key never created — cur=0")
}

// ── TryIncr: TTL refreshed on each allowed call ───────────────────────

func TestRedisQuotaMeter_TryIncr_TTLRefresh(t *testing.T) {
	m, mr := newQuotaMeter(t)

	_, _, _ = m.TryIncr(context.Background(), "quota:ttl2", 1, 100, 10*time.Second)

	// Advance 9s
	mr.FastForward(9 * time.Second)

	// Another allowed call refreshes TTL
	allowed, _, err := m.TryIncr(context.Background(), "quota:ttl2", 1, 100, 10*time.Second)
	require.NoError(t, err)
	assert.True(t, allowed)

	// After another 9s (18s total from first call), key should still exist
	mr.FastForward(9 * time.Second)
	got, _ := m.Get(context.Background(), "quota:ttl2")
	assert.True(t, got > 0, "key must survive past original TTL because allowed call refreshed it")
}

// ── TryIncr: nil client → error ────────────────────────────────────────

func TestRedisQuotaMeter_TryIncr_NilClient(t *testing.T) {
	m := NewRedisQuotaMeter(nil)
	allowed, _, err := m.TryIncr(context.Background(), "x", 1, 10, time.Minute)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
	assert.False(t, allowed)
}

// ── TryIncr: Redis disconnected → error ────────────────────────────────

func TestRedisQuotaMeter_TryIncr_Disconnected(t *testing.T) {
	m, mr := newQuotaMeter(t)
	mr.Close()

	allowed, _, err := m.TryIncr(context.Background(), "down", 1, 10, time.Minute)
	require.Error(t, err)
	assert.False(t, allowed)
	assert.NotContains(t, err.Error(), "client is nil")
}
