// wire_saml_replay.go — SAML Response/Assertion one-time consumption store.
package bootstrap

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/guliangiam/guliangiam/internal/infra/cache"
	"github.com/guliangiam/guliangiam/internal/infra/saml"
)

const samlReplayKeyPrefix = "guliangiam:saml:replay:v1:"

// redisSAMLReplayStore uses SET NX so concurrent replicas cannot consume the
// same Response/Assertion ID twice. Redis errors propagate and the validator
// fails closed.
type redisSAMLReplayStore struct {
	client *redis.Client
}

func (s redisSAMLReplayStore) Reserve(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return s.client.SetNX(ctx, samlReplayKeyPrefix+key, "1", ttl).Result()
}

// newSAMLReplayStore reuses the session-cache Redis connection when one exists
// (shared across replicas); otherwise it falls back to process-local state.
func newSAMLReplayStore(sessionCache cache.SessionCache) saml.ReplayStore {
	if provider, ok := sessionCache.(interface{ RedisClient() *redis.Client }); ok {
		if client := provider.RedisClient(); client != nil {
			return redisSAMLReplayStore{client: client}
		}
	}
	slog.Warn("SAML replay protection uses process-local state",
		"reason", "session cache is not Redis-backed; multi-replica replay detection is unavailable")
	return saml.NewMemoryReplayStore()
}
