package faceidsessions

import "context"

// Repository defines the interface for Face ID session persistence.
type Repository interface {
	GetBySessionID(ctx context.Context, sessionID string) (*Session, error)
	Create(ctx context.Context, session *Session) error
	Update(ctx context.Context, session *Session) error
	// ConsumeVerified deletes a verified session exactly once; a second caller
	// gets a conflict instead of a second token pair.
	ConsumeVerified(ctx context.Context, sessionID string) error
	DeleteBySessionID(ctx context.Context, sessionID string) error
	DeleteExpired(ctx context.Context) (int64, error)
}
