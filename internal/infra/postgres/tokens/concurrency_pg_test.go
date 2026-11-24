// Real-PostgreSQL evidence for AC-TOKEN-005's single-winner conditional revoke.
// Skipped unless GIAM_TEST_PG_DSN is set.
package tokens

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/tokens"
)

const refreshConcurrencyK = 8

func newTokensPGPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is not set; skipping real PostgreSQL token concurrency test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SELECT set_config('app.current_role', 'super_admin', false)`)
		return err
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))
	t.Cleanup(pool.Close)

	return pool
}

type refreshTokenFixture struct {
	OrgID     string
	AppID     string
	UserID    string
	AccessID  string
	RefreshID string
	FamilyID  string
}

// seedRefreshTokenFixture creates one live access/refresh pair; cleanup deletes
// the user first so the token rows cascade.
func seedRefreshTokenFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) refreshTokenFixture {
	t.Helper()

	fixture := refreshTokenFixture{
		OrgID:     uuid.NewString(),
		AppID:     uuid.NewString(),
		UserID:    uuid.NewString(),
		AccessID:  uuid.NewString(),
		RefreshID: uuid.NewString(),
	}
	// refresh_tokens.family_id is a self-referencing FK: the first token of a
	// rotation chain is the family root, and later generations point at it.
	fixture.FamilyID = fixture.RefreshID
	slug := "g38a-token-" + strings.ReplaceAll(fixture.OrgID[:8], "-", "")
	now := time.Now().UTC().Truncate(time.Microsecond)
	expiresAt := now.Add(time.Hour)

	_, err := pool.Exec(ctx,
		`INSERT INTO guliangiam.organizations (id, slug, display_name) VALUES ($1::uuid, $2, $3)`,
		fixture.OrgID, slug, "G3-8a token fixture",
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO guliangiam.applications (id, organization_id, slug, display_name) VALUES ($1::uuid, $2::uuid, $3, $4)`,
		fixture.AppID, fixture.OrgID, slug+"-app", "G3-8a app",
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO guliangiam.users (id, organization_id, username) VALUES ($1::uuid, $2::uuid, $3)`,
		fixture.UserID, fixture.OrgID, slug+"-user",
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO guliangiam.access_tokens
		    (id, user_id, application_id, token_hash, token_prefix, scopes, audience, expires_at, created_at, updated_at)
		 VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9, $9)`,
		fixture.AccessID, fixture.UserID, fixture.AppID,
		"g38a-access-"+strings.ReplaceAll(fixture.AccessID, "-", ""), "gl_pat_g38a_"+fixture.AccessID[:8],
		[]string{"openid"}, []string{"g38a"}, expiresAt, now,
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO guliangiam.refresh_tokens
		    (id, access_token_id, user_id, application_id, token_hash, token_prefix, family_id, scopes, audience, expires_at, created_at, updated_at)
		 VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, $7::uuid, $8, $9, $10, $11, $11)`,
		fixture.RefreshID, fixture.AccessID, fixture.UserID, fixture.AppID,
		"g38a-refresh-"+strings.ReplaceAll(fixture.RefreshID, "-", ""), "gl_prt_g38a_"+fixture.RefreshID[:8], fixture.FamilyID,
		[]string{"openid"}, []string{"g38a"}, expiresAt.Add(24*time.Hour), now,
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM guliangiam.users WHERE id = $1::uuid`, fixture.UserID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM guliangiam.applications WHERE id = $1::uuid`, fixture.AppID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM guliangiam.organizations WHERE id = $1::uuid`, fixture.OrgID)
	})

	return fixture
}

// TestTokenRepositoryPostgres_RevokeRefreshIsSingleWinner runs K conditional
// revokes against one row: exactly one affects a row, every other caller sees
// ErrRefreshAlreadyRevoked.
func TestTokenRepositoryPostgres_RevokeRefreshIsSingleWinner(t *testing.T) {
	pool := newTokensPGPool(t)
	ctx := context.Background()
	fixture := seedRefreshTokenFixture(t, ctx, pool)

	repo := NewRepository(pool)
	start := make(chan struct{})
	errs := make([]error, refreshConcurrencyK)

	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = repo.RevokeRefresh(ctx, fixture.RefreshID, time.Now().UTC())
		}(i)
	}

	close(start)
	wg.Wait()

	winners := 0
	rejected := 0
	for i, err := range errs {
		switch {
		case err == nil:
			winners++
		case errors.Is(err, domain.ErrRefreshAlreadyRevoked):
			rejected++
		default:
			t.Fatalf("goroutine %d returned unexpected error: %v", i, err)
		}
	}
	require.Equal(t, 1, winners, "exactly one concurrent conditional revoke may win")
	require.Equal(t, refreshConcurrencyK-1, rejected)

	var revokedAt *time.Time
	var revision int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT revoked_at, revision FROM guliangiam.refresh_tokens WHERE id = $1::uuid`,
		fixture.RefreshID,
	).Scan(&revokedAt, &revision))
	require.NotNil(t, revokedAt)
	require.Equal(t, int64(2), revision, "one winner means exactly one revision increment")
}
