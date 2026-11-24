package orders

import (
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/enumset"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type Order struct {
	ID             string        `json:"id"`
	OrganizationID string        `json:"organizationId"`
	Name           string        `json:"name"`
	DisplayName    string        `json:"displayName"`
	User           string        `json:"user,omitempty"`
	Products       []string      `json:"products,omitempty"`
	ProductInfos   []ProductInfo `json:"productInfos,omitempty"`
	Payment        string        `json:"payment,omitempty"`
	Price          float64       `json:"price"`
	Currency       string        `json:"currency,omitempty"`
	State          string        `json:"state"`
	Message        string        `json:"message,omitempty"`
	PaymentURL     string        `json:"paymentUrl,omitempty"`
	TradeNo        string        `json:"tradeNo,omitempty"`
	CouponName     string        `json:"couponName,omitempty"`
	CouponDiscount float64       `json:"couponDiscount"`
	IdempotencyKey string        `json:"idempotencyKey,omitempty"`
	CreatedAt      time.Time     `json:"createdAt"`
	UpdatedAt      time.Time     `json:"updatedAt"`
	// PaidAt is the settlement instant recorded when the state machine entered
	// `paid`. It is the basis a reader needs to trust a "paid" conclusion
	// (`设计报告 42.3.3`): a conclusion without its basis cannot be self-verified
	// afterwards. Nil means the order has never reached `paid`.
	PaidAt   *time.Time `json:"paidAt,omitempty"`
	Revision int64      `json:"revision"`
}

type ProductInfo struct {
	Owner       string  `json:"owner"`
	Name        string  `json:"name"`
	DisplayName string  `json:"displayName"`
	Image       string  `json:"image,omitempty"`
	Detail      string  `json:"detail,omitempty"`
	Price       float64 `json:"price"`
	Currency    string  `json:"currency,omitempty"`
	IsRecharge  bool    `json:"isRecharge"`
	Quantity    int     `json:"quantity"`
	PricingName string  `json:"pricingName,omitempty"`
	PlanName    string  `json:"planName,omitempty"`
}

// Order lifecycle states. The values, the transition edges and the terminal set
// come from the state machine registry (`registries/state-machine-registry.md`
// §1.1), which is the single source of truth for every lifecycle state in the
// system: DDL `ck_orders_state`, the API enum and the console all render from
// this one definition.
const (
	StateCreated        = "created"
	StatePaymentPending = "payment_pending"
	StatePaid           = "paid"
	StateCompleted      = "completed"
	StateFailed         = "failed"
	StateCancelled      = "cancelled"
	StateRefunded       = "refunded"
	StateClosed         = "closed"
)

// States is the ordered closed set of order states
// (`registries/state-machine-registry.md` §1.1「状态集」).
var States = enumset.NewSet("ordersState",
	StateCreated, StatePaymentPending, StatePaid, StateCompleted,
	StateFailed, StateCancelled, StateRefunded, StateClosed,
)

// Valid state transitions (§1.1 迁移图). `failed` is deliberately not terminal:
// the registry gives it a recovery edge back to `payment_pending`, so a failed
// payment can be retried without creating a new order.
var validOrderTransitions = map[string][]string{
	StateCreated:        {StatePaymentPending, StateCancelled},
	StatePaymentPending: {StatePaid, StateFailed, StateCancelled},
	StatePaid:           {StateCompleted},
	StateCompleted:      {StateRefunded, StateClosed},
	StateFailed:         {StatePaymentPending},
	StateCancelled:      {},
	StateRefunded:       {},
	StateClosed:         {},
}

// terminalOrderStates is §1.1「终态」— irreversible, and therefore the set that
// ends any client-side progress expectation.
var terminalOrderStates = map[string]bool{
	StateCancelled: true, StateRefunded: true, StateClosed: true,
}

// awaitingPaymentStates are the states in which the payment gateway may still
// move the order forward. A polling client must keep polling exactly while the
// order is in this set and must not derive that expectation itself.
var awaitingPaymentStates = map[string]bool{
	StateCreated: true, StatePaymentPending: true,
}

// paidStates are the states that presuppose a recorded settlement instant. The
// DDL constraint `ck_orders_paid_at` keeps `paid_at` in lockstep with this set,
// so "订单已支付" is always accompanied by its basis (`设计报告 42.3.3`).
var paidStates = map[string]bool{
	StatePaid: true, StateCompleted: true, StateRefunded: true, StateClosed: true,
}

// PollInterval is the server-suggested interval between conditional requests
// while an order is still awaiting payment (`设计报告 42.3.11` 条 2). The client
// must not hardcode an interval: the cost of an unchanged poll is a 304, and the
// server owns how often that request is worth making.
const PollInterval = 3 * time.Second

// SettledPollInterval is the suggested interval once the channel can no longer
// advance the order but the reader is still watching the page.
const SettledPollInterval = 15 * time.Second

// IsKnownState reports whether the value is in the registry's closed set.
func IsKnownState(state string) bool {
	return States.Contains(state)
}

// IsTerminal reports whether the state is a registry terminal state.
func IsTerminal(state string) bool {
	return terminalOrderStates[state]
}

// IsAwaitingPayment reports whether the order may still be advanced by the
// payment channel — the server-side authority behind the polling guidance.
func IsAwaitingPayment(state string) bool {
	return awaitingPaymentStates[state]
}

// RequiresPaidAt reports whether the state may only be entered with a recorded
// settlement instant.
func RequiresPaidAt(state string) bool {
	return paidStates[state]
}

// PollAfter returns the server-suggested interval for the next conditional read
// of this order. The decision is derived from the lifecycle, never from the
// client, and never from a wall-clock timeout.
func PollAfter(state string) time.Duration {
	if IsAwaitingPayment(state) {
		return PollInterval
	}
	return SettledPollInterval
}

// Validate checks the order fields for business rule violations.
func (o *Order) Validate() error {
	if o.State == "" {
		o.State = StateCreated
	}
	if err := States.Validate("state", o.State); err != nil {
		return err
	}
	if o.Price < 0 {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"price must be non-negative",
			map[string]any{"field": "price", "value": o.Price, "min": 0},
		)
	}
	if o.CouponDiscount < 0 {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"couponDiscount must be non-negative",
			map[string]any{"field": "couponDiscount", "value": o.CouponDiscount, "min": 0},
		)
	}
	return nil
}

// CanTransitionTo checks if the order can transition to the given state.
func (o *Order) CanTransitionTo(newState string) bool {
	allowed, ok := validOrderTransitions[o.State]
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

// AllowedTransitions returns the states this order may move to next, read from
// the registry transition table so a 422 rejection lists the same edges the
// guard will accept.
func (o *Order) AllowedTransitions() []string {
	return append([]string{}, validOrderTransitions[o.State]...)
}

// TransitionTo moves the aggregate to newState through the §1.5 graph.
//
// This is the only sanctioned way to write the lifecycle column: the guard
// lives with the table, so a caller cannot walk an edge the registry does not
// declare. Repeating the current order is a no-op, because a console re-submits
// the value it read back.
func (o *Order) TransitionTo(newState string) error {
	if newState == o.State {
		return nil
	}
	if err := States.Validate("state", newState); err != nil {
		return err
	}
	if !o.CanTransitionTo(newState) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"cannot move order from "+o.State+" to "+newState,
			map[string]any{
				"field":      "state",
				"value":      o.State,
				"transition": newState,
				"allowed":    o.AllowedTransitions(),
			},
		)
	}
	o.State = newState
	return nil
}
