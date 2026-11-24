package products

import (
	"fmt"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/enumset"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type Product struct {
	ID                    string    `json:"id"`
	OrganizationID        string    `json:"organizationId"`
	Name                  string    `json:"name"`
	DisplayName           string    `json:"displayName"`
	Image                 string    `json:"image,omitempty"`
	Detail                string    `json:"detail,omitempty"`
	Description           string    `json:"description,omitempty"`
	Tag                   string    `json:"tag,omitempty"`
	Currency              string    `json:"currency,omitempty"`
	Price                 float64   `json:"price"`
	Quantity              int       `json:"quantity"`
	Sold                  int       `json:"sold"`
	IsRecharge            bool      `json:"isRecharge"`
	RechargeOptions       []float64 `json:"rechargeOptions,omitempty"`
	DisableCustomRecharge bool      `json:"disableCustomRecharge"`
	Providers             []string  `json:"providers,omitempty"`
	SuccessUrl            string    `json:"successUrl,omitempty"`
	State                 string    `json:"state"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
	Revision              int64     `json:"revision"`
}

// Product lifecycle states. The values, the transition edges and the terminal
// set come from the state machine registry
// (`registries/state-machine-registry.md` §1.5), which is the single source of
// truth for every lifecycle state in the system: DDL `ck_products_state`, the
// API enum and the console all render from this one definition.
const (
	StateDraft        = "draft"
	StateActive       = "active"
	StateDiscontinued = "discontinued"
)

// States is the ordered closed set of product states (§1.5「状态集」).
var States = enumset.NewSet("productsState", StateDraft, StateActive, StateDiscontinued)

// validProductTransitions is §1.5 迁移图. `discontinued` is terminal and
// irreversible: the registry declares no recovery edge, so a withdrawn product
// is re-issued as a new product rather than resurrected.
var validProductTransitions = map[string][]string{
	StateDraft:        {StateActive, StateDiscontinued},
	StateActive:       {StateDiscontinued},
	StateDiscontinued: {},
}

// terminalProductStates is §1.5「终态」.
var terminalProductStates = map[string]bool{StateDiscontinued: true}

// IsKnownState reports whether the value is in the registry's closed set.
func IsKnownState(state string) bool {
	return States.Contains(state)
}

// IsTerminal reports whether the state is a registry terminal state.
func IsTerminal(state string) bool {
	return terminalProductStates[state]
}

// Validate checks the product fields for business rule violations.
func (p *Product) Validate() error {
	if p.State == "" {
		p.State = StateDraft
	}
	if !IsKnownState(p.State) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid product state: %s", p.State),
			map[string]any{"field": "state", "value": p.State, "allowed": States.SortedValues()},
		)
	}
	if p.Price < 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "price must be non-negative", map[string]any{"field": "price", "value": p.Price, "min": 0})
	}
	if p.Quantity < 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "quantity must be non-negative", map[string]any{"field": "quantity", "value": p.Quantity, "min": 0})
	}
	// Recharge product must have recharge options
	if p.IsRecharge && len(p.RechargeOptions) == 0 && !p.DisableCustomRecharge {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "recharge product must have at least one recharge option or allow custom recharge", map[string]any{"field": "rechargeOptions", "expected": "non-empty unless custom recharge is allowed"})
	}
	return nil
}

// CanTransitionTo checks if the product can transition to the given state.
func (p *Product) CanTransitionTo(newState string) bool {
	allowed, ok := validProductTransitions[p.State]
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
// declare. Repeating the current product is a no-op, because a console re-submits
// the value it read back.
func (p *Product) TransitionTo(newState string) error {
	if newState == p.State {
		return nil
	}
	if !IsKnownState(newState) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid product state: %s", newState),
			map[string]any{"field": "state", "value": newState, "allowed": States.SortedValues()},
		)
	}
	if !p.CanTransitionTo(newState) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("cannot move product from %s to %s", p.State, newState),
			map[string]any{"field": "state", "value": p.State, "allowed": append([]string{}, validProductTransitions[p.State]...)},
		)
	}
	p.State = newState
	return nil
}
