package sessions

import (
	"context"
	"time"
)

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	UserID         string
	ApplicationID  string
	State          string
	Keyword        string
	ActiveOnly     *bool
}

type Repository interface {
	GetByID(ctx context.Context, id string) (*Session, error)
	// GetByIDForOrganization returns the session only when its owning user
	// belongs to organizationID. sessions has no organization_id column, so the
	// tenant scope is resolved through users; a foreign or missing row is
	// reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Session, error)
	List(ctx context.Context, query ListQuery) ([]*Session, int64, error)
	Create(ctx context.Context, session *Session) error
	Update(ctx context.Context, session *Session) error
	Touch(ctx context.Context, id string, touchedAt time.Time) error
	Revoke(ctx context.Context, id string, revokedAt time.Time) error
	// RevokeForOrganization revokes only a session whose owning user belongs to
	// organizationID. Repeating a revoke stays idempotent; a foreign or missing
	// row is reported as not found.
	RevokeForOrganization(ctx context.Context, id, organizationID string, revokedAt time.Time) error
	RevokeByUserExcept(ctx context.Context, userID string, excludeSessionID string, revokedAt time.Time) (int64, error)
	RevokeByUserAndApp(ctx context.Context, userID string, applicationID string, excludeSessionID string, revokedAt time.Time) (int64, error)
	// RevokeByUser revokes all active sessions of a user (force logout, AC-RISK-005).
	RevokeByUser(ctx context.Context, userID string, revokedAt time.Time) (int64, error)
	// RevokeByOrganization revokes all active sessions in an organization (emergency lockdown).
	RevokeByOrganization(ctx context.Context, organizationID string, revokedAt time.Time) (int64, error)
	Delete(ctx context.Context, id string) error
	// DeleteForOrganization permanently removes a session only when its owning
	// user belongs to organizationID; zero affected rows is reported as not found.
	DeleteForOrganization(ctx context.Context, id, organizationID string) error

	// CleanupExpired permanently deletes sessions that expired before the given
	// cutoff time. Returns the number of deleted sessions.
	CleanupExpired(ctx context.Context, cutoff time.Time) (int64, error)
}
