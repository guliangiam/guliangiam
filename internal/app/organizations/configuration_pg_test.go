package organizations_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	apporgs "github.com/guliangiam/guliangiam/internal/app/organizations"
	postgresuow "github.com/guliangiam/guliangiam/internal/infra/postgres/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// TestUpdateOrganizationConfigurationPostgres proves the aggregate save is
// transactional: a failing settings section rolls back the core row change.
func TestUpdateOrganizationConfigurationPostgres(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", "BX1Ab1Jz6cRDESuWCleZWGRv7RHQRAf0sqikOVcVzkg=")
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is required for the organization configuration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	var orgID, originalName string
	var revision int64
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT id::text, display_name, revision FROM guliangiam.organizations WHERE slug = 'built-in' LIMIT 1").Scan(&orgID, &originalName, &revision))

	svc := apporgs.NewService(clock.SystemClock{}, platformuuid.V7Generator{}, postgresuow.New(pool, pool))
	systemCtx := tenantctx.System(ctx)
	originalNameCopy := originalName
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "UPDATE guliangiam.organizations SET display_name = $1 WHERE id = $2::uuid", originalNameCopy, orgID)
	})

	newName := originalName + " (config)"
	core := apporgs.PatchOrganizationCommand{ID: orgID, DisplayName: &newName, Revision: revision}
	settings := apporgs.UpdateOrganizationSettingsCommand{OrganizationID: orgID, Languages: []string{"en"}, CountryCodes: []string{"US"}}
	theme := apporgs.UpdateOrganizationThemeCommand{OrganizationID: orgID, ThemeName: "default"}
	security := apporgs.UpdateOrganizationSecurityPolicyCommand{OrganizationID: orgID}
	require.NoError(t, svc.UpdateOrganizationConfiguration(systemCtx, apporgs.UpdateOrganizationConfigurationCommand{
		Core: core, Settings: settings, Theme: theme, SecurityPolicy: security,
	}))

	var savedName string
	require.NoError(t, pool.QueryRow(ctx, "SELECT display_name FROM guliangiam.organizations WHERE id = $1::uuid", orgID).Scan(&savedName))
	require.Equal(t, newName, savedName)

	// A bad settings section must roll the whole save back.
	var currentRevision int64
	require.NoError(t, pool.QueryRow(ctx, "SELECT revision FROM guliangiam.organizations WHERE id = $1::uuid", orgID).Scan(&currentRevision))
	rollbackName := originalName + " (must rollback)"
	badSettings := apporgs.UpdateOrganizationSettingsCommand{OrganizationID: orgID, Languages: []string{"en"}, CountryCodes: []string{"invalid"}}
	err = svc.UpdateOrganizationConfiguration(systemCtx, apporgs.UpdateOrganizationConfigurationCommand{
		Core:           apporgs.PatchOrganizationCommand{ID: orgID, DisplayName: &rollbackName, Revision: currentRevision},
		Settings:       badSettings,
		Theme:          apporgs.UpdateOrganizationThemeCommand{OrganizationID: orgID, ThemeName: "default"},
		SecurityPolicy: apporgs.UpdateOrganizationSecurityPolicyCommand{OrganizationID: orgID},
	})
	require.Error(t, err)
	require.NoError(t, pool.QueryRow(ctx, "SELECT display_name FROM guliangiam.organizations WHERE id = $1::uuid", orgID).Scan(&savedName))
	require.Equal(t, newName, savedName, "core change must not commit when a later section fails")
}
