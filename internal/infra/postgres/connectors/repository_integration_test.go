// Real-PostgreSQL evidence for guliangiam.connectors and the connector repository.
//
// Skipped unless GIAM_TEST_PG_DSN points at an already-migrated disposable
// database (V1→V11). The DSN role must bypass FORCE RLS (migration role/table
// owner/superuser) so cross-tenant fixtures and zero-write assertions can read
// the physical rows; every repository statement still carries an explicit
// organization_id predicate, so isolation is exercised independently of RLS.
//
//	GIAM_TEST_PG_DSN='postgres://.../guliangiam_b4a_verify?sslmode=disable' \
//	  GOCACHE=/tmp/giam-b4a-cache go test ./internal/infra/postgres/connectors \
//	  -run TestConnectorRepositoryPostgres -count=1 -v
package connectors

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	domainconnectors "github.com/guliangiam/guliangiam/internal/domain/connectors"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestConnectorRepositoryPostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is not set; skipping real PostgreSQL connectors repository test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))
	t.Cleanup(pool.Close)

	enc, err := crypto.NewFieldEncryptorChecked()
	require.NoError(t, err)
	repo := NewRepository(pool, enc)

	orgA := newConnectorTestOrg(t, ctx, pool, "a")
	orgB := newConnectorTestOrg(t, ctx, pool, "b")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		for _, orgID := range []string{orgA, orgB} {
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM guliangiam.connectors WHERE organization_id = $1::uuid`, orgID)
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM guliangiam.organizations WHERE id = $1::uuid`, orgID)
		}
	})

	t.Run("same-tenant CRUD", func(t *testing.T) {
		connector := newConnectorTestRow(orgA, "crud", "crud-token")
		require.NoError(t, repo.Create(ctx, orgA, connector))
		require.Equal(t, int64(1), connector.Revision)

		got, err := repo.Get(ctx, connector.ID, orgA)
		require.NoError(t, err)
		require.Equal(t, "crud-token", got.BearerToken)
		require.Equal(t, orgA, got.OrganizationID)
		require.True(t, got.IsEnabled)
		require.Equal(t, domainconnectors.DefaultTimeoutMs, got.TimeoutMs)
		require.Equal(t, domainconnectors.DefaultMaxRatePerMin, got.MaxRatePerMin)

		connector.Name = "crud-updated"
		connector.BearerToken = "crud-token-rotated"
		connector.IsEnabled = false
		connector.TimeoutMs = 9000
		require.NoError(t, repo.Update(ctx, orgA, connector))
		require.Equal(t, int64(2), connector.Revision)

		got, err = repo.Get(ctx, connector.ID, orgA)
		require.NoError(t, err)
		require.Equal(t, "crud-updated", got.Name)
		require.Equal(t, "crud-token-rotated", got.BearerToken)
		require.False(t, got.IsEnabled)
		require.Equal(t, 9000, got.TimeoutMs)
		require.Equal(t, int64(2), got.Revision)

		require.NoError(t, repo.Delete(ctx, orgA, connector.ID))
		_, err = repo.Get(ctx, connector.ID, orgA)
		requireErrorCode(t, err, apperrors.CodeNotFound)
	})

	t.Run("cross-tenant reads, updates and deletes write nothing", func(t *testing.T) {
		connector := newConnectorTestRow(orgA, "cross", "cross-token")
		require.NoError(t, repo.Create(ctx, orgA, connector))

		_, err := repo.Get(ctx, connector.ID, orgB)
		requireErrorCode(t, err, apperrors.CodeNotFound)

		before := readConnectorRow(t, ctx, pool, connector.ID)

		forged := *connector
		forged.OrganizationID = orgB
		forged.Name = "forged-by-tenant-b"
		forged.Revision = connector.Revision
		err = repo.Update(ctx, orgB, &forged)
		requireErrorCode(t, err, apperrors.CodeConflict)
		require.Equal(t, before, readConnectorRow(t, ctx, pool, connector.ID),
			"a foreign-tenant update must not change any column")

		err = repo.Delete(ctx, orgB, connector.ID)
		requireErrorCode(t, err, apperrors.CodeNotFound)
		require.Equal(t, before, readConnectorRow(t, ctx, pool, connector.ID),
			"a foreign-tenant delete must leave the row intact")
	})

	t.Run("stale revision is rejected and does not write", func(t *testing.T) {
		connector := newConnectorTestRow(orgA, "stale", "stale-token")
		require.NoError(t, repo.Create(ctx, orgA, connector))

		connector.Name = "stale-first-write"
		require.NoError(t, repo.Update(ctx, orgA, connector))
		require.Equal(t, int64(2), connector.Revision)

		stale := *connector
		stale.Revision = 1
		stale.Name = "stale-second-write"
		stale.BearerToken = "stale-rotated-token"
		err := repo.Update(ctx, orgA, &stale)
		requireErrorCode(t, err, apperrors.CodeConflict)
		require.NotContains(t, err.Error(), "stale-rotated-token")

		row := readConnectorRow(t, ctx, pool, connector.ID)
		require.Equal(t, "stale-first-write", row.name)
		require.Equal(t, int64(2), row.revision)
	})

	t.Run("bearer token is sealed at rest and decrypted in memory", func(t *testing.T) {
		const plaintext = "at-rest-bearer-secret"
		connector := newConnectorTestRow(orgA, "sealed", plaintext)
		require.NoError(t, repo.Create(ctx, orgA, connector))

		row := readConnectorRow(t, ctx, pool, connector.ID)
		require.NotEqual(t, plaintext, row.sealed)
		require.True(t, crypto.IsEnvelopeToken(row.sealed), "stored value must be an envelope token")
		require.NotContains(t, row.sealed, plaintext)

		got, err := repo.Get(ctx, connector.ID, orgA)
		require.NoError(t, err)
		require.Equal(t, plaintext, got.BearerToken)
	})

	t.Run("list never crosses tenant", func(t *testing.T) {
		a := newConnectorTestRow(orgA, "list-a", "list-a-token")
		b := newConnectorTestRow(orgB, "list-b", "list-b-token")
		require.NoError(t, repo.Create(ctx, orgA, a))
		require.NoError(t, repo.Create(ctx, orgB, b))

		itemsA, err := repo.List(ctx, orgA)
		require.NoError(t, err)
		requireConnectorIDsScoped(t, itemsA, orgA)
		require.Contains(t, connectorIDSet(itemsA), a.ID)
		require.NotContains(t, connectorIDSet(itemsA), b.ID)

		itemsB, err := repo.List(ctx, orgB)
		require.NoError(t, err)
		requireConnectorIDsScoped(t, itemsB, orgB)
		require.Contains(t, connectorIDSet(itemsB), b.ID)
		require.NotContains(t, connectorIDSet(itemsB), a.ID)
	})
}

type connectorRowSnapshot struct {
	name     string
	sealed   string
	revision int64
}

func readConnectorRow(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id string) connectorRowSnapshot {
	t.Helper()
	var row connectorRowSnapshot
	err := pool.QueryRow(ctx,
		`SELECT name, bearer_token_sealed, revision FROM guliangiam.connectors WHERE id = $1::uuid`, id,
	).Scan(&row.name, &row.sealed, &row.revision)
	require.NoError(t, err)
	return row
}

func newConnectorTestOrg(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tag string) string {
	t.Helper()
	id := uuid.NewString()
	slug := "b4a-" + tag + "-" + strings.ReplaceAll(id, "-", "")
	_, err := pool.Exec(ctx,
		`INSERT INTO guliangiam.organizations (id, slug, display_name) VALUES ($1::uuid, $2, $3)`,
		id, slug, "B4a connectors "+tag)
	require.NoError(t, err)
	return id
}

func newConnectorTestRow(orgID, nameTag, token string) *domainconnectors.Connector {
	return &domainconnectors.Connector{
		ID:             uuid.NewString(),
		OrganizationID: orgID,
		Name:           "b4a-" + nameTag + "-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10],
		SCIMBaseURL:    "https://scim.example.com/v2",
		BearerToken:    token,
		IsEnabled:      true,
		TimeoutMs:      domainconnectors.DefaultTimeoutMs,
		MaxRatePerMin:  domainconnectors.DefaultMaxRatePerMin,
	}
}

func requireConnectorIDsScoped(t *testing.T, items []*domainconnectors.Connector, organizationID string) {
	t.Helper()
	for _, item := range items {
		require.Equal(t, organizationID, item.OrganizationID,
			"List must only return the requested tenant")
	}
}

func connectorIDSet(items []*domainconnectors.Connector) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		set[item.ID] = struct{}{}
	}
	return set
}
