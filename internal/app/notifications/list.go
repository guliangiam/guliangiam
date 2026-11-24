package notifications

import (
	"context"
)

// ListNotificationsQuery is the query for listing notifications.
type ListNotificationsQuery struct {
	OrganizationID string
	Status         string
	Keyword        string
	Page           int
	PageSize       int
}

// ListNotifications returns a paginated list of notifications.
func (s Service) ListNotifications(ctx context.Context, query ListNotificationsQuery) ([]*NotificationDTO, int64, error) {
	items, total, err := s.UOW.Notifications().List(ctx, query.OrganizationID, query.Status, query.Keyword, query.Page, query.PageSize)
	if err != nil {
		return nil, 0, err
	}

	dtos := make([]*NotificationDTO, 0, len(items))
	for _, item := range items {
		dtos = append(dtos, toNotificationDTO(item))
	}

	return dtos, total, nil
}
