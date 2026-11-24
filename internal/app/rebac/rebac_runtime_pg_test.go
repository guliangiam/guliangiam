package rebac_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	apprebac "github.com/guliangiam/guliangiam/internal/app/rebac"
	domainrebac "github.com/guliangiam/guliangiam/internal/domain/rebac"
	rebacpg "github.com/guliangiam/guliangiam/internal/infra/postgres/rebac"
	postgresuow "github.com/guliangiam/guliangiam/internal/infra/postgres/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	transportrebac "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/rebac"
)

// TestRebacCheckZookieRevocationPostgres is the real-PostgreSQL evidence for
// OPEN-DESIGN-01 (AC-REBAC-002): check correctness, Zookie read-after-write,
// revocation and audit rows, all through the real repositories and UOW.
func TestRebacCheckZookieRevocationPostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is required for the ReBAC runtime test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	var orgID string
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT o.id::text FROM guliangiam.organizations o WHERE o.slug = 'built-in' LIMIT 1").Scan(&orgID))

	uow := postgresuow.New(pool)
	svc := apprebac.NewService(clock.SystemClock{}, platformuuid.V7Generator{}, uow)
	systemCtx := tenantctx.System(ctx)

	namespace, err := svc.CreateNamespace(systemCtx, apprebac.CreateNamespaceCommand{
		OrganizationID: orgID, Name: "doc-" + uuid.NewString()[:8], DisplayName: "Document",
		RelationDefs: map[string]any{"roles": []string{"viewer"}, "subjectTypes": []string{"user"}},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = svc.DeleteNamespace(systemCtx, namespace.ID, orgID)
	})

	repo := rebacpg.NewRepository(pool)
	engine := apprebac.NewEngine(repo)
	handler := transportrebac.NewHandler(repo, engine, &svc)
	zookieBefore, err := repo.GetLatestZookie(systemCtx, orgID, namespace.ID)
	require.NoError(t, err)

	tuple, err := svc.CreateTuple(systemCtx, apprebac.CreateTupleCommand{
		OrganizationID: orgID, NamespaceID: namespace.ID,
		Subject: "user:jane", Relation: "viewer", Object: "doc:project_alpha",
	})
	require.NoError(t, err)

	zookieAfter, err := repo.GetLatestZookie(systemCtx, orgID, namespace.ID)
	require.NoError(t, err)
	require.NotEqual(t, zookieBefore, zookieAfter, "write must advance the Zookie revision")

	duplicate, err := svc.CreateTuple(systemCtx, apprebac.CreateTupleCommand{
		OrganizationID: orgID, NamespaceID: namespace.ID,
		Subject: "user:jane", Relation: "viewer", Object: "doc:project_alpha",
	})
	require.NoError(t, err, "repeating an existing tuple write must be idempotent")
	require.Equal(t, tuple.ID, duplicate.ID, "idempotent writes must return the persisted tuple ID")
	zookieAfterDuplicate, err := repo.GetLatestZookie(systemCtx, orgID, namespace.ID)
	require.NoError(t, err)
	require.NotEqual(t, zookieAfter, zookieAfterDuplicate, "idempotent writes still advance the namespace revision")
	require.Equal(t, zookieAfterDuplicate, duplicate.Zookie, "the response Zookie must reflect the current write revision")

	check := func(atZookie string) domainrebac.CheckResult {
		result, err := engine.Check(systemCtx, domainrebac.CheckRequest{
			OrganizationID: orgID, NamespaceID: namespace.ID,
			Tuple:    domainrebac.TupleKey{Subject: "user:jane", Relation: "viewer", Object: "doc:project_alpha"},
			AtZookie: atZookie,
		})
		require.NoError(t, err)
		return *result
	}
	require.False(t, check(zookieBefore).Allowed, "stale Zookie must not see the new tuple")
	require.True(t, check("").Allowed, "latest snapshot must allow the direct tuple")
	require.True(t, check(zookieAfterDuplicate).Allowed, "latest write Zookie must see the tuple")

	require.NoError(t, svc.DeleteTuple(systemCtx, tuple.ID, orgID))
	require.False(t, check("").Allowed, "tuple deletion must revoke the decision")

	var createdAudits, deletedAudits int
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT count(*) FROM guliangiam.records WHERE organization_id = $1::uuid AND action = 'rebac.tuple.created' AND resource_id = $2",
		orgID, tuple.ID).Scan(&createdAudits))
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT count(*) FROM guliangiam.records WHERE organization_id = $1::uuid AND action = 'rebac.tuple.deleted' AND resource_id = $2",
		orgID, tuple.ID).Scan(&deletedAudits))
	require.GreaterOrEqual(t, createdAudits, 1, "tuple creation must be audited")
	require.GreaterOrEqual(t, deletedAudits, 1, "tuple deletion must be audited")

	// AC-REBAC-001: unregistered namespace / relation / subject type are 400.
	assertInvalidArgument := func(t *testing.T, err error) {
		t.Helper()
		require.Error(t, err)
		require.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
	}
	_, err = handler.Check(systemCtx, transportrebac.CheckRequest{
		OrganizationID: orgID, NamespaceID: uuid.NewString(), Subject: "user:jane", Relation: "viewer", Object: "doc:x",
	})
	assertInvalidArgument(t, err)
	_, err = handler.Check(systemCtx, transportrebac.CheckRequest{
		OrganizationID: orgID, NamespaceID: namespace.ID, Subject: "user:jane", Relation: "editor", Object: "doc:x",
	})
	assertInvalidArgument(t, err)
	_, err = handler.Check(systemCtx, transportrebac.CheckRequest{
		OrganizationID: orgID, NamespaceID: namespace.ID, Subject: "service:bot", Relation: "viewer", Object: "doc:x",
	})
	assertInvalidArgument(t, err)
}
