package notificationproviders

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type PatchNotificationProviderCommand struct {
	ID             string
	OrganizationID string
	Name           *string
	ProviderType   *string
	WebhookURL     *string
	AccessToken    *string
	Config         map[string]any
	IsEnabled      *bool
	Revision       int64
}

func (s Service) PatchNotificationProvider(ctx context.Context, command PatchNotificationProviderCommand) error {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return err
	}
	if err := validatePatchNotificationProviderCommand(command); err != nil {
		return err
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		provider, err := tx.NotificationProviders().GetByIDForOrganization(ctx, command.ID, command.OrganizationID)
		if err != nil {
			return err
		}

		if command.Revision > 0 && provider.Revision != command.Revision {
			return apperrors.NewConflict("notification provider revision mismatch")
		}

		if command.Name != nil {
			provider.Name = *command.Name
		}
		if command.ProviderType != nil {
			provider.ProviderType = *command.ProviderType
		}
		if command.WebhookURL != nil {
			provider.WebhookURL = *command.WebhookURL
		}
		if command.AccessToken != nil {
			provider.AccessToken = *command.AccessToken
		}
		if command.Config != nil {
			provider.Config = command.Config
		}
		if command.IsEnabled != nil {
			provider.IsEnabled = *command.IsEnabled
		}

		provider.UpdatedAt = s.Clock.Now()
		provider.Revision++

		if err := tx.NotificationProviders().Update(ctx, provider); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			provider.OrganizationID, "",
			"notificationproviders.updated", "notificationproviders", provider.ID,
			"notificationproviders", "notificationproviders.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
