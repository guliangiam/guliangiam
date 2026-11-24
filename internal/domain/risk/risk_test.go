package risk

import (
	"encoding/json"
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecisionWorkflow_JSONRoundTripPreservesEngineFactors(t *testing.T) {
	now := time.Date(2026, 10, 26, 8, 0, 0, 0, time.UTC)
	workflow := NewDecisionWorkflow()
	require.NoError(t, workflow.ApplyReview(DecisionAllow, DecisionChallenge, "actor-1", "tighten", now))
	require.NoError(t, workflow.SubmitAppeal(DecisionChallenge, "actor-2", "possible false positive", now.Add(time.Minute)))
	require.NoError(t, workflow.ResolveAppeal(AppealRejected, "actor-3", "signal confirmed", now.Add(2*time.Minute)))
	engine := json.RawMessage(`{"level":"medium","hitSignals":["new_device"]}`)

	encoded, err := EncodeDecisionFactors(engine, workflow)
	require.NoError(t, err)
	decodedEngine, decodedWorkflow, err := DecodeDecisionFactors(encoded)
	require.NoError(t, err)
	assert.JSONEq(t, string(engine), string(decodedEngine))
	assert.Equal(t, int64(3), decodedWorkflow.Revision)
	assert.Equal(t, ReviewCompleted, decodedWorkflow.Review.Status)
	assert.Equal(t, AppealRejected, decodedWorkflow.Appeal.Status)
	assert.Len(t, decodedWorkflow.History, 3)
	assert.Equal(t, "appeal.resolved", decodedWorkflow.History[2].Type)
}

func TestDecisionWorkflow_StateTransitions(t *testing.T) {
	workflow := NewDecisionWorkflow()
	require.NoError(t, workflow.ApplyReview(DecisionChallenge, DecisionDeny, "actor-1", "confirmed", time.Now()))
	err := workflow.ApplyReview(DecisionDeny, DecisionDeny, "actor-1", "again", time.Now())
	assert.Equal(t, apperrors.CodeConflict, apperrors.FromError(err).Code)

	allowWorkflow := NewDecisionWorkflow()
	err = allowWorkflow.SubmitAppeal(DecisionAllow, "actor-1", "not applicable", time.Now())
	assert.Equal(t, apperrors.CodeConflict, apperrors.FromError(err).Code)
}

func TestValidateGovernanceTransition(t *testing.T) {
	tests := []struct {
		name    string
		current GovernanceAction
		next    GovernanceAction
		valid   bool
	}{
		{name: "monitor to step up", current: GovernanceMonitor, next: GovernanceStepUp, valid: true},
		{name: "monitor to revoke", current: GovernanceMonitor, next: GovernanceRevoke, valid: true},
		{name: "step up to monitor", current: GovernanceStepUp, next: GovernanceMonitor, valid: true},
		{name: "step up to revoke", current: GovernanceStepUp, next: GovernanceRevoke, valid: true},
		{name: "monitor self transition", current: GovernanceMonitor, next: GovernanceMonitor},
		{name: "step up self transition", current: GovernanceStepUp, next: GovernanceStepUp},
		{name: "revoke is terminal", current: GovernanceRevoke, next: GovernanceMonitor},
		{name: "revoke cannot return to step up", current: GovernanceRevoke, next: GovernanceStepUp},
		{name: "invalid next action", current: GovernanceMonitor, next: GovernanceAction("allow")},
		{name: "unknown current action", current: GovernanceAction("unknown"), next: GovernanceMonitor},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateGovernanceTransition(test.current, test.next)
			if test.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestDecodeDecisionFactors_LegacyValuesRemainUnchanged(t *testing.T) {
	for _, raw := range []string{`[]`, `[{"type":"new_device"}]`, `{"level":"high"}`} {
		engine, workflow, err := DecodeDecisionFactors(json.RawMessage(raw))
		require.NoError(t, err)
		assert.JSONEq(t, raw, string(engine))
		assert.Zero(t, workflow.Revision)
		assert.Equal(t, ReviewPending, workflow.Review.Status)
		assert.Equal(t, AppealNone, workflow.Appeal.Status)
	}
}

func TestDecisionWorkflow_AppealIsSingleShotAndResolutionIsTerminal(t *testing.T) {
	workflow := NewDecisionWorkflow()
	now := time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)
	assert.Equal(t, apperrors.CodeConflict, apperrors.FromError(workflow.ResolveAppeal(AppealAccepted, "actor-2", "not pending", now)).Code)
	require.NoError(t, workflow.SubmitAppeal(DecisionDeny, "actor-1", "false positive", now))
	assert.Equal(t, apperrors.CodeConflict, apperrors.FromError(workflow.SubmitAppeal(DecisionDeny, "actor-1", "duplicate", now)).Code)

	require.NoError(t, workflow.ResolveAppeal(AppealAccepted, "actor-2", "verified", now.Add(time.Minute)))
	assert.Equal(t, AppealAccepted, workflow.Appeal.Status)
	assert.Equal(t, int64(2), workflow.Revision)
	err := workflow.ResolveAppeal(AppealRejected, "actor-2", "reverse terminal state", now.Add(2*time.Minute))
	assert.Equal(t, apperrors.CodeConflict, apperrors.FromError(err).Code)
	assert.Equal(t, AppealAccepted, workflow.Appeal.Status)
	assert.Equal(t, int64(2), workflow.Revision)
}

func TestDecodeDecisionFactors_RejectsInconsistentWorkflowRevision(t *testing.T) {
	raw := json.RawMessage(`[{"_guliangiam":"guliangiam.risk_decision.workflow.v1","engineFactors":[],"workflow":{"revision":2,"review":{"status":"pending"},"appeal":{"status":"none"},"history":[]}}]`)
	_, _, err := DecodeDecisionFactors(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "revision")
}

// The predicates and the closed sets must agree value for value, otherwise the
// 422 `allowed` list a caller sees could name a value the predicate rejects
// (手册 4.14.3).
func TestClosedSetsMatchPredicates(t *testing.T) {
	for _, value := range DecisionActions.Values() {
		assert.True(t, IsDecisionActionValid(DecisionAction(value)), value)
	}
	assert.False(t, IsDecisionActionValid(DecisionAction("review")))
	assert.False(t, IsDecisionActionValid(DecisionAction("step_up")))
	assert.False(t, IsDecisionActionValid(DecisionAction("")))

	for _, value := range GovernanceActions.Values() {
		assert.True(t, IsGovernanceActionValid(GovernanceAction(value)), value)
	}
	assert.False(t, IsGovernanceActionValid(GovernanceAction("lockout")))
	assert.False(t, IsGovernanceActionValid(GovernanceAction("force_logout")))
}

func TestClosedSetsRejectUnknownValuesWith422(t *testing.T) {
	wantAppError := func(t *testing.T, err error, field, set string) {
		t.Helper()
		appErr, ok := err.(*apperrors.Error)
		require.True(t, ok)
		assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
		assert.Equal(t, 422, appErr.HTTPStatus())
		assert.Equal(t, field, appErr.Meta["field"])
		assert.Equal(t, set, appErr.Meta["set"])
		assert.NotEmpty(t, appErr.Meta["allowed"])
	}

	workflow := NewDecisionWorkflow()
	wantAppError(t, workflow.ApplyReview(DecisionAllow, DecisionAction("review"), "actor-1", "x", time.Now()),
		"decision", "riskDecisionAction")

	appeal := NewDecisionWorkflow()
	wantAppError(t, appeal.ResolveAppeal(AppealStatus("pending"), "actor-1", "x", time.Now()),
		"resolution", "riskAppealResolution")

	wantAppError(t, ValidateGovernanceTransition(GovernanceMonitor, GovernanceAction("allow")),
		"action", "riskGovernanceAction")
}

func TestClosedSetsTreatBlankAsMissing(t *testing.T) {
	err := DecisionActions.ValidateRequired("decision", "")
	require.Error(t, err)
	appErr, ok := err.(*apperrors.Error)
	require.True(t, ok)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
	assert.Equal(t, 400, appErr.HTTPStatus())
}
