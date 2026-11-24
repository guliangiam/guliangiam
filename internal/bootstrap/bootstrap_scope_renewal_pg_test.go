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

// TestRenewBootstrapScopeScriptPostgres is the real-PostgreSQL evidence for the
// OPEN-API-54 renewal path: bringing a near-expiry bootstrap all-scope row
// forward with the operator script extends it beyond the warning window.
func TestRenewBootstrapScopeScriptPostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is required for the bootstrap scope renewal test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	var scopeID string
	var original time.Time
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT s.id::text, s.expires_at FROM guliangiam.platform_admin_scopes s
		JOIN guliangiam.platform_admins pa ON pa.id = s.platform_admin_id
		JOIN guliangiam.users u ON u.id = pa.user_id
		JOIN guliangiam.organizations o ON o.id = u.organization_id
		WHERE o.slug = 'built-in' AND u.username = 'admin'
		  AND s.scope_type = 'all' AND s.approved_by IS NULL
		LIMIT 1`).Scan(&scopeID, &original))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "UPDATE guliangiam.platform_admin_scopes SET expires_at = $1 WHERE id = $2", original, scopeID)
	})
	_, err = pool.Exec(ctx, "UPDATE guliangiam.platform_admin_scopes SET expires_at = now() + interval '2 days' WHERE id = $1", scopeID)
	require.NoError(t, err)

	root := repoRootForTest(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "dev", "renew-platform-admin-scope-bootstrap.sql"))
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

	var renewed time.Time
	require.NoError(t, pool.QueryRow(ctx, "SELECT expires_at FROM guliangiam.platform_admin_scopes WHERE id = $1", scopeID).Scan(&renewed))
	require.True(t, renewed.After(time.Now().Add(300*24*time.Hour)), "renewal must extend the scope by ~1 year")
}

func repoRootForTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
