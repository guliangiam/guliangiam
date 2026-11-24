package risk

import (
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/risk"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var fixedNow = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return db, mock
}

func decisionColumns() []string {
	return []string{"id", "organization_id", "user_id", "score", "decision", "factors", "created_at"}
}

func governanceColumns() []string {
	return []string{"id", "organization_id", "session_id", "risk_score", "action", "reason", "created_at", "revision"}
}

func TestRepository_ListSignals(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) " + queryListSignalsBase)).
		WithArgs("org-001").WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))
	mock.ExpectQuery(regexp.QuoteMeta(queryListSignalsSelect+queryListSignalsBase+" ORDER BY created_at DESC, id DESC "+queryListSignalsOrderPage)).
		WithArgs("org-001", 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "organization_id", "user_id", "signal_type", "signal_value", "ip", "created_at"}).
			AddRow("sig-1", "org-001", "user-001", "new_device", []byte(`{}`), "1.2.3.4", fixedNow))

	items, total, err := NewRepository(db).ListSignals(context.Background(), domain.ListSignalsQuery{OrganizationID: "org-001", Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_ListSignals_WithFilters(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	userID, signalType := "user-001", "new_device"
	conditions := " AND user_id = ? AND signal_type = ?"
	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) "+queryListSignalsBase+conditions)).
		WithArgs("org-001", userID, signalType).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))
	mock.ExpectQuery(regexp.QuoteMeta(queryListSignalsSelect+queryListSignalsBase+conditions+" ORDER BY created_at DESC, id DESC "+queryListSignalsOrderPage)).
		WithArgs("org-001", userID, signalType, 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "organization_id", "user_id", "signal_type", "signal_value", "ip", "created_at"}))

	items, total, err := NewRepository(db).ListSignals(context.Background(), domain.ListSignalsQuery{
		OrganizationID: "org-001", UserID: &userID, SignalType: &signalType, Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(1), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_ListDecisions(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) " + queryListDecisionsBase)).
		WithArgs("org-001").WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))
	mock.ExpectQuery(regexp.QuoteMeta(queryListDecisionsSelect+queryListDecisionsBase+" ORDER BY created_at DESC, id DESC "+queryListSignalsOrderPage)).
		WithArgs("org-001", 20, 0).
		WillReturnRows(sqlmock.NewRows(decisionColumns()).AddRow("dec-1", "org-001", "user-001", 80, "challenge", []byte(`[]`), fixedNow))

	items, _, err := NewRepository(db).ListDecisions(context.Background(), domain.ListDecisionsQuery{OrganizationID: "org-001", Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, domain.DecisionChallenge, items[0].Decision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetDecision(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(queryGetDecision)).WithArgs("dec-1", "org-001").
		WillReturnRows(sqlmock.NewRows(decisionColumns()).AddRow("dec-1", "org-001", "user-001", 50, "allow", []byte(`[]`), fixedNow))

	decision, err := NewRepository(db).GetDecision(context.Background(), "dec-1", "org-001")
	require.NoError(t, err)
	assert.Equal(t, domain.DecisionAllow, decision.Decision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetDecisionForUpdate(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(queryGetDecisionForUpdate)).WithArgs("dec-1", "org-001").
		WillReturnRows(sqlmock.NewRows(decisionColumns()).AddRow("dec-1", "org-001", "user-001", 50, "challenge", []byte(`[]`), fixedNow))

	decision, err := NewRepository(db).GetDecisionForUpdate(context.Background(), "dec-1", "org-001")
	require.NoError(t, err)
	assert.Equal(t, domain.DecisionChallenge, decision.Decision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetDecision_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(queryGetDecision)).WithArgs("missing", "org-001").WillReturnError(sql.ErrNoRows)
	_, err := NewRepository(db).GetDecision(context.Background(), "missing", "org-001")
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

func TestRepository_UpdateDecision(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	oldFactors := json.RawMessage(`[{"type":"new_device"}]`)
	newFactors := json.RawMessage(`[{"_guliangiam":"workflow"}]`)
	mock.ExpectExec(regexp.QuoteMeta(queryUpdateDecision)).
		WithArgs("deny", newFactors, "dec-1", "org-001", "challenge", oldFactors).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := NewRepository(db).UpdateDecision(context.Background(), domain.UpdateDecisionCommand{
		OrganizationID: "org-001", DecisionID: "dec-1", ExpectedDecision: domain.DecisionChallenge,
		ExpectedFactors: oldFactors, NewDecision: domain.DecisionDeny, NewFactors: newFactors,
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_UpdateDecision_StaleRevision(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	mock.ExpectExec(regexp.QuoteMeta(queryUpdateDecision)).
		WithArgs("deny", []byte(`[]`), "dec-1", "org-001", "challenge", []byte(`[]`)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := NewRepository(db).UpdateDecision(context.Background(), domain.UpdateDecisionCommand{
		OrganizationID: "org-001", DecisionID: "dec-1", ExpectedDecision: domain.DecisionChallenge,
		ExpectedFactors: []byte(`[]`), NewDecision: domain.DecisionDeny, NewFactors: []byte(`[]`),
	})
	assert.Equal(t, apperrors.CodeConflict, apperrors.FromError(err).Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_ListGovernance(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) " + queryListGovernanceBase)).
		WithArgs("org-001").WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))
	mock.ExpectQuery(regexp.QuoteMeta(queryListGovernanceSelect+queryListGovernanceBase+" ORDER BY g.created_at DESC, g.id DESC "+queryListSignalsOrderPage)).
		WithArgs("org-001", 20, 0).
		WillReturnRows(sqlmock.NewRows(governanceColumns()).AddRow("gov-1", "org-001", "sess-001", 90, "step_up", "high risk", fixedNow, 3))

	items, _, err := NewRepository(db).ListGovernance(context.Background(), domain.ListGovernanceQuery{OrganizationID: "org-001", Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, domain.GovernanceStepUp, items[0].Action)
	assert.Equal(t, int64(3), items[0].Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetGovernanceForTransition(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	rows := func() *sqlmock.Rows {
		return sqlmock.NewRows(governanceColumns()).AddRow("gov-1", "org-001", "sess-001", 90, "step_up", "high risk", fixedNow, 2)
	}
	mock.ExpectQuery(regexp.QuoteMeta(queryLockGovernance)).WithArgs("gov-1", "org-001").WillReturnRows(rows())
	mock.ExpectQuery(regexp.QuoteMeta(queryGetLatestGovernance)).WithArgs("org-001", "sess-001").WillReturnRows(rows())

	current, err := NewRepository(db).GetGovernanceForTransition(context.Background(), "gov-1", "org-001")
	require.NoError(t, err)
	assert.Equal(t, "gov-1", current.ID)
	assert.Equal(t, int64(2), current.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetGovernanceForTransition_RejectsHistoricalRow(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(queryLockGovernance)).WithArgs("gov-old", "org-001").
		WillReturnRows(sqlmock.NewRows(governanceColumns()).AddRow("gov-old", "org-001", "sess-001", 90, "monitor", "old", fixedNow, 1))
	mock.ExpectQuery(regexp.QuoteMeta(queryGetLatestGovernance)).WithArgs("org-001", "sess-001").
		WillReturnRows(sqlmock.NewRows(governanceColumns()).AddRow("gov-new", "org-001", "sess-001", 90, "step_up", "new", fixedNow, 2))

	_, err := NewRepository(db).GetGovernanceForTransition(context.Background(), "gov-old", "org-001")
	assert.Equal(t, apperrors.CodeConflict, apperrors.FromError(err).Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_AppendGovernance(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	sessionID, reason := "sess-001", "confirmed risk"
	mock.ExpectExec(regexp.QuoteMeta(queryAppendGovernance)).
		WithArgs("gov-2", "org-001", sessionID, 90, "revoke", reason, fixedNow).
		WillReturnResult(sqlmock.NewResult(0, 1))
	err := NewRepository(db).AppendGovernance(context.Background(), domain.SessionGovernance{
		ID: "gov-2", OrganizationID: "org-001", SessionID: &sessionID, RiskScore: 90,
		Action: domain.GovernanceRevoke, Reason: &reason, CreatedAt: fixedNow, Revision: 2,
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_ListAuditChain(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	columns := []string{
		"organization_id", "actor_id", "action", "resource_type", "resource_id", "ip", "user_agent",
		"surface", "request_id", "trace_id", "metadata", "created_at", "chain_seq", "prev_hash", "entry_hash", "chain_algo",
	}
	mock.ExpectQuery(regexp.QuoteMeta(queryListAuditChain)).WithArgs("org-001", "org-001").
		WillReturnRows(sqlmock.NewRows(columns).AddRow(
			"org-001", "actor-001", "risk.decision.reviewed", "decision", "dec-1", "127.0.0.1", "agent",
			"management", "req-1", "trace-1", []byte(`{"revision":1}`), fixedNow, 1, nil, []byte{1, 2, 3}, "sha256",
		))

	chain, err := NewRepository(db).ListAuditChain(context.Background(), domain.AuditIntegrityQuery{OrganizationID: "org-001", ChainID: "org-001"})
	require.NoError(t, err)
	require.Len(t, chain.Entries, 1)
	assert.Equal(t, float64(1), chain.Entries[0].Metadata["revision"])
	assert.Equal(t, int64(1), chain.Entries[0].ChainSeq)
	assert.NoError(t, mock.ExpectationsWereMet())
}

var _ domain.Repository = Repository{}
