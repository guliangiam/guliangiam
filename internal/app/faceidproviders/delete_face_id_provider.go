package faceidproviders

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
)

func (s Service) DeleteFaceIdProvider(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Read first so a foreign id is rejected before the delete and the
		// audit row carries the entity's real organization.
		provider, err := tx.FaceIdProviders().GetByID(ctx, id, organizationID)
		if err != nil {
			return err
		}

		if err := tx.FaceIdProviders().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			provider.OrganizationID, "",
			"faceidproviders.deleted", "faceidproviders", id,
			"faceidproviders", "faceidproviders.deleted",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
