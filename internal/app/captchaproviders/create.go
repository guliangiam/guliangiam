package captchaproviders

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
)

func (s Service) CreateCaptchaProvider(ctx context.Context, command CreateCaptchaProviderCommand) error {
	if err := validateCreateCaptchaProviderCommand(command); err != nil {
		return err
	}

	provider, err := s.NewCaptchaProvider(command)
	if err != nil {
		return err
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.CaptchaProviders().Create(ctx, provider); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			provider.OrganizationID, "",
			"captchaproviders.created", "captchaproviders", provider.ID,
			"captchaproviders", "captchaproviders.created",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
