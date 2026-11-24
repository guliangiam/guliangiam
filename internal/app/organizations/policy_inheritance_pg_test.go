package organizations

import (
	"context"
	"os"
	"testing"
	"time"

	postgresorg "github.com/guliangiam/guliangiam/internal/infra/postgres/organizations"
	postgresuow "github.com/guliangiam/guliangiam/internal/infra/postgres/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPreviewPolicyInheritancePostgres exercises the root→child lock chain on
// the real runtime schema (AC-B2B-001 runtime evidence).
func TestPreviewPolicyInheritancePostgres(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", "BX1Ab1Jz6cRDESuWCleZWGRv7RHQRAf0sqikOVcVzkg=")
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	const rootID = "00000000-0000-4000-8000-00000000b2b1"
	const childID = "00000000-0000-4000-8000-00000000b2b2"
	cleanup := func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM guliangiam.organizations WHERE id = ANY($1)`, []string{childID, rootID})
	}
	cleanup()
	t.Cleanup(cleanup)
	_, err = pool.Exec(ctx, `
INSERT INTO guliangiam.organizations (id, slug, display_name) VALUES ($1, 'b2b-root-test', 'B2B Root'), ($2, 'b2b-child-test', 'B2B Child')`, rootID, childID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE guliangiam.organizations SET parent_id = $1 WHERE id = $2`, rootID, childID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO guliangiam.organization_security_policies (organization_id, password_policy)
VALUES ($1, '{"minLength":8}'::jsonb)`, rootID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO guliangiam.organization_policy_inheritance (id, organization_id, policy_key, mode)
VALUES (gen_random_uuid(), $1, 'passwordPolicy', 'lock')`, rootID)
	require.NoError(t, err)

	service := NewService(clock.SystemClock{}, platformuuid.V7Generator{}, postgresuow.New(pool))
	service.PolicyInheritance = postgresorg.NewRepository(pool)
	preview, err := service.PreviewPolicyInheritance(ctx, childID, "passwordPolicy")
	require.NoError(t, err)
	assert.True(t, preview.Locked)
	assert.Equal(t, rootID, preview.LockedByOrganizationID)
	assert.Equal(t, rootID, preview.EffectiveSourceOrganizationID)
	assert.EqualValues(t, 12, preview.EffectiveValue.(map[string]any)["minLength"])
}
