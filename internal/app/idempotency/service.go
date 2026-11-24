package idempotency

import domain "github.com/guliangiam/guliangiam/internal/domain/idempotency"

type (
	Repository = domain.Repository
	Record     = domain.Record
	Status     = domain.Status
)

const (
	StatusInProgress = domain.StatusInProgress
	StatusSucceeded  = domain.StatusSucceeded
	StatusFailed     = domain.StatusFailed
)

// ScopeFor derives the idempotency scope from an organization slug, the
// authenticated user ID and the request path (BE-E-01: user dimension prevents
// cross-user replay of stored responses within the same tenant).
func ScopeFor(orgSlug, userID, path string) string {
	return domain.ScopeFor(orgSlug, userID, path)
}
