package payments

import (
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/enumset"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Payment lifecycle states. The values, the transition edges and the terminal
// set come from the state machine registry
// (`registries/state-machine-registry.md` §1.2), which is the single source of
// truth for every lifecycle state in the system: DDL `ck_payments_state`, the
// API enum and the console all render from this one definition.
const (
	StateCreated           = "created"
	StateProcessing        = "processing"
	StateSucceeded         = "succeeded"
	StateFailed            = "failed"
	StateCancelled         = "cancelled"
	StateRefunded          = "refunded"
	StatePartiallyRefunded = "partially_refunded"
)

// States is the ordered closed set of payment states (§1.2「状态集」).
var States = enumset.NewSet("paymentsState",
	StateCreated, StateProcessing, StateSucceeded, StateFailed,
	StateCancelled, StateRefunded, StatePartiallyRefunded,
)

// validPaymentTransitions mirrors §1.2 迁移图. `succeeded` is the only source of
// `refunded`/`partially_refunded`, so a refund cannot exist for money that was
// never collected.
var validPaymentTransitions = map[string][]string{
	StateCreated:           {StateProcessing, StateSucceeded, StateFailed, StateCancelled},
	StateProcessing:        {StateSucceeded, StateFailed, StateCancelled, StateRefunded, StatePartiallyRefunded},
	StateSucceeded:         {StateRefunded, StatePartiallyRefunded},
	StatePartiallyRefunded: {StateRefunded, StatePartiallyRefunded},
	StateFailed:            {},
	StateCancelled:         {},
	StateRefunded:          {},
}

// terminalPaymentStates are the §1.2 states with no outgoing edge.
var terminalPaymentStates = map[string]bool{
	StateFailed:    true,
	StateCancelled: true,
	StateRefunded:  true,
}

type Payment struct {
	ID                 string    `json:"id"`
	OrganizationID     string    `json:"organizationId"`
	Name               string    `json:"name"`
	DisplayName        string    `json:"displayName"`
	Provider           string    `json:"provider,omitempty"`
	Type               string    `json:"type,omitempty"`
	Products           []string  `json:"products,omitempty"`
	ProductName        string    `json:"productName,omitempty"`
	ProductDisplayName string    `json:"productDisplayName,omitempty"`
	Detail             string    `json:"detail,omitempty"`
	Currency           string    `json:"currency,omitempty"`
	Price              float64   `json:"price"`
	User               string    `json:"user,omitempty"`
	PersonName         string    `json:"personName,omitempty"`
	PersonIdCard       string    `json:"personIdCard,omitempty"`
	PersonEmail        string    `json:"personEmail,omitempty"`
	PersonPhone        string    `json:"personPhone,omitempty"`
	InvoiceType        string    `json:"invoiceType,omitempty"`
	InvoiceTitle       string    `json:"invoiceTitle,omitempty"`
	InvoiceTaxId       string    `json:"invoiceTaxId,omitempty"`
	InvoiceRemark      string    `json:"invoiceRemark,omitempty"`
	InvoiceUrl         string    `json:"invoiceUrl,omitempty"`
	Order              string    `json:"order,omitempty"`
	OutOrderId         string    `json:"outOrderId,omitempty"`
	PayUrl             string    `json:"payUrl,omitempty"`
	SuccessUrl         string    `json:"successUrl,omitempty"`
	State              string    `json:"state"`
	Message            string    `json:"message,omitempty"`
	ChannelOrderNo     string    `json:"channelOrderNo,omitempty"`
	AttemptCount       int       `json:"attemptCount"`
	RefundedAmount     float64   `json:"refundedAmount"`
	RefundingAmount    float64   `json:"refundingAmount"`
	LastErrorCode      string    `json:"lastErrorCode,omitempty"`
	LastErrorMessage   string    `json:"lastErrorMessage,omitempty"`
	IdempotencyKey     string    `json:"idempotencyKey,omitempty"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
	Revision           int64     `json:"revision"`
}

// Validate checks the payment fields for business rule violations.
func (p *Payment) Validate() error {
	if p.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if p.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if p.State == "" {
		p.State = StateCreated
	}
	if err := States.Validate("state", p.State); err != nil {
		return err
	}
	if p.Price < 0 {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"price must be non-negative",
			map[string]any{"field": "price", "value": p.Price, "min": 0},
		)
	}
	return nil
}

// IsKnownState reports whether the value is in the registry's closed set.
func IsKnownState(state string) bool {
	return States.Contains(state)
}

// IsTerminal reports whether the state is a registry terminal state.
func IsTerminal(state string) bool {
	return terminalPaymentStates[state]
}

// IsSettled reports whether the payment collected money and has not been
// fully reversed. `succeeded` and `partially_refunded` both qualify — the
// latter means part of the collected money is still on the books.
func (p *Payment) IsSettled() bool {
	return p.State == StateSucceeded || p.State == StatePartiallyRefunded
}

// CanRefund reports whether the payment is in a state that allows any
// refund request to be placed — we only allow refunds against settled money.
func (p *Payment) CanRefund() bool {
	return p.State == StateSucceeded || p.State == StatePartiallyRefunded
}

// AvailableForRefund is the amount still open for refunds. The upper bound
// is price minus what is already refunded or in-flight; callers must also
// hold the SELECT FOR UPDATE row lock before writing.
func (p *Payment) AvailableForRefund() float64 {
	return p.Price - p.RefundedAmount - p.RefundingAmount
}

// CanTransitionTo checks whether the payment may move to the given state.
func (p *Payment) CanTransitionTo(newState string) bool {
	for _, s := range validPaymentTransitions[p.State] {
		if s == newState {
			return true
		}
	}
	return false
}

// AllowedTransitions returns the states this payment may move to next.
//
// It reads the registry transition table itself, so a 422 rejection can list
// exactly the edges the guard will accept instead of a second hand-written
// list that could drift from the graph.
func (p *Payment) AllowedTransitions() []string {
	return append([]string{}, validPaymentTransitions[p.State]...)
}

// TransitionTo moves the aggregate to newState through the §1.5 graph.
//
// This is the only sanctioned way to write the lifecycle column: the guard
// lives with the table, so a caller cannot walk an edge the registry does not
// declare. Repeating the current payment is a no-op, because a console re-submits
// the value it read back.
func (p *Payment) TransitionTo(newState string) error {
	if newState == p.State {
		return nil
	}
	if err := States.Validate("state", newState); err != nil {
		return err
	}
	if !p.CanTransitionTo(newState) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"cannot move payment from "+p.State+" to "+newState,
			map[string]any{
				"field":      "state",
				"value":      p.State,
				"transition": newState,
				"allowed":    p.AllowedTransitions(),
			},
		)
	}
	p.State = newState
	return nil
}
