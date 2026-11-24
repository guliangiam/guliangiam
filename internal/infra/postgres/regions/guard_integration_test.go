package regions

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/regions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// TestCrossRegionGuardPostgres is the real-PostgreSQL evidence for OPEN-API-38:
// organizations.region_code resolves, same-region access is allowed, cross-region
// access is rejected by the guard, and an organization without a binding fails
// with NotFound instead of silently comparing empty strings.
func TestCrossRegionGuardPostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is required for the cross-region guard test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	create := func(slug, region string) string {
		id := uuid.NewString()
		_, err := pool.Exec(ctx, "INSERT INTO guliangiam.organizations (id, slug, display_name, status, region_code) VALUES ($1::uuid, $2, $2, 'active', $3)", id, slug, region)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(), "DELETE FROM guliangiam.organizations WHERE id = $1::uuid", id)
		})
		return id
	}
	suffix := time.Now().Format("150405000")
	east := create("guard-east-"+suffix, "cn-north")
	eastPeer := create("guard-peereast-"+suffix, "cn-north")
	west := create("guard-west-"+suffix, "eu-west")
	unbound := create("guard-unbound-"+suffix, "")

	resolver := NewRepository(pool)
	guard := domain.NewCrossRegionGuard(resolver)

	require.NoError(t, guard.Validate(ctx, east, eastPeer, "user.list"))
	err = guard.Validate(ctx, east, west, "user.list")
	require.Error(t, err)
	var crossErr *domain.CrossRegionError
	require.ErrorAs(t, err, &crossErr)
	require.Equal(t, "cn-north", crossErr.RegionCodeA)
	require.Equal(t, "eu-west", crossErr.RegionCodeB)

	err = guard.Validate(ctx, east, unbound, "user.list")
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

// TestListDeploymentRegionsPostgres proves the management region picker reads
// the declared deployment region registry from the real database.
func TestListDeploymentRegionsPostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is required for the deployment region listing test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	regions, err := NewRepository(pool).ListDeploymentRegions(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, regions)
	for _, region := range regions {
		require.NotEmpty(t, region.RegionCode)
		require.NotEmpty(t, region.CountryCode)
	}
}
