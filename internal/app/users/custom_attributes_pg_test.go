package users

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
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// TestCustomAttributesPostgres is the real-PostgreSQL evidence for OPEN-API-59:
// definitions are stored/listed/updated and profile updates reject keys that
// the tenant has not declared.
func TestCustomAttributesPostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is required for the custom attributes test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	var orgID, userID string
	require.NoError(t, pool.QueryRow(ctx, "SELECT u.organization_id::text, u.id::text FROM guliangiam.users u JOIN guliangiam.organizations o ON o.id = u.organization_id WHERE u.username = 'admin' AND o.slug = 'built-in' LIMIT 1").Scan(&orgID, &userID))

	uow := postgresuow.New(pool)
	svc := &Service{Clock: clock.SystemClock{}, UUID: platformuuid.V7Generator{}, UOW: uow}
	systemCtx := tenantctx.System(ctx)

	key := "department_" + uuid.NewString()[:8]
	created, err := svc.CreateCustomAttribute(systemCtx, CreateCustomAttributeCommand{
		OrganizationID: orgID, Key: key, DataType: "string", Visibility: "private",
	})
	require.NoError(t, err)
	require.Equal(t, key, created.Key)
	t.Cleanup(func() {
		_ = svc.DeleteCustomAttribute(systemCtx, orgID, created.ID, "")
	})

	list, err := svc.ListCustomAttributes(systemCtx, orgID, false)
	require.NoError(t, err)
	found := false
	for _, definition := range list {
		if definition.Key == key {
			found = true
		}
	}
	require.True(t, found)

	required := true
	updated, err := svc.UpdateCustomAttribute(systemCtx, UpdateCustomAttributeCommand{
		OrganizationID: orgID, Key: key, DataType: "string", Required: required, Visibility: "private",
	})
	require.NoError(t, err)
	require.True(t, updated.Required)
	require.Equal(t, int64(2), updated.Revision)

	// A profile update with an undeclared key is rejected before persistence.
	err = svc.UpdateUserProfile(systemCtx, UpdateUserProfileCommand{
		OrganizationID: orgID, UserID: userID,
		Properties: map[string]any{"undeclared_key": "x"},
	})
	require.Error(t, err)
	require.Equal(t, apperrors.CodeUnprocessableEntity, apperrors.FromError(err).Code)

	// Declared keys pass the definition validation.
	err = svc.UpdateUserProfile(systemCtx, UpdateUserProfileCommand{
		OrganizationID: orgID, UserID: userID,
		Properties: map[string]any{key: "platform"},
	})
	require.NoError(t, err)
}
