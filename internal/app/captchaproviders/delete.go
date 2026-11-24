package captchaproviders

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
)

func (s Service) DeleteCaptchaProvider(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		provider, err := tx.CaptchaProviders().GetByID(ctx, id, organizationID)
		if err != nil {
			return err
		}

		if err := tx.CaptchaProviders().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			provider.OrganizationID, "",
			"captchaproviders.deleted", "captchaproviders", provider.ID,
			"captchaproviders", "captchaproviders.deleted",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
