package bootstrap

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMySQLRuntimeStarts is the composition-root evidence for OPEN-API-37:
// identity/organization/policy-store repositories, the MySQL audit chain and
// all drivers can be wired without a PostgreSQL pool.
func TestMySQLRuntimeStarts(t *testing.T) {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		t.Skip("MYSQL_DSN is required")
	}
	t.Setenv("DB_DRIVER", "mysql")
	t.Setenv("MYSQL_DSN", dsn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runtime, err := newRuntimeMySQLWithDialect(ctx)
	require.NoError(t, err)
	require.NotNil(t, runtime)
	assert.NotNil(t, runtime.DB.MySQLPool)
	runtime.Close()
}
