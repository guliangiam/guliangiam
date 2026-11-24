package radiusconfigs

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
)

func (s Service) DeleteRadiusConfig(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Read first so a foreign id is rejected before the delete and the
		// audit row carries the entity's real organization.
		config, err := tx.RadiusConfigs().GetByID(ctx, id, organizationID)
		if err != nil {
			return err
		}

		if err := tx.RadiusConfigs().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			config.OrganizationID, "",
			"radiusconfigs.deleted", "radiusconfigs", id,
			"radiusconfigs", "radiusconfigs.deleted",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
