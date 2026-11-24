package notificationproviders

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
)

type ListNotificationProvidersQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
}

func (s Service) ListNotificationProviders(ctx context.Context, query ListNotificationProvidersQuery) ([]NotificationProviderListItem, int64, error) {
	providers, total, err := s.UOW.NotificationProviders().List(ctx, domain.ListQuery(query))
	if err != nil {
		return nil, 0, err
	}

	items := make([]NotificationProviderListItem, 0, len(providers))
	for _, provider := range providers {
		items = append(items, toListItem(provider))
	}

	return items, total, nil
}
