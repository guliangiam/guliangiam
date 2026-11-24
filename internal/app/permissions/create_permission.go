package permissions

import (
	"context"
	"log/slog"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s *Service) CreatePermission(ctx context.Context, command CreatePermissionCommand) error {
	if err := validateCreatePermissionCommand(command); err != nil {
		return err
	}

	permission, err := s.NewPermission(command)
	if err != nil {
		return err
	}

	err = (s.UOW.(appuow.TxRunner)).WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// resources[0] 归属裁决必须先于写库与 Casbin 同步（OPEN-API-44）。
		if err := validatePermissionResourcesForOrganization(ctx, tx, permission.OrganizationID, permission.Resources); err != nil {
			return err
		}
		if err := tx.Permissions().Create(ctx, permission); err != nil {
			return err
		}
		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			permission.OrganizationID, "",
			"permission.created", "Permission", permission.ID,
			"Permission", "permission.created",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Sync the new permission to Casbin after successful creation.
	if syncErr := syncPermissionToCasbin(s.CasbinManager, permission.OrganizationID, permission); syncErr != nil {
		slog.ErrorContext(ctx, "permission casbin sync failed", "permission_id", permission.ID, "error", syncErr)
		// 补偿删除：策略同步失败时不允许留下一条“接口报错但记录已存在”的脏数据。
		if delErr := s.deletePermissionRow(ctx, permission.ID, permission.OrganizationID); delErr != nil {
			slog.ErrorContext(ctx, "compensate permission create failed",
				"permission_id", permission.ID, "error", delErr)
		}
		return apperrors.WrapInternal(syncErr, "create permission: casbin sync failed")
	}

	// 权限/策略变更后清除账户权限缓存（DEF-16）。
	s.invalidatePermissionCache()
	return nil
}

func (s *Service) deletePermissionRow(ctx context.Context, permissionID, organizationID string) error {
	return (s.UOW.(appuow.TxRunner)).WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		return tx.Permissions().Delete(ctx, permissionID, organizationID)
	})
}
