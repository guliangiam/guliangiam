package providers

import "context"

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Category       string
	ProviderType   string
}

type Repository interface {
	// GetByID is the system-scoped lookup used by login/social authentication,
	// which resolves a provider from configuration rather than from a
	// request-scoped id.
	GetByID(ctx context.Context, id string) (*Provider, error)
	// GetByIDForOrganization returns the provider only when it belongs to
	// organizationID; a foreign or missing row is reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Provider, error)
	List(ctx context.Context, query ListQuery) ([]*Provider, int64, error)
	Create(ctx context.Context, provider *Provider) error
	Update(ctx context.Context, provider *Provider) error
	Delete(ctx context.Context, id string) error
	GetConfig(ctx context.Context, providerID string) (*ProviderConfig, error)
	UpdateConfig(ctx context.Context, config *ProviderConfig) error
	ListSecretStatuses(ctx context.Context, providerID string) ([]ProviderSecretStatus, error)
}
