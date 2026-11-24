package iga_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	appiga "github.com/guliangiam/guliangiam/internal/app/iga"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainiga "github.com/guliangiam/guliangiam/internal/domain/iga"
	postgresuow "github.com/guliangiam/guliangiam/internal/infra/postgres/uow"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// TestExpireWorkerRuntimePostgres is the real-PostgreSQL evidence for
// OPEN-DESIGN-02: the reclaim worker scans an expired access lifecycle through
// the real unit of work, reclaims it, completes the lifecycle and writes the
// iga.jit.expired audit event.
func TestExpireWorkerRuntimePostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is required for the IGA worker runtime test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	var orgID, userID, roleID string
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT u.organization_id::text, u.id::text, r.id::text
		FROM guliangiam.users u
		JOIN guliangiam.organizations o ON o.id = u.organization_id
		JOIN guliangiam.roles r ON r.organization_id = o.id
		WHERE u.username = 'admin' AND o.slug = 'built-in' LIMIT 1`).Scan(&orgID, &userID, &roleID))

	uow := postgresuow.New(pool)
	now := time.Now().UTC()
	lifecycleID := uuid.NewString()
	systemCtx := tenantctx.System(ctx)
	require.NoError(t, uow.WithinTransaction(systemCtx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		return tx.IGA().CreateLifecycle(ctx, &domainiga.AccessLifecycle{
			ID: lifecycleID, OrganizationID: orgID, UserID: userID, RoleID: roleID,
			Resource: "orders", Action: "read",
			GrantedAt: now.Add(-2 * time.Hour), StartsAt: now.Add(-2 * time.Hour),
			ExpiresAt: now.Add(-time.Minute), Status: domainiga.LifecycleActive,
			CreatedAt: now.Add(-2 * time.Hour),
		})
	}))

	worker := appiga.NewExpireWorker(uow, time.Hour, 10, func() time.Time { return now }, []string{orgID})
	runCtx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = worker.Run(runCtx)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		var status string
		require.NoError(t, pool.QueryRow(ctx,
			"SELECT status FROM guliangiam.access_lifecycle WHERE id = $1::uuid", lifecycleID).Scan(&status))
		if status == string(domainiga.LifecycleExpired) {
			break
		}
		if time.Now().After(deadline) {
			stop()
			<-done
			t.Fatalf("lifecycle status = %q, want expired", status)
		}
		time.Sleep(20 * time.Millisecond)
	}
	stop()
	<-done

	var auditCount int
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT count(*) FROM guliangiam.records WHERE organization_id = $1::uuid AND action = 'iga.jit.expired' AND resource_id = $2",
		orgID, lifecycleID).Scan(&auditCount))
	require.GreaterOrEqual(t, auditCount, 1, "worker must write the iga.jit.expired audit event")
}
