package providers

import "context"

func (s Service) GetProviderConfig(ctx context.Context, providerID, organizationID string) (*ProviderConfigDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	provider, err := s.UOW.Providers().GetByIDForOrganization(ctx, providerID, organizationID)
	if err != nil {
		return nil, err
	}
	// The child config read is allowed only after the parent row was resolved
	// inside the trusted tenant; the assertion keeps the not-found shape if a
	// loader regression ever returns a foreign row.
	if err := ensureProviderTenantScope(ctx, provider.OrganizationID, organizationID); err != nil {
		return nil, err
	}

	config, err := s.UOW.Providers().GetConfig(ctx, providerID)
	if err != nil {
		return nil, err
	}

	return toConfigDetail(config), nil
}
