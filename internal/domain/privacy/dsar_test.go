package privacy

import (
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testNow() time.Time { return time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC) }

func newTestRequest(t *testing.T, requestType string) *DsarRequest {
	t.Helper()
	request, err := NewDsarRequest("dsar-1", "org-1", SubjectTypeUser, "user-1", requestType, nil, testNow())
	require.NoError(t, err)
	return request
}

func TestNewDsarRequest_Defaults(t *testing.T) {
	request := newTestRequest(t, RequestTypeAccess)
	assert.Equal(t, StateReceived, request.State)
	assert.Equal(t, testNow().Add(SLABudget), request.SLADueAt)
	assert.Equal(t, int64(1), request.Revision)
	assert.Nil(t, request.ClosedAt)
	assert.Len(t, request.AllowedTransitions(), 1)
}

func TestNewDsarRequest_InvalidInput(t *testing.T) {
	for _, tt := range []struct {
		name        string
		subjectType string
		requestType string
	}{
		{name: "bad_subject", subjectType: "employee", requestType: RequestTypeAccess},
		{name: "bad_type", subjectType: SubjectTypeUser, requestType: "delete_all"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDsarRequest("dsar-1", "org-1", tt.subjectType, "user-1", tt.requestType, nil, testNow())
			require.Error(t, err)
			var appErr *apperrors.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
		})
	}
}

func TestDsarRequest_HappyPath(t *testing.T) {
	request := newTestRequest(t, RequestTypeErase)
	now := testNow()

	require.NoError(t, request.TransitionTo(StateVerifying, "", now.Add(time.Hour)))
	require.NoError(t, request.TransitionTo(StateInProgress, "", now.Add(2*time.Hour)))
	// erase 请求的冷静期：in_progress 起 7 天，期间不可 fulfilled（AC-PRIV-002）。
	fulfilledAt := now.Add(2 * time.Hour).Add(CoolingWindow).Add(time.Hour)
	require.NoError(t, request.TransitionTo(StateFulfilled, "", fulfilledAt))

	assert.Equal(t, StateFulfilled, request.State)
	assert.True(t, request.IsTerminal())
	require.NotNil(t, request.ClosedAt)
	assert.Equal(t, fulfilledAt, *request.ClosedAt)
	assert.Equal(t, int64(4), request.Revision)
	// 终态不可逆
	assert.Empty(t, request.AllowedTransitions())
	require.Error(t, request.TransitionTo(StateVerifying, "", now.Add(4*time.Hour)))
}

func TestDsarRequest_RejectedRequiresReason(t *testing.T) {
	request := newTestRequest(t, RequestTypeAccess)
	require.NoError(t, request.TransitionTo(StateVerifying, "", testNow()))

	err := request.TransitionTo(StateRejected, "", testNow())
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)

	require.NoError(t, request.TransitionTo(StateRejected, "identity cannot be verified", testNow()))
	assert.Equal(t, StateRejected, request.State)
	require.NotNil(t, request.RejectionReason)
	assert.Equal(t, "identity cannot be verified", *request.RejectionReason)
	require.NotNil(t, request.ClosedAt)
}

func TestDsarRequest_InvalidEdge(t *testing.T) {
	request := newTestRequest(t, RequestTypeAccess)
	err := request.TransitionTo(StateFulfilled, "", testNow())
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "state", appErr.Meta["field"])
	assert.Equal(t, StateReceived, appErr.Meta["value"])
}

func TestDsarRequest_IsOverdue(t *testing.T) {
	request := newTestRequest(t, RequestTypePortability)
	assert.False(t, request.IsOverdue(request.SLADueAt))
	assert.True(t, request.IsOverdue(request.SLADueAt.Add(time.Second)))
	require.NoError(t, request.TransitionTo(StateVerifying, "", testNow()))
	require.NoError(t, request.TransitionTo(StateRejected, "out of scope", testNow()))
	assert.False(t, request.IsOverdue(request.SLADueAt.Add(48*time.Hour)), "terminal requests are not overdue")
}

func TestDsarRequest_ValidateCloses(t *testing.T) {
	request := newTestRequest(t, RequestTypeRectify)
	closed := testNow()
	request.ClosedAt = &closed
	assert.Error(t, request.Validate(), "open request must not carry closedAt")

	request = newTestRequest(t, RequestTypeRectify)
	request.State = StateFulfilled
	assert.Error(t, request.Validate(), "terminal request must carry closedAt")
}

func TestNewDsarEvent_ActionWhitelist(t *testing.T) {
	event, err := NewDsarEvent("evt-1", "org-1", "dsar-1", "created", nil, nil, testNow())
	require.NoError(t, err)
	assert.Equal(t, "created", event.Action)

	_, err = NewDsarEvent("evt-2", "org-1", "dsar-1", "deleted", nil, nil, testNow())
	require.Error(t, err)
}
