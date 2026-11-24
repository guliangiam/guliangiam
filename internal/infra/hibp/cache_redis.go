package hibp

import (
	"context"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/redis/go-redis/v9"
)

const redisKeyPrefix = "guliangiam:breach:range:v1:"

// RedisRangeCache shares range bodies across replicas, reusing the
// session-cache Redis connection (mirrors the SAML replay / DPoP nonce
// wiring). Request-level errors surface to the checker, which falls through
// to the origin; origin failure fails open at the app layer.
type RedisRangeCache struct {
	client *redis.Client
}

func NewRedisRangeCache(client *redis.Client) *RedisRangeCache {
	return &RedisRangeCache{client: client}
}

func (c *RedisRangeCache) Get(ctx context.Context, prefix string) (string, bool, error) {
	body, err := c.client.Get(ctx, redisKeyPrefix+prefix).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, apperrors.WrapUnavailable(err, "breach range cache GET")
	}
	return body, true, nil
}

func (c *RedisRangeCache) Set(ctx context.Context, prefix, body string) error {
	if err := c.client.Set(ctx, redisKeyPrefix+prefix, body, RangeCacheTTL).Err(); err != nil {
		return apperrors.WrapUnavailable(err, "breach range cache SET")
	}
	return nil
}
