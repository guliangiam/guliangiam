package qrcodesessions

import (
	"fmt"
	"slices"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Valid QR code status values and allowed transitions (strict one-way state machine).
const (
	statusScanned   = "scanned"
	statusConfirmed = "confirmed"
	statusExchanged = "exchanged"
	statusExpired   = "expired"
)

// validQRStatuses is the closed set the check and the 422 `allowed` metadata share.
var validQRStatuses = []string{"pending", statusScanned, statusConfirmed, statusExchanged, statusExpired}

var validQRTransitions = map[string][]string{
	"pending":       {statusScanned, statusExpired},
	statusScanned:   {statusConfirmed, statusExpired},
	statusConfirmed: {statusExchanged, statusExpired},
	statusExchanged: {}, // terminal
	statusExpired:   {}, // terminal
}

// Session represents a QR code login session stored in the database.
type Session struct {
	ID             string    // UUIDv7 primary key
	CodeID         string    // Unique code identifier for the QR session
	Code           string    // The QR code content (URL or token)
	OrganizationID string    // FK to organizations
	ApplicationID  string    // FK to applications (optional)
	ExpiresAt      time.Time // When the QR code expires
	Status         string    // "pending" / "scanned" / "confirmed" / "expired"
	UserID         string    // User who scanned (set on scan)
	Username       string    // Username of scanner (set on scan)
	IPAddress      string    // IP address of scanner
	UserAgent      string    // User agent of scanner
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

// Validate checks the QR code session fields for business rule violations.
func (s *Session) Validate() error {
	if s.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if s.CodeID == "" {
		return apperrors.NewInvalidArgument("codeId is required")
	}
	if s.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if !slices.Contains(validQRStatuses, s.Status) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid QR session status: %s", s.Status),
			map[string]any{"field": "status", "value": s.Status, "allowed": append([]string{}, validQRStatuses...)},
		)
	}
	if s.ExpiresAt.IsZero() {
		return apperrors.NewInvalidArgument("expiresAt is required")
	}
	return nil
}

// CanTransitionTo checks if the QR session can transition to the given status.
func (s *Session) CanTransitionTo(newStatus string) bool {
	allowed, ok := validQRTransitions[s.Status]
	if !ok {
		return false
	}
	for _, st := range allowed {
		if st == newStatus {
			return true
		}
	}
	return false
}

// IsExpired returns true if the QR code has passed its expiration time.
func (s *Session) IsExpired(now time.Time) bool {
	return now.After(s.ExpiresAt)
}

// IsTerminal returns true if the QR session is in a terminal state (exchanged or expired).
func (s *Session) IsTerminal() bool {
	return s.Status == statusExchanged || s.Status == statusExpired
}
