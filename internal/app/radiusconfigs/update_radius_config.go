package radiusconfigs

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type PatchRadiusConfigCommand struct {
	ID           string
	Name         *string
	Host         *string
	Port         *int
	SharedSecret *string
	AuthProtocol *string
	IsEnabled    *bool
	Revision     int64
}

func (s Service) PatchRadiusConfig(ctx context.Context, organizationID string, command PatchRadiusConfigCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	if err := validatePatchRadiusConfigCommand(command); err != nil {
		return err
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Ownership is decided before any mutation: a foreign or missing id
		// returns NotFound and leaves no write and no audit record behind.
		config, err := tx.RadiusConfigs().GetByID(ctx, command.ID, organizationID)
		if err != nil {
			return err
		}

		if command.Revision > 0 && config.Revision != command.Revision {
			return apperrors.NewConflict("radius config revision mismatch")
		}

		if command.Name != nil {
			config.Name = *command.Name
		}
		if command.Host != nil {
			config.Host = *command.Host
		}
		if command.Port != nil {
			config.Port = *command.Port
		}
		if command.SharedSecret != nil {
			config.SharedSecret = *command.SharedSecret
		}
		if command.AuthProtocol != nil {
			config.AuthProtocol = *command.AuthProtocol
		}
		if command.IsEnabled != nil {
			config.IsEnabled = *command.IsEnabled
		}

		config.UpdatedAt = s.Clock.Now()
		config.Revision++

		if err := tx.RadiusConfigs().Update(ctx, config); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			config.OrganizationID, "",
			"radiusconfigs.updated", "radiusconfigs", config.ID,
			"radiusconfigs", "radiusconfigs.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
