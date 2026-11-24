package platform_admins

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	postgresuow "github.com/guliangiam/guliangiam/internal/infra/postgres/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// TestSetActiveDeactivationPostgres is the real-PostgreSQL evidence for
// OPEN-API-55: deactivating a platform admin revokes every existing session in
// the same transaction and invalidates the principal cache after commit.
//
// Run:
//
//	GIAM_TEST_PG_DSN='postgres://owner@/disposable?host=/tmp&sslmode=disable' \
//	  go test -run TestSetActiveDeactivationPostgres ./internal/app/platform_admins/
func TestSetActiveDeactivationPostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is required for the platform-admin deactivation test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fixture, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(fixture.Close)

	var adminUserID, adminID, applicationID string
	require.NoError(t, fixture.QueryRow(ctx, `
		SELECT u.id::text, pa.id::text, a.id::text
		FROM guliangiam.users u
		JOIN guliangiam.organizations o ON o.id = u.organization_id
		JOIN guliangiam.platform_admins pa ON pa.user_id = u.id
		CROSS JOIN (SELECT id FROM guliangiam.applications LIMIT 1) a
		WHERE u.username = 'admin' AND o.slug = 'built-in'
		LIMIT 1`).Scan(&adminUserID, &adminID, &applicationID))
	sessionID1, sessionID2 := uuid.New(), uuid.New()
	fixtureExec := func(query string, args ...any) {
		t.Helper()
		tx, beginErr := fixture.Begin(ctx)
		require.NoError(t, beginErr)
		_, err := tx.Exec(ctx, "SELECT set_config('app.current_role', 'super_admin', true)")
		require.NoError(t, err)
		_, err = tx.Exec(ctx, query, args...)
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))
	}
	fixtureExec("UPDATE guliangiam.platform_admins SET is_active = true WHERE id = $1", adminID)
	fixtureExec("INSERT INTO guliangiam.sessions (id, user_id, application_id, expires_at) VALUES ($1, $2, $3, $4), ($5, $2, $3, $4)",
		sessionID1, adminUserID, applicationID, time.Now().Add(time.Hour), sessionID2)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		tx, beginErr := fixture.Begin(cleanupCtx)
		if beginErr != nil {
			return
		}
		_, _ = tx.Exec(cleanupCtx, "SELECT set_config('app.current_role', 'super_admin', true)")
		_, _ = tx.Exec(cleanupCtx, "DELETE FROM guliangiam.sessions WHERE id = ANY($1::uuid[])", []uuid.UUID{sessionID1, sessionID2})
		_ = tx.Commit(cleanupCtx)
	})

	uow := postgresuow.New(fixture)
	invalidator := &recordingInvalidator{}
	svc := NewService(clock.SystemClock{}, nil, uow, nil)
	svc.PrincipalCache = invalidator
	systemCtx := tenantctx.System(ctx)

	require.NoError(t, svc.SetActive(systemCtx, SetActiveCommand{ID: adminID, Active: false}))
	require.Equal(t, []string{adminUserID}, invalidator.ids)

	var active bool
	require.NoError(t, fixture.QueryRow(ctx, "SELECT is_active FROM guliangiam.platform_admins WHERE id = $1", adminID).Scan(&active))
	require.False(t, active)

	var unrevoked int
	require.NoError(t, fixture.QueryRow(ctx, "SELECT count(*) FROM guliangiam.sessions WHERE id = ANY($1::uuid[]) AND revoked_at IS NULL", []uuid.UUID{sessionID1, sessionID2}).Scan(&unrevoked))
	require.Zero(t, unrevoked, "deactivation must revoke every existing session")

	// Recovery takes effect immediately but sessions stay revoked: the admin must
	// sign in again.
	require.NoError(t, svc.SetActive(systemCtx, SetActiveCommand{ID: adminID, Active: true}))
	require.Equal(t, []string{adminUserID}, invalidator.ids)
	var remaining, unrevokedAfterRecovery int
	require.NoError(t, fixture.QueryRow(ctx, "SELECT count(*), count(*) FILTER (WHERE revoked_at IS NULL) FROM guliangiam.sessions WHERE id = ANY($1::uuid[])", []uuid.UUID{sessionID1, sessionID2}).Scan(&remaining, &unrevokedAfterRecovery))
	require.Equal(t, 2, remaining)
	require.Zero(t, unrevokedAfterRecovery, "reactivation must not un-revoke sessions")
}
