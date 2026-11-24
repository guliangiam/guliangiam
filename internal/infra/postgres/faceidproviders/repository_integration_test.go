// Real-PostgreSQL evidence for envelope-encrypting
// guliangiam.face_id_providers.access_key_secret (A-6.1).
//
// Skipped unless GIAM_TEST_PG_DSN points at an already-migrated disposable
// database. The DSN role must bypass FORCE RLS (migration role/table owner/
// superuser) so the raw-row assertions can read the physical ciphertext; every
// repository statement still carries an explicit organization_id predicate, so
// tenant isolation is exercised independently of RLS.
//
//	GIAM_TEST_PG_DSN='postgres://.../guliangiam_a61_verify?sslmode=disable' \
//	  GOCACHE=/tmp/giam-a61-cache go test ./internal/infra/postgres/faceidproviders \
//	  -run TestFaceIdProviderRepositoryPostgres -count=1 -v
package faceidproviders

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/faceidproviders"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestFaceIdProviderRepositoryPostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is not set; skipping real PostgreSQL face id provider repository test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))
	t.Cleanup(pool.Close)

	repo := NewRepository(pool)
	orgA := newFaceIdTestOrg(t, ctx, pool, "a")
	orgB := newFaceIdTestOrg(t, ctx, pool, "b")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		for _, orgID := range []string{orgA, orgB} {
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM guliangiam.face_id_providers WHERE organization_id = $1::uuid`, orgID)
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM guliangiam.organizations WHERE id = $1::uuid`, orgID)
		}
	})

	t.Run("same-tenant CRUD decrypts in memory", func(t *testing.T) {
		provider := newFaceIdTestRow(orgA, "crud", "sk-crud")
		require.NoError(t, repo.Create(ctx, provider))
		require.Equal(t, int64(1), provider.Revision)

		got, err := repo.GetByID(ctx, provider.ID, orgA)
		require.NoError(t, err)
		require.Equal(t, "sk-crud", got.AccessKeySecret)
		require.False(t, got.SecretNeedsRotation)
		require.Equal(t, orgA, got.OrganizationID)
		require.True(t, got.IsEnabled)

		provider.Name = "crud-updated"
		provider.AccessKeySecret = "sk-crud-rotated"
		provider.Revision++
		provider.UpdatedAt = time.Now()
		require.NoError(t, repo.Update(ctx, provider))
		require.Equal(t, int64(2), provider.Revision)

		got, err = repo.GetByID(ctx, provider.ID, orgA)
		require.NoError(t, err)
		require.Equal(t, "crud-updated", got.Name)
		require.Equal(t, "sk-crud-rotated", got.AccessKeySecret)
		require.Equal(t, int64(2), got.Revision)

		require.NoError(t, repo.Delete(ctx, provider.ID, orgA))
		_, err = repo.GetByID(ctx, provider.ID, orgA)
		requireErrorCode(t, err, apperrors.CodeNotFound)
	})

	t.Run("stored secret is a row-bound v2 envelope", func(t *testing.T) {
		provider := newFaceIdTestRow(orgA, "sealed", "sk-at-rest")
		require.NoError(t, repo.Create(ctx, provider))

		stored := readFaceIdSecret(t, ctx, pool, provider.ID)
		require.NotEqual(t, "sk-at-rest", stored)
		require.True(t, crypto.IsEnvelopeToken(stored), "stored value must be an envelope token")
		require.NotContains(t, stored, "sk-at-rest")
		header, _, _ := strings.Cut(stored, ":")
		require.True(t, strings.HasSuffix(header, "-aad"), "new writes must be v2 (AAD-bound)")

		provider.AccessKeySecret = "sk-rotated-at-rest"
		provider.Revision++
		require.NoError(t, repo.Update(ctx, provider))
		stored = readFaceIdSecret(t, ctx, pool, provider.ID)
		require.True(t, crypto.IsEnvelopeToken(stored))
		require.NotContains(t, stored, "sk-rotated-at-rest")
	})

	t.Run("cross-tenant reads, updates and deletes write nothing", func(t *testing.T) {
		provider := newFaceIdTestRow(orgA, "cross", "sk-cross")
		require.NoError(t, repo.Create(ctx, provider))

		_, err := repo.GetByID(ctx, provider.ID, orgB)
		requireErrorCode(t, err, apperrors.CodeNotFound)

		before := readFaceIdRow(t, ctx, pool, provider.ID)

		forged := *provider
		forged.OrganizationID = orgB
		forged.Name = "forged-by-tenant-b"
		err = repo.Update(ctx, &forged)
		requireErrorCode(t, err, apperrors.CodeConflict)
		require.Equal(t, before, readFaceIdRow(t, ctx, pool, provider.ID),
			"a foreign-tenant update must not change any column")

		err = repo.Delete(ctx, provider.ID, orgB)
		requireErrorCode(t, err, apperrors.CodeNotFound)
		require.Equal(t, before, readFaceIdRow(t, ctx, pool, provider.ID),
			"a foreign-tenant delete must leave the row intact")
	})

	t.Run("list never crosses tenant", func(t *testing.T) {
		a := newFaceIdTestRow(orgA, "list-a", "sk-list-a")
		b := newFaceIdTestRow(orgB, "list-b", "sk-list-b")
		require.NoError(t, repo.Create(ctx, a))
		require.NoError(t, repo.Create(ctx, b))

		itemsA, _, err := repo.List(ctx, domain.ListQuery{OrganizationID: orgA, Page: 1, PageSize: 50})
		require.NoError(t, err)
		for _, item := range itemsA {
			require.Equal(t, orgA, item.OrganizationID, "List must only return the requested tenant")
		}
		idsA := faceIdTestIDSet(itemsA)
		require.Contains(t, idsA, a.ID)
		require.NotContains(t, idsA, b.ID)

		itemsB, _, err := repo.List(ctx, domain.ListQuery{OrganizationID: orgB, Page: 1, PageSize: 50})
		require.NoError(t, err)
		for _, item := range itemsB {
			require.Equal(t, orgB, item.OrganizationID, "List must only return the requested tenant")
		}
		idsB := faceIdTestIDSet(itemsB)
		require.Contains(t, idsB, b.ID)
		require.NotContains(t, idsB, a.ID)
	})
}

type faceIdRowSnapshot struct {
	name     string
	secret   string
	revision int64
}

func readFaceIdRow(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id string) faceIdRowSnapshot {
	t.Helper()
	var row faceIdRowSnapshot
	err := pool.QueryRow(ctx,
		`SELECT name, access_key_secret, revision FROM guliangiam.face_id_providers WHERE id = $1::uuid`, id,
	).Scan(&row.name, &row.secret, &row.revision)
	require.NoError(t, err)
	return row
}

func readFaceIdSecret(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id string) string {
	t.Helper()
	var secret string
	err := pool.QueryRow(ctx,
		`SELECT access_key_secret FROM guliangiam.face_id_providers WHERE id = $1::uuid`, id,
	).Scan(&secret)
	require.NoError(t, err)
	return secret
}

func newFaceIdTestOrg(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tag string) string {
	t.Helper()
	id := uuid.NewString()
	slug := "a61-" + tag + "-" + strings.ReplaceAll(id, "-", "")
	_, err := pool.Exec(ctx,
		`INSERT INTO guliangiam.organizations (id, slug, display_name) VALUES ($1::uuid, $2, $3)`,
		id, slug, "A6.1 face id "+tag)
	require.NoError(t, err)
	return id
}

func newFaceIdTestRow(orgID, tag, secret string) *domain.FaceIdProvider {
	return &domain.FaceIdProvider{
		ID:              uuid.NewString(),
		OrganizationID:  orgID,
		Name:            "a61-" + tag + "-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10],
		ProviderType:    "Aliyun",
		AccessKeyID:     "ak-" + tag,
		AccessKeySecret: secret,
		Endpoint:        "https://faceid.example.com",
		Config:          map[string]any{"region": "cn-hangzhou"},
		IsEnabled:       true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		Revision:        1,
	}
}

func faceIdTestIDSet(items []*domain.FaceIdProvider) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		set[item.ID] = struct{}{}
	}
	return set
}

func requireErrorCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, code, appErr.Code)
}
