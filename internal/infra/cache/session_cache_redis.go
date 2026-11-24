package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"

	domainsessions "github.com/guliangiam/guliangiam/internal/domain/sessions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
)

// RedisOptions configures the Redis client.
type RedisOptions struct {
	Addr     string
	Password string
	DB       int

	// --- Advanced tuning (optional) ---

	// MaxRetries Redis command retry count (default 3).
	MaxRetries int

	// PoolSize Redis connection pool max size (default 10 * GOMAXPROCS).
	PoolSize int

	// CircuitFailureThreshold consecutive Redis errors before circuit opens
	// (default 3). When open, all operations return miss immediately — prevents
	// storming Redis during brief outages while DB absorbs the load.
	CircuitFailureThreshold int

	// CircuitRecoveryInterval how often to ping Redis for recovery when open
	// (default 5s).
	CircuitRecoveryInterval time.Duration

	// DialTimeout Redis connection dial timeout (default 2s).
	DialTimeout time.Duration

	// ReadTimeout Redis read timeout (default 2s).
	ReadTimeout time.Duration

	// WriteTimeout Redis write timeout (default 2s).
	WriteTimeout time.Duration
}

// RedisSessionCache is a distributed SessionCache backed by Redis.
//
// Key format: session:{sessionID}
// Value: JSON-encoded domainsessions.Session
// TTL: derived from session.ExpiresAt (capped at 7d max to prevent runaway)
//
// Design notes:
//   - JSON instead of gob: zero global lock, standard library, Session.DeviceInfo
//     is map[string]any which JSON handles natively.
//   - sync.Pool[*bytes.Buffer] for encode/decode buffers — reduces GC pressure
//     from per-op allocation.
//   - Fail-open: Get returns (nil, false) on any Redis error. Set/Delete silently
//     swallow errors. Hot path stays alive.
//   - Circuit breaker: consecutive Redis errors > threshold opens the circuit.
//     Open = immediate miss on every op, no Redis calls at all. A background
//     ticker pings Redis every CircuitRecoveryInterval; success closes circuit.
//
// Observability hooks (set via RedisSessionCache.Hooks, nil-safe):
//   - GetHit / GetMiss / GetError fire on every Get result
//   - SetError / DeleteError fire on write failures
//   - CircuitOpen / CircuitClose fire on circuit state transitions
//
// All hooks are non-blocking (run in caller goroutine, keep them fast).
// Wire them to slog / Prometheus / OTel in bootstrap.
type RedisSessionCache struct {
	client *redis.Client

	maxTTL time.Duration

	// Buffer pool for JSON encode/decode — eliminates per-op allocation.
	bufPool sync.Pool

	// --- Circuit breaker state ---

	// failureCount is the current consecutive Redis error count.
	failureCount atomic.Int64
	// circuitOpen is 1 when circuit is open, 0 when closed.
	circuitOpen atomic.Int32
	// failureThreshold is the open threshold (default 3).
	failureThreshold int
	// recoveryInterval is the ping interval when open (default 5s).
	recoveryInterval time.Duration
	// stopCh signals the background recovery goroutine to exit.
	stopCh chan struct{}

	// --- Observability ---

	Hooks RedisHooks

	// closeOnce guards Close() idempotency.
	closeOnce sync.Once
	// closed is 1 after Close().
	closed atomic.Int32
}

// RedisHooks is a set of optional observability callbacks.
// All are nil-safe — if a field is nil, the call is skipped.
// Fired synchronously in the caller goroutine, so keep them lightweight
// (e.g. increment an atomic counter or emit a metric).
type RedisHooks struct {
	GetHit       func(ctx context.Context, id string)
	GetMiss      func(ctx context.Context, id string)
	GetError     func(ctx context.Context, id string, err error)
	SetError     func(ctx context.Context, id string, err error)
	DeleteError  func(ctx context.Context, id string, err error)
	CircuitOpen  func(failures int64, lastErr error)
	CircuitClose func()
}

// redisMaxTTL caps per-session TTL to prevent unbounded Redis memory usage.
const redisMaxTTL = 7 * 24 * time.Hour

func NewRedisSessionCache(opts RedisOptions) (*RedisSessionCache, error) {
	// Apply defaults.
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 3
	}
	if opts.CircuitFailureThreshold == 0 {
		opts.CircuitFailureThreshold = 3
	}
	if opts.CircuitRecoveryInterval == 0 {
		opts.CircuitRecoveryInterval = 5 * time.Second
	}
	if opts.DialTimeout == 0 {
		opts.DialTimeout = 2 * time.Second
	}
	if opts.ReadTimeout == 0 {
		opts.ReadTimeout = 2 * time.Second
	}
	if opts.WriteTimeout == 0 {
		opts.WriteTimeout = 2 * time.Second
	}

	client := redis.NewClient(&redis.Options{
		Addr:         opts.Addr,
		Password:     opts.Password,
		DB:           opts.DB,
		MaxRetries:   opts.MaxRetries,
		DialTimeout:  opts.DialTimeout,
		ReadTimeout:  opts.ReadTimeout,
		WriteTimeout: opts.WriteTimeout,
		PoolSize:     opts.PoolSize,
	})

	// Fail-fast connectivity check.
	ctx, cancel := context.WithTimeout(context.Background(), opts.DialTimeout)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, apperrors.WrapUnavailable(err, "redis ping failed")
	}

	c := &RedisSessionCache{
		client:           client,
		maxTTL:           redisMaxTTL,
		failureThreshold: opts.CircuitFailureThreshold,
		recoveryInterval: opts.CircuitRecoveryInterval,
		stopCh:           make(chan struct{}),
		bufPool: sync.Pool{
			New: func() any { return new(bytes.Buffer) },
		},
	}

	// Start circuit recovery goroutine — idempotent; Close() stops it.
	recover.SafeGo("session-cache-redis-recovery", c.recoveryLoop)

	return c, nil
}

// RedisClient exposes the underlying client so bootstrap can construct
// protocol-level single-use stores (SAML replay protection) that share this
// connection's configuration and availability.
func (c *RedisSessionCache) RedisClient() *redis.Client {
	if c == nil {
		return nil
	}
	return c.client
}

// Close tears down the Redis client and stops the recovery goroutine.
// Safe to call multiple times.
func (c *RedisSessionCache) Close() error {
	var err error
	c.closeOnce.Do(func() {
		if c.closed.Swap(1) == 1 {
			return
		}
		close(c.stopCh)
		if c.client != nil {
			err = c.client.Close()
		}
	})
	return err
}

// IsAvailable reports whether the circuit is currently closed (Redis reachable).
// Useful for health-check endpoints and observability.
func (c *RedisSessionCache) IsAvailable() bool {
	return c.circuitOpen.Load() == 0
}

// RecoveryLoop runs in a goroutine; pings Redis every recoveryInterval when
// the circuit is open, closes it on first successful PING.
func (c *RedisSessionCache) recoveryLoop() {
	ticker := time.NewTicker(c.recoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			if c.circuitOpen.Load() == 0 {
				continue // circuit closed, nothing to do
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			err := c.client.Ping(ctx).Err()
			cancel()
			if err == nil {
				// Redis back → close circuit, reset failure counter.
				c.failureCount.Store(0)
				if c.circuitOpen.Swap(0) == 1 && c.Hooks.CircuitClose != nil {
					c.Hooks.CircuitClose()
				}
			}
		}
	}
}

// recordError increments the consecutive failure counter and opens the circuit
// if the threshold is crossed.
func (c *RedisSessionCache) recordError(err error) {
	n := c.failureCount.Add(1)
	if n >= int64(c.failureThreshold) && c.circuitOpen.Swap(1) == 0 {
		if c.Hooks.CircuitOpen != nil {
			c.Hooks.CircuitOpen(n, err)
		}
	}
}

// recordSuccess resets the failure counter on any successful Redis op
// (opportunistic circuit close — don't wait for the background ticker).
func (c *RedisSessionCache) recordSuccess() {
	c.failureCount.Store(0)
	if c.circuitOpen.Swap(0) == 1 && c.Hooks.CircuitClose != nil {
		c.Hooks.CircuitClose()
	}
}

// circuitOpenFast-path check — returns true if circuit is open.
// Inlined by Get/Set/Delete to avoid a function call on the hot path.
func (c *RedisSessionCache) circuitIsOpen() bool {
	return c.circuitOpen.Load() == 1
}

// ---------------------------------------------------------------------------
// SessionCache interface implementation
// ---------------------------------------------------------------------------

// Get returns cached session or (nil, false) on miss.
// Fail-open: Redis errors → miss, never blocks the hot path.
func (c *RedisSessionCache) Get(ctx context.Context, id string) (*domainsessions.Session, bool) {
	// Fast-path: circuit open → immediate miss, no Redis call.
	if c.circuitIsOpen() {
		if c.Hooks.GetMiss != nil {
			c.Hooks.GetMiss(ctx, id)
		}
		return nil, false
	}

	val, err := c.client.Get(ctx, sessionKey(id)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// Key not present — normal cache miss, not an error.
			c.recordSuccess()
			if c.Hooks.GetMiss != nil {
				c.Hooks.GetMiss(ctx, id)
			}
			return nil, false
		}
		// Real Redis error — record for circuit breaker.
		c.recordError(err)
		if c.Hooks.GetError != nil {
			c.Hooks.GetError(ctx, id, err)
		}
		return nil, false
	}

	c.recordSuccess()
	s, ok := c.decode(val)
	if !ok {
		if c.Hooks.GetError != nil {
			c.Hooks.GetError(ctx, id, apperrors.NewInternal("session cache decode failed"))
		}
		return nil, false
	}
	if c.Hooks.GetHit != nil {
		c.Hooks.GetHit(ctx, id)
	}
	return s, true
}

// Set stores session with TTL derived from session.ExpiresAt.
// Fail-open: any error is silently swallowed — cache write is best-effort.
func (c *RedisSessionCache) Set(ctx context.Context, s *domainsessions.Session) {
	if s == nil {
		return
	}
	ttl := time.Until(s.ExpiresAt)
	if ttl <= 0 {
		return // already expired, don't cache
	}
	if ttl > c.maxTTL {
		ttl = c.maxTTL
	}

	val, err := c.encode(s)
	if err != nil {
		return // encoding failure — skip, no need to record
	}

	if c.circuitIsOpen() {
		return // circuit open, don't even try
	}

	err = c.client.Set(ctx, sessionKey(s.ID), val, ttl).Err()
	if err != nil {
		c.recordError(err)
		if c.Hooks.SetError != nil {
			c.Hooks.SetError(ctx, s.ID, err)
		}
		return
	}
	c.recordSuccess()
}

// Delete removes cached session by ID.
// Fail-open: errors silently swallowed.
func (c *RedisSessionCache) Delete(ctx context.Context, id string) {
	if c.circuitIsOpen() {
		return
	}
	err := c.client.Del(ctx, sessionKey(id)).Err()
	if err != nil {
		c.recordError(err)
		if c.Hooks.DeleteError != nil {
			c.Hooks.DeleteError(ctx, id, err)
		}
		return
	}
	c.recordSuccess()
}

// InvalidateAll removes all cached sessions using SCAN + DEL (O(N) iteration,
// not KEYS * which is blocking). Safe to call even when circuit is open
// (will just find nothing).
func (c *RedisSessionCache) InvalidateAll(ctx context.Context) {
	if c.circuitIsOpen() {
		return
	}
	var cursor uint64
	for {
		keys, next, err := c.client.Scan(ctx, cursor, sessionKeyPrefix+"*", 100).Result()
		if err != nil {
			c.recordError(err)
			return
		}
		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				c.recordError(err)
				return
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	c.recordSuccess()
}

// ---------------------------------------------------------------------------
// Encode / Decode (thread-safe, no global lock, uses sync.Pool buffers)
// ---------------------------------------------------------------------------

// encode serializes Session to JSON bytes using a pooled buffer.
func (c *RedisSessionCache) encode(s *domainsessions.Session) ([]byte, error) {
	buf := c.bufPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		c.bufPool.Put(buf)
	}()
	if err := json.NewEncoder(buf).Encode(s); err != nil {
		return nil, err
	}
	// Return a copy — the pooled buffer will be reset on Put.
	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	return out, nil
}

// decode deserializes JSON bytes to Session.
// JSON decoder is stateless per-call — no lock needed.
func (c *RedisSessionCache) decode(data []byte) (*domainsessions.Session, bool) {
	buf := bytes.NewReader(data)
	var s domainsessions.Session
	if err := json.NewDecoder(buf).Decode(&s); err != nil {
		return nil, false
	}
	return &s, true
}
