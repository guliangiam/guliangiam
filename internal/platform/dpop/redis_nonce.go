package dpop

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/redis/go-redis/v9"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const redisNonceKeyPrefix = "guliangiam:dpop:nonce:v1:"

// RedisNonceStore shares issued nonces across replicas (OPEN-API-86). It
// mirrors the SAML replay store pattern: SET NX issues a one-time entry with
// server-side TTL, GETDEL consumes it atomically so two replicas cannot accept
// the same nonce twice. Redis errors fail closed — an unverifiable nonce must
// not be treated as valid.
type RedisNonceStore struct {
	client *redis.Client
}

func NewRedisNonceStore(client *redis.Client) *RedisNonceStore {
	return &RedisNonceStore{client: client}
}

func (s *RedisNonceStore) Generate(ctx context.Context) (string, time.Duration, error) {
	buf := make([]byte, 32) // 256 bits → collision resistance 2^-128
	if _, err := rand.Read(buf); err != nil {
		return "", 0, apperrors.NewInternalf("generate DPoP nonce: %s", err.Error())
	}
	nonce := base64.RawURLEncoding.EncodeToString(buf)

	ok, err := s.client.SetNX(ctx, redisNonceKeyPrefix+nonce, "1", NonceTTL).Result()
	if err != nil {
		return "", 0, apperrors.WrapUnavailable(err, "DPoP nonce store unavailable")
	}
	if !ok {
		return "", 0, apperrors.NewInternal("DPoP nonce collision")
	}
	return nonce, NonceTTL, nil
}

func (s *RedisNonceStore) Verify(ctx context.Context, nonce string) error {
	if nonce == "" {
		return apperrors.NewInvalidArgument("DPoP nonce is required")
	}

	_, err := s.client.GetDel(ctx, redisNonceKeyPrefix+nonce).Result()
	if err == redis.Nil {
		// Never issued, already consumed, or TTL-expired — all reject.
		return apperrors.NewInvalidArgument("DPoP nonce not recognized")
	}
	if err != nil {
		return apperrors.WrapUnavailable(err, "DPoP nonce store unavailable")
	}
	return nil
}
