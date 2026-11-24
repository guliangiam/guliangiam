package cache

import (
	"context"
	"fmt"
	"time"

	domainsessions "github.com/guliangiam/guliangiam/internal/domain/sessions"
	"github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// sessionKeyPrefix is the Redis key prefix for session cache entries.
// Final key: "session:{sessionID}" — traceable, clear prefix, matches ADR-010.
const sessionKeyPrefix = "session:"

// SessionCache is the port for session-level caching.
// Two implementations: in-memory (local TTLCache) and Redis (distributed).
//
// Cache strategy: Cache-Aside + Write-Delete
//
//	Read:  Get -> miss -> DB GetByID -> Set
//	Write: DB mutation -> Delete (not Set, avoids CAS mismatch on Revocation)
type SessionCache interface {
	// Get returns cached session or (nil, false) on miss.
	Get(ctx context.Context, id string) (*domainsessions.Session, bool)

	// Set stores session with TTL derived from session.ExpiresAt.
	Set(ctx context.Context, s *domainsessions.Session)

	// Delete removes cached session by ID.
	Delete(ctx context.Context, id string)

	// InvalidateAll removes all cached sessions (use sparingly).
	InvalidateAll(ctx context.Context)
}

// DriverName identifies which cache backend to use.
type DriverName string

const (
	DriverMemory DriverName = "memory"
	DriverRedis  DriverName = "redis"
)

// FactoryConfig holds factory-level options; all fields optional.
type FactoryConfig struct {
	// Hooks is wired into *RedisSessionCache when driver=redis.
	// Leave nil for observability to be set externally via type-assertion.
	Hooks RedisHooks
}

// Factory returns the appropriate SessionCache based on environment.
//
// Env vars:
//
//	SESSION_CACHE_DRIVER        = memory | redis (default: memory)
//	REDIS_ADDR                  = host:port  (required for redis driver)
//	REDIS_PASSWORD              = optional
//	REDIS_DB                    = default 0
//	REDIS_MAX_RETRIES           = default 3
//	REDIS_POOL_SIZE             = default 0 (go-redis default: 10 * GOMAXPROCS)
//	CIRCUIT_FAILURE_THRESHOLD   = consecutive errors to open circuit, default 3
//	CIRCUIT_RECOVERY_INTERVAL   = Go duration string, default "5s"
//	REDIS_DIAL_TIMEOUT          = default "2s"
//	REDIS_READ_TIMEOUT          = default "2s"
//	REDIS_WRITE_TIMEOUT         = default "2s"
//
// ADR-010: Redis Cache for Sessions. Graceful degradation — if SESSION_CACHE_DRIVER
// is set to "redis" but Redis is unavailable, Factory falls back to memory AND
// returns an informational error (caller should log it, not panic).
func Factory() (SessionCache, DriverName, error) {
	return FactoryWithConfig(FactoryConfig{})
}

// FactoryWithConfig is the same as Factory but allows passing observability hooks.
func FactoryWithConfig(cfg FactoryConfig) (SessionCache, DriverName, error) {
	driver := DriverName(config.SessionCacheDriver())
	if driver == "" {
		driver = DriverMemory
	}

	switch driver {
	case DriverMemory:
		return NewMemorySessionCache(defaultMemoryTTL), driver, nil
	case DriverRedis:
		addr := config.RedisAddr()
		if addr == "" {
			return NewMemorySessionCache(defaultMemoryTTL), DriverMemory,
				apperrors.NewInternal("redis driver requested but REDIS_ADDR not set; falling back to memory")
		}
		opts := RedisOptions{
			Addr:                    addr,
			Password:                config.RedisPassword(),
			DB:                      config.RedisDB(),
			MaxRetries:              config.RedisMaxRetries(),
			PoolSize:                config.RedisPoolSize(),
			CircuitFailureThreshold: config.CircuitFailureThreshold(),
			CircuitRecoveryInterval: config.CircuitRecoveryInterval(),
			DialTimeout:             config.RedisDialTimeout(),
			ReadTimeout:             config.RedisReadTimeout(),
			WriteTimeout:            config.RedisWriteTimeout(),
		}
		c, err := NewRedisSessionCache(opts)
		if err != nil {
			return NewMemorySessionCache(defaultMemoryTTL), DriverMemory,
				apperrors.NewInternal(fmt.Sprintf("redis connection failed; falling back to memory: %v", err))
		}
		c.Hooks = cfg.Hooks
		return c, DriverRedis, nil
	default:
		return NewMemorySessionCache(defaultMemoryTTL), DriverMemory,
			apperrors.NewInternal(fmt.Sprintf("unknown SESSION_CACHE_DRIVER=%q; defaulting to memory", driver))
	}
}

// defaultMemoryTTL is the TTL for in-memory cache when session ExpiresAt
// is unreasonably far into the future (> 24h). Prevents unbounded memory.
var defaultMemoryTTL = 24 * time.Hour

func sessionKey(id string) string {
	return sessionKeyPrefix + id
}
