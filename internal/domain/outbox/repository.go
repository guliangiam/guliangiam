// Package outbox defines the domain-level contract for the Transactional
// Outbox Pattern (ADR-008). Business services write to the outbox inside
// the same transaction as the business fact; a separate worker reads and
// dispatches.

package outbox

import (
	"context"
	"time"
)

// OutboxEvent is the minimal domain representation of an outbox row.
// AggregateType + AggregateID + EventType form the "what happened" tuple.
type OutboxEvent struct {
	ID             string
	OrganizationID *string
	AggregateType  string // e.g. "webhook", "notification", "syncer"
	AggregateID    string // e.g. webhook ID, notification ID
	EventType      string // e.g. "user.created", "org.deleted"
	EventVersion   int
	Payload        []byte
	Headers        []byte // JSON: transport-specific metadata (URL, secret, headers for webhook)
	MaxAttempts    int
	AvailableAt    time.Time
}

// Repository defines the insert-only contract for the outbox.
// Workers read via the platform-level outbox.Repository which adds the
// locking / retry / dead-letter primitives; domain code only enqueues.
type Repository interface {
	// Enqueue writes one outbox row. The caller is responsible for sharing
	// the transaction — typically via UoW.WithinTransaction.
	Enqueue(ctx context.Context, item *OutboxEvent) error

	// EnqueueBatch writes multiple rows atomically (single INSERT ... VALUES).
	EnqueueBatch(ctx context.Context, items []*OutboxEvent) error
}
