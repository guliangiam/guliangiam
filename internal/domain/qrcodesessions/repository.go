package qrcodesessions

import "context"

// Repository defines the interface for QR code session persistence.
type Repository interface {
	// GetByCodeID retrieves a QR code session by its code_id.
	GetByCodeID(ctx context.Context, codeID string) (*Session, error)

	// GetByCode retrieves a QR code session by the QR payload (the opaque code
	// the scanning device holds). Scan/confirm address the session by this
	// handle, so the exchange handle (code_id) never has to leave the
	// initiating device's browser.
	GetByCode(ctx context.Context, code string) (*Session, error)

	// Create persists a new QR code session.
	Create(ctx context.Context, session *Session) error

	// UpdateStatus updates the status and related fields of a QR code session.
	UpdateStatus(ctx context.Context, session *Session) error

	// DeleteByCodeID removes a QR code session by code_id.
	DeleteByCodeID(ctx context.Context, codeID string) error

	// DeleteExpired removes all expired QR code sessions.
	DeleteExpired(ctx context.Context) (int64, error)
}
