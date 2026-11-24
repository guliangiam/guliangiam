package notificationproviders

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
)

func (s Service) CreateNotificationProvider(ctx context.Context, command CreateNotificationProviderCommand) error {
	if err := validateCreateNotificationProviderCommand(command); err != nil {
		return err
	}

	provider, err := s.NewNotificationProvider(command)
	if err != nil {
		return err
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.NotificationProviders().Create(ctx, provider); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			provider.OrganizationID, "",
			"notificationproviders.created", "notificationproviders", provider.ID,
			"notificationproviders", "notificationproviders.created",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
