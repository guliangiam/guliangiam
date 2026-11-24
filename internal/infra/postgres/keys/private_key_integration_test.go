//go:build integration

package keys

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/keys"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	platformdb "github.com/guliangiam/guliangiam/internal/platform/db"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// TestPrivateKeyEncryption_Integration runs against a dedicated, already
// migrated database supplied by KEYS_PRIVATE_KEY_TEST_DSN. It covers the
// operator sequence on a real PostgreSQL schema: sealed write -> read, legacy
// plaintext read rejection, generic backfill, and verify with zero plaintext.
func TestPrivateKeyEncryption_Integration(t *testing.T) {
	dsn := os.Getenv("KEYS_PRIVATE_KEY_TEST_DSN")
	if dsn == "" {
		t.Skip("KEYS_PRIVATE_KEY_TEST_DSN not set — isolated PostgreSQL integration test skipped")
	}

	ctx, cancel := context.WithTimeout(tenantctx.System(context.Background()), 30*time.Second)
	defer cancel()

	pool, err := platformdb.NewPostgresPool(ctx, platformdb.PostgresConfig{
		DataSourceName: dsn,
		MaxConns:       4,
	})
	require.NoError(t, err)
	defer pool.Close()

	orgID := "01900000-0000-7000-8000-00000000a001"
	_, err = pool.Exec(ctx, `
		INSERT INTO guliangiam.organizations (id, slug, display_name)
		VALUES ($1, 'r6-verify', 'GM R6 verify')
		ON CONFLICT (id) DO NOTHING`, orgID)
	require.NoError(t, err)

	// The test owns these fixed ids; deleting them first makes the scenario
	// re-runnable on the same isolated database.
	_, err = pool.Exec(ctx, `
		DELETE FROM guliangiam.keys
		WHERE id IN ($1, $2, $3, $4)`,
		"01900000-0000-7000-8000-00000000a002",
		"01900000-0000-7000-8000-00000000a003",
		"01900000-0000-7000-8000-00000000a004",
		"01900000-0000-7000-8000-00000000a005",
	)
	require.NoError(t, err)

	repo := NewRepository(pool)
	now := time.Now()
	material := testPrivateKeyMaterial

	t.Run("sealed write then read", func(t *testing.T) {
		keyID := "01900000-0000-7000-8000-00000000a002"
		require.NoError(t, repo.Create(ctx, &domain.Key{
			ID: keyID, OrganizationID: orgID, Name: "r6-sealed", DisplayName: "R6 sealed",
			KeyType: "RSA", PublicKey: "public-material", PrivateKey: material,
			Algorithm: "RS256", IsEnabled: true, CreatedAt: now, UpdatedAt: now, Revision: 1,
		}))

		var stored string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT private_key FROM guliangiam.keys WHERE id = $1`, keyID).Scan(&stored))
		require.True(t, crypto.IsEnvelopeToken(stored), "private_key must be stored as an envelope token")

		got, err := repo.GetByID(ctx, keyID, orgID)
		require.NoError(t, err)
		require.Equal(t, material, got.PrivateKey)
		t.Logf("write->read OK, stored header=%q", strings.SplitN(stored, ":", 2)[0])
	})

	t.Run("legacy plaintext fails closed until backfill", func(t *testing.T) {
		keyID := "01900000-0000-7000-8000-00000000a003"
		_, err := pool.Exec(ctx, `
			INSERT INTO guliangiam.keys (id, organization_id, name, key_type, public_key, private_key)
			VALUES ($1, $2, 'r6-legacy', 'RSA', 'legacy-public', $3)`, keyID, orgID, material)
		require.NoError(t, err)

		_, err = repo.GetByID(ctx, keyID, orgID)
		require.Error(t, err, "pre-R6 plaintext must not be returned as usable material")
		require.Contains(t, err.Error(), "backfill-encrypted-columns")

		columns := []crypto.EncryptedColumn{{Table: keyTable, Column: keyColumnPrivateKey, PKColumn: "id"}}
		sqlDB := stdlib.OpenDBFromPool(pool)
		defer sqlDB.Close()

		stats, remaining, err := crypto.BackfillEncryptedColumns(ctx, sqlDB, crypto.GlobalFieldEncryptor(), columns,
			crypto.EncryptedColumnBackfillOptions{})
		require.NoError(t, err)
		require.False(t, remaining)
		require.Len(t, stats, 1)
		require.GreaterOrEqual(t, stats[0].Encrypted, int64(1))
		t.Logf("backfill: scanned=%d encrypted=%d skipped=%d failed=%d",
			stats[0].Scanned, stats[0].Encrypted, stats[0].Skipped, stats[0].Failed)

		got, err := repo.GetByID(ctx, keyID, orgID)
		require.NoError(t, err, "backfilled row must be readable")
		require.Equal(t, material, got.PrivateKey)

		stats, remaining, err = crypto.BackfillEncryptedColumns(ctx, sqlDB, crypto.GlobalFieldEncryptor(), columns,
			crypto.EncryptedColumnBackfillOptions{Verify: true})
		require.NoError(t, err)
		require.False(t, remaining)
		require.Len(t, stats, 1)
		require.Zero(t, stats[0].Plaintext)
		t.Logf("verify: plaintext=%d", stats[0].Plaintext)
	})

	t.Run("empty private key stays SQL NULL", func(t *testing.T) {
		keyID := "01900000-0000-7000-8000-00000000a004"
		require.NoError(t, repo.Create(ctx, &domain.Key{
			ID: keyID, OrganizationID: orgID, Name: "r6-empty", DisplayName: "R6 empty",
			KeyType: "RSA", PublicKey: "public-only", Algorithm: "RS256",
			IsEnabled: true, CreatedAt: now, UpdatedAt: now, Revision: 1,
		}))

		var stored sql.NullString
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT private_key FROM guliangiam.keys WHERE id = $1`, keyID).Scan(&stored))
		require.False(t, stored.Valid, "empty private key must stay NULL, not become an envelope of \"\"")

		got, err := repo.GetByID(ctx, keyID, orgID)
		require.NoError(t, err)
		require.Empty(t, got.PrivateKey)
	})
}
