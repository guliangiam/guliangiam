package roles

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/roles"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s *Service) DeleteRole(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}

	var deletedRole *domain.Role
	err := (s.UOW.(appuow.TxRunner)).WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Ownership is decided before deletion, so a cross-tenant id leaves no
		// delete and no audit record behind.
		role, err := tx.Roles().GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}

		if err := tx.Roles().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			role.OrganizationID, "",
			"role.deleted", "Role", id,
			"Role", "role.deleted",
			map[string]any{},
		); err != nil {
			return err
		}
		deletedRole = role
		return nil
	})
	if err != nil {
		return err
	}

	if deletedRole != nil && s.CasbinManager != nil {
		if syncErr := s.CasbinManager.RemoveRole(ctx, deletedRole.OrganizationID, deletedRole.ID); syncErr != nil {
			return apperrors.WrapInternal(syncErr, "delete role: casbin cleanup failed")
		}
	}

	return nil
}
