package webhookevents

import (
	"context"
)

// ListQuery defines the query parameters for listing webhookevents.
//
// OrganizationID is the trusted tenant the query must be bound to. It is
// resolved by transport and re-checked by the use case; an empty value is the
// platform all-tenant scope and only reaches the repository after that check.
// webhook_events has no organization_id column (and no RLS), so the repository
// enforces it through the parent webhook's organization_id.
type ListQuery struct {
	Page, PageSize int
	WebhookID      string
	EventType      string
	Status         *string
	OrganizationID string
}

// Repository defines the interface for webhookevents persistence operations.
type Repository interface {
	// GetByID is the system-scoped lookup used by the delivery worker.
	GetByID(ctx context.Context, id string) (*WebhookEvent, error)
	// GetByIDForOrganization returns the event only when its parent webhook
	// belongs to organizationID. webhook_events has no organization_id column,
	// so the predicate is bound through the parent webhook; a foreign or
	// missing id is reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*WebhookEvent, error)
	// ListExistingIDs returns the subset of ids that already exist. The
	// domain-event fan-out uses it to settle idempotency for the whole
	// subscriber set instead of one GetByID per subscriber.
	//
	// Contract (dialect parity): input ids are canonicalized to lowercase
	// (trimmed; blanks dropped) before binding, and the returned set is in
	// the same canonical lowercase form — PG uuid normalizes case and
	// id::text returns lowercase, while MySQL CHAR(36) matching depends on
	// collation, so callers may compare the result directly against
	// deterministic delivery ids. Large inputs stay below the dialect
	// placeholder limits per implementation (MySQL splits the IN list into
	// chunks; PG binds one uuid[] array parameter).
	ListExistingIDs(ctx context.Context, ids []string) ([]string, error)
	List(ctx context.Context, q ListQuery) ([]*WebhookEvent, int64, error)
	// CreateBatch writes multiple events with chunked multi-row INSERTs
	// (each chunk stays below the 65,535 bind-parameter limit). Callers run
	// it inside a transaction so the whole batch commits or rolls back as a
	// unit.
	CreateBatch(ctx context.Context, events []*WebhookEvent) error
	Create(ctx context.Context, event *WebhookEvent) error
	Update(ctx context.Context, event *WebhookEvent) error
	Delete(ctx context.Context, id string) error
}
