package local

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/ratelimit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllow_PassThrough(t *testing.T) {
	l := NewLimiter(5*time.Minute, 2*time.Minute)
	defer l.Close()

	q := domain.Quota{Limit: 100, Burst: 10, PolicyID: "test"}
	dec, err := l.Allow(context.Background(), "key-1", 1, q)
	require.NoError(t, err)
	assert.True(t, dec.Allowed)
	assert.Equal(t, "test", dec.PolicyID)
}

func TestAllow_ExhaustBurst(t *testing.T) {
	l := NewLimiter(5*time.Minute, 2*time.Minute)
	defer l.Close()

	q := domain.Quota{Limit: 100, Burst: 3}
	ctx := context.Background()

	// 连续消耗 3 个令牌（burst=3）
	for i := 0; i < 3; i++ {
		dec, err := l.Allow(ctx, "key-burst", 1, q)
		require.NoError(t, err)
		assert.True(t, dec.Allowed)
	}

	// 第 4 个应该被拒绝
	dec, err := l.Allow(ctx, "key-burst", 1, q)
	require.NoError(t, err)
	assert.False(t, dec.Allowed)
	assert.True(t, dec.ResetAfter > 0, "应该有 ResetAfter 让客户端知道等多久")
}

func TestAllow_DifferentKeys_Independent(t *testing.T) {
	l := NewLimiter(5*time.Minute, 2*time.Minute)
	defer l.Close()

	q := domain.Quota{Limit: 100, Burst: 1}
	ctx := context.Background()

	// key-a 用完 burst
	dec1, _ := l.Allow(ctx, "a", 1, q)
	assert.True(t, dec1.Allowed)
	dec1, _ = l.Allow(ctx, "a", 1, q)
	assert.False(t, dec1.Allowed)

	// key-b 独立计数
	dec2, _ := l.Allow(ctx, "b", 1, q)
	assert.True(t, dec2.Allowed, "不同 key 应该独立计数")
}

func TestAllow_KeyPrefix(t *testing.T) {
	l := NewLimiter(5*time.Minute, 2*time.Minute)
	defer l.Close()

	qIP := domain.Quota{Limit: 100, Burst: 1, KeyPrefix: "ip:", PolicyID: "auth-ip"}
	qUser := domain.Quota{Limit: 100, Burst: 1, KeyPrefix: "user:", PolicyID: "auth-user"}

	ctx := context.Background()
	dec1, _ := l.Allow(ctx, "10.0.0.1", 1, qIP)
	assert.True(t, dec1.Allowed)
	dec2, _ := l.Allow(ctx, "alice", 1, qUser)
	assert.True(t, dec2.Allowed, "不同 prefix 下同名 key 不冲突")
}
