package subscriptions

import (
	"fmt"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/enumset"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type Subscription struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	Name           string     `json:"name"`
	DisplayName    string     `json:"displayName"`
	User           string     `json:"user,omitempty"`
	Plan           string     `json:"plan,omitempty"`
	Pricing        string     `json:"pricing,omitempty"`
	State          string     `json:"state"`
	StartAt        *time.Time `json:"startAt,omitempty"`
	EndAt          *time.Time `json:"endAt,omitempty"`
	ExpireAt       *time.Time `json:"expireAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	Revision       int64      `json:"revision"`
}

// Subscription lifecycle states. The values, the transition edges and the
// terminal set come from the state machine registry
// (`registries/state-machine-registry.md` §1.3): DDL `ck_subscriptions_state`,
// the API enum and the console all render from this one definition.
const (
	StatePending   = "pending"
	StateTrialing  = "trialing"
	StateActive    = "active"
	StatePastDue   = "past_due"
	StateExpired   = "expired"
	StateCancelled = "cancelled"
)

// States is the ordered closed set of subscription states (§1.3「状态集」).
var States = enumset.NewSet("subscriptionsState",
	StatePending, StateTrialing, StateActive, StatePastDue, StateExpired, StateCancelled,
)

// validSubscriptionTransitions is §1.3 迁移图. `past_due → active` is the one
// explicitly declared recovery edge (settlement during the grace period); every
// other terminal state is irreversible.
var validSubscriptionTransitions = map[string][]string{
	StatePending:   {StateTrialing, StateActive, StateCancelled},
	StateTrialing:  {StateActive, StatePastDue, StateCancelled},
	StateActive:    {StatePastDue, StateExpired, StateCancelled},
	StatePastDue:   {StateActive, StateExpired, StateCancelled},
	StateExpired:   {},
	StateCancelled: {},
}

// terminalSubscriptionStates is §1.3「终态」.
var terminalSubscriptionStates = map[string]bool{StateExpired: true, StateCancelled: true}

// IsKnownState reports whether the value is in the registry's closed set.
func IsKnownState(state string) bool {
	return States.Contains(state)
}

// IsTerminal reports whether the state is a registry terminal state.
func IsTerminal(state string) bool {
	return terminalSubscriptionStates[state]
}

// GrantsEntitlement reports whether a subscription in this state entitles its
// user to the paid capability — the single definition of "paid user is entitled"
// (§1.3). `past_due` is the grace window the registry declares a recovery edge
// for, so it keeps entitlement while recovery is still possible; `expired` and
// `cancelled` do not.
func GrantsEntitlement(state string) bool {
	return state == StateActive || state == StatePastDue
}

// IsEntitled reports whether this subscription grants entitlement at the given
// instant. The `expire_at` comparison lives here rather than at each call site so
// that "has the subscription run out" has exactly one answer: `state` says the
// lifecycle has not been closed, and `expire_at` bounds it in wall-clock terms.
// A nil `expire_at` means no expiry bound was ever set.
func (s *Subscription) IsEntitled(now time.Time) bool {
	if !GrantsEntitlement(s.State) {
		return false
	}
	return s.ExpireAt == nil || now.Before(*s.ExpireAt)
}

// Validate checks the subscription fields for business rule violations.
func (s *Subscription) Validate() error {
	if !IsKnownState(s.State) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid subscription state: %s", s.State),
			map[string]any{"field": "state", "value": s.State, "allowed": States.SortedValues()},
		)
	}
	// Date range consistency: start < end < expire
	if s.StartAt != nil && s.EndAt != nil && !s.EndAt.After(*s.StartAt) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "endAt must be after startAt", map[string]any{"field": "endAt", "value": s.EndAt.UTC().Format(time.RFC3339), "expected": "after startAt"})
	}
	if s.EndAt != nil && s.ExpireAt != nil && !s.ExpireAt.After(*s.EndAt) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "expireAt must be after endAt", map[string]any{"field": "expireAt", "value": s.ExpireAt.UTC().Format(time.RFC3339), "expected": "after endAt"})
	}
	if s.StartAt != nil && s.ExpireAt != nil && !s.ExpireAt.After(*s.StartAt) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "expireAt must be after startAt", map[string]any{"field": "expireAt", "value": s.ExpireAt.UTC().Format(time.RFC3339), "expected": "after startAt"})
	}
	return nil
}

// CanTransitionTo checks if the subscription can transition to the given state.
func (s *Subscription) CanTransitionTo(newState string) bool {
	allowed, ok := validSubscriptionTransitions[s.State]
	if !ok {
		return false
	}
	for _, st := range allowed {
		if st == newState {
			return true
		}
	}
	return false
}

// TransitionTo moves the aggregate to newState through the §1.5 graph.
//
// This is the only sanctioned way to write the lifecycle column: the guard
// lives with the table, so a caller cannot walk an edge the registry does not
// declare. Repeating the current subscription is a no-op, because a console re-submits
// the value it read back.
func (s *Subscription) TransitionTo(newState string) error {
	if newState == s.State {
		return nil
	}
	if !IsKnownState(newState) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid subscription state: %s", newState),
			map[string]any{"field": "state", "value": newState, "allowed": States.SortedValues()},
		)
	}
	if !s.CanTransitionTo(newState) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("cannot move subscription from %s to %s", s.State, newState),
			map[string]any{"field": "state", "value": s.State, "allowed": append([]string{}, validSubscriptionTransitions[s.State]...)},
		)
	}
	s.State = newState
	return nil
}
