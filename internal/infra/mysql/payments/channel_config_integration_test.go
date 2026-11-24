package payments

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/stretchr/testify/require"
)

// TestChannelConfigRepositoryMySQL proves the payments channel-config chain
// works on the generated MySQL DDL (OPEN-API-37 non-identity runtime evidence).
func TestChannelConfigRepositoryMySQL(t *testing.T) {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		t.Skip("MYSQL_DSN is required")
	}
	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	const orgID = "01a0ea76-0c00-7000-8000-000000000001"
	_, err = db.ExecContext(ctx, `INSERT IGNORE INTO organizations (id, slug, display_name, revision) VALUES (?, 'built-in', 'Built-in', 1)`, orgID)
	require.NoError(t, err)

	repo := NewChannelConfigsRepository(db)
	config := &domain.PayChannelConfig{
		ID: uuid.NewString(), OrganizationID: orgID, Channel: "alipay", Environment: "sandbox",
		VersionNo: 1, Status: "draft", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Revision: 1,
	}
	require.NoError(t, repo.Create(ctx, config))
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM pay_channel_configs WHERE id = ?`, config.ID) })

	loaded, err := repo.GetByIDForOrganization(ctx, config.ID, orgID)
	require.NoError(t, err)
	require.Equal(t, "alipay", loaded.Channel)

	items, err := repo.List(ctx, orgID)
	require.NoError(t, err)
	require.NotEmpty(t, items)
}

// TestReconBatchRepositoryMySQL proves the payments reconciliation batch chain
// works on the generated MySQL DDL (OPEN-API-37 non-identity runtime evidence).
func TestReconBatchRepositoryMySQL(t *testing.T) {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		t.Skip("MYSQL_DSN is required")
	}
	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	const orgID = "01a0ea76-0c00-7000-8000-000000000001"
	_, err = db.ExecContext(ctx, `INSERT IGNORE INTO organizations (id, slug, display_name, revision) VALUES (?, 'built-in', 'Built-in', 1)`, orgID)
	require.NoError(t, err)

	repo := NewReconRepository(db)
	batch := &domain.ReconBatch{
		ID: uuid.NewString(), OrganizationID: orgID, Channel: "alipay",
		BatchDate: time.Now().UTC().Truncate(24 * time.Hour), Status: "pending",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Revision: 1,
	}
	require.NoError(t, repo.CreateBatch(ctx, batch))
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM pay_recon_batches WHERE id = ?`, batch.ID) })

	loaded, err := repo.GetBatchForOrganization(ctx, batch.ID, orgID)
	require.NoError(t, err)
	require.Equal(t, "alipay", loaded.Channel)
	require.Equal(t, "pending", loaded.Status)

	batches, total, err := repo.Batches(ctx, orgID, 10, 0)
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, int64(1))
	require.NotEmpty(t, batches)

	batch.Status = "completed"
	batch.UpdatedAt = time.Now().UTC()
	require.NoError(t, repo.UpdateBatch(ctx, batch))
	updated, err := repo.GetBatchForOrganization(ctx, batch.ID, orgID)
	require.NoError(t, err)
	require.Equal(t, "completed", updated.Status)
}
