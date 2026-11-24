package coupons

import (
	"fmt"
	"slices"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/enumset"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// discountTypes is the closed set the coupon rule accepts; the check and the
// 422 `allowed` metadata read the same slice.
var discountTypes = []string{"percentage", "fixed"}

type Coupon struct {
	ID              string    `json:"id"`
	OrganizationID  string    `json:"organizationId"`
	Name            string    `json:"name"`
	DisplayName     string    `json:"displayName"`
	Description     string    `json:"description,omitempty"`
	Code            string    `json:"code"`
	DiscountType    string    `json:"discountType"`
	Discount        float64   `json:"discount"`
	MaxDiscount     float64   `json:"maxDiscount"`
	Scope           string    `json:"scope,omitempty"`
	Products        []string  `json:"products,omitempty"`
	Users           []string  `json:"users,omitempty"`
	Quantity        int       `json:"quantity"`
	UsedCount       int       `json:"usedCount"`
	MaxUsagePerUser int       `json:"maxUsagePerUser"`
	StartTime       string    `json:"startTime,omitempty"`
	ExpireTime      string    `json:"expireTime,omitempty"`
	MinOrderAmount  float64   `json:"minOrderAmount"`
	Currency        string    `json:"currency,omitempty"`
	State           string    `json:"state"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	Revision        int64     `json:"revision"`
}

// Coupon lifecycle states. The values, the transition edges and the terminal
// set come from the state machine registry
// (`registries/state-machine-registry.md` §1.5): DDL `ck_coupons_state`, the API
// enum and the console all render from this one definition.
const (
	StateActive   = "active"
	StateDisabled = "disabled"
	StateExpired  = "expired"
)

// States is the ordered closed set of coupon states (§1.5「状态集」).
var States = enumset.NewSet("couponsState", StateActive, StateDisabled, StateExpired)

// validCouponTransitions is §1.5 迁移图. `expired` is terminal — a coupon that
// has run out of its validity window cannot be brought back into force.
var validCouponTransitions = map[string][]string{
	StateActive:   {StateDisabled, StateExpired},
	StateDisabled: {StateActive, StateExpired},
	StateExpired:  {},
}

// terminalCouponStates is §1.5「终态」.
var terminalCouponStates = map[string]bool{StateExpired: true}

// IsKnownState reports whether the value is in the registry's closed set.
func IsKnownState(state string) bool {
	return States.Contains(state)
}

// IsTerminal reports whether the state is a registry terminal state.
func IsTerminal(state string) bool {
	return terminalCouponStates[state]
}

// Validate checks the coupon fields for business rule violations.
func (c *Coupon) Validate() error {
	if c.State == "" {
		c.State = StateActive
	}
	if !IsKnownState(c.State) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid coupon state: %s", c.State),
			map[string]any{"field": "state", "value": c.State, "allowed": States.SortedValues()},
		)
	}
	if c.Code == "" {
		return apperrors.NewInvalidArgument("coupon code is required")
	}
	if !slices.Contains(discountTypes, c.DiscountType) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "discountType must be 'percentage' or 'fixed'", map[string]any{"field": "discountType", "value": c.DiscountType, "allowed": append([]string{}, discountTypes...)})
	}
	if c.Discount < 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "discount must be non-negative", map[string]any{"field": "discount", "value": c.Discount, "min": 0})
	}
	if c.DiscountType == "percentage" && c.Discount > 100 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "percentage discount cannot exceed 100", map[string]any{"field": "discount", "value": c.Discount, "max": 100})
	}
	if c.MaxDiscount < 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "maxDiscount must be non-negative", map[string]any{"field": "maxDiscount", "value": c.MaxDiscount, "min": 0})
	}
	if c.Quantity < 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "quantity must be non-negative", map[string]any{"field": "quantity", "value": c.Quantity, "min": 0})
	}
	if c.MaxUsagePerUser < 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "maxUsagePerUser must be non-negative", map[string]any{"field": "maxUsagePerUser", "value": c.MaxUsagePerUser, "min": 0})
	}
	if c.MinOrderAmount < 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "minOrderAmount must be non-negative", map[string]any{"field": "minOrderAmount", "value": c.MinOrderAmount, "min": 0})
	}
	// Validate time range consistency
	if err := c.ValidateTimeWindow(); err != nil {
		return err
	}
	// Scope-product consistency
	if c.Scope == "product" && len(c.Products) == 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "products must be specified when scope is 'product'", map[string]any{"field": "products", "expected": "non-empty when scope=product"})
	}
	return nil
}

// ValidateTimeWindow checks an optional validity window: every present bound
// must be RFC3339, and expireTime must be strictly after startTime. It is
// exported so patch paths can re-check only the time columns without re-judging
// unrelated legacy values through Validate.
func (c *Coupon) ValidateTimeWindow() error {
	var start, expire time.Time
	if c.StartTime != "" {
		parsed, err := time.Parse(time.RFC3339, c.StartTime)
		if err != nil {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "startTime must be a valid RFC3339 timestamp", map[string]any{"field": "startTime", "value": c.StartTime, "expected": "RFC3339"})
		}
		start = parsed
	}
	if c.ExpireTime != "" {
		parsed, err := time.Parse(time.RFC3339, c.ExpireTime)
		if err != nil {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "expireTime must be a valid RFC3339 timestamp", map[string]any{"field": "expireTime", "value": c.ExpireTime, "expected": "RFC3339"})
		}
		expire = parsed
	}
	if c.StartTime != "" && c.ExpireTime != "" && !expire.After(start) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "expireTime must be after startTime", map[string]any{"field": "expireTime", "value": c.ExpireTime, "expected": "after startTime"})
	}
	return nil
}

// CanTransitionTo checks if the coupon can transition to the given state.
func (c *Coupon) CanTransitionTo(newState string) bool {
	allowed, ok := validCouponTransitions[c.State]
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
// declare. Repeating the current coupon is a no-op, because a console re-submits
// the value it read back.
func (c *Coupon) TransitionTo(newState string) error {
	if newState == c.State {
		return nil
	}
	if !IsKnownState(newState) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid coupon state: %s", newState),
			map[string]any{"field": "state", "value": newState, "allowed": States.SortedValues()},
		)
	}
	if !c.CanTransitionTo(newState) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("cannot move coupon from %s to %s", c.State, newState),
			map[string]any{"field": "state", "value": c.State, "allowed": append([]string{}, validCouponTransitions[c.State]...)},
		)
	}
	c.State = newState
	return nil
}
