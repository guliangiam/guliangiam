package captchaproviders

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type PatchCaptchaProviderCommand struct {
	ID             string
	OrganizationID string
	Name           *string
	ProviderType   *string
	SiteKey        *string
	SecretKey      *string
	Config         map[string]any
	IsEnabled      *bool
	Revision       int64
}

func (s Service) PatchCaptchaProvider(ctx context.Context, command PatchCaptchaProviderCommand) error {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return err
	}
	if err := validatePatchCaptchaProviderCommand(command); err != nil {
		return err
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		provider, err := tx.CaptchaProviders().GetByID(ctx, command.ID, command.OrganizationID)
		if err != nil {
			return err
		}

		if command.Revision > 0 && provider.Revision != command.Revision {
			return apperrors.NewConflict("captcha provider revision mismatch")
		}

		if command.Name != nil {
			provider.Name = *command.Name
		}
		if command.ProviderType != nil {
			provider.ProviderType = *command.ProviderType
		}
		if command.SiteKey != nil {
			provider.SiteKey = *command.SiteKey
		}
		if command.SecretKey != nil {
			provider.SecretKey = *command.SecretKey
		}
		if command.Config != nil {
			provider.Config = mergeConfig(provider.Config, command.Config)
		}
		if command.IsEnabled != nil {
			provider.IsEnabled = *command.IsEnabled
		}

		provider.UpdatedAt = s.Clock.Now()
		provider.Revision++

		if err := tx.CaptchaProviders().Update(ctx, provider); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			provider.OrganizationID, "",
			"captchaproviders.updated", "captchaproviders", provider.ID,
			"captchaproviders", "captchaproviders.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
