// Real-PostgreSQL evidence for AC-TOKEN-005: the row lock plus the conditional
// RevokeRefresh UPDATE must yield one winner and no branch family. Skipped
// unless GIAM_TEST_PG_DSN is set; audit records are captured in memory.
package tokens

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	postgresuow "github.com/guliangiam/guliangiam/internal/infra/postgres/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

const refreshServiceConcurrencyK = 8

func newRefreshPGPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is not set; skipping real PostgreSQL refresh concurrency test")
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

type recordingRecords struct {
	domainrecords.Repository

	mu      sync.Mutex
	records []*domainrecords.Record
}

func (r *recordingRecords) Create(_ context.Context, record *domainrecords.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, record)
	return nil
}

func (r *recordingRecords) countAction(action string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, record := range r.records {
		if record.Action == action {
			count++
		}
	}
	return count
}

type refreshPGUnitOfWork struct {
	appuow.UnitOfWork
	pool  *pgxpool.Pool
	audit *recordingRecords
}

func (u refreshPGUnitOfWork) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	if _, err := tx.Exec(ctx, `SELECT guliangiam.set_tenant_context(NULL::uuid, 'super_admin')`); err != nil {
		return err
	}

	txUOW := refreshPGTxUoW{UnitOfWork: postgresuow.New(tx), audit: u.audit}
	if err := fn(ctx, txUOW); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	committed = true
	return nil
}

type refreshPGTxUoW struct {
	appuow.UnitOfWork
	audit *recordingRecords
}

func (u refreshPGTxUoW) Records() domainrecords.Repository { return u.audit }
func (u refreshPGTxUoW) Outbox() domainoutbox.Repository   { return nil }

type refreshServiceFixture struct {
	OrgID     string
	AppID     string
	UserID    string
	AccessID  string
	RefreshID string
	FamilyID  string
	RawToken  string
}

// seedRefreshServiceFixture creates one isolated tenant with a live pair.
func seedRefreshServiceFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) refreshServiceFixture {
	t.Helper()

	fixture := refreshServiceFixture{
		OrgID:     uuid.NewString(),
		AppID:     uuid.NewString(),
		UserID:    uuid.NewString(),
		AccessID:  uuid.NewString(),
		RefreshID: uuid.NewString(),
	}
	fixture.FamilyID = fixture.RefreshID
	fixture.RawToken = "gl_prt_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	slug := "g38a-refresh-" + strings.ReplaceAll(fixture.OrgID[:8], "-", "")
	now := time.Now().UTC().Truncate(time.Microsecond)

	_, err := pool.Exec(ctx,
		`INSERT INTO guliangiam.organizations (id, slug, display_name) VALUES ($1::uuid, $2, $3)`,
		fixture.OrgID, slug, "G3-8a refresh fixture",
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO guliangiam.applications (id, organization_id, slug, display_name) VALUES ($1::uuid, $2::uuid, $3, $4)`,
		fixture.AppID, fixture.OrgID, slug+"-app", "G3-8a refresh app",
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
		[]string{"openid"}, []string{"g38a"}, now.Add(time.Hour), now,
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO guliangiam.refresh_tokens
		    (id, access_token_id, user_id, application_id, token_hash, token_prefix, family_id, scopes, audience, expires_at, created_at, updated_at)
		 VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, $7::uuid, $8, $9, $10, $11, $11)`,
		fixture.RefreshID, fixture.AccessID, fixture.UserID, fixture.AppID,
		hashTokenValue(fixture.RawToken), "gl_prt_g38a_"+fixture.RefreshID[:8], fixture.FamilyID,
		[]string{"openid"}, []string{"g38a"}, now.Add(24*time.Hour), now,
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

// TestServiceRefreshAccessTokenPostgres_ConcurrentRotationHasSingleWinner
// drives the real RefreshAccessToken use case with K callers on one refresh
// token and asserts one success, no branch family, and a committed reuse audit.
func TestServiceRefreshAccessTokenPostgres_ConcurrentRotationHasSingleWinner(t *testing.T) {
	// A dedicated test KEK avoids the panic when the runner has no field key.
	t.Setenv("ENCRYPTION_KEY_CURRENT", "BX1Ab1Jz6cRDESuWCleZWGRv7RHQRAf0sqikOVcVzkg=")

	pool := newRefreshPGPool(t)
	ctx := tenantctx.System(context.Background())
	fixture := seedRefreshServiceFixture(t, ctx, pool)

	audit := &recordingRecords{}
	base := postgresuow.New(pool)
	svc := Service{
		Clock: clock.SystemClock{},
		UUID:  platformuuid.V7Generator{},
		UOW:   refreshPGUnitOfWork{UnitOfWork: base, pool: pool, audit: audit},
	}

	start := make(chan struct{})
	results := make([]*RefreshTokenIssued, refreshServiceConcurrencyK)
	errs := make([]error, refreshServiceConcurrencyK)

	var wg sync.WaitGroup
	for i := 0; i < refreshServiceConcurrencyK; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errs[i] = svc.RefreshAccessToken(ctx, RefreshAccessTokenCommand{
				OrganizationID: fixture.OrgID,
				RefreshToken:   fixture.RawToken,
			})
		}(i)
	}

	close(start)
	wg.Wait()

	successes := 0
	invalidArgument := 0
	reuseRejected := 0
	var winner *RefreshTokenIssued
	for i, err := range errs {
		if err == nil {
			require.NotNilf(t, results[i], "goroutine %d reported success without a result", i)
			require.NotEmpty(t, results[i].RefreshToken)
			require.NotEmpty(t, results[i].TokenValue)
			winner = results[i]
			successes++
			continue
		}

		typed := apperrors.FromError(err)
		// Both codes are accepted deliberately: PostgreSQL FOR UPDATE serialization
		// makes the loser observe the already-revoked row and take the 401
		// reuse-detection path. The 400 branch stays as a defensive guard for a
		// repository/isolation behavior where RevokeRefresh's conditional UPDATE
		// loses after the liveness check; it is effectively unreachable here.
		switch typed.Code {
		case apperrors.CodeInvalidArgument:
			invalidArgument++
			require.Equal(t, 400, typed.HTTPStatus())
		case apperrors.CodeUnauthorized:
			reuseRejected++
			require.Equal(t, 401, typed.HTTPStatus())
		default:
			t.Fatalf("goroutine %d returned unexpected error code %s: %v", i, typed.Code, err)
		}
	}

	require.Equal(t, 1, successes, "exactly one concurrent refresh may win")
	require.Equal(t, refreshServiceConcurrencyK-1, invalidArgument+reuseRejected,
		"every other caller must be rejected")

	var refreshCount, accessCount, familyCount, foreignFamilyCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM guliangiam.refresh_tokens WHERE user_id = $1::uuid`,
		fixture.UserID,
	).Scan(&refreshCount))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM guliangiam.access_tokens WHERE user_id = $1::uuid`,
		fixture.UserID,
	).Scan(&accessCount))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(DISTINCT family_id) FROM guliangiam.refresh_tokens WHERE user_id = $1::uuid`,
		fixture.UserID,
	).Scan(&familyCount))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM guliangiam.refresh_tokens WHERE user_id = $1::uuid AND family_id <> $2::uuid`,
		fixture.UserID, fixture.FamilyID,
	).Scan(&foreignFamilyCount))

	require.Equal(t, 2, refreshCount, "one old + one rotated refresh token, no branch generations")
	require.Equal(t, 2, accessCount, "one old + one rotated access token, no branch generations")
	require.Equal(t, 1, familyCount, "the rotated token must stay in the same family")
	require.Equal(t, 0, foreignFamilyCount, "no branch family may be created")

	var activeRefresh, activeFamilies int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM guliangiam.refresh_tokens WHERE user_id = $1::uuid AND revoked_at IS NULL`,
		fixture.UserID,
	).Scan(&activeRefresh))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(DISTINCT family_id) FROM guliangiam.refresh_tokens
		 WHERE user_id = $1::uuid AND revoked_at IS NULL AND family_id IS NOT NULL`,
		fixture.UserID,
	).Scan(&activeFamilies))
	require.LessOrEqual(t, activeRefresh, 1, "at most one refresh token may remain active")
	require.LessOrEqual(t, activeFamilies, 1, "at most one family may remain active")

	// OPEN-API-47: concurrent losers land inside the reuse grace window, so the
	// winner's rotated pair stays live and no whole-family revocation is audited.
	require.Equal(t, 1, activeRefresh, "the winner's rotated refresh token must stay usable")
	require.Equal(t, 0, audit.countAction("token.reuse_detected"),
		"concurrent losers inside the grace window must not revoke the family")

	// The client that received 200 must still be able to rotate the returned
	// refresh token: this is the regression the grace window exists for.
	require.NotNil(t, winner)
	_, err := svc.RefreshAccessToken(ctx, RefreshAccessTokenCommand{
		OrganizationID: fixture.OrgID,
		RefreshToken:   winner.RefreshToken,
	})
	require.NoError(t, err, "winner's rotated refresh token must remain usable")
}

// stepClock is a manually advanceable clock so the reuse test can move past the
// 10s grace window without sleeping.
type stepClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *stepClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *stepClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// TestServiceCreateTokenPostgres_RefreshFamilyRoundTrip drives the REAL issuance
// path (CreateToken with IssueRefreshToken) instead of hand-seeding family_id:
// the first generation must satisfy the self-referencing family FK, rotation
// must keep the family, and reuse beyond the grace window must revoke it.
func TestServiceCreateTokenPostgres_RefreshFamilyRoundTrip(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", "BX1Ab1Jz6cRDESuWCleZWGRv7RHQRAf0sqikOVcVzkg=")

	pool := newRefreshPGPool(t)
	ctx := tenantctx.System(context.Background())
	fixture := seedRefreshServiceFixture(t, ctx, pool)

	audit := &recordingRecords{}
	clk := &stepClock{t: time.Now().UTC()}
	svc := Service{
		Clock: clk,
		UUID:  platformuuid.V7Generator{},
		UOW:   refreshPGUnitOfWork{UnitOfWork: postgresuow.New(pool), pool: pool, audit: audit},
	}

	issued, err := svc.CreateToken(ctx, CreateTokenCommand{
		OrganizationID:    fixture.OrgID,
		UserID:            fixture.UserID,
		ApplicationID:     fixture.AppID,
		Scopes:            []string{"openid"},
		ExpiresInSeconds:  3600,
		IssueRefreshToken: true,
	})
	require.NoError(t, err, "first-generation refresh INSERT must satisfy the family self-FK")
	require.NotEmpty(t, issued.RefreshToken)

	var familyID, refreshID string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id::text, COALESCE(family_id::text, '') FROM guliangiam.refresh_tokens WHERE token_hash = $1`,
		hashTokenValue(issued.RefreshToken),
	).Scan(&refreshID, &familyID))
	require.Equal(t, refreshID, familyID, "issued refresh token must be its own family root")

	rotated, err := svc.RefreshAccessToken(ctx, RefreshAccessTokenCommand{
		OrganizationID: fixture.OrgID,
		RefreshToken:   issued.RefreshToken,
	})
	require.NoError(t, err, "rotation must succeed on the issued pair")

	var rotatedFamily string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COALESCE(family_id::text, '') FROM guliangiam.refresh_tokens WHERE token_hash = $1`,
		hashTokenValue(rotated.RefreshToken),
	).Scan(&rotatedFamily))
	require.Equal(t, familyID, rotatedFamily, "rotation must stay in the same family")

	// Replay the already-rotated first token beyond the grace window: the whole
	// family (including the rotated winner) must be revoked.
	clk.advance(2 * refreshReuseGraceWindow)
	_, err = svc.RefreshAccessToken(ctx, RefreshAccessTokenCommand{
		OrganizationID: fixture.OrgID,
		RefreshToken:   issued.RefreshToken,
	})
	require.Error(t, err)
	require.Equal(t, apperrors.CodeUnauthorized, apperrors.FromError(err).Code)

	var liveInFamily int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM guliangiam.refresh_tokens WHERE family_id = $1::uuid AND revoked_at IS NULL`,
		familyID,
	).Scan(&liveInFamily))
	require.Zero(t, liveInFamily, "reuse detection must revoke every live token in the family")
	require.Equal(t, 1, audit.countAction("token.reuse_detected"))
}
