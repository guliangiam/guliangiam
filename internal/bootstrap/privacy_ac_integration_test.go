package bootstrap

import (
	"context"
	"os"
	"testing"
	"time"

	appprivacy "github.com/guliangiam/guliangiam/internal/app/privacy"
	domain "github.com/guliangiam/guliangiam/internal/domain/privacy"
	postgresuow "github.com/guliangiam/guliangiam/internal/infra/postgres/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestACPRIV001_DSARLifecyclePostgres is the real-database evidence for the six
// DSAR request types: creation, SLA clock, state machine, immutable events and
// audit records (AC-PRIV-001).
func TestACPRIV001_DSARLifecyclePostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	var organizationID, subjectID string
	err = pool.QueryRow(ctx, `
SELECT u.organization_id, u.id
FROM guliangiam.users u
JOIN guliangiam.organizations o ON o.id = u.organization_id
WHERE u.username = 'admin' AND u.deleted_at IS NULL AND o.deleted_at IS NULL
ORDER BY u.created_at
LIMIT 1`).Scan(&organizationID, &subjectID)
	require.NoError(t, err)

	service := appprivacy.NewService(clock.SystemClock{}, platformuuid.V7Generator{}, newPrivacyRunner(postgresuow.New(pool)))
	requestTypes := []string{
		domain.RequestTypeAccess,
		domain.RequestTypeRectify,
		domain.RequestTypeFreeze,
		domain.RequestTypeErase,
		domain.RequestTypePortability,
		domain.RequestTypeWithdrawConsent,
	}
	ids := make([]string, 0, len(requestTypes))
	for _, requestType := range requestTypes {
		created, err := service.Create(ctx, appprivacy.CreateDsarRequestCommand{
			OrganizationID: organizationID,
			SubjectType:    domain.SubjectTypeUser,
			SubjectID:      subjectID,
			RequestType:    requestType,
			ActorID:        subjectID,
		})
		require.NoError(t, err)
		ids = append(ids, created.ID)
		assert.WithinDuration(t, time.Now().Add(domain.SLABudget), created.SLADueAt, 10*time.Second)
		assert.Equal(t, domain.StateReceived, created.State)

		revision := created.Revision
		for _, target := range []string{domain.StateVerifying, domain.StateInProgress, domain.StateFulfilled} {
			updated, err := service.Transition(ctx, appprivacy.TransitionDsarRequestCommand{
				OrganizationID: organizationID,
				ID:             created.ID,
				Target:         target,
				ActorID:        subjectID,
				Revision:       revision,
			})
			require.NoError(t, err)
			assert.Equal(t, target, updated.State)
			revision = updated.Revision
		}
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM guliangiam.dsar_events WHERE dsar_id = ANY($1)`, ids)
		_, _ = pool.Exec(context.Background(), `DELETE FROM guliangiam.dsar_requests WHERE id = ANY($1)`, ids)
	})

	for _, id := range ids {
		var events int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM guliangiam.dsar_events WHERE dsar_id = $1`, id).Scan(&events))
		assert.GreaterOrEqual(t, events, 4, "created + 3 transitions must be immutable events")
		var audits int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM guliangiam.records WHERE organization_id = $1 AND resource_id = $2 AND action LIKE 'dsar.request.%'`,
			organizationID, id).Scan(&audits))
		assert.GreaterOrEqual(t, audits, 4, "every state change must write an audit record")
	}
}
