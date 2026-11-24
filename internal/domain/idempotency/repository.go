// Package idempotency defines the domain port for unified request idempotency
// per ADR-018 (Unified Idempotency).
//
// The key idea: a client sends an Idempotency-Key header with a write request.
// The server records the first response under a (scope, key) unique constraint.
// Subsequent requests with the same key and identical request body hash get the
// cached response back with Idempotency-Replayed: true. Same key + different
// body = 409 Conflict.
package idempotency

import (
	"context"
	"time"
)

// Status is the lifecycle state of an idempotency key record.
type Status string

const (
	StatusInProgress Status = "in_progress"
	StatusSucceeded  Status = "succeeded"
	StatusFailed     Status = "failed"
)

// Record is the domain projection of an idempotency_keys row.
type Record struct {
	ID             string
	OrganizationID *string
	Scope          string
	Key            string
	RequestMethod  string
	RequestPath    string
	RequestHash    string
	Status         Status
	ResponseStatus *int
	ResponseBody   *string
	// ContentType is the captured response Content-Type header (BE-E-12).
	// nil for legacy rows written before the content_type column existed;
	// replay then keeps the default behavior (no explicit header).
	ContentType *string
	CompletedAt *time.Time
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

// Repository is the domain port for idempotency key storage.
// Implemented by infra/postgres/idempotency.
type Repository interface {
	// GetByScopeAndKey returns the record for (scope, key), or (nil, nil) if absent.
	GetByScopeAndKey(ctx context.Context, scope, key string) (*Record, error)

	// TryAcquire creates an in_progress record atomically.
	// Returns (record, true) on success, (existing, false) if a record already exists.
	// Used as the distributed "first writer wins" gate.
	TryAcquire(ctx context.Context, rec *Record) (*Record, bool, error)

	// Complete transitions an in_progress record to succeeded/failed with the response.
	// respContentType is the response Content-Type header; an empty string is
	// stored as NULL and replay falls back to the default behavior (BE-E-12).
	Complete(ctx context.Context, id string, status Status, respStatus int, respBody, respContentType string) error
}

// ScopeFor derives the idempotency scope from an organization slug, the
// authenticated user and the request path (BE-E-01).
// Example: ScopeFor("acme", "user-uuid", "/api/v1/users") → "acme:user-uuid:/api/v1/users"
//
// The user dimension is mandatory: without it any user in the same tenant
// could reuse another user's Idempotency-Key and replay that user's stored
// response body (cross-user data leak). Separators are always emitted so an
// empty slug/userID can never alias a different identity combination.
// Rows written under the old two-part scope expire naturally within 24h.
func ScopeFor(orgSlug, userID, path string) string {
	return orgSlug + ":" + userID + ":" + path
}

// ExpiredCleaner is an optional maintenance capability implemented by
// repositories that can reclaim expired idempotency rows. Cleanup is a
// background concern (ADR-018): the request path never blocks on it, and a
// repository without the capability simply skips the maintenance tick.
type ExpiredCleaner interface {
	// DeleteExpired removes at most limit rows whose expires_at is before
	// cutoff and returns the number of deleted rows. Callers loop until the
	// returned count is below the batch size.
	DeleteExpired(ctx context.Context, cutoff time.Time, limit int) (int64, error)
}
