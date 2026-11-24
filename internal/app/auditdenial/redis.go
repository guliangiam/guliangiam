package auditdenial

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const redisDedupScript = `
local n = redis.call('INCR', KEYS[1])
if n == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
return n
`

type redisDeduper struct {
	client   *redis.Client
	fallback Deduper
}

// NewRedisDeduper shares the denial dedup window across replicas. Redis errors
// fall back to the in-process deduper so audit never blocks the request path.
func NewRedisDeduper(client *redis.Client, fallback Deduper) Deduper {
	if client == nil {
		return fallback
	}
	return &redisDeduper{client: client, fallback: fallback}
}

func (d *redisDeduper) Hit(ctx context.Context, key string, window time.Duration) (int, bool) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	count, err := d.client.Eval(timeoutCtx, redisDedupScript, []string{"audit:denial:" + key}, window.Milliseconds()).Int()
	if err != nil || count <= 0 {
		if d.fallback != nil {
			return d.fallback.Hit(ctx, key, window)
		}
		return 1, false
	}
	return count, count > 1
}
