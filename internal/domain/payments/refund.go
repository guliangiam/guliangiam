package payments

import (
	"fmt"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	RefundStateRequested  = "requested"
	RefundStateProcessing = "processing"
	RefundStateSucceeded  = "succeeded"
	RefundStateFailed     = "failed"
	RefundStateCancelled  = "cancelled"
)

var validRefundTransitions = map[string][]string{
	RefundStateRequested:  {RefundStateProcessing, RefundStateFailed, RefundStateCancelled},
	RefundStateProcessing: {RefundStateSucceeded, RefundStateFailed, RefundStateCancelled},
	RefundStateSucceeded:  {},
	RefundStateFailed:     {},
	RefundStateCancelled:  {},
}

// refundStates is the closed set the checks and the 422 `allowed` metadata share.
var refundStates = []string{
	RefundStateRequested, RefundStateProcessing, RefundStateSucceeded,
	RefundStateFailed, RefundStateCancelled,
}

type Refund struct {
	ID              string     `json:"id"`
	OrganizationID  string     `json:"organizationId"`
	PaymentID       string     `json:"paymentId"`
	OutRefundNo     string     `json:"outRefundNo"`
	Channel         string     `json:"channel"`
	Amount          float64    `json:"amount"`
	Currency        string     `json:"currency"`
	State           string     `json:"state"`
	Reason          string     `json:"reason,omitempty"`
	ChannelRefundNo string     `json:"channelRefundNo,omitempty"`
	ErrorCode       *string    `json:"errorCode,omitempty"`
	ErrorMessage    *string    `json:"errorMessage,omitempty"`
	RequestedAt     time.Time  `json:"requestedAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	DeletedAt       *time.Time `json:"deletedAt,omitempty"`
	Revision        int64      `json:"revision"`
}

// LockedPayment is the projection a refund needs while the payment row is held
// with SELECT FOR UPDATE. Refund totals are derived from pay_refunds rather
// than stored counters so the lock cannot drift from the refund aggregate.
type LockedPayment struct {
	RefundedAmount  float64
	RefundingAmount float64
	Price           float64
	Status          string
	Provider        string
	OutTradeNo      string
	ChannelOrderNo  string
	Currency        string
}

// CalculateNewRefundedAmount is the lock-free estimate the caller can use
// before acquiring the SELECT FOR UPDATE guard. Actual write-side locking
// happens in the App layer — this method only computes the target total.
func (r *Refund) CalculateNewRefundedAmount(currentRefunded, currentRefunding float64) float64 {
	return currentRefunded + currentRefunding + r.Amount
}

func (r *Refund) Validate() error {
	if r.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if r.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if r.PaymentID == "" {
		return apperrors.NewInvalidArgument("paymentId is required")
	}
	if r.OutRefundNo == "" {
		return apperrors.NewInvalidArgument("outRefundNo is required")
	}
	if r.Amount <= 0 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "amount must be positive", map[string]any{"field": "amount", "value": r.Amount, "min": 0})
	}
	if r.State == "" {
		r.State = RefundStateRequested
	}
	switch r.State {
	case RefundStateRequested, RefundStateProcessing, RefundStateSucceeded, RefundStateFailed, RefundStateCancelled:
	default:
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid refund state: %s", r.State),
			map[string]any{"field": "state", "value": r.State, "allowed": append([]string{}, refundStates...)},
		)
	}
	return nil
}

func (r *Refund) CanTransitionTo(newState string) bool {
	for _, s := range validRefundTransitions[r.State] {
		if s == newState {
			return true
		}
	}
	return false
}

func (r *Refund) TransitionTo(newState string) error {
	if newState == r.State {
		return nil
	}
	switch newState {
	case RefundStateRequested, RefundStateProcessing, RefundStateSucceeded, RefundStateFailed, RefundStateCancelled:
	default:
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid refund state: %s", newState),
			map[string]any{"field": "state", "value": newState, "allowed": append([]string{}, refundStates...)},
		)
	}
	if !r.CanTransitionTo(newState) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("cannot move refund from %s to %s", r.State, newState),
			map[string]any{"field": "state", "value": r.State, "allowed": append([]string{}, validRefundTransitions[r.State]...)},
		)
	}
	r.State = newState
	if newState == RefundStateSucceeded || newState == RefundStateFailed || newState == RefundStateCancelled {
		t := time.Now()
		r.CompletedAt = &t
	}
	return nil
}
