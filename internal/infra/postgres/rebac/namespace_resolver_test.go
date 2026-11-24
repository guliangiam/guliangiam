package rebac

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestNamespaceResolverDoesNotCacheMiss(t *testing.T) {
	calls := 0
	resolver := &Resolver{
		lookup: func(context.Context, string) (string, error) {
			calls++
			if calls == 1 {
				return "", nil
			}
			return "namespace-1", nil
		},
		cache:   make(map[string]string),
		queried: make(map[string]bool),
	}

	got, err := resolver.Resolve(context.Background(), "org-1")
	require.NoError(t, err)
	require.Empty(t, got)

	got, err = resolver.Resolve(context.Background(), "org-1")
	require.NoError(t, err)
	require.Equal(t, "namespace-1", got)

	got, err = resolver.Resolve(context.Background(), "org-1")
	require.NoError(t, err)
	require.Equal(t, "namespace-1", got)
	require.Equal(t, 2, calls, "a miss should be retried, then a successful namespace lookup should be cached")
}

func TestNamespaceResolverDoesNotCacheLookupError(t *testing.T) {
	calls := 0
	resolver := &Resolver{
		lookup: func(context.Context, string) (string, error) {
			calls++
			if calls == 1 {
				return "", errors.New("temporary database failure")
			}
			return "namespace-1", nil
		},
		cache:   make(map[string]string),
		queried: make(map[string]bool),
	}

	_, err := resolver.Resolve(context.Background(), "org-1")
	require.Error(t, err)

	got, err := resolver.Resolve(context.Background(), "org-1")
	require.NoError(t, err)
	require.Equal(t, "namespace-1", got)
	require.Equal(t, 2, calls)
}

func TestNamespaceResolverDoesNotLeakElevatedGUC(t *testing.T) {
	dsn := os.Getenv("P1_VERIFY_DSN")
	if dsn == "" {
		t.Skip("P1_VERIFY_DSN is required for the isolated PostgreSQL resolver test")
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(cfg.ConnConfig.Database, "_verify"), "P1_VERIFY_DSN must target a *_verify database")
	cfg.MaxConns = 1

	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	defer pool.Close()

	var relationNamespaces string
	require.NoError(t, pool.QueryRow(ctx, "SELECT COALESCE(to_regclass('guliangiam.relation_namespaces')::text, '')").Scan(&relationNamespaces))
	require.NotEmpty(t, relationNamespaces, "isolated database must be migrated")

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, "SELECT set_config('app.current_role', 'member', false)")
	require.NoError(t, err)
	var before string
	require.NoError(t, conn.QueryRow(ctx, "SELECT COALESCE(current_setting('app.current_role', true), '')").Scan(&before))
	conn.Release()

	resolver := NewNamespaceResolver(pool)
	_, err = resolver.Resolve(ctx, "00000000-0000-7000-8000-000000000001")
	require.NoError(t, err)

	conn, err = pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	var after string
	require.NoError(t, conn.QueryRow(ctx, "SELECT COALESCE(current_setting('app.current_role', true), '')").Scan(&after))
	require.Equal(t, before, after, "resolver must not leave super_admin in the pooled session GUC")
}
