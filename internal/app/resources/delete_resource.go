package resources

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
)

func (s Service) DeleteResource(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Read first so a foreign id is rejected before the delete and the
		// audit row carries the entity's real organization.
		resource, err := tx.Resources().GetByID(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if err := tx.Resources().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			resource.OrganizationID,
			"resources.deleted",
			id,
			"resources.deleted",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
