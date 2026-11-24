package notificationproviders

import "context"

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
}

type Repository interface {
	// GetByID is the unscoped, system-scoped lookup reserved for background
	// delivery and alerting. Request-scoped by-ID paths must use
	// GetByIDForOrganization so a caller-supplied id cannot cross tenants.
	GetByID(ctx context.Context, id string) (*NotificationProvider, error)
	// GetByIDForOrganization returns the provider only when it belongs to
	// organizationID; a foreign or missing row is reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*NotificationProvider, error)
	List(ctx context.Context, query ListQuery) ([]*NotificationProvider, int64, error)
	Create(ctx context.Context, provider *NotificationProvider) error
	Update(ctx context.Context, provider *NotificationProvider) error
	Delete(ctx context.Context, id, organizationID string) error
}
