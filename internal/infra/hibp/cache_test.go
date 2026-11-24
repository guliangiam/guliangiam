package hibp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestLocalRangeCache_SetGet(t *testing.T) {
	c := NewLocalRangeCache()
	ctx := context.Background()

	_, found, err := c.Get(ctx, "AAAAA")
	require.NoError(t, err)
	assert.False(t, found)

	require.NoError(t, c.Set(ctx, "AAAAA", "SUFFIX:1"))
	body, found, err := c.Get(ctx, "AAAAA")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "SUFFIX:1", body)
}

func TestLocalRangeCache_TTLExpiry(t *testing.T) {
	c := NewLocalRangeCache()
	ctx := context.Background()
	require.NoError(t, c.Set(ctx, "AAAAA", "SUFFIX:1"))

	c.mu.Lock()
	entry := c.entries["AAAAA"]
	entry.expiresAt = time.Now().Add(-time.Second)
	c.entries["AAAAA"] = entry
	c.mu.Unlock()

	_, found, err := c.Get(ctx, "AAAAA")
	require.NoError(t, err)
	assert.False(t, found, "过期项必须视为未命中")
}

func TestLocalRangeCache_CapacityBounded(t *testing.T) {
	c := NewLocalRangeCache()
	ctx := context.Background()
	for i := 0; i < localCacheMaxEntries+10; i++ {
		require.NoError(t, c.Set(ctx, fmt.Sprintf("P%04d", i), "SUFFIX:1"))
	}
	c.mu.Lock()
	size := len(c.entries)
	c.mu.Unlock()
	assert.LessOrEqual(t, size, localCacheMaxEntries, "容量必须有界")
}

func newRangeRedisClient(t *testing.T, mr *miniredis.Miniredis) *redis.Client {
	t.Helper()
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestRedisRangeCache_SetGetTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	c := NewRedisRangeCache(newRangeRedisClient(t, mr))
	ctx := context.Background()

	_, found, err := c.Get(ctx, "AAAAA")
	require.NoError(t, err)
	assert.False(t, found, "redis.Nil 必须是未命中而非错误")

	require.NoError(t, c.Set(ctx, "AAAAA", "SUFFIX:1"))
	body, found, err := c.Get(ctx, "AAAAA")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "SUFFIX:1", body)
	assert.Equal(t, RangeCacheTTL, mr.TTL(redisKeyPrefix+"AAAAA"))
}

// 跨副本共享：副本 A 缓存的 range 响应必须能被副本 B 命中。
func TestRedisRangeCache_CrossReplicaSharing(t *testing.T) {
	mr := miniredis.RunT(t)
	replicaA := NewRedisRangeCache(newRangeRedisClient(t, mr))
	replicaB := NewRedisRangeCache(newRangeRedisClient(t, mr))
	ctx := context.Background()

	require.NoError(t, replicaA.Set(ctx, "BBBBB", "SUFFIX:2"))
	body, found, err := replicaB.Get(ctx, "BBBBB")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "SUFFIX:2", body)
}

func TestRedisRangeCache_ErrorIsUnavailable(t *testing.T) {
	mr := miniredis.RunT(t)
	client := newRangeRedisClient(t, mr)
	c := NewRedisRangeCache(client)
	mr.Close()

	ctx := context.Background()
	_, _, err := c.Get(ctx, "AAAAA")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnavailable, appErr.Code)

	err = c.Set(ctx, "AAAAA", "SUFFIX:1")
	require.Error(t, err)
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnavailable, appErr.Code)
}
