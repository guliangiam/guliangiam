package aigateway

import (
	"fmt"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/enumset"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Circuit breaker states. The half_open probe is the only edge back into
// healthy — it lets one call through and either closes the breaker or reopens.
const (
	HealthStateHealthy  = "healthy"
	HealthStateDegraded = "degraded"
	HealthStateOpen     = "open"
	HealthStateHalfOpen = "half_open"
)

var ValidHealthStates = enumset.NewSet("aigatewayHealthState",
	HealthStateHealthy, HealthStateDegraded, HealthStateOpen, HealthStateHalfOpen,
)

// consecutiveFailures threshold for moving healthy → open.
const openThreshold = 5

// openDuration default duration for open state (seconds).
const defaultOpenDuration = 30 * time.Second

type ProviderHealth struct {
	ID                  string     `json:"id"`
	ProviderID          string     `json:"providerId"`
	State               string     `json:"state"`
	ConsecutiveFailures int        `json:"consecutiveFailures"`
	OpenUntil           *time.Time `json:"openUntil,omitempty"`
	LastCheckedAt       *time.Time `json:"lastCheckedAt,omitempty"`
	LastLatencyMs       *int       `json:"lastLatencyMs,omitempty"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

func (h *ProviderHealth) Validate() error {
	if h.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if h.ProviderID == "" {
		return apperrors.NewInvalidArgument("providerId is required")
	}
	if !ValidHealthStates.Contains(h.State) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid health state: %s", h.State),
			map[string]any{"field": "state", "value": string(h.State), "allowed": ValidHealthStates.SortedValues()},
		)
	}
	if h.ConsecutiveFailures < 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "consecutiveFailures must be non-negative", map[string]any{"field": "consecutiveFailures", "value": h.ConsecutiveFailures, "min": 0})
	}
	return nil
}

// RecordSuccess feeds a positive signal into the state machine. A success in
// any state wipes the failure counter — even if we were open, because this
// call succeeded somewhere (a failover, perhaps) and we should stop counting.
func (h *ProviderHealth) RecordSuccess(latencyMs *int) {
	h.ConsecutiveFailures = 0
	h.LastLatencyMs = latencyMs
	h.LastCheckedAt = ptrNow()
	switch h.State {
	case HealthStateHalfOpen:
		h.State = HealthStateHealthy
		h.OpenUntil = nil
	case HealthStateOpen:
		// success during open is unusual (likely a stale probe) — keep open
		// but don't failover any further
		h.ConsecutiveFailures = openThreshold
	case HealthStateHealthy:
		// stay healthy; degraded clears on any success
	case HealthStateDegraded:
		h.State = HealthStateHealthy
	}
}

// RecordFailure increments the failure counter and escalates the state
// machine. A failure in half_open reopens the breaker.
func (h *ProviderHealth) RecordFailure() {
	h.ConsecutiveFailures++
	h.LastCheckedAt = ptrNow()
	switch h.State {
	case HealthStateHealthy:
		if h.ConsecutiveFailures >= 3 {
			h.State = HealthStateDegraded
		}
		if h.ConsecutiveFailures >= openThreshold {
			h.State = HealthStateOpen
			t := time.Now().Add(defaultOpenDuration)
			h.OpenUntil = &t
		}
	case HealthStateDegraded:
		if h.ConsecutiveFailures >= openThreshold {
			h.State = HealthStateOpen
			t := time.Now().Add(defaultOpenDuration)
			h.OpenUntil = &t
		}
	case HealthStateHalfOpen:
		h.State = HealthStateOpen
		t := time.Now().Add(defaultOpenDuration)
		h.OpenUntil = &t
	}
}

// EnterHalfOpen transitions open → half_open once the open window expires.
// Callers must check IsOpenExpired first.
func (h *ProviderHealth) EnterHalfOpen(now time.Time) error {
	if h.State != HealthStateOpen {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("cannot enter half_open from %s", h.State),
			map[string]any{"field": "status", "value": string(h.State), "expected": string(HealthStateOpen)},
		)
	}
	if h.OpenUntil != nil && !now.After(*h.OpenUntil) {
		return apperrors.NewInvalidArgument("open window has not expired")
	}
	h.State = HealthStateHalfOpen
	h.OpenUntil = nil
	return nil
}

// IsOpenExpired reports whether the open window has elapsed and a half_open
// probe should be scheduled.
func (h *ProviderHealth) IsOpenExpired(now time.Time) bool {
	if h.State != HealthStateOpen || h.OpenUntil == nil {
		return false
	}
	return now.After(*h.OpenUntil)
}

// IsAvailable answers whether the provider should accept live traffic right
// now — healthy, degraded and half_open (probe) are yes, open is no.
func (h *ProviderHealth) IsAvailable() bool {
	switch h.State {
	case HealthStateHealthy, HealthStateDegraded, HealthStateHalfOpen:
		return true
	case HealthStateOpen:
		return false
	}
	return false
}

func ptrNow() *time.Time {
	t := time.Now()
	return &t
}
