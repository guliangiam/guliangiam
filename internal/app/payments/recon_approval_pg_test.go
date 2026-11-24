package payments

import (
	"context"
	"os"
	"testing"
	"time"

	infrapgpayments "github.com/guliangiam/guliangiam/internal/infra/postgres/payments"
	postgresuow "github.com/guliangiam/guliangiam/internal/infra/postgres/uow"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReconciliationApprovalPostgres is the AC-PAY-008 runtime evidence:
// the DB CHECK, second-person rule, audit record and outbox event are exercised
// against a real PostgreSQL schema.
func TestReconciliationApprovalPostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	const orgID = "01a0ea76-0c00-7000-8000-000000000001"
	const resolverID = "01a0ea76-0c00-7000-8000-00000000c001"
	const approverID = "01a0ea76-0c00-7000-8000-00000000c002"
	const batchID = "01a0ea76-0c00-7000-8000-00000000c010"
	const itemID = "01a0ea76-0c00-7000-8000-00000000c011"
	cleanup := func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM guliangiam.records WHERE resource_id = $1", itemID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM guliangiam.outbox WHERE aggregate_id = $1", itemID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM guliangiam.pay_recon_items WHERE id = $1", itemID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM guliangiam.pay_recon_batches WHERE id = $1", batchID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM guliangiam.users WHERE id = ANY($1)", []string{resolverID, approverID})
	}
	cleanup()
	t.Cleanup(cleanup)

	_, err = pool.Exec(ctx, "INSERT INTO guliangiam.users (id, organization_id, username) VALUES ($1, $2, 'l18-resolver'), ($3, $2, 'l18-approver')", resolverID, orgID, approverID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "INSERT INTO guliangiam.pay_recon_batches (id, organization_id, channel, batch_date) VALUES ($1, $2, 'stripe', CURRENT_DATE)", batchID, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO guliangiam.pay_recon_items
		(id, batch_id, diff_type, resolved, resolution_state, resolved_by, resolved_at, created_at)
		VALUES ($1, $2, 'amount_mismatch', TRUE, 'resolved', $3, now(), now())`, itemID, batchID, resolverID)
	require.NoError(t, err)

	svc := NewReconService(infrapgpayments.NewReconRepository(pool), nil, platformuuid.V7Generator{}, postgresuow.New(pool))
	require.NoError(t, svc.ApproveItem(ctx, orgID, itemID, approverID))

	var approvedBy, state string
	require.NoError(t, pool.QueryRow(ctx, "SELECT approved_by::text, resolution_state FROM guliangiam.pay_recon_items WHERE id = $1", itemID).Scan(&approvedBy, &state))
	assert.Equal(t, approverID, approvedBy)
	assert.Equal(t, "approved", state)

	var auditCount, outboxCount int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM guliangiam.records WHERE resource_id = $1 AND action = 'payment.recon.item.approved'", itemID).Scan(&auditCount))
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM guliangiam.outbox WHERE aggregate_id = $1 AND event_type = 'payment.recon.item.approved'", itemID).Scan(&outboxCount))
	assert.Equal(t, 1, auditCount)
	assert.Equal(t, 1, outboxCount)

	// Same person cannot approve their own resolution; the DB CHECK and SQL
	// predicate both reject it.
	err = svc.ApproveItem(ctx, orgID, itemID, resolverID)
	require.Error(t, err)
}
