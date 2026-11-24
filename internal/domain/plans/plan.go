package plans

import (
	"fmt"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/enumset"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type Plan struct {
	ID               string    `json:"id"`
	OrganizationID   string    `json:"organizationId"`
	Name             string    `json:"name"`
	DisplayName      string    `json:"displayName"`
	Description      string    `json:"description,omitempty"`
	Price            float64   `json:"price"`
	Currency         string    `json:"currency,omitempty"`
	Period           string    `json:"period,omitempty"`
	Product          string    `json:"product,omitempty"`
	PaymentProviders []string  `json:"paymentProviders,omitempty"`
	State            string    `json:"state"`
	IsExclusive      bool      `json:"isExclusive"`
	Role             string    `json:"role,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	Revision         int64     `json:"revision"`
}

// Plan lifecycle states. The values, the transition edges and the terminal set
// come from the state machine registry
// (`registries/state-machine-registry.md` §1.5): DDL `ck_plans_state`, the API
// enum and the console all render from this one definition.
//
// The former `is_enabled` boolean is gone. Sellability had two sources of truth
// — the boolean and the lifecycle — and they could disagree, so "can this plan
// be bought" had no single answer. `active` is now the only definition of
// sellable.
const (
	StateDraft    = "draft"
	StateActive   = "active"
	StateArchived = "archived"
)

// States is the ordered closed set of plan states (§1.5「状态集」).
var States = enumset.NewSet("plansState", StateDraft, StateActive, StateArchived)

// validPlanTransitions is §1.5 迁移图. `archived` is terminal and irreversible:
// the registry declares no recovery edge, so a retired plan stays retired.
var validPlanTransitions = map[string][]string{
	StateDraft:    {StateActive, StateArchived},
	StateActive:   {StateArchived},
	StateArchived: {},
}

// terminalPlanStates is §1.5「终态」.
var terminalPlanStates = map[string]bool{StateArchived: true}

// IsKnownState reports whether the value is in the registry's closed set.
func IsKnownState(state string) bool {
	return States.Contains(state)
}

// IsTerminal reports whether the state is a registry terminal state.
func IsTerminal(state string) bool {
	return terminalPlanStates[state]
}

// IsSellable reports whether the plan may be offered for purchase. It is the
// single replacement for the removed `is_enabled` boolean.
func IsSellable(state string) bool {
	return state == StateActive
}

// Plan billing periods. The set is closed for the same reason the states are:
// the console's select and the server's validator must render from one list.
// They did not — the console offered `Monthly`/`Yearly` while this validator
// accepted only the lowercase forms, so every plan created through the console
// was rejected. Nothing here normalises case; accepting `Monthly` would hide the
// disagreement instead of removing it.
const (
	PeriodDaily     = "daily"
	PeriodWeekly    = "weekly"
	PeriodMonthly   = "monthly"
	PeriodQuarterly = "quarterly"
	PeriodYearly    = "yearly"
	PeriodLifetime  = "lifetime"
)

// Periods is the ordered closed set of billing periods. DDL `ck_plans_period`
// carries the same values.
var Periods = enumset.NewSet("plansPeriod",
	PeriodDaily, PeriodWeekly, PeriodMonthly, PeriodQuarterly, PeriodYearly, PeriodLifetime)

// Validate checks the plan fields for business rule violations.
func (p *Plan) Validate() error {
	if p.State == "" {
		p.State = StateDraft
	}
	if !IsKnownState(p.State) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid plan state: %s", p.State),
			map[string]any{"field": "state", "value": p.State, "allowed": States.SortedValues()},
		)
	}
	if p.Price < 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "price must be non-negative", map[string]any{"field": "price", "value": p.Price, "min": 0})
	}
	if p.Period != "" && !Periods.Contains(p.Period) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid period: %s, must be one of %s", p.Period, Periods.JoinedValues()),
			map[string]any{"field": "period", "value": p.Period, "allowed": Periods.SortedValues()},
		)
	}
	return nil
}

// CanTransitionTo checks if the plan can transition to the given state.
func (p *Plan) CanTransitionTo(newState string) bool {
	allowed, ok := validPlanTransitions[p.State]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == newState {
			return true
		}
	}
	return false
}

// TransitionTo moves the aggregate to newState through the §1.5 graph.
//
// This is the only sanctioned way to write the lifecycle column: the guard
// lives with the table, so a caller cannot walk an edge the registry does not
// declare. Repeating the current plan is a no-op, because a console re-submits
// the value it read back.
func (p *Plan) TransitionTo(newState string) error {
	if newState == p.State {
		return nil
	}
	if !IsKnownState(newState) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid plan state: %s", newState),
			map[string]any{"field": "state", "value": newState, "allowed": States.SortedValues()},
		)
	}
	if !p.CanTransitionTo(newState) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("cannot move plan from %s to %s", p.State, newState),
			map[string]any{"field": "state", "value": p.State, "allowed": append([]string{}, validPlanTransitions[p.State]...)},
		)
	}
	p.State = newState
	return nil
}
