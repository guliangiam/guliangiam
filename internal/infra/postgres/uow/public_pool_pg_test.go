package uow

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
)

// TestWithinTransactionPreservesPublicPool guards the public-discovery path:
// the transaction handle must keep the pool, otherwise GetBySlugPublic falls
// back to the full RLS query (which both reads credential columns and breaks on
// older schemas).
func TestWithinTransactionPreservesPublicPool(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is required for the public pool propagation test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	u := New(pool, pool)
	require.NoError(t, u.WithinTransaction(context.Background(), func(ctx context.Context, tx appuow.UnitOfWork) error {
		withPool, ok := tx.(interface{ HasPublicPool() bool })
		require.True(t, ok)
		require.True(t, withPool.HasPublicPool())
		organization, err := tx.Organizations().GetBySlugPublic(ctx, "built-in")
		require.NoError(t, err)
		require.NotEmpty(t, organization.ID)
		return nil
	}))
}
