package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// TestBackfillBootstrapScopePostgres is the real-PostgreSQL upgrade evidence
// for OPEN-API-52: an existing database whose seed row kept is_bootstrap=false
// is repaired by the one-shot backfill, the SELECT judgement sees exactly one
// bootstrap row, and the production startup self-check passes afterwards.
func TestBackfillBootstrapScopePostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is required for the bootstrap scope backfill test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	var scopeID string
	var original bool
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT s.id::text, s.is_bootstrap FROM guliangiam.platform_admin_scopes s
		JOIN guliangiam.platform_admins pa ON pa.id = s.platform_admin_id
		JOIN guliangiam.users u ON u.id = pa.user_id
		JOIN guliangiam.organizations o ON o.id = u.organization_id
		WHERE o.slug = 'built-in' AND u.username = 'admin' AND s.scope_type = 'all' AND s.approved_by IS NULL
		LIMIT 1`).Scan(&scopeID, &original))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "UPDATE guliangiam.platform_admin_scopes SET is_bootstrap = $1 WHERE id = $2", original, scopeID)
	})
	_, err = pool.Exec(ctx, "UPDATE guliangiam.platform_admin_scopes SET is_bootstrap = false WHERE id = $1", scopeID)
	require.NoError(t, err)

	raw, err := os.ReadFile(filepath.Join(repoRootForTest(t), "scripts", "dev", "backfill-platform-admin-scope-bootstrap.sql"))
	require.NoError(t, err)
	lines := strings.Split(string(raw), "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		kept = append(kept, line)
	}
	for _, statement := range strings.Split(strings.Join(kept, "\n"), ";") {
		trimmed := strings.TrimSpace(statement)
		if trimmed == "" {
			continue
		}
		_, err := pool.Exec(ctx, trimmed)
		require.NoError(t, err)
	}

	var bootstrapRows int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*) FROM guliangiam.platform_admin_scopes s
		JOIN guliangiam.platform_admins pa ON pa.id = s.platform_admin_id
		JOIN guliangiam.users u ON u.id = pa.user_id
		JOIN guliangiam.organizations o ON o.id = u.organization_id
		WHERE o.slug = 'built-in' AND u.username = 'admin'
		  AND s.scope_type = 'all' AND s.approved_by IS NULL AND s.is_bootstrap = true`).Scan(&bootstrapRows))
	require.Equal(t, 1, bootstrapRows)

	require.NoError(t, verifyBootstrapAdminAllScope(ctx, queryBuiltinPlatformAdmin(pool), listBootstrapAdminScopes(pool), true))
}
