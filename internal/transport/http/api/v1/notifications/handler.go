package notifications

import (
	"context"

	appnotifications "github.com/guliangiam/guliangiam/internal/app/notifications"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

func (h Handler) List(ctx context.Context, request ListNotificationsRequest) (ListNotificationsResponse, error) {
	items, total, err := h.Service.ListNotifications(ctx, appnotifications.ListNotificationsQuery{
		OrganizationID: request.OrganizationID,
		Status:         request.Status,
		Keyword:        request.Keyword,
		Page:           request.Page,
		PageSize:       request.PageSize,
	})
	if err != nil {
		return ListNotificationsResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Create(ctx context.Context, request CreateNotificationRequest) (*appnotifications.NotificationDTO, error) {
	return h.Service.CreateNotification(ctx, appnotifications.CreateNotificationCommand{
		OrganizationID: request.OrganizationID,
		ProviderID:     request.ProviderID,
		Recipient:      request.Recipient,
		Subject:        request.Subject,
		Content:        request.Content,
		TemplateID:     request.TemplateID,
		Variables:      request.Variables,
	})
}

func (h Handler) MarkRead(ctx context.Context, organizationID, id string) error {
	return h.Service.MarkNotificationRead(ctx, organizationID, id)
}

func (h Handler) MarkAllRead(ctx context.Context, organizationID string) error {
	return h.Service.MarkAllNotificationsRead(ctx, organizationID)
}
