package rememberme

import (
	"context"
	"time"
)

// Repository persists remember-me families and their rotating tokens. Token
// lookups for the anonymous exchange are intentionally not organization-scoped
// (the hash itself is globally unique); every explicit family/token mutation
// still carries organization_id where the row exposes it.
type Repository interface {
	CreateFamily(ctx context.Context, f *Family) error
	// GetFamily returns NotFound when the row does not exist. Callers must verify
	// organization/user ownership before acting on it.
	GetFamily(ctx context.Context, id string) (*Family, error)
	// RevokeFamily is idempotent: only rows with revoked_at IS NULL are written.
	RevokeFamily(ctx context.Context, id string, at time.Time, reason string) error
	// RevokeAllForUser revokes every still-active family of one user and returns
	// the number of rows transitioned.
	RevokeAllForUser(ctx context.Context, organizationID, userID string, at time.Time, reason string) (int64, error)
	CreateToken(ctx context.Context, t *Token) error
	// GetTokenByHash returns NotFound when no token matches. Used by the
	// anonymous exchange, so no tenant filter is applied.
	GetTokenByHash(ctx context.Context, hash string) (*Token, error)
	// ConsumeToken is a conditional single-use update. It returns NotFound when
	// the row is already consumed or missing.
	ConsumeToken(ctx context.Context, id string, at time.Time) error
	// RevokeTokensByFamily invalidates every not-yet-consumed token of a family.
	RevokeTokensByFamily(ctx context.Context, familyID string, at time.Time) error
}

// ExpiredCleaner is implemented by repositories that can physically delete
// expired rows in bounded batches. It is optional so existing fakes stay valid.
type ExpiredCleaner interface {
	DeleteExpired(ctx context.Context, before time.Time, limit int) (int64, error)
}
