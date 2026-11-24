package risk

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/risk"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var postgresTestNow = time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)

type postgresRow struct {
	values []any
	err    error
}

func (r postgresRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return fmt.Errorf("scan destination count %d does not match value count %d", len(dest), len(r.values))
	}
	for i, value := range r.values {
		if value == nil {
			continue
		}
		if err := assignPostgresValue(dest[i], value); err != nil {
			return fmt.Errorf("scan column %d: %w", i, err)
		}
	}
	return nil
}

func assignPostgresValue(destination, source any) error {
	target := reflect.ValueOf(destination)
	if target.Kind() != reflect.Pointer || target.IsNil() {
		return fmt.Errorf("destination must be a non-nil pointer")
	}
	return setPostgresValue(target.Elem(), reflect.ValueOf(source))
}

func setPostgresValue(target, source reflect.Value) error {
	if target.Kind() == reflect.Pointer {
		value := reflect.New(target.Type().Elem())
		if err := setPostgresValue(value.Elem(), source); err != nil {
			return err
		}
		target.Set(value)
		return nil
	}
	if source.Type().AssignableTo(target.Type()) {
		target.Set(source)
		return nil
	}
	if source.Type().ConvertibleTo(target.Type()) {
		target.Set(source.Convert(target.Type()))
		return nil
	}
	return fmt.Errorf("cannot assign %s to %s", source.Type(), target.Type())
}

func decisionRowValues() []any {
	return []any{"dec-1", "org-1", "user-1", 80, "challenge", []byte(`[{"type":"new_device"}]`), postgresTestNow}
}

func governanceRowValues(id string, sessionID any, action string, revision int64) []any {
	return []any{id, "org-1", sessionID, 90, action, "high risk", postgresTestNow, revision}
}

func TestRepository_GetDecisionQueries(t *testing.T) {
	for _, test := range []struct {
		name      string
		statement string
		forUpdate bool
	}{
		{name: "detail", statement: queryGetDecision},
		{name: "for update", statement: queryGetDecisionForUpdate, forUpdate: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := &shared.MockDBTX{QueryRowFn: func(_ context.Context, statement string, args ...any) pgx.Row {
				assert.Equal(t, test.statement, statement)
				assert.Equal(t, []any{"dec-1", "org-1"}, args)
				return postgresRow{values: decisionRowValues()}
			}}
			repository := NewRepository(db)
			var decision *domain.Decision
			var err error
			if test.forUpdate {
				decision, err = repository.GetDecisionForUpdate(context.Background(), "dec-1", "org-1")
			} else {
				decision, err = repository.GetDecision(context.Background(), "dec-1", "org-1")
			}
			require.NoError(t, err)
			assert.Equal(t, "org-1", decision.OrganizationID)
			require.NotNil(t, decision.UserID)
			assert.Equal(t, "user-1", *decision.UserID)
			assert.Equal(t, domain.DecisionChallenge, decision.Decision)
			assert.JSONEq(t, `[{"type":"new_device"}]`, string(decision.Factors))
			assert.Equal(t, postgresTestNow, decision.CreatedAt)
		})
	}
}

func TestRepository_GetDecisionNotFound(t *testing.T) {
	db := &shared.MockDBTX{QueryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
		return postgresRow{err: pgx.ErrNoRows}
	}}
	_, err := NewRepository(db).GetDecision(context.Background(), "missing", "org-1")
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

func TestRepository_UpdateDecisionCAS(t *testing.T) {
	for _, test := range []struct {
		name     string
		affected int64
		wantCode apperrors.Code
	}{
		{name: "updated", affected: 1},
		{name: "stale revision", affected: 0, wantCode: apperrors.CodeConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			oldFactors := json.RawMessage(`[{"type":"new_device"}]`)
			newFactors := json.RawMessage(`[{"_guliangiam":"workflow"}]`)
			db := &shared.MockDBTX{ExecFn: func(_ context.Context, statement string, args ...any) (pgconn.CommandTag, error) {
				assert.Equal(t, queryUpdateDecision, statement)
				assert.Equal(t, []any{"deny", newFactors, "dec-1", "org-1", "challenge", oldFactors}, args)
				return pgconn.NewCommandTag(fmt.Sprintf("UPDATE %d", test.affected)), nil
			}}
			err := NewRepository(db).UpdateDecision(context.Background(), domain.UpdateDecisionCommand{
				OrganizationID: "org-1", DecisionID: "dec-1", ExpectedDecision: domain.DecisionChallenge,
				ExpectedFactors: oldFactors, NewDecision: domain.DecisionDeny, NewFactors: newFactors,
			})
			if test.wantCode == "" {
				require.NoError(t, err)
			} else {
				assert.Equal(t, test.wantCode, apperrors.FromError(err).Code)
			}
		})
	}
}

func TestRepository_GetGovernanceForTransition(t *testing.T) {
	db := &shared.MockDBTX{QueryRowFn: func(_ context.Context, statement string, args ...any) pgx.Row {
		switch statement {
		case queryLockGovernance:
			assert.Equal(t, []any{"gov-1", "org-1"}, args)
		case queryGetLatestGovernance:
			assert.Equal(t, []any{"org-1", "session-1"}, args)
		default:
			t.Fatalf("unexpected statement: %s", statement)
		}
		return postgresRow{values: governanceRowValues("gov-1", "session-1", "step_up", 2)}
	}}
	current, err := NewRepository(db).GetGovernanceForTransition(context.Background(), "gov-1", "org-1")
	require.NoError(t, err)
	assert.Equal(t, "gov-1", current.ID)
	assert.Equal(t, domain.GovernanceStepUp, current.Action)
	assert.Equal(t, int64(2), current.Revision)
	require.NotNil(t, current.SessionID)
	assert.Equal(t, "session-1", *current.SessionID)
}

func TestRepository_GetGovernanceForTransitionRejectsHistoricalAndDetachedRows(t *testing.T) {
	t.Run("historical", func(t *testing.T) {
		db := &shared.MockDBTX{QueryRowFn: func(_ context.Context, statement string, _ ...any) pgx.Row {
			if statement == queryLockGovernance {
				return postgresRow{values: governanceRowValues("gov-old", "session-1", "monitor", 1)}
			}
			assert.Equal(t, queryGetLatestGovernance, statement)
			return postgresRow{values: governanceRowValues("gov-new", "session-1", "step_up", 2)}
		}}
		_, err := NewRepository(db).GetGovernanceForTransition(context.Background(), "gov-old", "org-1")
		assert.Equal(t, apperrors.CodeConflict, apperrors.FromError(err).Code)
	})

	t.Run("detached", func(t *testing.T) {
		db := &shared.MockDBTX{QueryRowFn: func(_ context.Context, statement string, _ ...any) pgx.Row {
			assert.Equal(t, queryLockGovernance, statement)
			return postgresRow{values: governanceRowValues("gov-1", nil, "monitor", 1)}
		}}
		_, err := NewRepository(db).GetGovernanceForTransition(context.Background(), "gov-1", "org-1")
		assert.Equal(t, apperrors.CodeConflict, apperrors.FromError(err).Code)
	})
}

func TestRepository_AppendGovernance(t *testing.T) {
	sessionID, reason := "session-1", "step-up failed"
	db := &shared.MockDBTX{ExecFn: func(_ context.Context, statement string, args ...any) (pgconn.CommandTag, error) {
		assert.Equal(t, queryAppendGovernance, statement)
		assert.Equal(t, []any{"gov-2", "org-1", &sessionID, 90, "revoke", &reason, postgresTestNow}, args)
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	}}
	err := NewRepository(db).AppendGovernance(context.Background(), domain.SessionGovernance{
		ID: "gov-2", OrganizationID: "org-1", SessionID: &sessionID, RiskScore: 90,
		Action: domain.GovernanceRevoke, Reason: &reason, CreatedAt: postgresTestNow, Revision: 2,
	})
	require.NoError(t, err)
}

func TestRepository_ListAuditChain(t *testing.T) {
	db := &shared.MockDBTX{QueryFn: func(_ context.Context, statement string, args ...any) (pgx.Rows, error) {
		assert.Equal(t, []any{"org-1", "org-1"}, args)
		switch statement {
		case queryListAuditChain:
			return shared.NewMockRows([][]any{{
				"org-1", "actor-1", "risk.decision.reviewed", "decision", "dec-1", "127.0.0.1", "agent",
				"management", "request-1", "trace-1", []byte(`{"revision":1}`), postgresTestNow,
				int64(1), nil, []byte{1, 2, 3}, "sha256",
			}}), nil
		case queryListAuditChainAnchors:
			return shared.NewMockRows([][]any{{int64(1), []byte{1, 2, 3}}}), nil
		default:
			t.Fatalf("unexpected query %q", statement)
			return nil, nil
		}
	}}
	chain, err := NewRepository(db).ListAuditChain(context.Background(), domain.AuditIntegrityQuery{
		OrganizationID: "org-1", ChainID: "org-1",
	})
	require.NoError(t, err)
	require.Len(t, chain.Entries, 1)
	entry := chain.Entries[0]
	assert.Equal(t, "org-1", entry.OrganizationID)
	assert.Equal(t, int64(1), entry.ChainSeq)
	assert.Equal(t, float64(1), entry.Metadata["revision"])
	assert.Equal(t, []byte{1, 2, 3}, entry.EntryHash)
	assert.Equal(t, "sha256", entry.ChainAlgo)
	require.Len(t, chain.Anchors, 1)
	assert.Equal(t, int64(1), chain.Anchors[0].AnchorSeq)
	assert.Equal(t, []byte{1, 2, 3}, chain.Anchors[0].EntryHash)
}

// auditChainRowValues mirrors the 16-column audit chain select list.
func auditChainRowValues(seq int64) []any {
	return []any{
		"org-1", "actor-1", "risk.decision.reviewed", "decision", "dec-1", "127.0.0.1", "agent",
		"management", "request-1", "trace-1", []byte(`{"revision":1}`), postgresTestNow,
		seq, []byte{0}, []byte{1, 2, 3}, "sha256",
	}
}

func TestRepository_ListAuditChainSegment(t *testing.T) {
	db := &shared.MockDBTX{QueryFn: func(_ context.Context, statement string, args ...any) (pgx.Rows, error) {
		assert.Equal(t, queryListAuditChainSegment, statement)
		assert.Equal(t, []any{"org-1", "org-1", int64(500), 500}, args)
		assert.Contains(t, statement, "AND chain_seq > $3")
		assert.Contains(t, statement, "ORDER BY chain_seq ASC")
		assert.Contains(t, statement, "LIMIT $4")
		return shared.NewMockRows([][]any{auditChainRowValues(501), auditChainRowValues(502)}), nil
	}}
	entries, err := NewRepository(db).ListAuditChainSegment(context.Background(), "org-1", "org-1", 500, 500)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, int64(501), entries[0].ChainSeq)
	assert.Equal(t, int64(502), entries[1].ChainSeq)
	assert.Equal(t, float64(1), entries[0].Metadata["revision"])
	assert.Equal(t, []byte{1, 2, 3}, entries[0].EntryHash)
	assert.Equal(t, "sha256", entries[0].ChainAlgo)
}

func TestRepository_ListAuditChainSegmentRejectsNonPositiveLimit(t *testing.T) {
	db := &shared.MockDBTX{QueryFn: func(_ context.Context, statement string, _ ...any) (pgx.Rows, error) {
		t.Fatalf("must not query with a non-positive limit: %s", statement)
		return nil, nil
	}}
	_, err := NewRepository(db).ListAuditChainSegment(context.Background(), "org-1", "org-1", 0, 0)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
}

func TestRepository_ListAuditChainAnchors(t *testing.T) {
	db := &shared.MockDBTX{QueryFn: func(_ context.Context, statement string, args ...any) (pgx.Rows, error) {
		assert.Equal(t, queryListAuditChainAnchors, statement)
		assert.Equal(t, []any{"org-1", "org-1"}, args)
		return shared.NewMockRows([][]any{{int64(1), []byte{1}}, {int64(500), []byte{2}}}), nil
	}}
	anchors, err := NewRepository(db).ListAuditChainAnchors(context.Background(), "org-1", "org-1")
	require.NoError(t, err)
	require.Len(t, anchors, 2)
	assert.Equal(t, int64(1), anchors[0].AnchorSeq)
	assert.Equal(t, int64(500), anchors[1].AnchorSeq)
	assert.Equal(t, []byte{2}, anchors[1].EntryHash)
}

// TestRepository_ListGovernanceUsesWindowRevision pins the BE-D-15 fix: the
// list page computes revisions with one window pass instead of a correlated
// COUNT per row, while the count query stays unchanged. The optional action
// filter must apply OUTSIDE the derived table so filtered-out sibling rows
// still count toward the revision.
func TestRepository_ListGovernanceUsesWindowRevision(t *testing.T) {
	action := domain.GovernanceRevoke
	for _, test := range []struct {
		name   string
		action *domain.GovernanceAction
	}{
		{name: "unfiltered"},
		{name: "action filtered", action: &action},
	} {
		t.Run(test.name, func(t *testing.T) {
			var countSQL string
			var countArgs, listArgs []any
			var listSQL string
			db := &shared.MockDBTX{
				QueryRowFn: func(_ context.Context, statement string, args ...any) pgx.Row {
					countSQL = statement
					countArgs = args
					return postgresRow{values: []any{int64(1)}}
				},
				QueryFn: func(_ context.Context, statement string, args ...any) (pgx.Rows, error) {
					listSQL = statement
					listArgs = args
					return shared.NewMockRows([][]any{governanceRowValues("gov-1", "session-1", "revoke", 2)}), nil
				},
			}
			items, total, err := NewRepository(db).ListGovernance(context.Background(), domain.ListGovernanceQuery{
				OrganizationID: "org-1", Action: test.action, Page: 1, PageSize: 20,
			})
			require.NoError(t, err)
			assert.Equal(t, int64(1), total)
			require.Len(t, items, 1)
			assert.Equal(t, int64(2), items[0].Revision)

			assert.Contains(t, listSQL, "COUNT(*) OVER (")
			assert.Contains(t, listSQL, "PARTITION BY g.session_id")
			assert.Contains(t, listSQL, "ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW")
			assert.NotContains(t, listSQL, "SELECT COUNT(*) FROM guliangiam.session_governance h")

			if test.action == nil {
				assert.Equal(t, "SELECT count(*) "+queryListGovernanceBase, countSQL)
				assert.Equal(t, []any{"org-1"}, countArgs)
				assert.Equal(t,
					queryListGovernanceWindowPage+" ORDER BY g.created_at DESC, g.id DESC"+fmt.Sprintf(queryListSignalsOrderPage, 2, 3),
					listSQL)
				assert.Equal(t, []any{"org-1", 20, 0}, listArgs)
			} else {
				assert.Equal(t, "SELECT count(*) "+queryListGovernanceBase+" AND g.action = $2", countSQL)
				assert.Equal(t, []any{"org-1", "revoke"}, countArgs)
				assert.Equal(t,
					queryListGovernanceWindowPage+" WHERE g.action = $2 ORDER BY g.created_at DESC, g.id DESC"+fmt.Sprintf(queryListSignalsOrderPage, 3, 4),
					listSQL)
				assert.Equal(t, []any{"org-1", "revoke", 20, 0}, listArgs)
				// The filter sits outside the window's derived table.
				assert.Contains(t, listSQL, ") g WHERE g.action = $2")
			}
		})
	}
}

// TestRepository_GovernanceSingleRowQueriesKeepCorrelatedRevision guards the
// lock paths: FOR UPDATE + LIMIT must keep the correlated-subquery revision,
// never the window variant (which would change lock semantics).
func TestRepository_GovernanceSingleRowQueriesKeepCorrelatedRevision(t *testing.T) {
	for _, statement := range []string{queryGetGovernance, queryLockGovernance, queryGetLatestGovernance} {
		assert.Contains(t, statement, "SELECT COUNT(*) FROM guliangiam.session_governance h")
		assert.NotContains(t, statement, "OVER (")
	}
	db := &shared.MockDBTX{QueryRowFn: func(_ context.Context, statement string, args ...any) pgx.Row {
		assert.Equal(t, queryGetGovernance, statement)
		assert.Equal(t, []any{"gov-1", "org-1"}, args)
		return postgresRow{values: governanceRowValues("gov-1", "session-1", "revoke", 2)}
	}}
	governance, err := NewRepository(db).GetGovernance(context.Background(), "gov-1", "org-1")
	require.NoError(t, err)
	assert.Equal(t, int64(2), governance.Revision)
}
