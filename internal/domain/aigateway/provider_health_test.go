package aigateway_test

import (
	"testing"
	"time"

	"github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestProviderHealth_RecordSuccessClearsFailures(t *testing.T) {
	h := &aigateway.ProviderHealth{
		ID: "h-1", ProviderID: "p-1",
		State: aigateway.HealthStateHealthy, ConsecutiveFailures: 3,
	}
	h.RecordSuccess(nil)
	if h.ConsecutiveFailures != 0 {
		t.Errorf("ConsecutiveFailures should be 0 after success, got %d", h.ConsecutiveFailures)
	}
	if h.State != aigateway.HealthStateHealthy {
		t.Errorf("State should stay healthy, got %s", h.State)
	}
}

func TestProviderHealth_RecordSuccessInDegradedHeals(t *testing.T) {
	h := &aigateway.ProviderHealth{
		ID: "h-1", ProviderID: "p-1",
		State: aigateway.HealthStateDegraded, ConsecutiveFailures: 4,
	}
	h.RecordSuccess(nil)
	if h.State != aigateway.HealthStateHealthy {
		t.Errorf("degraded → success should move to healthy, got %s", h.State)
	}
}

func TestProviderHealth_CircuitOpensOnThreshold(t *testing.T) {
	h := &aigateway.ProviderHealth{
		ID: "h-1", ProviderID: "p-1",
		State: aigateway.HealthStateHealthy, ConsecutiveFailures: 0,
	}
	// 5 failures = threshold
	for i := 0; i < 4; i++ {
		h.RecordFailure()
	}
	if h.State != aigateway.HealthStateDegraded {
		t.Errorf("4 failures should be degraded, got %s", h.State)
	}
	h.RecordFailure() // 5th
	if h.State != aigateway.HealthStateOpen {
		t.Errorf("5th failure should open circuit, got %s", h.State)
	}
	if h.OpenUntil == nil {
		t.Error("OpenUntil should be set after opening")
	}
}

func TestProviderHealth_HalfOpenSuccessClosesBreaker(t *testing.T) {
	openUntil := time.Now().Add(-time.Second) // already expired
	h := &aigateway.ProviderHealth{
		ID: "h-1", ProviderID: "p-1",
		State: aigateway.HealthStateOpen, OpenUntil: &openUntil,
	}
	if !h.IsOpenExpired(time.Now()) {
		t.Fatal("circuit should be expired")
	}
	if err := h.EnterHalfOpen(time.Now()); err != nil {
		t.Fatalf("EnterHalfOpen: %v", err)
	}
	if h.State != aigateway.HealthStateHalfOpen {
		t.Fatalf("state should be half_open, got %s", h.State)
	}
	h.RecordSuccess(nil)
	if h.State != aigateway.HealthStateHealthy {
		t.Errorf("half_open + success → healthy, got %s", h.State)
	}
}

func TestProviderHealth_HalfOpenFailureReopens(t *testing.T) {
	openUntil := time.Now().Add(-time.Second)
	h := &aigateway.ProviderHealth{
		ID: "h-1", ProviderID: "p-1",
		State: aigateway.HealthStateOpen, OpenUntil: &openUntil,
	}
	if err := h.EnterHalfOpen(time.Now()); err != nil {
		t.Fatalf("EnterHalfOpen() error = %v", err)
	}
	h.RecordFailure()
	if h.State != aigateway.HealthStateOpen {
		t.Errorf("half_open + failure → open, got %s", h.State)
	}
}

func TestProviderHealth_IsAvailable(t *testing.T) {
	cases := []struct {
		state string
		want  bool
	}{
		{aigateway.HealthStateHealthy, true},
		{aigateway.HealthStateDegraded, true},
		{aigateway.HealthStateHalfOpen, true},
		{aigateway.HealthStateOpen, false},
	}
	for _, c := range cases {
		h := &aigateway.ProviderHealth{State: c.state}
		if got := h.IsAvailable(); got != c.want {
			t.Errorf("state %s IsAvailable = %v, want %v", c.state, got, c.want)
		}
	}
}

func TestProviderHealth_Validate(t *testing.T) {
	h := &aigateway.ProviderHealth{ID: "", ProviderID: "p-1", State: aigateway.HealthStateHealthy}
	if err := h.Validate(); err == nil {
		t.Error("missing id should fail Validate")
	}
	h2 := &aigateway.ProviderHealth{ID: "h-1", ProviderID: "p-1", State: "weird"}
	err := h2.Validate()
	if err == nil {
		t.Fatal("invalid state should fail Validate")
	}
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Errorf("code = %q, want %q", appErr.Code, apperrors.CodeUnprocessableEntity)
	}
	if appErr.Meta["field"] != "state" || appErr.Meta["value"] != "weird" {
		t.Errorf("meta = %v, want field=state value=weird", appErr.Meta)
	}
	allowed, ok := appErr.Meta["allowed"].([]string)
	if !ok || len(allowed) != 4 {
		t.Errorf("allowed = %v, want the four health states", appErr.Meta["allowed"])
	}
}
