package postgres

import (
	"context"
	"hash/fnv"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AdvisoryLockProvider implements a cross-process lock using PostgreSQL
// advisory locks. It is the default LockProvider for scheduled jobs (DB-061):
// without it, every replica would pick up the same due LDAP sync configs and
// sync the same directory concurrently.
type AdvisoryLockProvider struct {
	pool *pgxpool.Pool
}

// NewAdvisoryLockProvider wires the provider to a connection pool.
func NewAdvisoryLockProvider(pool *pgxpool.Pool) *AdvisoryLockProvider {
	return &AdvisoryLockProvider{pool: pool}
}

// TryLock takes a session-level advisory lock keyed by the FNV-1a hash of name.
// Session-level (rather than transaction-level) is required because the guarded
// work spans many transactions. The lock is released explicitly by the returned
// function; the connection also drops it when the session ends.
func (p *AdvisoryLockProvider) TryLock(ctx context.Context, name string) (unlock func(context.Context) error, acquiredOut bool, err error) {
	if p.pool == nil {
		return nil, false, apperrors.NewInternalf("advisory lock: pool is nil")
	}

	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return nil, false, apperrors.WrapInternalf(err, "advisory lock: acquire connection")
	}

	key := advisoryKey(name)

	var acquired bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, key).Scan(&acquired); err != nil {
		conn.Release()
		return nil, false, apperrors.WrapInternalf(err, "advisory lock: pg_try_advisory_lock")
	}
	if !acquired {
		conn.Release()
		return nil, false, nil
	}

	release := func(releaseCtx context.Context) error {
		defer conn.Release()
		if _, err := conn.Exec(releaseCtx, `SELECT pg_advisory_unlock($1)`, key); err != nil {
			return apperrors.WrapInternalf(err, "advisory lock: pg_advisory_unlock")
		}
		return nil
	}
	return release, true, nil
}

// advisoryKey maps a human-readable lock name onto a bigint key. PostgreSQL
// advisory locks are keyed by int64, so names must be folded deterministically.
// uint64 → int64 采用二进制补码回绕：键值只需稳定且分布均匀，改掩码会改变
// 既有锁键，滚动发布期间新旧实例将不再互斥。
func advisoryKey(name string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	return int64(h.Sum64()) //nolint:gosec // G115: two's complement wrap is the intended, stable key mapping
}
