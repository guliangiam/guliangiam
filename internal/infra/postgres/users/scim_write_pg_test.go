// Real-PostgreSQL evidence for the SCIM user write path (SCIM P1).
//
// Skipped unless GIAM_TEST_PG_DSN points at an already-migrated disposable
// database (V1→current). Run it with:
//
//	GIAM_TEST_PG_DSN='postgres://.../guliangiam_scim_write?sslmode=disable' \
//	  GOCACHE=/tmp/giam-scim-cache go test ./internal/infra/postgres/users \
//	  -run TestSCIMUserWritesPostgres -count=1 -v
//
// The DSN must belong to a role that bypasses FORCE RLS (migration role, table
// owner, or superuser): the fixture creates its own organizations/users without
// a tenant GUC and removes exactly those rows afterwards.
//
// What this pins, against a real schema rather than mocks:
//   - UpdateForOrganization compares the caller's Revision-1 (the value loaded
//     from the row), so the SCIM path must increment Revision before writing;
//   - a stale revision in the caller's own tenant is a Conflict and writes
//     nothing (no child-table upsert);
//   - a foreign organization is NotFound, writes nothing, and remains invisible;
//   - the SCIM DELETE/disable write keeps balance/MFA/collections/custom rows
//     intact because it carries the loaded aggregate, not zero values.
package users

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestSCIMUserWritesPostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is not set; skipping real PostgreSQL SCIM user write test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, pool.Ping(ctx))

	orgA := insertSCIMWriteOrg(t, ctx, pool, "a")
	orgB := insertSCIMWriteOrg(t, ctx, pool, "b")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		for _, orgID := range []string{orgA, orgB} {
			// users cascades to profiles/credentials/child tables; organizations
			// uses RESTRICT, so the user rows must go first.
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM guliangiam.users WHERE organization_id = $1::uuid`, orgID)
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM guliangiam.organizations WHERE id = $1::uuid`, orgID)
		}
	})

	repo := NewRepository(pool)
	user := scimWriteFixture(t, orgA)
	require.NoError(t, repo.Create(ctx, user))

	t.Run("scoped replace bumps revision and keeps the aggregate", func(t *testing.T) {
		loaded := loadSCIMWriteUser(t, ctx, repo, user.ID, orgA)
		loaded.Username = "scim-p1-replace"
		loaded.ExternalID = "ext-replace"
		loaded.UpdatedAt = time.Now().UTC()
		loaded.Revision++

		require.NoError(t, repo.UpdateForOrganization(ctx, loaded, orgA))

		got := loadSCIMWriteUser(t, ctx, repo, user.ID, orgA)
		assert.Equal(t, int64(2), got.Revision, "revision must be the loaded value + 1")
		assert.Equal(t, "ext-replace", got.ExternalID)
		assertAggregatePreserved(t, got)
	})

	t.Run("foreign tenant read stays not found", func(t *testing.T) {
		_, err := repo.GetByIDForOrganization(ctx, user.ID, orgB)
		var appErr *apperrors.Error
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	})

	t.Run("stale revision is a conflict and writes nothing", func(t *testing.T) {
		// A concurrent scoped writer moves the row from revision 2 to 3.
		current := loadSCIMWriteUser(t, ctx, repo, user.ID, orgA)
		require.Equal(t, int64(2), current.Revision)
		current.ExternalID = "ext-concurrent"
		current.UpdatedAt = time.Now().UTC()
		current.Revision++
		require.NoError(t, repo.UpdateForOrganization(ctx, current, orgA))

		// The SCIM request loaded revision 2 before that write. Even after the
		// SCIM-side increment, the repository guard is revision 2 and misses.
		stale := loadSCIMWriteUser(t, ctx, repo, user.ID, orgA)
		stale.Revision = 2
		stale.ExternalID = "ext-stale"
		stale.Revision++

		err := repo.UpdateForOrganization(ctx, stale, orgA)
		var appErr *apperrors.Error
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, apperrors.CodeConflict, appErr.Code)

		after := loadSCIMWriteUser(t, ctx, repo, user.ID, orgA)
		assert.Equal(t, int64(3), after.Revision, "stale write must not bump revision")
		assert.Equal(t, "ext-concurrent", after.ExternalID, "stale write must not land")
		assertAggregatePreserved(t, after)
	})

	t.Run("foreign tenant write is not found and writes nothing", func(t *testing.T) {
		before := loadSCIMWriteUser(t, ctx, repo, user.ID, orgA)
		before.ExternalID = "ext-cross-tenant"
		before.Revision++

		err := repo.UpdateForOrganization(ctx, before, orgB)
		var appErr *apperrors.Error
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, apperrors.CodeNotFound, appErr.Code)

		after := loadSCIMWriteUser(t, ctx, repo, user.ID, orgA)
		assert.Equal(t, "ext-concurrent", after.ExternalID, "foreign write must not land")
		assertAggregatePreserved(t, after)
	})

	t.Run("disable keeps balance mfa collections and custom attributes", func(t *testing.T) {
		loaded := loadSCIMWriteUser(t, ctx, repo, user.ID, orgA)
		loaded.IsForbidden = true
		loaded.UpdatedAt = time.Now().UTC()
		loaded.Revision++

		require.NoError(t, repo.UpdateForOrganization(ctx, loaded, orgA))

		got := loadSCIMWriteUser(t, ctx, repo, user.ID, orgA)
		assert.True(t, got.IsForbidden)
		assert.False(t, got.IsDeleted, "SCIM DELETE is disable, not a soft delete")
		assert.Equal(t, int64(4), got.Revision)
		assertAggregatePreserved(t, got)
	})
}

func insertSCIMWriteOrg(t *testing.T, ctx context.Context, pool *pgxpool.Pool, suffix string) string {
	t.Helper()
	orgID := uuid.NewString()
	slug := "scim-p1-" + suffix + "-" + strings.ReplaceAll(orgID, "-", "")
	_, err := pool.Exec(ctx,
		`INSERT INTO guliangiam.organizations (id, slug, display_name) VALUES ($1::uuid, $2, $3)`,
		orgID, slug, "SCIM P1 write integration")
	require.NoError(t, err)
	return orgID
}

func scimWriteFixture(t *testing.T, orgID string) *domain.User {
	t.Helper()
	id := uuid.NewString()
	now := time.Now().UTC()
	return &domain.User{
		ID:                 id,
		OrganizationID:     orgID,
		Username:           "scim-p1-" + strings.ReplaceAll(id, "-", ""),
		ExternalID:         "ext-initial",
		UserType:           "normal",
		Status:             "active",
		CreatedAt:          now,
		UpdatedAt:          now,
		Revision:           1,
		Balance:            123.45,
		BalanceCredit:      6.78,
		BalanceCurrency:    "CNY",
		PreferredMfaType:   "totp",
		MfaPushEnabled:     true,
		MfaPushReceiver:    "receiver-1",
		RecoveryCodeHashes: []string{"recovery-code-1"},
		Custom1:            "keep-custom1",
		Custom9:            "keep-custom9",
		Addresses:          []domain.Address{{Region: "CN", Street: "main", Details: "1"}},
		ApplicationScopes: []domain.ConsentRecord{
			{Application: "app-1", Scopes: []string{"openid"}, GrantedAt: "2026-11-03T00:00:00Z"},
		},
		Profile: domain.UserProfile{UserID: id, DisplayName: "Initial Display"},
	}
}

func loadSCIMWriteUser(t *testing.T, ctx context.Context, repo Repository, id, organizationID string) *domain.User {
	t.Helper()
	user, err := repo.GetByIDForOrganization(ctx, id, organizationID)
	require.NoError(t, err)
	return user
}

func assertAggregatePreserved(t *testing.T, got *domain.User) {
	t.Helper()
	assert.InDelta(t, 123.45, got.Balance, 0.0001)
	assert.InDelta(t, 6.78, got.BalanceCredit, 0.0001)
	assert.Equal(t, "CNY", got.BalanceCurrency)
	assert.Equal(t, "totp", got.PreferredMfaType)
	assert.True(t, got.MfaPushEnabled)
	assert.Equal(t, "receiver-1", got.MfaPushReceiver)
	require.Equal(t, []string{"recovery-code-1"}, got.RecoveryCodeHashes)
	assert.Equal(t, "keep-custom1", got.Custom1)
	assert.Equal(t, "keep-custom9", got.Custom9)
	require.Len(t, got.Addresses, 1)
	require.Len(t, got.ApplicationScopes, 1)
}
