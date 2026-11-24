package transactions

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/enumset"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type Transaction struct {
	ID              string          `json:"id"`
	OrganizationID  string          `json:"organizationId"`
	UserID          string          `json:"userId,omitempty"`
	OrderID         string          `json:"orderId,omitempty"`
	PaymentID       string          `json:"paymentId,omitempty"`
	TransactionType string          `json:"transactionType"`
	Amount          float64         `json:"amount"`
	Currency        string          `json:"currency,omitempty"`
	Status          string          `json:"status"`
	ProviderType    string          `json:"providerType,omitempty"`
	ProviderID      string          `json:"providerId,omitempty"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
	Revision        int64           `json:"revision"`
}

// Transaction kinds. A ledger row is append-only: the kind is fixed at creation
// and a reversal is recorded as a new row, never as an edit of history
// (`registries/state-machine-registry.md` §1.4).
const (
	TypePayment      = "payment"
	TypeRefund       = "refund"
	TypeCancellation = "cancellation"
)

// Types is the ordered closed set of transaction kinds (§1.4). It mirrors DDL
// `ck_transactions_type`.
var Types = enumset.NewSet("transactionsType", TypePayment, TypeRefund, TypeCancellation)

// Settlement statuses of a ledger row.
const (
	StatusPending   = "pending"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// Statuses is the ordered closed set of transaction statuses (§1.4). It mirrors
// DDL `ck_transactions_status`.
var Statuses = enumset.NewSet("transactionsStatus",
	StatusPending, StatusCompleted, StatusFailed, StatusCancelled,
)

// validTransactionTransitions is §1.4 迁移图. `completed` is immutable — a
// settled row is never rewritten, which is what makes the ledger auditable.
var validTransactionTransitions = map[string][]string{
	StatusPending:   {StatusCompleted, StatusFailed, StatusCancelled},
	StatusCompleted: {},
	StatusFailed:    {},
	StatusCancelled: {},
}

// IsKnownType reports whether the value is in the registry's closed set.
func IsKnownType(kind string) bool {
	return Types.Contains(kind)
}

// IsKnownStatus reports whether the value is in the registry's closed set.
func IsKnownStatus(status string) bool {
	return Statuses.Contains(status)
}

// Validate checks the transaction fields for business rule violations.
func (t *Transaction) Validate() error {
	if t.Status == "" {
		t.Status = StatusPending
	}
	if !IsKnownType(t.TransactionType) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid transaction type: %s", t.TransactionType),
			map[string]any{"field": "type", "value": t.TransactionType, "allowed": Types.SortedValues()},
		)
	}
	if !IsKnownStatus(t.Status) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid transaction status: %s", t.Status),
			map[string]any{"field": "status", "value": t.Status, "allowed": Statuses.SortedValues()},
		)
	}
	return nil
}

// CanTransitionTo checks if the transaction can transition to the given status.
func (t *Transaction) CanTransitionTo(newStatus string) bool {
	allowed, ok := validTransactionTransitions[t.Status]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == newStatus {
			return true
		}
	}
	return false
}

// TransitionTo moves the aggregate to newState through the §1.5 graph.
//
// This is the only sanctioned way to write the lifecycle column: the guard
// lives with the table, so a caller cannot walk an edge the registry does not
// declare. Repeating the current transaction is a no-op, because a console re-submits
// the value it read back.
func (t *Transaction) TransitionTo(newStatus string) error {
	if newStatus == t.Status {
		return nil
	}
	if !IsKnownStatus(newStatus) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid transaction status: %s", newStatus),
			map[string]any{"field": "status", "value": newStatus, "allowed": Statuses.SortedValues()},
		)
	}
	if !t.CanTransitionTo(newStatus) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("cannot move transaction from %s to %s", t.Status, newStatus),
			map[string]any{"field": "status", "value": t.Status, "allowed": append([]string{}, validTransactionTransitions[t.Status]...)},
		)
	}
	t.Status = newStatus
	return nil
}
