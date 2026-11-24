package invitations

import (
	"fmt"
	"slices"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Invitation status constants.
const (
	StatusPending  = "pending"
	StatusAccepted = "accepted"
	StatusRevoked  = "revoked"
	StatusExpired  = "expired"
)

type Invitation struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	Email          string     `json:"email"`
	Code           string     `json:"code"`
	RoleIDs        []string   `json:"roleIds"`
	GroupIDs       []string   `json:"groupIds"`
	Status         string     `json:"status"`
	ExpiresAt      time.Time  `json:"expiresAt"`
	AcceptedAt     *time.Time `json:"acceptedAt,omitempty"`
	InvitedBy      string     `json:"invitedBy"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	Revision       int64      `json:"revision"`
}

// IsExpired returns true if the invitation has passed its expiration time.
func (inv *Invitation) IsExpired(now time.Time) bool {
	return !inv.ExpiresAt.IsZero() && now.After(inv.ExpiresAt)
}

// IsAcceptable returns true if the invitation can still be accepted.
func (inv *Invitation) IsAcceptable(now time.Time) bool {
	return inv.Status == StatusPending && !inv.IsExpired(now)
}

// Valid invitation statuses. The slice is the single source for both the rule
// and the 422 `allowed` metadata.
var validInvitationStatuses = []string{StatusPending, StatusAccepted, StatusRevoked, StatusExpired}

// Valid status transitions
var validInvitationTransitions = map[string][]string{
	StatusPending:  {StatusAccepted, StatusRevoked, StatusExpired},
	StatusAccepted: {},
	StatusRevoked:  {},
	StatusExpired:  {},
}

// Validate checks the invitation fields for business rule violations.
func (inv *Invitation) Validate() error {
	if inv.Email == "" {
		return apperrors.NewInvalidArgument("email is required")
	}
	if inv.Code == "" {
		return apperrors.NewInvalidArgument("code is required")
	}
	if !slices.Contains(validInvitationStatuses, inv.Status) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid invitation status: %s", inv.Status),
			map[string]any{"field": "status", "value": inv.Status, "allowed": append([]string{}, validInvitationStatuses...)},
		)
	}
	if inv.ExpiresAt.IsZero() {
		return apperrors.NewInvalidArgument("expiresAt is required")
	}
	return nil
}

// CanTransitionTo checks if the invitation can transition to the given status.
func (inv *Invitation) CanTransitionTo(newStatus string) bool {
	allowed, ok := validInvitationTransitions[inv.Status]
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
