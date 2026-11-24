package risk_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	apprisk "github.com/guliangiam/guliangiam/internal/app/risk"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainrisk "github.com/guliangiam/guliangiam/internal/domain/risk"
	postgresuow "github.com/guliangiam/guliangiam/internal/infra/postgres/uow"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// TestRiskDecisionAppealAuditPostgres is the real-PostgreSQL evidence for
// OPEN-DESIGN-03: a login-risk decision is persisted, reviewed, appealed and the
// resulting audit chain verifies. The whole write path runs inside the real
// unit of work (RLS, records chain, outbox).
func TestRiskDecisionAppealAuditPostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is required for the risk runtime test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	var orgID, userID string
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT u.organization_id::text, u.id::text
		FROM guliangiam.users u JOIN guliangiam.organizations o ON o.id = u.organization_id
		WHERE u.username = 'admin' AND o.slug = 'built-in' LIMIT 1`).Scan(&orgID, &userID))

	uow := postgresuow.New(pool)
	svc := apprisk.NewService(uow)
	systemCtx := tenantctx.System(ctx)
	ip := "203.0.113.10"

	var decisionID, decision string
	require.NoError(t, uow.WithinTransaction(systemCtx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		result, err := svc.EvaluateAndRecordLoginRisk(ctx, tx, domainrisk.LoginRiskCommand{
			OrganizationID: orgID,
			UserID:         &userID,
			LoginChannel:   "password",
			IP:             &ip,
			UserAgent:      "curl/8.4.0",
			Signals:        map[string]any{"new_device": true, "impossible_travel": true},
		})
		if err != nil {
			return err
		}
		decisionID = result.DecisionID
		decision = result.Decision
		return nil
	}))
	require.NotEmpty(t, decisionID)

	current, err := svc.GetDecision(systemCtx, orgID, decisionID)
	require.NoError(t, err)
	require.Equal(t, decision, current.Decision)

	metadata := apprisk.CommandMetadata{
		ActorID: userID, IPAddress: "203.0.113.10", UserAgent: "risk-runtime-test",
		Surface: "test", RequestID: "risk-runtime-1", TraceID: "risk-trace-1",
	}
	reviewRevision := current.Revision
	reviewed, err := svc.ReviewDecision(systemCtx, apprisk.ReviewDecisionCommand{
		OrganizationID: orgID, DecisionID: decisionID, NewDecision: domainrisk.DecisionDeny,
		Reason: "confirmed anomalous login", ExpectedRevision: &reviewRevision, Metadata: metadata,
	})
	require.NoError(t, err)
	require.Equal(t, string(domainrisk.DecisionDeny), reviewed.Decision)

	appealRevision := reviewed.Revision
	submitted, err := svc.SubmitAppeal(systemCtx, apprisk.SubmitAppealCommand{
		OrganizationID: orgID, DecisionID: decisionID,
		Reason: "the anomalous network was a corporate VPN", ExpectedRevision: &appealRevision, Metadata: metadata,
	})
	require.NoError(t, err)
	require.Equal(t, string(domainrisk.AppealPending), string(submitted.Workflow.Appeal.Status))

	resolveRevision := submitted.Revision
	resolved, err := svc.ResolveAppeal(systemCtx, apprisk.ResolveAppealCommand{
		OrganizationID: orgID, DecisionID: decisionID, Resolution: domainrisk.AppealAccepted,
		Reason: "VPN evidence verified", ExpectedRevision: &resolveRevision, Metadata: metadata,
	})
	require.NoError(t, err)
	require.Equal(t, string(apprisk.AppealAccepted), string(resolved.Workflow.Appeal.Status))
	require.Equal(t, string(domainrisk.DecisionDeny), resolved.Decision, "appeal acceptance must not bypass enforcement")

	// The audit chain written with decision + review + appeal must verify.
	integrity, err := svc.VerifyAuditIntegrity(systemCtx, apprisk.AuditIntegrityQuery{OrganizationID: orgID})
	require.NoError(t, err)
	require.NotNil(t, integrity)
	require.True(t, integrity.Valid)
	require.GreaterOrEqual(t, integrity.TotalCount, 3)
}
