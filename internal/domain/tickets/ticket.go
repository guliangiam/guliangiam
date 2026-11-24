package tickets

import (
	"fmt"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/enumset"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type Ticket struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	Name           string     `json:"name"`
	DisplayName    string     `json:"displayName"`
	Type           string     `json:"type,omitempty"`
	Status         string     `json:"status"`
	Priority       string     `json:"priority,omitempty"`
	Assignee       string     `json:"assignee,omitempty"`
	Creator        string     `json:"creator,omitempty"`
	Description    string     `json:"description,omitempty"`
	Attachment     string     `json:"attachment,omitempty"`
	ResolvedAt     *time.Time `json:"resolvedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	Revision       int64      `json:"revision"`
}

// Ticket lifecycle statuses. The values, the transition edges and the terminal
// set come from the state machine registry
// (`registries/state-machine-registry.md` §1.5): DDL `ck_tickets_status`, the
// API enum and the console all render from this one definition.
//
// The column is `status` and not `state` (§通用规则 ④): a ticket is not a strict
// lifecycle state machine — it can move back to `pending` for further work — so
// it uses the generic status column convention.
const (
	StatusOpen     = "open"
	StatusPending  = "pending"
	StatusResolved = "resolved"
	StatusClosed   = "closed"
)

// Statuses is the ordered closed set of ticket statuses (§1.5「状态集」).
var Statuses = enumset.NewSet("ticketsStatus", StatusOpen, StatusPending, StatusResolved, StatusClosed)

// Ticket submission targets. `org_request` routes the ticket to the submitting
// organization's own administrators; `vendor_feedback` routes it to the vendor
// (谷梁科技). The value set is registered in contracts/registries/naming.yaml.
// The whitelist guards new writes only: historical rows may keep a legacy
// free-text type, which reads and lists still serve unchanged.
const (
	TypeOrgRequest     = "org_request"
	TypeVendorFeedback = "vendor_feedback"
)

// Types is the closed set of ticket submission targets.
var Types = enumset.NewSet("ticketsType", TypeOrgRequest, TypeVendorFeedback)

// IsKnownType reports whether the value is in the closed target set.
func IsKnownType(t string) bool {
	return Types.Contains(t)
}

// validTicketTransitions is §1.5 迁移图. `closed` is terminal: the registry
// forbids reopening a closed ticket — a recurrence is filed as a new ticket, so
// the closed record keeps its original resolution story.
var validTicketTransitions = map[string][]string{
	StatusOpen:     {StatusPending, StatusResolved, StatusClosed},
	StatusPending:  {StatusOpen, StatusResolved, StatusClosed},
	StatusResolved: {StatusPending, StatusClosed},
	StatusClosed:   {},
}

// terminalTicketStatuses is §1.5「终态」.
var terminalTicketStatuses = map[string]bool{StatusClosed: true}

// IsKnownStatus reports whether the value is in the registry's closed set.
func IsKnownStatus(status string) bool {
	return Statuses.Contains(status)
}

// IsTerminal reports whether the status is a registry terminal status.
func IsTerminal(status string) bool {
	return terminalTicketStatuses[status]
}

// ResolvesWork reports whether entering the status records a resolution
// instant. There is no DDL equivalence constraint on `resolved_at` (unlike
// `orders.paid_at`), because a ticket may be reopened and closed again while the
// earlier resolution instant keeps its meaning.
func ResolvesWork(status string) bool {
	return status == StatusResolved || status == StatusClosed
}

// Validate checks the ticket fields for business rule violations.
func (t *Ticket) Validate() error {
	if t.Type == "" {
		t.Type = TypeOrgRequest
	}
	if !IsKnownType(t.Type) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid ticket type: %s", t.Type),
			map[string]any{"field": "type", "value": t.Type, "allowed": Types.SortedValues()},
		)
	}
	if t.Status == "" {
		t.Status = StatusOpen
	}
	if !IsKnownStatus(t.Status) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid ticket status: %s", t.Status),
			map[string]any{"field": "status", "value": t.Status, "allowed": Statuses.SortedValues()},
		)
	}
	return nil
}

// CanTransitionTo checks if the ticket can transition to the given status.
func (t *Ticket) CanTransitionTo(newStatus string) bool {
	allowed, ok := validTicketTransitions[t.Status]
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
// declare. Repeating the current ticket is a no-op, because a console re-submits
// the value it read back.
func (t *Ticket) TransitionTo(newStatus string) error {
	if newStatus == t.Status {
		return nil
	}
	if !IsKnownStatus(newStatus) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid ticket status: %s", newStatus),
			map[string]any{"field": "status", "value": newStatus, "allowed": Statuses.SortedValues()},
		)
	}
	if !t.CanTransitionTo(newStatus) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("cannot move ticket from %s to %s", t.Status, newStatus),
			map[string]any{"field": "status", "value": t.Status, "allowed": append([]string{}, validTicketTransitions[t.Status]...)},
		)
	}
	t.Status = newStatus
	return nil
}
