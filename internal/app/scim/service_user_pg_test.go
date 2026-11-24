package scim

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/domain/users"
	postgresuow "github.com/guliangiam/guliangiam/internal/infra/postgres/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// TestSCIMUserWritePathsPostgres is the real-PostgreSQL evidence for the SCIM
// ①② close condition: POST creates a complete aggregate (id/timestamps/
// profile), PUT/PATCH persist user_profiles, a foreign tenant stays 404, a
// stale revision stays 409, and a profile failure rolls the aggregate back.
//
// Skipped unless GIAM_TEST_PG_DSN points at an already-migrated disposable
// database. The role must bypass FORCE RLS (migration role, BYPASSRLS or
// superuser): Cleanup and the fixtures write without a tenant GUC.
//
//	GIAM_TEST_PG_DSN='postgres://.../guliangiam_scim_write2?sslmode=disable' \
//	  GOCACHE=/tmp/giam-scim2-cache go test ./internal/app/scim \
//	  -run TestSCIMUserWritePathsPostgres -count=1 -v -vet=off
func TestSCIMUserWritePathsPostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is not set; skipping real PostgreSQL SCIM write-path test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, pool.Ping(ctx))

	orgA := insertSCIMWritePathsOrg(t, ctx, pool, "a")
	orgB := insertSCIMWritePathsOrg(t, ctx, pool, "b")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		for _, orgID := range []string{orgA, orgB} {
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM guliangiam.users WHERE organization_id = $1::uuid`, orgID)
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM guliangiam.organizations WHERE id = $1::uuid`, orgID)
		}
	})

	uow := postgresuow.New(pool)
	svc := NewService(clock.SystemClock{}, platformuuid.V7Generator{}, uow)
	ctxA := tenantctx.WithTenantScope(context.Background(), orgA, "", tenantctx.TenantFilterStrict)
	ctxB := tenantctx.WithTenantScope(context.Background(), orgB, "", tenantctx.TenantFilterStrict)

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	email := "scim-write-" + suffix + "@example.com"

	var createdID string

	t.Run("POST returns a complete aggregate and persists profile", func(t *testing.T) {
		created, err := svc.CreateUser(ctxA, orgA, ScimUser{
			UserName:     "scim-create-" + suffix,
			ExternalID:   "ext-create-" + suffix,
			DisplayName:  "SCIM Created",
			Active:       true,
			Name:         &ScimName{GivenName: "Created", FamilyName: "User"},
			Emails:       []ScimEmail{{Value: email}},
			PhoneNumbers: []ScimPhone{{Value: "+8613900000000"}},
		})
		require.NoError(t, err)
		require.NotNil(t, created)
		_, err = uuid.Parse(created.ID)
		require.NoError(t, err, "POST must return a generated UUID")
		assert.False(t, created.Meta.Created.IsZero())
		assert.Equal(t, "SCIM Created", created.DisplayName)

		stored := loadSCIMUserInTenant(t, ctxA, uow, created.ID, orgA)
		assert.Equal(t, int64(1), stored.Revision)
		assert.Equal(t, created.ID, stored.Profile.UserID)
		assert.Equal(t, "SCIM Created", stored.Profile.DisplayName)
		assert.Equal(t, "Created", stored.Profile.FirstName)
		assert.Equal(t, "User", stored.Profile.LastName)
		assert.Equal(t, email, stored.Profile.Email)
		assert.Equal(t, "+8613900000000", stored.Profile.Phone)
		assert.False(t, stored.CreatedAt.IsZero())
		assert.False(t, stored.UpdatedAt.IsZero())
		createdID = created.ID
	})

	t.Run("PUT persists displayName name emails and phoneNumbers", func(t *testing.T) {
		updated, err := svc.ReplaceUser(ctxA, orgA, createdID, ScimUser{
			UserName:     "scim-replaced-" + suffix,
			ExternalID:   "ext-replaced-" + suffix,
			DisplayName:  "SCIM Replaced",
			Active:       true,
			Name:         &ScimName{GivenName: "Replaced", FamilyName: "Person"},
			Emails:       []ScimEmail{{Value: "scim-replaced-" + suffix + "@example.com"}},
			PhoneNumbers: []ScimPhone{{Value: "+8613900000001"}},
		})
		require.NoError(t, err)
		require.NotNil(t, updated)

		stored := loadSCIMUserInTenant(t, ctxA, uow, createdID, orgA)
		assert.Equal(t, int64(2), stored.Revision)
		assert.Equal(t, "ext-replaced-"+suffix, stored.ExternalID)
		assert.Equal(t, "SCIM Replaced", stored.Profile.DisplayName)
		assert.Equal(t, "Replaced", stored.Profile.FirstName)
		assert.Equal(t, "Person", stored.Profile.LastName)
		assert.Equal(t, "scim-replaced-"+suffix+"@example.com", stored.Profile.Email)
		assert.Equal(t, "+8613900000001", stored.Profile.Phone)
	})

	t.Run("PATCH persists mutated profile attributes", func(t *testing.T) {
		patched, err := svc.PatchUser(ctxA, orgA, createdID, ScimPatchOp{
			Operations: []ScimOperation{
				{Op: "replace", Path: "displayName", Value: "SCIM Patched"},
				{Op: "replace", Path: "emails", Value: map[string]any{"value": "scim-patched-" + suffix + "@example.com"}},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, patched)

		stored := loadSCIMUserInTenant(t, ctxA, uow, createdID, orgA)
		assert.Equal(t, int64(3), stored.Revision)
		assert.Equal(t, "SCIM Patched", stored.Profile.DisplayName)
		assert.Equal(t, "scim-patched-"+suffix+"@example.com", stored.Profile.Email)
	})

	t.Run("foreign tenant is 404 and writes nothing", func(t *testing.T) {
		_, err := svc.ReplaceUser(ctxB, orgB, createdID, ScimUser{
			UserName:    "scim-foreign-" + suffix,
			ExternalID:  "ext-foreign-" + suffix,
			DisplayName: "Foreign",
			Active:      true,
		})
		requireAppErrorCode(t, err, apperrors.CodeNotFound)

		stored := loadSCIMUserInTenant(t, ctxA, uow, createdID, orgA)
		assert.Equal(t, "ext-replaced-"+suffix, stored.ExternalID)
		assert.Equal(t, "SCIM Patched", stored.Profile.DisplayName)
	})

	t.Run("stale revision is 409 and writes nothing", func(t *testing.T) {
		racing := &scimRacingUOW{AuthUoW: uow, raceRevision: true}
		racingSvc := NewService(clock.SystemClock{}, platformuuid.V7Generator{}, racing)

		_, err := racingSvc.ReplaceUser(ctxA, orgA, createdID, ScimUser{
			UserName:    "scim-stale-" + suffix,
			ExternalID:  "ext-stale-" + suffix,
			DisplayName: "Stale",
			Active:      true,
		})
		requireAppErrorCode(t, err, apperrors.CodeConflict)

		stored := loadSCIMUserInTenant(t, ctxA, uow, createdID, orgA)
		assert.Equal(t, "ext-concurrent-writer", stored.ExternalID, "concurrent writer must win")
		assert.Equal(t, "SCIM Patched", stored.Profile.DisplayName, "stale profile update must not land")
		assert.Equal(t, int64(4), stored.Revision)
	})

	t.Run("DELETE deactivates without deleting the row", func(t *testing.T) {
		require.NoError(t, svc.DeleteUser(ctxA, orgA, createdID))

		stored := loadSCIMUserInTenant(t, ctxA, uow, createdID, orgA)
		assert.True(t, stored.IsForbidden, "SCIM DELETE is deactivation, not physical delete")
		assert.Nil(t, stored.DeletedAt, "soft-delete column must stay NULL for deactivation")
		assert.Greater(t, stored.Revision, int64(4))
	})

	t.Run("profile failure rolls back the whole create", func(t *testing.T) {
		ownerEmail := "scim-rollback-" + suffix + "@example.com"
		_, err := svc.CreateUser(ctxA, orgA, ScimUser{
			UserName:    "scim-rollback-owner-" + suffix,
			DisplayName: "Rollback Owner",
			Active:      true,
			Emails:      []ScimEmail{{Value: ownerEmail}},
		})
		require.NoError(t, err)

		duplicateUserName := "scim-rollback-dup-" + suffix
		_, err = svc.CreateUser(ctxA, orgA, ScimUser{
			UserName:    duplicateUserName,
			DisplayName: "Rollback Duplicate",
			Active:      true,
			Emails:      []ScimEmail{{Value: ownerEmail}},
		})
		require.Error(t, err, "duplicate profile email must fail")

		var lookupErr error
		_ = uow.WithinTransaction(ctxA, func(ctx context.Context, tx appuow.UnitOfWork) error {
			_, lookupErr = tx.Users().GetByUsername(ctx, orgA, duplicateUserName)
			return nil
		})
		requireAppErrorCode(t, lookupErr, apperrors.CodeNotFound)
	})
}

func insertSCIMWritePathsOrg(t *testing.T, ctx context.Context, pool *pgxpool.Pool, suffix string) string {
	t.Helper()
	orgID := uuid.NewString()
	slug := "scim-write2-" + suffix + "-" + strings.ReplaceAll(orgID, "-", "")
	_, err := pool.Exec(ctx,
		`INSERT INTO guliangiam.organizations (id, slug, display_name) VALUES ($1::uuid, $2, $3)`,
		orgID, slug, "SCIM write paths integration")
	require.NoError(t, err)
	return orgID
}

func loadSCIMUserInTenant(t *testing.T, ctx context.Context, uow appuow.UnitOfWork, id, organizationID string) *users.User {
	t.Helper()
	var loaded *users.User
	err := uow.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		user, err := tx.Users().GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}
		loaded = user
		return nil
	})
	require.NoError(t, err)
	return loaded
}

// scimRacingUOW performs a competing write after the service's tenant-scoped
// load, so the subsequent PUT deterministically hits the repository's
// optimistic-lock guard instead of racing on wall-clock timing.
type scimRacingUOW struct {
	appuow.AuthUoW
	raceRevision bool
}

func (u *scimRacingUOW) Users() users.Repository {
	return &scimRacingRepo{
		Repository:   u.AuthUoW.Users(),
		uow:          u.AuthUoW,
		raceRevision: u.raceRevision,
	}
}

func (u *scimRacingUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return u.AuthUoW.WithinTransaction(ctx, fn)
}

type scimRacingRepo struct {
	users.Repository
	uow          appuow.AuthUoW
	raceRevision bool
}

func (r *scimRacingRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*users.User, error) {
	var loaded *users.User
	err := r.uow.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		user, err := tx.Users().GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if r.raceRevision {
			concurrent := *user
			concurrent.ExternalID = "ext-concurrent-writer"
			concurrent.UpdatedAt = time.Now().UTC()
			concurrent.Revision++
			if err := tx.Users().UpdateForOrganization(ctx, &concurrent, organizationID); err != nil {
				return err
			}
		}
		loaded = user
		return nil
	})
	return loaded, err
}
