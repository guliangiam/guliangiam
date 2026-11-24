package permissions

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s *Service) DeletePermission(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}

	var deletedPermission *domain.Permission

	err := (s.UOW.(appuow.TxRunner)).WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// 事务内先做 scoped 读取；跨租户在删除与审计之前就返回 NotFound。
		permission, err := tx.Permissions().GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if err := tx.Permissions().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			permission.OrganizationID, "",
			"permission.deleted", "Permission", id,
			"Permission", "permission.deleted",
			map[string]any{},
		); err != nil {
			return err
		}
		deletedPermission = permission
		return nil
	})
	if err != nil {
		return err
	}

	// Remove Casbin policies for the deleted permission.
	if deletedPermission != nil {
		if syncErr := removePermissionFromCasbin(s.CasbinManager, deletedPermission.OrganizationID, deletedPermission.ID); syncErr != nil {
			return apperrors.WrapInternal(syncErr, "delete permission: casbin cleanup failed")
		}
	}

	s.invalidatePermissionCache()
	return nil
}
