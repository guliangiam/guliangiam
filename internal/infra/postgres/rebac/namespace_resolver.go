package rebac

import (
	"context"
	"errors"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// namespaceResolver resolves the "default" ReBAC namespace UUID for a given
// organization, with in-memory per-org caching.
//
// relation_namespaces has FORCE ROW LEVEL SECURITY, while authz middleware runs
// before any tenant GUC is set. Resolve therefore uses an exclusive connection
// and a transaction-local super_admin GUC to read the namespace without leaving
// elevated session state on the pooled connection.
type Resolver struct {
	lookup  func(context.Context, string) (string, error)
	mu      sync.RWMutex
	cache   map[string]string
	queried map[string]bool
}

// NewNamespaceResolver constructs the pgx-backed resolver. A nil pool returns a
// nil *Resolver so callers can decide whether to inject the port at all (avoiding
// the typed-nil-in-interface trap).
func NewNamespaceResolver(pool *pgxpool.Pool) *Resolver {
	if pool == nil {
		return nil
	}
	return &Resolver{
		lookup: func(ctx context.Context, organizationID string) (namespaceUUID string, err error) {
			conn, err := pool.Acquire(ctx)
			if err != nil {
				return "", err
			}
			defer conn.Release()

			tx, err := conn.Begin(ctx)
			if err != nil {
				return "", err
			}
			defer func() {
				if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
					err = errors.Join(err, rollbackErr)
				}
			}()

			if _, err := tx.Exec(ctx, "SELECT set_config('app.current_role','super_admin',true)"); err != nil {
				return "", err
			}

			err = tx.QueryRow(ctx, `
				SELECT id::text FROM guliangiam.relation_namespaces
				WHERE organization_id = $1 AND name = 'default'
			`, organizationID).Scan(&namespaceUUID)
			if errors.Is(err, pgx.ErrNoRows) {
				return "", nil
			}
			if err != nil {
				return "", err
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return "", commitErr
			}
			return namespaceUUID, nil
		},
		cache:   make(map[string]string),
		queried: make(map[string]bool),
	}
}

// Resolve returns the "default" namespace UUID for the org, or "" if not found.
// Successful lookups are cached per org. Misses are not cached because a
// namespace may be provisioned after the first request; retaining a miss would
// otherwise keep ReBAC disabled for that org until process restart.
func (r *Resolver) Resolve(ctx context.Context, organizationID string) (string, error) {
	if r == nil || r.lookup == nil {
		return "", errors.New("ReBAC namespace resolver unavailable")
	}

	r.mu.RLock()
	if r.queried[organizationID] {
		value := r.cache[organizationID]
		r.mu.RUnlock()
		return value, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.queried[organizationID] {
		return r.cache[organizationID], nil
	}

	namespaceUUID, err := r.lookup(ctx, organizationID)
	if err != nil {
		return "", err
	}
	if namespaceUUID == "" {
		return "", nil
	}
	r.queried[organizationID] = true
	r.cache[organizationID] = namespaceUUID
	return namespaceUUID, nil
}
