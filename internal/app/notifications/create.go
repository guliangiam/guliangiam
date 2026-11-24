package notifications

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/notifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// CreateNotificationCommand is the command for creating a notification.
type CreateNotificationCommand struct {
	OrganizationID string
	ProviderID     string
	Recipient      string
	Subject        string
	Content        string
	TemplateID     string
	Variables      map[string]any
}

// CreateNotification creates a new notification record.
func (s Service) CreateNotification(ctx context.Context, command CreateNotificationCommand) (*NotificationDTO, error) {
	// BE-B-3: the provider must belong to the trusted organization; a
	// caller-supplied providerId from another tenant must never be usable to
	// route mail through that tenant's credentials.
	if command.ProviderID == "" {
		return nil, apperrors.NewInvalidArgument("providerId is required")
	}
	provider, err := s.UOW.NotificationProviders().GetByIDForOrganization(ctx, command.ProviderID, command.OrganizationID)
	if err != nil {
		return nil, err
	}
	if provider.OrganizationID != command.OrganizationID {
		return nil, apperrors.NewNotFound("notification provider not found")
	}

	id, err := s.UUID.NewString()
	if err != nil {
		return nil, apperrors.WrapInternal(err, "generate notification id")
	}

	now := s.Clock.Now()
	notification := &domain.Notification{
		ID:             id,
		OrganizationID: command.OrganizationID,
		ProviderID:     command.ProviderID,
		Recipient:      command.Recipient,
		Subject:        command.Subject,
		Content:        command.Content,
		TemplateID:     command.TemplateID,
		Variables:      command.Variables,
		Status:         "pending",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.UOW.Notifications().Create(ctx, notification); err != nil {
		return nil, apperrors.WrapInternal(err, "create notification")
	}

	return toNotificationDTO(notification), nil
}
