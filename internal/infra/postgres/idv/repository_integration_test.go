// Real-PostgreSQL evidence for envelope-encrypting
// guliangiam.identity_verifications.session_token (A-6.2).
//
// Skipped unless GIAM_TEST_PG_DSN points at an already-migrated disposable
// database. The DSN role must bypass FORCE RLS (migration role/table owner/
// superuser) so the raw-row assertion can read the physical ciphertext; every
// repository statement still carries an explicit organization_id predicate.
//
//	GIAM_TEST_PG_DSN='postgres://.../guliangiam_a62_verify?sslmode=disable' \
//	  GOCACHE=/tmp/giam-a62-cache go test ./internal/infra/postgres/idv \
//	  -run TestIdvRepositoryPostgres -count=1 -v
package idv

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/idv"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestIdvRepositoryPostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is not set; skipping real PostgreSQL idv repository test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))
	t.Cleanup(pool.Close)

	repo := NewRepository(pool)
	orgA := newA62IdvTestOrg(t, ctx, pool, "a")
	orgB := newA62IdvTestOrg(t, ctx, pool, "b")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		for _, orgID := range []string{orgA, orgB} {
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM guliangiam.identity_verifications WHERE organization_id = $1::uuid`, orgID)
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM guliangiam.organizations WHERE id = $1::uuid`, orgID)
		}
	})

	t.Run("create seals session_token and reads decrypt in memory", func(t *testing.T) {
		verification := newA62IdvTestRow(orgA, "crud", strPtr("sess-crud"))
		require.NoError(t, repo.Create(ctx, verification))

		stored := readA62IdvSessionToken(t, ctx, pool, verification.ID)
		require.NotEqual(t, "sess-crud", *stored)
		require.True(t, crypto.IsEnvelopeToken(*stored), "stored value must be an envelope token")
		require.NotContains(t, *stored, "sess-crud")
		header, _, _ := strings.Cut(*stored, ":")
		require.True(t, strings.HasSuffix(header, "-aad"), "new writes must be v2 (AAD-bound)")

		got, err := repo.GetByID(ctx, verification.ID, orgA)
		require.NoError(t, err)
		require.NotNil(t, got.SessionToken)
		require.Equal(t, "sess-crud", *got.SessionToken)

		byRef, err := repo.GetByProviderRef(ctx, verification.Provider, "a62-ref-crud")
		require.NoError(t, err)
		require.Equal(t, "sess-crud", *byRef.SessionToken)
	})

	t.Run("update keeps the sealed token untouched", func(t *testing.T) {
		verification := newA62IdvTestRow(orgA, "update", strPtr("sess-update"))
		require.NoError(t, repo.Create(ctx, verification))
		before := readA62IdvSessionToken(t, ctx, pool, verification.ID)

		got, err := repo.GetByID(ctx, verification.ID, orgA)
		require.NoError(t, err)
		require.Equal(t, "sess-update", *got.SessionToken)
		got.Status = domain.StatusApproved
		completed := time.Now()
		got.CompletedAt = &completed
		got.UpdatedAt = completed
		require.NoError(t, repo.Update(ctx, got))

		require.Equal(t, *before, *readA62IdvSessionToken(t, ctx, pool, verification.ID),
			"UPDATE does not write session_token, so the envelope must survive")
	})

	t.Run("nil session_token stays NULL", func(t *testing.T) {
		verification := newA62IdvTestRow(orgA, "nil", nil)
		require.NoError(t, repo.Create(ctx, verification))

		var stored *string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT session_token FROM guliangiam.identity_verifications WHERE id = $1::uuid`, verification.ID).
			Scan(&stored))
		require.Nil(t, stored)
	})

	t.Run("cross-tenant read creates no leak", func(t *testing.T) {
		verification := newA62IdvTestRow(orgA, "cross", strPtr("sess-cross"))
		require.NoError(t, repo.Create(ctx, verification))

		_, err := repo.GetByID(ctx, verification.ID, orgB)
		requireErrorCode(t, err, apperrors.CodeNotFound)
	})

	t.Run("PII sealed columns are written and plaintext fallback reads", func(t *testing.T) {
		idNumber := "AB1234567"
		verification := newA62IdvTestRow(orgA, "pii", strPtr("sess-pii"))
		verification.IDNumber = &idNumber
		verification.RawResult = json.RawMessage(`{"document":"AB1234567"}`)
		require.NoError(t, repo.Create(ctx, verification))

		var idPlain, idSealed, rawSealed *string
		var rawPlain []byte
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT id_number, id_number_sealed, raw_result, raw_result_sealed
			   FROM guliangiam.identity_verifications WHERE id = $1::uuid`, verification.ID).
			Scan(&idPlain, &idSealed, &rawPlain, &rawSealed))
		require.NotNil(t, idPlain)
		require.Equal(t, idNumber, *idPlain, "the legacy plaintext column stays authoritative")
		require.JSONEq(t, string(verification.RawResult), string(rawPlain))
		require.NotNil(t, idSealed)
		require.NotNil(t, rawSealed)

		idCol, err := identityVerificationSealedColumn(identityVerificationColumnIDNumber)
		require.NoError(t, err)
		opened, err := crypto.GlobalFieldEncryptor().DecryptColumn(idCol.AAD(verification.ID), *idSealed)
		require.NoError(t, err)
		require.Equal(t, idNumber, opened)

		rawCol, err := identityVerificationSealedColumn(identityVerificationColumnRawResult)
		require.NoError(t, err)
		openedRaw, err := crypto.GlobalFieldEncryptor().DecryptColumn(rawCol.AAD(verification.ID), *rawSealed)
		require.NoError(t, err)
		require.JSONEq(t, string(verification.RawResult), openedRaw)

		// Simulate an upgrade-window row written before the dual-write batch.
		_, err = pool.Exec(ctx,
			`UPDATE guliangiam.identity_verifications
			    SET id_number_sealed = NULL, raw_result_sealed = NULL
			  WHERE id = $1::uuid`, verification.ID)
		require.NoError(t, err)

		got, err := repo.GetByID(ctx, verification.ID, orgA)
		require.NoError(t, err)
		require.NotNil(t, got.IDNumber)
		require.Equal(t, idNumber, *got.IDNumber)
		require.JSONEq(t, string(verification.RawResult), string(got.RawResult))
	})
}

func readA62IdvSessionToken(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id string) *string {
	t.Helper()
	var token *string
	err := pool.QueryRow(ctx,
		`SELECT session_token FROM guliangiam.identity_verifications WHERE id = $1::uuid`, id).Scan(&token)
	require.NoError(t, err)
	return token
}

func newA62IdvTestOrg(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tag string) string {
	t.Helper()
	id := uuid.NewString()
	slug := "a62-idv-" + tag + "-" + strings.ReplaceAll(id, "-", "")
	_, err := pool.Exec(ctx,
		`INSERT INTO guliangiam.organizations (id, slug, display_name) VALUES ($1::uuid, $2, $3)`,
		id, slug, "A6.2 idv "+tag)
	require.NoError(t, err)
	return id
}

func newA62IdvTestRow(orgID, tag string, sessionToken *string) *domain.Verification {
	now := time.Now()
	return &domain.Verification{
		ID:             uuid.NewString(),
		OrganizationID: orgID,
		Provider:       "jumio",
		Status:         domain.StatusPending,
		SessionToken:   sessionToken,
		ProviderRef:    strPtr("a62-ref-" + tag),
		RawResult:      json.RawMessage(`{"a62":"ok"}`),
		InitiatedAt:    now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func requireErrorCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, code, appErr.Code)
}
