package radiusconfigs

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
)

func (s Service) CreateRadiusConfig(ctx context.Context, command CreateRadiusConfigCommand) error {
	if err := validateCreateRadiusConfigCommand(command); err != nil {
		return err
	}
	config, err := s.NewRadiusConfig(command)
	if err != nil {
		return err
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.RadiusConfigs().Create(ctx, config); err != nil {
			return err
		}
		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			config.OrganizationID, "",
			"radiusconfigs.created", "radiusconfigs", config.ID,
			"radiusconfigs", "radiusconfigs.created",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
