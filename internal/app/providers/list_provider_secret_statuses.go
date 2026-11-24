package providers

import "context"

func (s Service) ListProviderSecretStatuses(ctx context.Context, providerID, organizationID string) ([]ProviderSecretStatusItem, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	provider, err := s.UOW.Providers().GetByIDForOrganization(ctx, providerID, organizationID)
	if err != nil {
		return nil, err
	}
	if err := ensureProviderTenantScope(ctx, provider.OrganizationID, organizationID); err != nil {
		return nil, err
	}

	items, err := s.UOW.Providers().ListSecretStatuses(ctx, providerID)
	if err != nil {
		return nil, err
	}

	return toSecretStatusItems(items), nil
}
