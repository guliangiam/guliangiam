package providers

import "context"

type ListProvidersQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Category       string
	ProviderType   string
	Family         string
	Kind           string
	Variant        string
	Status         string
}

func (s Service) ListProviders(ctx context.Context, query ListProvidersQuery) ([]ProviderListItem, int64, error) {
	providers, total, err := s.unified().List(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	items := make([]ProviderListItem, 0, len(providers))
	for i := range providers {
		items = append(items, toUnifiedListItem(&providers[i]))
	}

	return items, total, nil
}
