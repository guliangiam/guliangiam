package privacy

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// TestCollectSubjectDataPostgres verifies the cross-table subject aggregation on
// the real runtime schema (seed admin user in the built-in organization).
func TestCollectSubjectDataPostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	var userID, organizationID string
	err = pool.QueryRow(ctx, `
SELECT u.id, u.organization_id
FROM guliangiam.users u
JOIN guliangiam.organizations o ON o.id = u.organization_id
WHERE u.username = 'admin' AND u.deleted_at IS NULL AND o.deleted_at IS NULL
ORDER BY u.created_at
LIMIT 1`).Scan(&userID, &organizationID)
	require.NoError(t, err)

	records, region, err := NewRepository(pool).CollectSubjectData(ctx, organizationID, "user", userID)
	require.NoError(t, err)
	require.NotEmpty(t, records)
	require.NotEmpty(t, region)
	for _, record := range records {
		require.Equal(t, userID, record.SubjectID)
		require.NotEmpty(t, record.Category)
	}
}
