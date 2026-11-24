package privacy

import (
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func newEraseRequest(t *testing.T, now time.Time) *DsarRequest {
	t.Helper()
	request, err := NewDsarRequest("dsar-1", "org-1", SubjectTypeUser, "user-1", RequestTypeErase, nil, now)
	if err != nil {
		t.Fatalf("NewDsarRequest: %v", err)
	}
	if err := request.TransitionTo(StateVerifying, "", now.Add(time.Minute)); err != nil {
		t.Fatalf("received->verifying: %v", err)
	}
	if err := request.TransitionTo(StateInProgress, "", now.Add(2*time.Minute)); err != nil {
		t.Fatalf("verifying->in_progress: %v", err)
	}
	return request
}

func TestEraseRequest_CoolingWindowSetOnInProgress(t *testing.T) {
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	request := newEraseRequest(t, now)
	if request.CoolingUntil == nil {
		t.Fatal("erase request must register cooling_until when entering in_progress")
	}
	want := now.Add(2 * time.Minute).Add(CoolingWindow)
	if !request.CoolingUntil.Equal(want) {
		t.Fatalf("coolingUntil = %s, want %s", request.CoolingUntil, want)
	}
}

func TestEraseRequest_CannotFulfillInsideCooling(t *testing.T) {
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	request := newEraseRequest(t, now)
	err := request.TransitionTo(StateFulfilled, "", now.Add(time.Hour))
	if err == nil {
		t.Fatal("fulfilled inside cooling window must be rejected")
	}
	if typed, ok := err.(*apperrors.Error); !ok || typed.Code != apperrors.CodeConflict {
		t.Fatalf("error = %v, want conflict", err)
	}
}

func TestEraseRequest_WithdrawInsideCooling(t *testing.T) {
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	request := newEraseRequest(t, now)
	if err := request.TransitionTo(StateRejected, "subject withdrew during cooling", now.Add(time.Hour)); err != nil {
		t.Fatalf("withdraw inside cooling: %v", err)
	}
	if request.State != StateRejected || request.ClosedAt == nil {
		t.Fatalf("state=%s closedAt=%v, want rejected+closed", request.State, request.ClosedAt)
	}
}

func TestEraseRequest_FulfillAfterCooling(t *testing.T) {
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	request := newEraseRequest(t, now)
	after := now.Add(2 * time.Minute).Add(CoolingWindow).Add(time.Minute)
	if err := request.TransitionTo(StateFulfilled, "", after); err != nil {
		t.Fatalf("fulfilled after cooling: %v", err)
	}
}

func TestWithdrawRejectedOnlyForErase(t *testing.T) {
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	request, err := NewDsarRequest("dsar-2", "org-1", SubjectTypeUser, "user-1", RequestTypeAccess, nil, now)
	if err != nil {
		t.Fatalf("NewDsarRequest: %v", err)
	}
	_ = request.TransitionTo(StateVerifying, "", now.Add(time.Minute))
	_ = request.TransitionTo(StateInProgress, "", now.Add(2*time.Minute))
	if err := request.TransitionTo(StateRejected, "no", now.Add(3*time.Minute)); err == nil {
		t.Fatal("rejecting in_progress non-erase request must be refused")
	}
}
