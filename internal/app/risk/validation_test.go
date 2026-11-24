package risk

import (
	"strings"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/risk"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wantAppError asserts the 手册 4.14.1 contract: a missing field is 400, a
// value that parsed but violated a rule is 422 with field metadata.
func wantAppError(t *testing.T, err error, code apperrors.Code, status int) *apperrors.Error {
	t.Helper()
	require.Error(t, err)
	appErr, ok := err.(*apperrors.Error)
	require.True(t, ok, "validation must return *apperrors.Error, got %T", err)
	assert.Equal(t, code, appErr.Code)
	assert.Equal(t, status, appErr.HTTPStatus())
	return appErr
}

func TestValidateUUID(t *testing.T) {
	assert.NoError(t, validateUUID("organizationId", testOrgID))

	appErr := wantAppError(t, validateUUID("organizationId", ""), apperrors.CodeInvalidArgument, 400)
	assert.Equal(t, "organizationId is required", appErr.Message)

	appErr = wantAppError(t, validateUUID("organizationId", "not-a-uuid"), apperrors.CodeUnprocessableEntity, 422)
	assert.Equal(t, map[string]any{
		"field":  "organizationId",
		"value":  "not-a-uuid",
		"format": "uuid",
	}, appErr.Meta)
}

func TestValidateListQuery(t *testing.T) {
	assert.NoError(t, validateListQuery(testOrgID, 1, platformpagination.DefaultPageSize))

	appErr := wantAppError(t, validateListQuery("", 1, 20), apperrors.CodeInvalidArgument, 400)
	assert.Equal(t, "organizationId is required", appErr.Message)

	appErr = wantAppError(t, validateListQuery(testOrgID, 0, 20), apperrors.CodeUnprocessableEntity, 422)
	assert.Equal(t, "page", appErr.Meta["field"])
	assert.Equal(t, 0, appErr.Meta["value"])
	assert.Equal(t, 1, appErr.Meta["min"])

	appErr = wantAppError(t, validateListQuery(testOrgID, 1, 0), apperrors.CodeUnprocessableEntity, 422)
	assert.Equal(t, "pageSize", appErr.Meta["field"])
	assert.Equal(t, platformpagination.MaxPageSize, appErr.Meta["max"])

	appErr = wantAppError(t, validateListQuery(testOrgID, 1, platformpagination.MaxPageSize+1), apperrors.CodeUnprocessableEntity, 422)
	assert.Equal(t, platformpagination.MaxPageSize, appErr.Meta["max"])
}

func TestValidateReason(t *testing.T) {
	assert.NoError(t, validateReason("suspicious login"))

	appErr := wantAppError(t, validateReason(""), apperrors.CodeInvalidArgument, 400)
	assert.Equal(t, "reason is required", appErr.Message)

	appErr = wantAppError(t, validateReason(strings.Repeat("x", maxReasonLength+1)), apperrors.CodeUnprocessableEntity, 422)
	assert.Equal(t, "reason", appErr.Meta["field"])
	assert.Equal(t, maxReasonLength, appErr.Meta["max"])
}

func TestValidateExpectedRevision(t *testing.T) {
	zero := int64(0)
	assert.NoError(t, validateExpectedRevision(&zero))

	appErr := wantAppError(t, validateExpectedRevision(nil), apperrors.CodeInvalidArgument, 400)
	assert.Equal(t, "revision is required", appErr.Message)

	negative := int64(-1)
	appErr = wantAppError(t, validateExpectedRevision(&negative), apperrors.CodeUnprocessableEntity, 422)
	assert.Equal(t, "revision", appErr.Meta["field"])
	assert.Equal(t, int64(-1), appErr.Meta["value"])
	assert.Equal(t, 0, appErr.Meta["min"])
}

func TestValidateSignalType(t *testing.T) {
	assert.NoError(t, validateSignalType(strings.Repeat("a", maxSignalTypeLen)))

	appErr := wantAppError(t, validateSignalType(strings.Repeat("a", maxSignalTypeLen+1)), apperrors.CodeUnprocessableEntity, 422)
	assert.Equal(t, "signalType", appErr.Meta["field"])
	assert.Equal(t, maxSignalTypeLen, appErr.Meta["max"])
}

func TestValidateDecisionFilter(t *testing.T) {
	allow := domain.DecisionAllow
	assert.NoError(t, validateDecisionFilter(nil))
	assert.NoError(t, validateDecisionFilter(&allow))

	blank := domain.DecisionAction("")
	appErr := wantAppError(t, validateDecisionFilter(&blank), apperrors.CodeInvalidArgument, 400)
	assert.Equal(t, "decision must not be blank", appErr.Message)

	unknown := domain.DecisionAction("step_up")
	appErr = wantAppError(t, validateDecisionFilter(&unknown), apperrors.CodeUnprocessableEntity, 422)
	assert.Equal(t, map[string]any{
		"field":   "decision",
		"value":   "step_up",
		"allowed": []string{"allow", "challenge", "deny"},
		"set":     "riskDecisionAction",
	}, appErr.Meta)
}

func TestValidateNewDecision(t *testing.T) {
	assert.NoError(t, validateNewDecision(domain.DecisionChallenge))

	appErr := wantAppError(t, validateNewDecision(""), apperrors.CodeInvalidArgument, 400)
	assert.Equal(t, "decision is required", appErr.Message)

	appErr = wantAppError(t, validateNewDecision("review"), apperrors.CodeUnprocessableEntity, 422)
	assert.Equal(t, "review", appErr.Meta["value"])
}

func TestValidateGovernanceFilter(t *testing.T) {
	monitor := domain.GovernanceMonitor
	assert.NoError(t, validateGovernanceFilter(nil))
	assert.NoError(t, validateGovernanceFilter(&monitor))

	blank := domain.GovernanceAction("")
	appErr := wantAppError(t, validateGovernanceFilter(&blank), apperrors.CodeInvalidArgument, 400)
	assert.Equal(t, "action must not be blank", appErr.Message)

	unknown := domain.GovernanceAction("lockout")
	appErr = wantAppError(t, validateGovernanceFilter(&unknown), apperrors.CodeUnprocessableEntity, 422)
	assert.Equal(t, []string{"monitor", "revoke", "step_up"}, appErr.Meta["allowed"])
	assert.Equal(t, "riskGovernanceAction", appErr.Meta["set"])
}

func TestValidateNewAction(t *testing.T) {
	assert.NoError(t, validateNewAction(domain.GovernanceRevoke))

	appErr := wantAppError(t, validateNewAction(domain.GovernanceAction("force_logout")), apperrors.CodeUnprocessableEntity, 422)
	assert.Equal(t, "force_logout", appErr.Meta["value"])
}

func TestValidateAppealResolution(t *testing.T) {
	assert.NoError(t, validateAppealResolution(domain.AppealAccepted))

	appErr := wantAppError(t, validateAppealResolution(domain.AppealPending), apperrors.CodeUnprocessableEntity, 422)
	assert.Equal(t, map[string]any{
		"field":   "resolution",
		"value":   "pending",
		"allowed": []string{"accepted", "rejected"},
		"set":     "riskAppealResolution",
	}, appErr.Meta)
}

func TestValidateChainMatch(t *testing.T) {
	assert.NoError(t, validateChainMatch(testOrgID, testOrgID))

	other := "77777777-7777-4777-8777-777777777777"
	appErr := wantAppError(t, validateChainMatch(other, testOrgID), apperrors.CodeUnprocessableEntity, 422)
	assert.Equal(t, map[string]any{
		"field":    "chainId",
		"value":    other,
		"expected": testOrgID,
	}, appErr.Meta)
}

func TestValidateEvaluateRiskCommand(t *testing.T) {
	cmd := domain.EvaluateRiskCommand{
		OrganizationID: testOrgID,
		LoginChannel:   "password",
	}
	assert.NoError(t, validateEvaluateRiskCommand(cmd))

	cmd.OrganizationID = ""
	appErr := wantAppError(t, validateEvaluateRiskCommand(cmd), apperrors.CodeInvalidArgument, 400)
	assert.Equal(t, "organizationId is required", appErr.Message)

	cmd.OrganizationID = "not-a-uuid"
	appErr = wantAppError(t, validateEvaluateRiskCommand(cmd), apperrors.CodeUnprocessableEntity, 422)
	assert.Equal(t, "organizationId", appErr.Meta["field"])

	cmd.OrganizationID = testOrgID
	cmd.LoginChannel = "  "
	appErr = wantAppError(t, validateEvaluateRiskCommand(cmd), apperrors.CodeInvalidArgument, 400)
	assert.Equal(t, "loginChannel is required", appErr.Message)
}
