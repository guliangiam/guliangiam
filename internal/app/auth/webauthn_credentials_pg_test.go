package auth

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	postgresuow "github.com/guliangiam/guliangiam/internal/infra/postgres/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// TestWebAuthnCredentialsOwnershipPostgres is the real-PostgreSQL L4 evidence
// for OPEN-API-56 ④: a self-service caller can only list/revoke credentials that
// belong to the session user in the session organization.
//
// Run:
//
//	GIAM_TEST_PG_DSN='postgres://owner@/disposable?host=/tmp&sslmode=disable' \
//	  go test -run TestWebAuthnCredentialsOwnershipPostgres ./internal/app/auth/
func TestWebAuthnCredentialsOwnershipPostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is required for the WebAuthn credential ownership test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fixture, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(fixture.Close)

	suffix := uuid.NewString()[:8]
	orgA, orgB := uuid.New(), uuid.New()
	userA, userB := uuid.New(), uuid.New()
	credA, credB := uuid.New(), uuid.New()

	tx, err := fixture.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "SELECT set_config('app.current_role', 'super_admin', true)")
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "INSERT INTO guliangiam.organizations (id, slug, display_name) VALUES ($1, $2, $3), ($4, $5, $6)",
		orgA, "wa-a-"+suffix, "WA A", orgB, "wa-b-"+suffix, "WA B")
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "INSERT INTO guliangiam.users (id, organization_id, username) VALUES ($1, $2, $3), ($4, $5, $6)",
		userA, orgA, "wa-user-a-"+suffix, userB, orgB, "wa-user-b-"+suffix)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "INSERT INTO guliangiam.webauthn_credentials (id, user_id, credential_id, public_key, name, status) VALUES ($1, $2, $3, $4, $5, 'active'), ($6, $7, $8, $9, $10, 'active')",
		credA, userA, []byte("credential-a-"+suffix), []byte{1, 2, 3}, "Laptop A",
		credB, userB, []byte("credential-b-"+suffix), []byte{4, 5, 6}, "Laptop B")
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		cleanupTx, beginErr := fixture.Begin(cleanupCtx)
		if beginErr != nil {
			return
		}
		_, _ = cleanupTx.Exec(cleanupCtx, "SELECT set_config('app.current_role', 'super_admin', true)")
		_, _ = cleanupTx.Exec(cleanupCtx, "DELETE FROM guliangiam.webauthn_credentials WHERE id = ANY($1::uuid[])", []uuid.UUID{credA, credB})
		_, _ = cleanupTx.Exec(cleanupCtx, "DELETE FROM guliangiam.users WHERE id = ANY($1::uuid[])", []uuid.UUID{userA, userB})
		_, _ = cleanupTx.Exec(cleanupCtx, "DELETE FROM guliangiam.organizations WHERE id = ANY($1::uuid[])", []uuid.UUID{orgA, orgB})
		_ = cleanupTx.Commit(cleanupCtx)
	})

	uow := postgresuow.New(fixture)
	svc := &Service{UOW: uow}
	ctxA := tenantctx.WithTenantScope(context.Background(), orgA.String(), "", tenantctx.TenantFilterStrict)
	ctxB := tenantctx.WithTenantScope(context.Background(), orgB.String(), "", tenantctx.TenantFilterStrict)

	credentials, err := svc.ListWebAuthnCredentials(ctxA, userA.String(), orgA.String())
	require.NoError(t, err)
	require.Len(t, credentials, 1)
	require.Equal(t, credA.String(), credentials[0].ID)

	_, err = svc.ListWebAuthnCredentials(ctxB, userA.String(), orgB.String())
	require.Error(t, err)
	require.Equal(t, apperrors.CodeForbidden, apperrors.FromError(err).Code)

	err = svc.RevokeWebAuthnCredential(ctxA, userA.String(), orgA.String(), credB.String())
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	require.Equal(t, "active", webauthnCredentialStatusForTest(t, ctx, fixture, credB))

	require.NoError(t, svc.RevokeWebAuthnCredential(ctxA, userA.String(), orgA.String(), credA.String()))
	require.Equal(t, "revoked", webauthnCredentialStatusForTest(t, ctx, fixture, credA))
}

func webauthnCredentialStatusForTest(t *testing.T, ctx context.Context, fixture *pgxpool.Pool, id uuid.UUID) string {
	t.Helper()
	var status string
	require.NoError(t, fixture.QueryRow(ctx, "SELECT status FROM guliangiam.webauthn_credentials WHERE id = $1", id).Scan(&status))
	return status
}

// TestWebAuthnCredentialIDUniqueConstraintPostgres proves the DB constraint,
// not the application pre-check, rejects duplicate credential IDs. The insert
// below bypasses the app layer entirely.
func TestWebAuthnCredentialIDUniqueConstraintPostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is required for the WebAuthn duplicate constraint test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fixture, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(fixture.Close)

	suffix := uuid.NewString()[:8]
	orgID, userA, userB := uuid.New(), uuid.New(), uuid.New()
	credA, credB := uuid.New(), uuid.New()
	credentialID := []byte("duplicate-credential-" + suffix)

	tx, err := fixture.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "SELECT set_config('app.current_role', 'super_admin', true)")
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "INSERT INTO guliangiam.organizations (id, slug, display_name) VALUES ($1, $2, $3)",
		orgID, "wa-dup-"+suffix, "WA Duplicate")
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "INSERT INTO guliangiam.users (id, organization_id, username) VALUES ($1, $2, $3), ($4, $5, $6)",
		userA, orgID, "wa-dup-a-"+suffix, userB, orgID, "wa-dup-b-"+suffix)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "INSERT INTO guliangiam.webauthn_credentials (id, user_id, credential_id, public_key, name, status) VALUES ($1, $2, $3, $4, $5, 'active')",
		credA, userA, credentialID, []byte{1, 2, 3}, "Laptop A")
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		cleanupTx, beginErr := fixture.Begin(cleanupCtx)
		if beginErr != nil {
			return
		}
		_, _ = cleanupTx.Exec(cleanupCtx, "SELECT set_config('app.current_role', 'super_admin', true)")
		_, _ = cleanupTx.Exec(cleanupCtx, "DELETE FROM guliangiam.webauthn_credentials WHERE id = ANY($1::uuid[])", []uuid.UUID{credA, credB})
		_, _ = cleanupTx.Exec(cleanupCtx, "DELETE FROM guliangiam.users WHERE id = ANY($1::uuid[])", []uuid.UUID{userA, userB})
		_, _ = cleanupTx.Exec(cleanupCtx, "DELETE FROM guliangiam.organizations WHERE id = ANY($1::uuid[])", []uuid.UUID{orgID})
		_ = cleanupTx.Commit(cleanupCtx)
	})

	tx, err = fixture.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "SELECT set_config('app.current_role', 'super_admin', true)")
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "INSERT INTO guliangiam.webauthn_credentials (id, user_id, credential_id, public_key, name, status) VALUES ($1, $2, $3, $4, $5, 'active')",
		credB, userB, credentialID, []byte{4, 5, 6}, "Attacker")
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "expected a PostgreSQL error, got %T", err)
	require.Equal(t, "23505", pgErr.Code)
	require.Equal(t, "uq_webauthn_credentials_credential_id", pgErr.ConstraintName)
	require.NoError(t, tx.Rollback(ctx))
}

// TestWebAuthnCredentialRollbackLeavesNoPartialRowPostgres proves a credential
// written inside WithinTransaction disappears when a later step fails.
func TestWebAuthnCredentialRollbackLeavesNoPartialRowPostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is required for the WebAuthn rollback test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fixture, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(fixture.Close)

	suffix := uuid.NewString()[:8]
	orgID, userID, credRowID := uuid.New(), uuid.New(), uuid.New()
	credentialID := []byte("rollback-credential-" + suffix)

	tx, err := fixture.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "SELECT set_config('app.current_role', 'super_admin', true)")
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "INSERT INTO guliangiam.organizations (id, slug, display_name) VALUES ($1, $2, $3)",
		orgID, "wa-rollback-"+suffix, "WA Rollback")
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "INSERT INTO guliangiam.users (id, organization_id, username) VALUES ($1, $2, $3)",
		userID, orgID, "wa-rollback-"+suffix)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		cleanupTx, beginErr := fixture.Begin(cleanupCtx)
		if beginErr != nil {
			return
		}
		_, _ = cleanupTx.Exec(cleanupCtx, "SELECT set_config('app.current_role', 'super_admin', true)")
		_, _ = cleanupTx.Exec(cleanupCtx, "DELETE FROM guliangiam.webauthn_credentials WHERE id = $1", credRowID)
		_, _ = cleanupTx.Exec(cleanupCtx, "DELETE FROM guliangiam.users WHERE id = $1", userID)
		_, _ = cleanupTx.Exec(cleanupCtx, "DELETE FROM guliangiam.organizations WHERE id = $1", orgID)
		_ = cleanupTx.Commit(cleanupCtx)
	})

	uow := postgresuow.New(fixture)
	txCtx := tenantctx.WithTenantScope(context.Background(), orgID.String(), "", tenantctx.TenantFilterStrict)
	injected := errors.New("injected failure after credential create")
	err = uow.WithinTransaction(txCtx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		require.NoError(t, tx.WebAuthnCredentials().Create(ctx, &webauthndomain.Credential{
			ID:              credRowID.String(),
			UserID:          userID.String(),
			CredentialID:    credentialID,
			PublicKey:       []byte{1, 2, 3},
			AttestationType: "none",
			Transport:       []string{},
			Name:            "Rollback",
			Status:          webauthndomain.CredentialStatusActive,
			CreatedAt:       time.Now().UTC(),
			UpdatedAt:       time.Now().UTC(),
		}))
		return injected
	})
	require.ErrorIs(t, err, injected)

	_, err = uow.WebAuthnCredentials().GetByCredentialID(txCtx, credentialID)
	require.Error(t, err)
	require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}
