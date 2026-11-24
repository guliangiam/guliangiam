// Real-PostgreSQL evidence for envelope-encrypting guliangiam.mcp_servers.token
// (A-6.2).
//
// Skipped unless GIAM_TEST_PG_DSN points at an already-migrated disposable
// database. The DSN role must bypass FORCE RLS (migration role/table owner/
// superuser) so the raw-row assertions can read the physical ciphertext; every
// repository statement still carries an explicit organization_id predicate, so
// tenant isolation is exercised independently of RLS.
//
//	GIAM_TEST_PG_DSN='postgres://.../guliangiam_a62_verify?sslmode=disable' \
//	  GOCACHE=/tmp/giam-a62-cache go test ./internal/infra/postgres/mcpservers \
//	  -run TestMcpServerRepositoryPostgres -count=1 -v
package mcpservers

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestMcpServerRepositoryPostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is not set; skipping real PostgreSQL mcp server repository test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))
	t.Cleanup(pool.Close)

	repo := NewRepository(pool)
	orgA := newA62McpTestOrg(t, ctx, pool, "a")
	orgB := newA62McpTestOrg(t, ctx, pool, "b")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		for _, orgID := range []string{orgA, orgB} {
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM guliangiam.mcp_servers WHERE organization_id = $1::uuid`, orgID)
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM guliangiam.organizations WHERE id = $1::uuid`, orgID)
		}
	})

	t.Run("same-tenant CRUD decrypts in memory", func(t *testing.T) {
		server := newA62McpTestRow(orgA, "crud", "tok-crud")
		require.NoError(t, repo.Create(ctx, server))

		got, err := repo.GetByIDForOrganization(ctx, server.ID, orgA)
		require.NoError(t, err)
		require.Equal(t, "tok-crud", got.Token)
		require.Equal(t, orgA, got.OrganizationID)

		server.Token = "tok-crud-rotated"
		server.Revision++
		server.UpdatedAt = time.Now()
		require.NoError(t, repo.Update(ctx, server))

		got, err = repo.GetByIDForOrganization(ctx, server.ID, orgA)
		require.NoError(t, err)
		require.Equal(t, "tok-crud-rotated", got.Token)
		require.Equal(t, int64(2), got.Revision)

		require.NoError(t, repo.Delete(ctx, server.ID, orgA))
		_, err = repo.GetByIDForOrganization(ctx, server.ID, orgA)
		requireErrorCode(t, err, apperrors.CodeNotFound)
	})

	t.Run("stored token is a row-bound v2 envelope", func(t *testing.T) {
		server := newA62McpTestRow(orgA, "sealed", "tok-at-rest")
		require.NoError(t, repo.Create(ctx, server))

		stored := readA62McpToken(t, ctx, pool, server.ID)
		require.NotEqual(t, "tok-at-rest", stored)
		require.True(t, crypto.IsEnvelopeToken(stored), "stored value must be an envelope token")
		require.NotContains(t, stored, "tok-at-rest")
		header, _, _ := strings.Cut(stored, ":")
		require.True(t, strings.HasSuffix(header, "-aad"), "new writes must be v2 (AAD-bound)")

		server.Token = "tok-rotated-at-rest"
		server.Revision++
		require.NoError(t, repo.Update(ctx, server))
		stored = readA62McpToken(t, ctx, pool, server.ID)
		require.True(t, crypto.IsEnvelopeToken(stored))
		require.NotContains(t, stored, "tok-rotated-at-rest")
	})

	t.Run("empty token stays empty on disk", func(t *testing.T) {
		server := newA62McpTestRow(orgA, "empty", "")
		require.NoError(t, repo.Create(ctx, server))
		require.Empty(t, readA62McpToken(t, ctx, pool, server.ID))

		got, err := repo.GetByIDForOrganization(ctx, server.ID, orgA)
		require.NoError(t, err)
		require.Empty(t, got.Token)
	})

	t.Run("cross-tenant read returns not found and writes nothing", func(t *testing.T) {
		server := newA62McpTestRow(orgA, "cross", "tok-cross")
		require.NoError(t, repo.Create(ctx, server))

		_, err := repo.GetByIDForOrganization(ctx, server.ID, orgB)
		requireErrorCode(t, err, apperrors.CodeNotFound)

		before := readA62McpToken(t, ctx, pool, server.ID)
		forged := *server
		forged.OrganizationID = orgB
		forged.Token = "tok-forged"
		err = repo.Update(ctx, &forged)
		requireErrorCode(t, err, apperrors.CodeNotFound)
		require.Equal(t, before, readA62McpToken(t, ctx, pool, server.ID),
			"a foreign-tenant update must not change the stored token")
	})
}

func readA62McpToken(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id string) string {
	t.Helper()
	var token string
	err := pool.QueryRow(ctx, `SELECT token FROM guliangiam.mcp_servers WHERE id = $1::uuid`, id).Scan(&token)
	require.NoError(t, err)
	return token
}

func newA62McpTestOrg(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tag string) string {
	t.Helper()
	id := uuid.NewString()
	slug := "a62-mcp-" + tag + "-" + strings.ReplaceAll(id, "-", "")
	_, err := pool.Exec(ctx,
		`INSERT INTO guliangiam.organizations (id, slug, display_name) VALUES ($1::uuid, $2, $3)`,
		id, slug, "A6.2 mcp "+tag)
	require.NoError(t, err)
	return id
}

func newA62McpTestRow(orgID, tag, token string) *domain.McpServer {
	return &domain.McpServer{
		ID:             uuid.NewString(),
		OrganizationID: orgID,
		Name:           "a62-" + tag + "-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10],
		DisplayName:    "A6.2 mcp " + tag,
		URL:            "https://mcp.example.com",
		Token:          token,
		Application:    "",
		Tools:          []domain.Tool{},
		IsEnabled:      true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Revision:       1,
	}
}

func requireErrorCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, code, appErr.Code)
}
