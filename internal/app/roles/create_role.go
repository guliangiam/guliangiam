package roles

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s *Service) CreateRole(ctx context.Context, command CreateRoleCommand) error {
	// Transport converges command.OrganizationID through TrustedOrganizationID;
	// Casbin sync below therefore targets the tenant that was actually written.
	if err := validateCreateRoleCommand(command); err != nil {
		return err
	}

	role, err := s.NewRole(command)
	if err != nil {
		return err
	}

	err = (s.UOW.(appuow.TxRunner)).WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Roles().Create(ctx, role); err != nil {
			return err
		}
		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			role.OrganizationID, "",
			"role.created", "Role", role.ID,
			"Role", "role.created",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	if s.CasbinManager != nil {
		if syncErr := s.CasbinManager.SyncRole(ctx, role.OrganizationID, role); syncErr != nil {
			return apperrors.WrapInternal(syncErr, "create role: casbin sync failed")
		}
	}

	return nil
}
