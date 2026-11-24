package partitioning

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// TestProberReadsRealRelationSizes is the real-PostgreSQL evidence for
// OPEN-API-40: pg_total_relation_size/pg_indexes_size are actually queried.
//
// Run:
//
//	POSTGRES_DSN='postgres://owner@/disposable?host=/tmp&sslmode=disable' \
//	  go test -run TestProberReadsRealRelationSizes ./internal/infra/postgres/partitioning/
func TestProberReadsRealRelationSizes(t *testing.T) {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("POSTGRES_DSN or DATABASE_URL is required for the partitioning prober test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	size, err := NewProber(pool).Size(ctx, "records")
	require.NoError(t, err)
	require.Positive(t, size.TotalBytes)
	require.GreaterOrEqual(t, size.IndexBytes, int64(0))

	_, err = NewProber(pool).Size(ctx, "bad-table-name; DROP TABLE users")
	require.Error(t, err)
}
