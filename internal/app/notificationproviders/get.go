package notificationproviders

import "context"

func (s Service) GetNotificationProvider(ctx context.Context, organizationID, id string) (*NotificationProviderDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}

	provider, err := s.UOW.NotificationProviders().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}

	detail := toDetail(provider)
	return &detail, nil
}
