package dpop

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

var (
	_ NonceStore = (*LocalNonceStore)(nil)
	_ NonceStore = (*RedisNonceStore)(nil)
)

func newRedisNonceClient(t *testing.T, mr *miniredis.Miniredis) *redis.Client {
	t.Helper()
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func nonceErrorCode(t *testing.T, err error) apperrors.Code {
	t.Helper()
	var appErr *apperrors.Error
	if !stderrors.As(err, &appErr) {
		t.Fatalf("expected apperrors.Error, got %T: %v", err, err)
	}
	return appErr.Code
}

// 跨副本互认：副本 A 签发的 nonce 必须能被副本 B 消费（OPEN-API-86 关闭条件）。
func TestRedisNonceStore_CrossReplicaRecognition(t *testing.T) {
	mr := miniredis.RunT(t)
	replicaA := NewRedisNonceStore(newRedisNonceClient(t, mr))
	replicaB := NewRedisNonceStore(newRedisNonceClient(t, mr))
	ctx := context.Background()

	nonce, ttl, err := replicaA.Generate(ctx)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if nonce == "" || ttl != NonceTTL {
		t.Fatalf("unexpected nonce/ttl: %q %v", nonce, ttl)
	}
	if err := replicaB.Verify(ctx, nonce); err != nil {
		t.Fatalf("cross-replica verify: %v", err)
	}
}

// 一次性消费：同一 nonce 第二次校验必须拒绝（GETDEL 原子性）。
func TestRedisNonceStore_OneTimeUse(t *testing.T) {
	mr := miniredis.RunT(t)
	store := NewRedisNonceStore(newRedisNonceClient(t, mr))
	ctx := context.Background()

	nonce, _, err := store.Generate(ctx)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := store.Verify(ctx, nonce); err != nil {
		t.Fatalf("first verify: %v", err)
	}
	err = store.Verify(ctx, nonce)
	if err == nil {
		t.Fatal("second verify must fail")
	}
	if code := nonceErrorCode(t, err); code != apperrors.CodeInvalidArgument {
		t.Fatalf("expected invalid_argument, got %v", code)
	}
}

// TTL 过期：超过 NonceTTL 后 nonce 不再被识别。
func TestRedisNonceStore_TTLExpiry(t *testing.T) {
	mr := miniredis.RunT(t)
	store := NewRedisNonceStore(newRedisNonceClient(t, mr))
	ctx := context.Background()

	nonce, _, err := store.Generate(ctx)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	mr.FastForward(NonceTTL + time.Second)
	err = store.Verify(ctx, nonce)
	if err == nil {
		t.Fatal("expired nonce must fail")
	}
	if code := nonceErrorCode(t, err); code != apperrors.CodeInvalidArgument {
		t.Fatalf("expected invalid_argument, got %v", code)
	}
}

// 未签发/空 nonce 拒绝。
func TestRedisNonceStore_UnknownAndEmptyNonce(t *testing.T) {
	mr := miniredis.RunT(t)
	store := NewRedisNonceStore(newRedisNonceClient(t, mr))
	ctx := context.Background()

	if err := store.Verify(ctx, "never-issued"); err == nil {
		t.Fatal("unknown nonce must fail")
	}
	if err := store.Verify(ctx, ""); err == nil {
		t.Fatal("empty nonce must fail")
	}
}

// fail-close：Redis 不可用时 Verify 必须拒绝（Unavailable），不得放行。
func TestRedisNonceStore_VerifyFailsClosedOnRedisError(t *testing.T) {
	mr := miniredis.RunT(t)
	store := NewRedisNonceStore(newRedisNonceClient(t, mr))
	ctx := context.Background()

	nonce, _, err := store.Generate(ctx)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	mr.Close()
	err = store.Verify(ctx, nonce)
	if err == nil {
		t.Fatal("verify must fail when Redis is down")
	}
	if code := nonceErrorCode(t, err); code != apperrors.CodeUnavailable {
		t.Fatalf("expected unavailable, got %v", code)
	}
}

// fail-close：Redis 不可用时 Generate 上抛 Unavailable，不签发无法共享的 nonce。
func TestRedisNonceStore_GenerateFailsClosedOnRedisError(t *testing.T) {
	mr := miniredis.RunT(t)
	store := NewRedisNonceStore(newRedisNonceClient(t, mr))
	mr.Close()

	_, _, err := store.Generate(context.Background())
	if err == nil {
		t.Fatal("generate must fail when Redis is down")
	}
	if code := nonceErrorCode(t, err); code != apperrors.CodeUnavailable {
		t.Fatalf("expected unavailable, got %v", code)
	}
}
