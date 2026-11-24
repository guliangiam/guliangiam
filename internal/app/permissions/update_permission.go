package permissions

import (
	"context"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type PatchPermissionCommand struct {
	ID           string
	DisplayName  *string
	Description  *string
	Users        []string
	Groups       []string
	Roles        []string
	Domains      []string
	ResourceType *string
	Resources    []string
	Actions      []string
	Effect       *string
	Model        *string
	Adapter      *string
	Sorter       *string
	Application  *string
	IsEnabled    *bool
	Submitter    *string
	Approver     *string
	State        *string
	ApproveTime  *time.Time
	Revision     int64
}

func (s *Service) PatchPermission(ctx context.Context, organizationID string, command PatchPermissionCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	if err := validatePatchPermissionCommand(command); err != nil {
		return err
	}

	var updatedPermission *domain.Permission

	err := (s.UOW.(appuow.TxRunner)).WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// 事务内先做 scoped 读取；跨租户在写与审计之前就返回 NotFound。
		permission, err := tx.Permissions().GetByIDForOrganization(ctx, command.ID, organizationID)
		if err != nil {
			return err
		}

		if command.Revision > 0 && permission.Revision != command.Revision {
			return apperrors.NewConflict("permission revision mismatch")
		}

		if command.DisplayName != nil {
			permission.DisplayName = *command.DisplayName
		}
		if command.Description != nil {
			permission.Description = *command.Description
		}
		if command.Users != nil {
			permission.Users = append([]string(nil), command.Users...)
		}
		if command.Groups != nil {
			permission.Groups = append([]string(nil), command.Groups...)
		}
		if command.Roles != nil {
			permission.Roles = append([]string(nil), command.Roles...)
		}
		if command.Domains != nil {
			permission.Domains = append([]string(nil), command.Domains...)
		}
		if command.ResourceType != nil {
			permission.ResourceType = *command.ResourceType
		}
		if command.Resources != nil {
			permission.Resources = append([]string(nil), command.Resources...)
		}
		if command.Actions != nil {
			permission.Actions = append([]string(nil), command.Actions...)
		}
		if command.Effect != nil {
			permission.Effect = normalizeEffect(*command.Effect)
		}
		if command.Model != nil {
			permission.Model = *command.Model
		}
		if command.Adapter != nil {
			permission.Adapter = *command.Adapter
		}
		if command.Sorter != nil {
			permission.Sorter = *command.Sorter
		}
		if command.Application != nil {
			permission.Application = *command.Application
		}
		if command.IsEnabled != nil {
			permission.IsEnabled = *command.IsEnabled
		}
		if command.Submitter != nil {
			permission.Submitter = *command.Submitter
		}
		if command.Approver != nil {
			permission.Approver = *command.Approver
		}
		if command.State != nil {
			permission.State = normalizeState(*command.State)
		}
		if command.ApproveTime != nil {
			permission.ApproveTime = command.ApproveTime
		}

		// 变更后的 resources[0] 归属裁决必须先于写库与 Casbin 重同步。
		if err := validatePermissionResourcesForOrganization(ctx, tx, permission.OrganizationID, permission.Resources); err != nil {
			return err
		}

		permission.UpdatedAt = s.Clock.Now()
		permission.Revision++

		if err := tx.Permissions().Update(ctx, permission); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			permission.OrganizationID, "",
			"permission.updated", "Permission", permission.ID,
			"Permission", "permission.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		updatedPermission = permission
		return nil
	})
	if err != nil {
		return err
	}

	// Re-sync Casbin policies after the permission update.
	// First remove old policies, then add the updated ones.
	if updatedPermission != nil {
		if syncErr := removePermissionFromCasbin(s.CasbinManager, updatedPermission.OrganizationID, updatedPermission.ID); syncErr != nil {
			return apperrors.WrapInternal(syncErr, "patch permission: casbin cleanup failed")
		}
		if syncErr := syncPermissionToCasbin(s.CasbinManager, updatedPermission.OrganizationID, updatedPermission); syncErr != nil {
			return apperrors.WrapInternal(syncErr, "patch permission: casbin sync failed")
		}
		// 与 Create/Delete 对称：策略重同步成功后立即失效权限缓存，
		// 否则更新后的 actions/resources 在 TTL 窗口内仍按旧策略裁决。
		s.invalidatePermissionCache()
	}

	return nil
}
