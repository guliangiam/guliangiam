package webhooks

import "context"

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
	Method         string
}

type Repository interface {
	// GetByID is the system-scoped lookup used by the delivery worker, which
	// runs outside any request tenant. Management routes must use
	// GetByIDForOrganization so a caller-supplied id cannot cross tenants.
	GetByID(ctx context.Context, id string) (*Webhook, error)
	// GetByIDForOrganization returns the webhook only when it belongs to
	// organizationID; a foreign or missing row is reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Webhook, error)
	List(ctx context.Context, query ListQuery) ([]*Webhook, int64, error)
	ListByOrganizationAndEvent(ctx context.Context, organizationID, event string) ([]*Webhook, error)
	Create(ctx context.Context, webhook *Webhook) error
	Update(ctx context.Context, webhook *Webhook) error
	Delete(ctx context.Context, id string) error
}
